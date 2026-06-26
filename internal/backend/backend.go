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
	Zombie   bool // server 不在なのに list に生きて見える応答なし状態。tmux は常に false
}

// Command は「実行用 Argv」と「表示/コピー用文字列」を分離して持つ。
type Command struct {
	Argv    []string
	Display string
}

// OpPreset は右ペインに並べる 1 つの mux 操作。
type OpPreset struct {
	Label   string  // 例 "分割(縦)"
	Command Command // 実行/表示用
	Enabled bool    // false ならグレーアウト（実行不可）
	Capture bool    // true なら stdout を取得してクリップボードに入れる操作
}

// Backend は multiplexer の抽象。変更系は実行せずコマンドを返す。
type Backend interface {
	Name() string
	Available() bool
	List() ([]Session, error)

	AttachCmd(name string) Command
	SwitchCmd(name string) (Command, bool)             // tmux のみ true
	NewCmd(name string) Command                        // detached 作成
	RenameCmd(oldName, newName string) (Command, bool)     // zellij は false
	RenameHintCmd(oldName, newName string) (Command, bool) // 表示専用（zellij も true）
	KillCmd(name string) Command

	// SessionOps は対象セッションに実行できる mux 操作の一覧（backend 固有）。
	SessionOps(s Session) []OpPreset
	CanRename() bool

	// PurgeSocket は name のソケット残骸を削除する（ゾンビ/EXITED 掃除用）。残骸が無ければ nil。
	PurgeSocket(name string) error
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
