package action

import (
	"testing"

	"github.com/jss826/navmux/internal/backend"
)

func TestCommandForTmux(t *testing.T) {
	tx := backend.NewTmux()
	cases := []struct {
		k      Kind
		want   string
		wantOk bool
	}{
		{Attach, "tmux attach -t foo", true},
		{New, "tmux new-session -d -s foo", true},
		{Rename, "tmux rename-session -t foo bar", true},
		{Kill, "tmux kill-session -t foo", true},
	}
	for _, c := range cases {
		got, ok := CommandFor(tx, c.k, "foo", "bar")
		if got != c.want || ok != c.wantOk {
			t.Fatalf("CommandFor(%v) = %q,%v want %q,%v", c.k, got, ok, c.want, c.wantOk)
		}
	}
}

func TestCommandForZellijRenameUnsupported(t *testing.T) {
	z := backend.NewZellij()
	if _, ok := CommandFor(z, Rename, "foo", "bar"); ok {
		t.Fatalf("zellij Rename CommandFor ok = true, want false")
	}
	if got, ok := CommandFor(z, Attach, "foo", ""); !ok || got != "zellij attach foo" {
		t.Fatalf("zellij Attach = %q,%v", got, ok)
	}
}

func TestAllHasKeysAndExplain(t *testing.T) {
	for _, a := range All() {
		if a.Key == "" || a.Label == "" || a.Explain == "" {
			t.Fatalf("Action に空フィールド: %+v", a)
		}
	}
}

func TestRunnable(t *testing.T) {
	tx := backend.NewTmux()  // CanRename() == true
	z := backend.NewZellij() // CanRename() == false

	cases := []struct {
		name string
		b    backend.Backend
		k    Kind
		sel  string
		want bool
	}{
		{"new は常に可(選択なし)", tx, New, "", true},
		{"attach は選択必要", tx, Attach, "", false},
		{"attach 選択あり", tx, Attach, "foo", true},
		{"kill は選択必要", tx, Kill, "", false},
		{"kill 選択あり", tx, Kill, "foo", true},
		{"rename tmux 選択あり", tx, Rename, "foo", true},
		{"rename tmux 選択なし", tx, Rename, "", false},
		{"rename zellij は不可", z, Rename, "foo", false},
	}
	for _, c := range cases {
		if got := Runnable(c.b, c.k, c.sel); got != c.want {
			t.Fatalf("%s: Runnable = %v want %v", c.name, got, c.want)
		}
	}
}
