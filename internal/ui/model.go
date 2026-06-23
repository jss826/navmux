package ui

import (
	"fmt"
	"strings"

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

// captureDoneMsg は画面コピー操作の完了。
type captureDoneMsg struct {
	lines int
	err   error
}

// seam（テストで差し替え可能）。
var (
	captureRunner  = runCapture
	clipboardWrite = clipboard.WriteAll
)

// countLines はキャプチャ文字列の行数を返す（末尾改行は無視）。
func countLines(out string) int {
	s := strings.TrimRight(out, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// Model は navmux の TUI 状態。
type Model struct {
	backends []backend.Backend
	active   int
	current  string // 現在の multiplexer（env）

	sessions []backend.Session
	cursor   int

	focus      int // 0=リスト, 1=メニュー
	menuCursor int // 右ペインのカーソル位置

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
		backends:   backends,
		current:    current,
		input:      ti,
		menuCursor: 1, // index 1 = "新規セッション"（常に enabled）
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

// menu はアクティブな backend と選択中セッションに応じたメニュー項目を返す。
func (m Model) menu() []menuItem {
	var sel backend.Session
	if m.cursor >= 0 && m.cursor < len(m.sessions) {
		sel = m.sessions[m.cursor]
	}
	return buildMenu(m.ActiveBackend(), sel)
}

// startRename はリネームの前提条件をガードしてから modePrompt に遷移する。
func (m Model) startRename() (tea.Model, tea.Cmd) {
	if !m.ActiveBackend().CanRename() {
		m.status = "この multiplexer はリネーム非対応"
		return m, nil
	}
	if m.selectedName() == "" {
		m.status = "セッションが選択されていません"
		return m, nil
	}
	m.pending = action.Rename
	m.mode = modePrompt
	m.input.Focus()
	return m, textinput.Blink
}

// runMenuItem は右ペインで Enter を押したときに menuCursor 位置の項目を実行する。
func (m Model) runMenuItem() (tea.Model, tea.Cmd) {
	items := m.menu()
	if m.menuCursor < 0 || m.menuCursor >= len(items) {
		return m, nil
	}
	it := items[m.menuCursor]
	if !it.enabled {
		return m, nil
	}
	switch it.kind {
	case kindOp:
		c := it.command
		return m, func() tea.Msg {
			_, err := runCommand(c)
			return opDoneMsg{err: err}
		}
	case kindCapture:
		c := it.command
		return m, func() tea.Msg {
			out, err := captureRunner(c)
			if err != nil {
				return captureDoneMsg{err: err}
			}
			if werr := clipboardWrite(out); werr != nil {
				return captureDoneMsg{err: werr}
			}
			return captureDoneMsg{lines: countLines(out)}
		}
	case kindAction:
		switch it.act {
		case action.Attach:
			return m, m.attachSelected()
		case action.New:
			m.pending = action.New
			m.mode = modePrompt
			m.input.Focus()
			return m, textinput.Blink
		case action.Rename:
			return m.startRename()
		case action.Kill:
			if m.selectedName() == "" {
				return m, nil
			}
			m.mode = modeConfirm
			return m, nil
		}
	}
	return m, nil
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

	case captureDoneMsg:
		if msg.err != nil {
			m.status = "コピー失敗: " + msg.err.Error()
		} else {
			m.status = fmt.Sprintf("%d 行コピーしました", msg.lines)
		}
		return m, nil

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

	case "u":
		return m, m.refresh()

	case "tab":
		m.active = (m.active + 1) % len(m.backends)
		m.cursor = 0
		return m, m.refresh()

	case "?":
		m.showExplain = !m.showExplain

	case "left":
		m.focus = 0

	case "right":
		m.focus = 1

	case "up", "k":
		if m.focus == 1 {
			items := m.menu()
			m.menuCursor = nextSelectable(items, m.menuCursor, -1)
		} else {
			if m.cursor > 0 {
				m.cursor--
			}
		}

	case "down", "j":
		if m.focus == 1 {
			items := m.menu()
			m.menuCursor = nextSelectable(items, m.menuCursor, 1)
		} else {
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
			}
		}

	case "enter":
		if m.focus == 1 {
			return m.runMenuItem()
		}
		return m, m.attachSelected()

	case "n":
		m.pending = action.New
		m.mode = modePrompt
		m.input.Focus()
		return m, textinput.Blink

	case "r":
		return m.startRename()

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
	if k == action.New {
		return m.newSessionCmd(b, arg)
	}
	sel := m.selectedName()
	return func() tea.Msg {
		var c backend.Command
		switch k {
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

// newSessionCmd は新規セッションを実端末で生成する。detached 生成（例 zellij attach -b）は
// 実コンソールが無いと server が永続しないため、attach と同様に TUI を一時停止して端末を渡す。
// （hidden 新コンソールへ投げる旧 spawnDetached 方式では server が使い捨てコンソールに紐づき、
// 生成コマンド終了とともに死ぬ＝navmux 経由のセッションが永続しなかった原因。）
// exit 0 を信用せず confirmCreated で実在も確認する。
func (m Model) newSessionCmd(b backend.Backend, name string) tea.Cmd {
	c := execCommand(b.NewCmd(name))
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return opDoneMsg{err: err}
		}
		return opDoneMsg{err: confirmCreated(b, name)}
	})
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
	// 右ペインにフォーカスがある場合はカーソル項目の display を使う
	if m.focus == 1 {
		items := m.menu()
		disp := currentDisplay(items, m.menuCursor)
		if disp == "" {
			m.status = "コピーできるコマンドがありません"
			return
		}
		if err := clipboard.WriteAll(disp); err != nil {
			m.status = "コピー失敗: " + err.Error()
			return
		}
		m.status = "コピーしました: " + disp
		return
	}
	// 左ペイン（一覧）フォーカス時はアタッチコマンドをコピー
	if m.selectedName() == "" {
		m.status = "コピーできるコマンドがありません"
		return
	}
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
	items := m.menu()
	title := "navmux — " + m.ActiveBackend().Name()
	// 土台（タイトル/2ペイン/実行行/フッター/status）は装飾層 styleDashboard で組む。
	// 純コンテンツ層（RenderList/RenderMenu/currentDisplay/RenderFooter）はプレーンのまま渡す。
	out := styleDashboard(
		title,
		RenderList(m.sessions, m.cursor),
		RenderMenu(items, m.menuCursor, m.focus == 1),
		currentDisplay(items, m.menuCursor),
		RenderFooter(action.All(), m.ActiveBackend(), m.selectedName()),
		m.status,
		m.focus,
	)

	if m.mode == modePrompt {
		out += "\n名前: " + m.input.View() + "\n(enter 確定 / esc キャンセル)"
	}
	if m.mode == modeConfirm {
		out += "\n削除しますか? " + m.selectedName() + " [y/N]"
	}
	if m.showExplain {
		// カーソル項目（左ペイン=アタッチ、右ペイン=メニュー項目）に連動した解説
		var explainLabel, explainDisp string
		if m.focus == 1 {
			if m.menuCursor >= 0 && m.menuCursor < len(items) {
				explainLabel = items[m.menuCursor].label
			}
			explainDisp = currentDisplay(items, m.menuCursor)
		} else {
			explainLabel = action.All()[0].Label // "アタッチ"
			explainDisp, _ = action.CommandFor(m.ActiveBackend(), action.Attach, m.selectedName(), "")
		}
		a := action.Action{Key: "?", Label: explainLabel}
		out += "\n" + RenderExplain(a, explainDisp)
	}
	return out
}
