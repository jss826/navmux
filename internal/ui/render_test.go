package ui

import (
	"strings"
	"testing"

	"github.com/jss826/navmux/internal/action"
	"github.com/jss826/navmux/internal/backend"
)

func TestRenderListMarksCursorAndState(t *testing.T) {
	sessions := []backend.Session{
		{Name: "main", Attached: true, Windows: 3},
		{Name: "old", Dead: true},
	}
	out := RenderList(sessions, 1)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.Contains(lines[0], "main") || !strings.Contains(lines[0], "*") {
		t.Fatalf("行0 に attached マーク無し: %q", lines[0])
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[1]), ">") || !strings.Contains(lines[1], "EXITED") {
		t.Fatalf("行1 にカーソル/EXITED 無し: %q", lines[1])
	}
}

func TestRenderFooterShowsKeys(t *testing.T) {
	out := RenderFooter(action.All(), true)
	for _, want := range []string{"enter", "アタッチ", "n", "d"} {
		if !strings.Contains(out, want) {
			t.Fatalf("footer に %q が無い: %q", want, out)
		}
	}
}

func TestRenderFooterGreysRenameWhenUnsupported(t *testing.T) {
	out := RenderFooter(action.All(), false)
	if !strings.Contains(out, "非対応") {
		t.Fatalf("リネーム非対応の目印が無い: %q", out)
	}
}

func TestRenderExplainShowsCommand(t *testing.T) {
	a := action.Action{Key: "enter", Label: "アタッチ", Explain: "接続する。"}
	out := RenderExplain(a, "tmux attach -t main")
	if !strings.Contains(out, "接続する。") || !strings.Contains(out, "tmux attach -t main") {
		t.Fatalf("解説/コマンドが欠落: %q", out)
	}
}
