package backend

import (
	"reflect"
	"testing"
	"testing/fstest"
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

func TestValidSessionName(t *testing.T) {
	cases := map[string]bool{
		"nav": true, "den2": true,
		"": false, "..": false, "a/b": false, `a\b`: false, "a..b": false,
	}
	for name, want := range cases {
		if got := validSessionName(name); got != want {
			t.Fatalf("validSessionName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestFindSocket(t *testing.T) {
	fsys := fstest.MapFS{
		"contract_version_1/nav":  {Data: []byte("x")},
		"contract_version_1/den2": {Data: []byte("x")},
	}
	got, ok := findSocket(fsys, "den2")
	if !ok || got != "contract_version_1/den2" {
		t.Fatalf("findSocket den2 = %q,%v", got, ok)
	}
	if _, ok := findSocket(fsys, "missing"); ok {
		t.Fatal("findSocket missing は false のはず")
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
