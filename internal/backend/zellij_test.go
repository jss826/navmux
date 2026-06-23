package backend

import (
	"reflect"
	"strings"
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

func TestZellijSessionOps(t *testing.T) {
	ops := NewZellij().SessionOps(Session{Name: "foo"})
	want := map[string]string{
		"新規タブ":  "zellij -s foo action new-tab",
		"分割(縦)": "zellij -s foo action new-pane -d right",
		"分割(横)": "zellij -s foo action new-pane -d down",
		"次タブ":   "zellij -s foo action go-to-next-tab",
		"閉じる":   "zellij -s foo action close-pane",
	}
	got := map[string]string{}
	for _, o := range ops {
		got[o.Label] = o.Command.Display
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: got %q want %q", k, got[k], v)
		}
	}
	// EXITED セッションでは無効
	for _, o := range NewZellij().SessionOps(Session{Name: "foo", Dead: true}) {
		if o.Enabled {
			t.Fatalf("EXITED で %s が有効になっている", o.Label)
		}
	}
}

func TestZellijCaptureOps(t *testing.T) {
	ops := NewZellij().SessionOps(Session{Name: "foo"})
	want := map[string]bool{
		"zellij -s foo action dump-screen":    false,
		"zellij -s foo action dump-screen -f": false,
	}
	for _, op := range ops {
		if _, ok := want[op.Command.Display]; ok {
			want[op.Command.Display] = true
			if !op.Capture {
				t.Fatalf("capture op の Capture=false: %q", op.Command.Display)
			}
		}
	}
	for disp, seen := range want {
		if !seen {
			t.Fatalf("capture op が無い: %q", disp)
		}
	}
}

func TestZellijSessionOpsDetachOthersIsHint(t *testing.T) {
	ops := NewZellij().SessionOps(Session{Name: "foo"})
	found := false
	for _, op := range ops {
		if strings.Contains(op.Label, "他クライアント切断") {
			found = true
			if op.Enabled {
				t.Fatal("zellij は他クライアント切断 CLI 非対応のはず（Enabled=false）")
			}
			if !strings.Contains(op.Label, "Ctrl o w Ctrl x") {
				t.Fatalf("キーヒントがラベルに無い: %q", op.Label)
			}
		}
	}
	if !found {
		t.Fatalf("zellij に他クライアント切断 op が無い: %+v", ops)
	}
}
