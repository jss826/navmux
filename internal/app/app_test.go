package app

import (
	"testing"

	"github.com/jss826/navmux/internal/backend"
)

// fakeBackend は選択ロジックのテスト用。Name/Available のみ意味を持つ。
type fakeBackend struct {
	name      string
	available bool
}

func (f fakeBackend) Name() string                  { return f.name }
func (f fakeBackend) Available() bool                { return f.available }
func (f fakeBackend) List() ([]backend.Session, error) { return nil, nil }
func (f fakeBackend) AttachCmd(string) backend.Command { return backend.Command{} }
func (f fakeBackend) SwitchCmd(string) (backend.Command, bool) {
	return backend.Command{}, false
}
func (f fakeBackend) NewCmd(string) backend.Command { return backend.Command{} }
func (f fakeBackend) RenameCmd(string, string) (backend.Command, bool) {
	return backend.Command{}, false
}
func (f fakeBackend) RenameHintCmd(string, string) (backend.Command, bool) {
	return backend.Command{}, false
}
func (f fakeBackend) KillCmd(string) backend.Command { return backend.Command{} }
func (f fakeBackend) CanRename() bool                { return false }
func (f fakeBackend) SessionOps(backend.Session) []backend.OpPreset { return nil }

func names(bs []backend.Backend) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Name()
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestOrderedBackends_FiltersUnavailable(t *testing.T) {
	all := []backend.Backend{
		fakeBackend{name: "tmux", available: false},
		fakeBackend{name: "zellij", available: true},
	}
	got := names(OrderedBackends(all, ""))
	if want := []string{"zellij"}; !eq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestOrderedBackends_CurrentFirst(t *testing.T) {
	all := []backend.Backend{
		fakeBackend{name: "tmux", available: true},
		fakeBackend{name: "zellij", available: true},
	}
	got := names(OrderedBackends(all, "zellij"))
	if want := []string{"zellij", "tmux"}; !eq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestOrderedBackends_PreservesOrderWhenNoCurrent(t *testing.T) {
	all := []backend.Backend{
		fakeBackend{name: "tmux", available: true},
		fakeBackend{name: "zellij", available: true},
	}
	got := names(OrderedBackends(all, ""))
	if want := []string{"tmux", "zellij"}; !eq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestOrderedBackends_CurrentUnavailableIgnored(t *testing.T) {
	all := []backend.Backend{
		fakeBackend{name: "tmux", available: true},
		fakeBackend{name: "zellij", available: false},
	}
	// current=zellij だが利用不可なので無視され、tmux のみ残る。
	got := names(OrderedBackends(all, "zellij"))
	if want := []string{"tmux"}; !eq(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
