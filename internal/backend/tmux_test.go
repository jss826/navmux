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

func TestParseTmuxList(t *testing.T) {
	out := "main|1|3\nwork|0|1\n"
	got, err := parseTmuxList(out)
	if err != nil {
		t.Fatal(err)
	}
	want := []Session{
		{Name: "main", Attached: true, Windows: 3},
		{Name: "work", Attached: false, Windows: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseTmuxList = %+v, want %+v", got, want)
	}
}

func TestParseTmuxListEmpty(t *testing.T) {
	got, err := parseTmuxList("")
	if err != nil || got != nil {
		t.Fatalf("parseTmuxList(\"\") = %+v, %v", got, err)
	}
}

func TestTmuxListNoServer(t *testing.T) {
	tx := newTmuxWithRun(func(args ...string) (string, error) {
		return "no server running on /tmp/tmux-1000/default", errFake
	})
	got, err := tx.List()
	if err != nil || got != nil {
		t.Fatalf("List() with no server = %+v, %v (want nil, nil)", got, err)
	}
}

func TestTmuxSessionOps(t *testing.T) {
	ops := NewTmux().SessionOps(Session{Name: "foo"})
	want := map[string]string{
		"新規ウィンドウ": "tmux new-window -t foo",
		"分割(縦)":   "tmux split-window -h -t foo",
		"分割(横)":   "tmux split-window -v -t foo",
		"次ウィンドウ":  "tmux next-window -t foo",
		"閉じる":     "tmux kill-window -t foo",
	}
	got := map[string]string{}
	for _, o := range ops {
		got[o.Label] = o.Command.Display
		if !o.Enabled {
			t.Fatalf("%s が無効になっている", o.Label)
		}
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: got %q want %q", k, got[k], v)
		}
	}
}

func TestTmuxSessionOpsHasDetachOthers(t *testing.T) {
	ops := NewTmux().SessionOps(Session{Name: "foo"})
	found := false
	for _, op := range ops {
		if op.Command.Display == "tmux detach-client -a -t foo" {
			found = true
			if !op.Enabled {
				t.Fatal("選択ありで他クライアント切断が無効")
			}
		}
	}
	if !found {
		t.Fatalf("tmux に他クライアント切断 op が無い: %+v", ops)
	}
}

func TestTmuxCaptureOps(t *testing.T) {
	ops := NewTmux().SessionOps(Session{Name: "foo"})
	want := map[string]bool{
		"tmux capture-pane -t foo -p":      false,
		"tmux capture-pane -t foo -p -S -": false,
	}
	for _, op := range ops {
		if _, ok := want[op.Command.Display]; ok {
			want[op.Command.Display] = true
			if !op.Capture {
				t.Fatalf("capture op の Capture=false: %q", op.Command.Display)
			}
			if !op.Enabled {
				t.Fatalf("選択ありで capture が無効: %q", op.Command.Display)
			}
		}
	}
	for disp, seen := range want {
		if !seen {
			t.Fatalf("capture op が無い: %q", disp)
		}
	}
}

var errFake = errorsNew("exit status 1")

func errorsNew(s string) error { return &fakeErr{s} }

type fakeErr struct{ s string }

func (e *fakeErr) Error() string { return e.s }
