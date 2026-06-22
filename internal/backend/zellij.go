package backend

import (
	"os/exec"
	"strings"
)

const zellijBin = "zellij"

// Zellij は zellij backend。
type Zellij struct {
	run runFunc
}

// NewZellij は実プロセスで動く zellij backend を返す。
func NewZellij() *Zellij { return &Zellij{run: execRun(zellijBin)} }

// newZellijWithRun はテスト用に runFunc を差し替える。
func newZellijWithRun(run runFunc) *Zellij { return &Zellij{run: run} }

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

func (z *Zellij) KillCmd(name string) Command {
	return cmd(zellijBin, "delete-session", "-f", name)
}

func (z *Zellij) CanRename() bool { return false }

func (z *Zellij) List() ([]Session, error) {
	out, err := z.run("list-sessions", "-n")
	if err != nil {
		if strings.Contains(out, "No active zellij sessions") {
			return nil, nil
		}
		return nil, err
	}
	return parseZellijList(out), nil
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
