package ui

import (
	"os"
	"os/exec"

	"github.com/jss826/navmux/internal/backend"
)

// runCommand は非対話コマンド（new/rename/kill/switch）を実行し結合出力を返す。
func runCommand(c backend.Command) (string, error) {
	if len(c.Argv) == 0 {
		return "", nil
	}
	out, err := exec.Command(c.Argv[0], c.Argv[1:]...).CombinedOutput()
	return string(out), err
}

// execCommand は端末を渡して実行する *exec.Cmd を組む（tea.ExecProcess 用）。
func execCommand(c backend.Command) *exec.Cmd {
	cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// containsSession は name のセッションが一覧に存在するか返す。
func containsSession(sessions []backend.Session, name string) bool {
	for _, s := range sessions {
		if s.Name == name {
			return true
		}
	}
	return false
}
