package ui

import (
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jss826/navmux/internal/action"
	"github.com/jss826/navmux/internal/attach"
	"github.com/jss826/navmux/internal/backend"
)

type mode int

const (
	modeNormal mode = iota
	modePrompt        // 新規/リネームの名前入力中
	modeConfirm       // 削除確認中
)

// sessionsMsg は refresh の結果。
type sessionsMsg struct {
	sessions []backend.Session
	err      error
}

// opDoneMsg は変更系操作の完了。
type opDoneMsg struct{ err error }

// Model は navmux の TUI 状態。
type Model struct {
	backends []backend.Backend
	active   int
	current  string // 現在の multiplexer（env）

	sessions []backend.Session
	cursor   int

	mode    mode
	pending action.Kind // prompt/confirm の対象アクション
	input   textinput.Model

	showExplain bool
	status      string
	quitting    bool
}

// New は初期 Model を作る。
func New(backends []backend.Backend, current string) Model {
	ti := textinput.New()
	ti.Placeholder = "セッション名"
	return Model{
		backends: backends,
		current:  current,
		input:    ti,
	}
}

// ActiveBackend は現在タブの backend。
func (m Model) ActiveBackend() backend.Backend { return m.backends[m.active] }

func (m Model) Init() tea.Cmd { return m.refresh() }

// refresh は active backend の一覧を取得する tea.Cmd。
func (m Model) refresh() tea.Cmd {
	b := m.ActiveBackend()
	return func() tea.Msg {
		s, err := b.List()
		return sessionsMsg{sessions: s, err: err}
	}
}

func (m Model) selectedName() string {
	if m.cursor >= 0 && m.cursor < len(m.sessions) {
		return m.sessions[m.cursor].Name
	}
	return ""
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionsMsg:
		if msg.err != nil {
			m.status = "一覧取得エラー: " + msg.err.Error()
		} else {
			m.sessions = msg.sessions
			if m.cursor >= len(m.sessions) {
				m.cursor = 0
			}
		}
		return m, nil

	case opDoneMsg:
		if msg.err != nil {
			m.status = "操作エラー: " + msg.err.Error()
		} else {
			m.status = "完了"
		}
		return m, m.refresh()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// プロンプト入力中
	if m.mode == modePrompt {
		switch msg.Type {
		case tea.KeyEnter:
			name := m.input.Value()
			m.mode = modeNormal
			m.input.Blur()
			m.input.SetValue("")
			return m, m.runOp(m.pending, name)
		case tea.KeyEsc:
			m.mode = modeNormal
			m.input.Blur()
			m.input.SetValue("")
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	// 削除確認中
	if m.mode == modeConfirm {
		switch msg.String() {
		case "y":
			m.mode = modeNormal
			return m, m.runOp(action.Kill, m.selectedName())
		default:
			m.mode = modeNormal
			m.status = "削除をキャンセル"
			return m, nil
		}
	}

	// 通常モード
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.sessions)-1 {
			m.cursor++
		}
	case "tab":
		m.active = (m.active + 1) % len(m.backends)
		m.cursor = 0
		return m, m.refresh()
	case "?":
		m.showExplain = !m.showExplain
	case "enter":
		return m, m.attachSelected()
	case "n":
		m.pending = action.New
		m.mode = modePrompt
		m.input.Focus()
		return m, textinput.Blink
	case "r":
		if !m.ActiveBackend().CanRename() {
			m.status = "この multiplexer はリネーム非対応"
			return m, nil
		}
		// 簡易版: 新名のみ入力（対象は選択中セッション）
		m.pending = action.Rename
		m.mode = modePrompt
		m.input.Focus()
		return m, textinput.Blink
	case "d":
		if m.selectedName() == "" {
			return m, nil
		}
		m.mode = modeConfirm
	case "y":
		m.copyCurrentCommand()
	}
	return m, nil
}

// runOp は変更系操作を tea.Cmd 化する（New/Rename/Kill）。
func (m Model) runOp(k action.Kind, arg string) tea.Cmd {
	b := m.ActiveBackend()
	sel := m.selectedName()
	return func() tea.Msg {
		var c backend.Command
		switch k {
		case action.New:
			return opDoneMsg{err: newSession(b, b.NewCmd(arg), arg)}
		case action.Rename:
			rc, ok := b.RenameCmd(sel, arg)
			if !ok {
				return opDoneMsg{err: backend.ErrUnsupported}
			}
			c = rc
		case action.Kill:
			c = b.KillCmd(sel)
		default:
			return opDoneMsg{}
		}
		_, err := runCommand(c)
		return opDoneMsg{err: err}
	}
}

// attachSelected はアタッチを実行する。外からは tea.ExecProcess で端末を渡す。
func (m Model) attachSelected() tea.Cmd {
	name := m.selectedName()
	if name == "" {
		return nil
	}
	plan := attach.Resolve(m.ActiveBackend(), name, m.current)
	if plan.Mode == attach.ModeSwitch {
		c := plan.Command
		return tea.Sequence(
			func() tea.Msg { _, _ = runCommand(c); return nil },
			tea.Quit,
		)
	}
	// ModeChild: TUI を一時停止して端末を渡し、detach 後に戻る。
	c := execCommand(plan.Command)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return opDoneMsg{err: err}
	})
}

func (m *Model) copyCurrentCommand() {
	disp, ok := action.CommandFor(m.ActiveBackend(), action.Attach, m.selectedName(), "")
	if !ok || disp == "" {
		m.status = "コピーできるコマンドがありません"
		return
	}
	if err := clipboard.WriteAll(disp); err != nil {
		m.status = "コピー失敗: " + err.Error()
		return
	}
	m.status = "コピーしました: " + disp
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	out := "navmux — " + m.ActiveBackend().Name() + "\n\n"
	out += RenderList(m.sessions, m.cursor) + "\n"

	if m.mode == modePrompt {
		out += "\n名前: " + m.input.View() + "\n(enter 確定 / esc キャンセル)\n"
	}
	if m.mode == modeConfirm {
		out += "\n削除しますか? " + m.selectedName() + " [y/N]\n"
	}
	if m.showExplain {
		disp, _ := action.CommandFor(m.ActiveBackend(), action.Attach, m.selectedName(), "")
		out += "\n" + RenderExplain(action.All()[0], disp) + "\n"
	}
	out += "\n" + RenderFooter(action.All(), m.ActiveBackend().CanRename()) + "\n"
	if m.status != "" {
		out += "\n" + m.status + "\n"
	}
	return out
}
