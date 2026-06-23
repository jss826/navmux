package backend

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const tmuxBin = "tmux"

const tmuxListFormat = "#{session_name}|#{?session_attached,1,0}|#{session_windows}"

// Tmux は tmux backend。
type Tmux struct {
	run runFunc
}

// NewTmux は実プロセスで動く tmux backend を返す。
func NewTmux() *Tmux { return &Tmux{run: execRun(tmuxBin)} }

func (t *Tmux) Name() string { return "tmux" }

func (t *Tmux) Available() bool {
	_, err := exec.LookPath(tmuxBin)
	return err == nil
}

func (t *Tmux) AttachCmd(name string) Command {
	return cmd(tmuxBin, "attach", "-t", name)
}

func (t *Tmux) SwitchCmd(name string) (Command, bool) {
	return cmd(tmuxBin, "switch-client", "-t", name), true
}

func (t *Tmux) NewCmd(name string) Command {
	return cmd(tmuxBin, "new-session", "-d", "-s", name)
}

func (t *Tmux) RenameCmd(oldName, newName string) (Command, bool) {
	return cmd(tmuxBin, "rename-session", "-t", oldName, newName), true
}

func (t *Tmux) KillCmd(name string) Command {
	return cmd(tmuxBin, "kill-session", "-t", name)
}

func (t *Tmux) CanRename() bool { return true }

func (t *Tmux) SessionOps(s Session) []OpPreset {
	en := s.Name != ""
	n := s.Name
	return []OpPreset{
		{Label: "新規ウィンドウ", Command: cmd(tmuxBin, "new-window", "-t", n), Enabled: en},
		{Label: "分割(縦)", Command: cmd(tmuxBin, "split-window", "-h", "-t", n), Enabled: en},
		{Label: "分割(横)", Command: cmd(tmuxBin, "split-window", "-v", "-t", n), Enabled: en},
		{Label: "次ウィンドウ", Command: cmd(tmuxBin, "next-window", "-t", n), Enabled: en},
		{Label: "閉じる", Command: cmd(tmuxBin, "kill-window", "-t", n), Enabled: en},
		{Label: "他クライアント切断", Command: cmd(tmuxBin, "detach-client", "-a", "-t", n), Enabled: en},
	}
}

// newTmuxWithRun はテスト用に runFunc を差し替えた backend を返す。
func newTmuxWithRun(run runFunc) *Tmux { return &Tmux{run: run} }

func (t *Tmux) List() ([]Session, error) {
	out, err := t.run("list-sessions", "-F", tmuxListFormat)
	if err != nil {
		// セッションが無いと tmux は非ゼロ終了で "no server running" を返す。
		if strings.Contains(out, "no server running") {
			return nil, nil
		}
		return nil, err
	}
	return parseTmuxList(out)
}

func parseTmuxList(out string) ([]Session, error) {
	var sessions []Session
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			return nil, fmt.Errorf("tmux list 行が不正: %q", line)
		}
		windows, _ := strconv.Atoi(parts[2])
		sessions = append(sessions, Session{
			Name:     parts[0],
			Attached: parts[1] == "1",
			Windows:  windows,
		})
	}
	return sessions, nil
}
