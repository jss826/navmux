package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const zellijBin = "zellij"

// procLister は実行中プロセスのコマンドライン一覧を返す。テストで差し替える。
type procLister func() ([]string, error)

// Zellij は zellij backend。
type Zellij struct {
	run  runFunc
	proc procLister
}

// NewZellij は実プロセスで動く zellij backend を返す（proc は Task 4 で注入）。
func NewZellij() *Zellij { return &Zellij{run: execRun(zellijBin), proc: serverCommandLines} }

// newZellijWithRun はテスト用に runFunc を差し替える（proc は nil）。
func newZellijWithRun(run runFunc) *Zellij { return &Zellij{run: run} }

// newZellijWithProc はテスト用に run と proc を差し替える。
func newZellijWithProc(run runFunc, proc procLister) *Zellij { return &Zellij{run: run, proc: proc} }

func (z *Zellij) Name() string { return "zellij" }

func (z *Zellij) Available() bool {
	_, err := exec.LookPath(zellijBin)
	return err == nil
}

func (z *Zellij) AttachCmd(name string) Command {
	return cmd(zellijBin, "attach", name)
}

// SwitchCmd: zellij はセッション内切替が弱いため非対応。
func (z *Zellij) SwitchCmd(name string) (Command, bool) {
	return Command{}, false
}

func (z *Zellij) NewCmd(name string) Command {
	return cmd(zellijBin, "attach", "-b", name)
}

// RenameCmd: zellij は detached のリネーム不可。
func (z *Zellij) RenameCmd(oldName, newName string) (Command, bool) {
	return Command{}, false
}

// RenameHintCmd は表示専用。alive セッションは action rename-session で改名可能だが、
// Windows のコンソールなし実行でハングするため navmux からは実行しない（提示のみ）。
func (z *Zellij) RenameHintCmd(oldName, newName string) (Command, bool) {
	return cmd(zellijBin, "-s", oldName, "action", "rename-session", newName), true
}

func (z *Zellij) KillCmd(name string) Command {
	return cmd(zellijBin, "delete-session", "-f", name)
}

func (z *Zellij) CanRename() bool { return false }

func (z *Zellij) PurgeSocket(name string) error {
	if !validSessionName(name) {
		return fmt.Errorf("不正なセッション名: %q", name)
	}
	root := socketRoot()
	rel, ok := findSocket(os.DirFS(root), name)
	if !ok {
		return nil
	}
	return os.Remove(filepath.Join(root, filepath.FromSlash(rel)))
}

func (z *Zellij) SessionOps(s Session) []OpPreset {
	en := s.Name != "" && !s.Dead && !s.Zombie
	n := s.Name
	return []OpPreset{
		{Label: "新規タブ", Command: cmd(zellijBin, "-s", n, "action", "new-tab"), Enabled: en},
		{Label: "分割(縦)", Command: cmd(zellijBin, "-s", n, "action", "new-pane", "-d", "right"), Enabled: en},
		{Label: "分割(横)", Command: cmd(zellijBin, "-s", n, "action", "new-pane", "-d", "down"), Enabled: en},
		{Label: "次タブ", Command: cmd(zellijBin, "-s", n, "action", "go-to-next-tab"), Enabled: en},
		{Label: "閉じる", Command: cmd(zellijBin, "-s", n, "action", "close-pane"), Enabled: en},
		{Label: "他クライアント切断  Ctrl o w Ctrl x", Command: Command{Display: "Ctrl o w Ctrl x（手動）"}, Enabled: false},
		{Label: "画面コピー", Command: cmd(zellijBin, "-s", n, "action", "dump-screen"), Enabled: en, Capture: true},
		{Label: "全履歴コピー", Command: cmd(zellijBin, "-s", n, "action", "dump-screen", "-f"), Enabled: en, Capture: true},
	}
}

func (z *Zellij) List() ([]Session, error) {
	out, err := z.run("list-sessions", "-n")
	if err != nil {
		if strings.Contains(out, "No active zellij sessions") {
			return nil, nil
		}
		return nil, err
	}
	sessions := parseZellijList(out)
	if z.proc != nil {
		if cmdlines, perr := z.proc(); perr == nil {
			sessions = markZombies(sessions, parseServerNames(cmdlines))
		}
	}
	return sessions, nil
}

func parseZellijList(out string) []Session {
	var sessions []Session
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		sessions = append(sessions, Session{
			Name:     fields[0],
			Attached: strings.Contains(line, "(current)"),
			Dead:     strings.Contains(line, "EXITED"),
		})
	}
	return sessions
}
