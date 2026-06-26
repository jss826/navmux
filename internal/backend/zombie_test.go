package backend

import (
	"reflect"
	"testing"
)

func TestParseServerNames(t *testing.T) {
	in := []string{
		`C:\Users\soon7\AppData\Local\Zellij\zellij.exe --server C:\Users\soon7\AppData\Local\Temp\zellij\contract_version_1\nav`,
		`/usr/bin/zellij --server /tmp/zellij-1000/0.44.3/work`,
		`zellij.exe attach -c den2`,
	}
	got := parseServerNames(in)
	want := []string{"nav", "work"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseServerNames = %v, want %v", got, want)
	}
}

func TestMarkZombies(t *testing.T) {
	sessions := []Session{
		{Name: "nav", Attached: true},
		{Name: "work"},
		{Name: "den2"},
		{Name: "old", Dead: true},
	}
	got := markZombies(sessions, []string{"nav", "work"})
	want := []Session{
		{Name: "nav", Attached: true},
		{Name: "work"},
		{Name: "den2", Zombie: true},
		{Name: "old", Dead: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("markZombies = %+v, want %+v", got, want)
	}
}

func TestZellijListMarksZombie(t *testing.T) {
	run := func(args ...string) (string, error) {
		return "nav [Created 1m ago] (current)\nden2 [Created 8h ago]\n", nil
	}
	proc := func() ([]string, error) {
		return []string{`zellij --server /tmp/zellij/contract_version_1/nav`}, nil
	}
	z := newZellijWithProc(run, proc)
	got, err := z.List()
	if err != nil {
		t.Fatalf("List() err = %v", err)
	}
	want := []Session{
		{Name: "nav", Attached: true},
		{Name: "den2", Zombie: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %+v, want %+v", got, want)
	}
}
