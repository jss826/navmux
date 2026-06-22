package backend

import (
	"reflect"
	"testing"
)

func TestTmuxBuilders(t *testing.T) {
	tx := NewTmux()

	if got := tx.Name(); got != "tmux" {
		t.Fatalf("Name() = %q, want tmux", got)
	}
	if !tx.CanRename() {
		t.Fatalf("CanRename() = false, want true")
	}

	if got := tx.AttachCmd("foo"); got.Display != "tmux attach -t foo" ||
		!reflect.DeepEqual(got.Argv, []string{"tmux", "attach", "-t", "foo"}) {
		t.Fatalf("AttachCmd = %+v", got)
	}

	sw, ok := tx.SwitchCmd("foo")
	if !ok || sw.Display != "tmux switch-client -t foo" {
		t.Fatalf("SwitchCmd = %+v ok=%v", sw, ok)
	}

	if got := tx.NewCmd("foo"); got.Display != "tmux new-session -d -s foo" {
		t.Fatalf("NewCmd = %+v", got)
	}

	rn, ok := tx.RenameCmd("old", "new")
	if !ok || rn.Display != "tmux rename-session -t old new" {
		t.Fatalf("RenameCmd = %+v ok=%v", rn, ok)
	}

	if got := tx.KillCmd("foo"); got.Display != "tmux kill-session -t foo" {
		t.Fatalf("KillCmd = %+v", got)
	}
}
