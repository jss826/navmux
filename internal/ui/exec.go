package ui

import (
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/jss826/navmux/internal/backend"
)

// runCapture は stdout のみを取得する（画面ダンプのコピー用）。
func runCapture(c backend.Command) (string, error) {
	if len(c.Argv) == 0 {
		return "", nil
	}
	out, err := exec.Command(c.Argv[0], c.Argv[1:]...).Output()
	return string(out), err
}

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

// errNotCreated は生成コマンドが exit 0 でも実在確認に失敗したときに返す。
var errNotCreated = errors.New("作成に失敗した可能性があります（一覧に現れませんでした）")

// confirmCreated は name のセッションが List() に現れるまで短くポーリングして
// 実在を確認する。生成コマンドの exit 0 を信用しない。
func confirmCreated(b backend.Backend, name string) error {
	for i := 0; i < 15; i++ {
		if ss, err := b.List(); err == nil && containsSession(ss, name) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errNotCreated
}
