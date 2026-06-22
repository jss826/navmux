package backend

import (
	"errors"
	"os/exec"
	"strings"
)

// ErrUnsupported は、その backend がサポートしない操作で返す。
var ErrUnsupported = errors.New("この multiplexer ではサポートされていない操作です")

// Session は 1 つのセッションの状態。
type Session struct {
	Name     string
	Attached bool // 今このセッションにアタッチ中か
	Windows  int  // tmux のみ。zellij は 0
	Dead     bool // zellij の EXITED セッション。tmux は常に false
}

// Command は「実行用 Argv」と「表示/コピー用文字列」を分離して持つ。
type Command struct {
	Argv    []string
	Display string
}

// Backend は multiplexer の抽象。変更系は実行せずコマンドを返す。
type Backend interface {
	Name() string
	Available() bool
	List() ([]Session, error)

	AttachCmd(name string) Command
	SwitchCmd(name string) (Command, bool)            // tmux のみ true
	NewCmd(name string) Command                       // detached 作成
	RenameCmd(oldName, newName string) (Command, bool) // zellij は false
	KillCmd(name string) Command

	CanRename() bool
}

// cmd は Argv から Display を導出して Command を組む。
func cmd(argv ...string) Command {
	return Command{Argv: argv, Display: strings.Join(argv, " ")}
}

// runFunc はコマンドを実行し結合出力を返す。テストで差し替える。
type runFunc func(args ...string) (string, error)

// execRun は bin を実プロセスとして実行する runFunc を返す。
func execRun(bin string) runFunc {
	return func(args ...string) (string, error) {
		out, err := exec.Command(bin, args...).CombinedOutput()
		return string(out), err
	}
}
