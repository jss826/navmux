package attach

import (
	"testing"

	"github.com/jss826/navmux/internal/backend"
)

func TestResolve(t *testing.T) {
	tmux := backend.NewTmux()
	zj := backend.NewZellij()

	// tmux の中から tmux セッション → switch-client
	p := Resolve(tmux, "foo", "tmux")
	if p.Mode != ModeSwitch || p.Command.Display != "tmux switch-client -t foo" {
		t.Fatalf("tmux 内: %+v", p)
	}

	// tmux の外から → 子プロセス attach
	p = Resolve(tmux, "foo", "")
	if p.Mode != ModeChild || p.Command.Display != "tmux attach -t foo" {
		t.Fatalf("tmux 外: %+v", p)
	}

	// zellij は中からでも切替非対応 → 子プロセス attach
	p = Resolve(zj, "foo", "zellij")
	if p.Mode != ModeChild || p.Command.Display != "zellij attach foo" {
		t.Fatalf("zellij 内: %+v", p)
	}

	// 別 multiplexer の中にいる場合も子プロセス attach
	p = Resolve(tmux, "foo", "zellij")
	if p.Mode != ModeChild {
		t.Fatalf("cross: %+v", p)
	}
}
