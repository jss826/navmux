package attach

import "github.com/jss826/navmux/internal/backend"

// Mode はアタッチの実行方式。
type Mode int

const (
	// ModeChild: 子プロセスで起動し端末を渡して待つ（detach で TUI に戻る）。
	ModeChild Mode = iota
	// ModeSwitch: switch-client 系を発火（端末ハンドオフ不要）。
	ModeSwitch
)

// Plan は「どのコマンドをどの方式で実行するか」。
type Plan struct {
	Command backend.Command
	Mode    Mode
}

// Resolve は現在の multiplexer に応じてアタッチ方式を決める。
func Resolve(b backend.Backend, name, current string) Plan {
	if current == b.Name() {
		if c, ok := b.SwitchCmd(name); ok {
			return Plan{Command: c, Mode: ModeSwitch}
		}
	}
	return Plan{Command: b.AttachCmd(name), Mode: ModeChild}
}
