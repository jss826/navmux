package backend

import "os/exec"

const tmuxBin = "tmux"

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

// List は Task 2 で実装する（一旦コンパイルを通すため最小実装）。
func (t *Tmux) List() ([]Session, error) { return nil, nil }
