package backend

import (
	"reflect"
	"testing"
)

func TestZellijBuilders(t *testing.T) {
	z := NewZellij()

	if z.Name() != "zellij" {
		t.Fatalf("Name() = %q", z.Name())
	}
	if z.CanRename() {
		t.Fatalf("CanRename() = true, want false")
	}
	if got := z.AttachCmd("foo"); got.Display != "zellij attach foo" {
		t.Fatalf("AttachCmd = %+v", got)
	}
	if _, ok := z.SwitchCmd("foo"); ok {
		t.Fatalf("SwitchCmd ok = true, want false")
	}
	if got := z.NewCmd("foo"); got.Display != "zellij attach -b foo" {
		t.Fatalf("NewCmd = %+v", got)
	}
	if _, ok := z.RenameCmd("a", "b"); ok {
		t.Fatalf("RenameCmd ok = true, want false")
	}
	if got := z.KillCmd("foo"); got.Display != "zellij delete-session -f foo" {
		t.Fatalf("KillCmd = %+v", got)
	}
}

func TestParseZellijList(t *testing.T) {
	out := "tui [Created 36m 31s ago] (current)\n" +
		"work [Created 2h 10s ago]\n" +
		"old [Created 5d ago] (EXITED - attach to resurrect)\n"
	got := parseZellijList(out)
	want := []Session{
		{Name: "tui", Attached: true},
		{Name: "work"},
		{Name: "old", Dead: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseZellijList = %+v, want %+v", got, want)
	}
}

func TestZellijListNoSessions(t *testing.T) {
	z := newZellijWithRun(func(args ...string) (string, error) {
		return "No active zellij sessions found.", errFake
	})
	got, err := z.List()
	if err != nil || got != nil {
		t.Fatalf("List() no sessions = %+v, %v", got, err)
	}
}
