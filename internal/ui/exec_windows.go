//go:build windows

package ui

import (
	"os/exec"
	"syscall"

	"github.com/jss826/navmux/internal/backend"
)

// CREATE_NEW_CONSOLE: 子プロセスに新しいコンソールを割り当てる。
// zellij の detached セッションは中のシェルが生存するためにコンソールを要するため、
// パイプ実行（CombinedOutput）では即終了してしまう。新コンソールを与えて投げっぱなしにする。
const createNewConsole = 0x00000010

func spawnDetached(c backend.Command) error {
	if len(c.Argv) == 0 {
		return nil
	}
	cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole, HideWindow: true}
	return cmd.Start() // Wait しない（独立プロセスとして生かす）
}
