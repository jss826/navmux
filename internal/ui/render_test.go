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
	// tmux + 選択あり → 全アクション実行可（× や 非対応 が付かない）
	out := RenderFooter(action.All(), backend.NewTmux(), "main")
	for _, want := range []string{"enter", "アタッチ", "n", "d"} {
		if !strings.Contains(out, want) {
			t.Fatalf("footer に %q が無い: %q", want, out)
		}
	}
	if strings.Contains(out, "×") || strings.Contains(out, "非対応") {
		t.Fatalf("選択ありでは不可マークが出ないはず: %q", out)
	}
}

func TestRenderFooterGreysRenameWhenUnsupported(t *testing.T) {
	out := RenderFooter(action.All(), backend.NewZellij(), "main")
	if !strings.Contains(out, "非対応") {
		t.Fatalf("リネーム非対応の目印が無い: %q", out)
	}
}

func TestRenderFooterMarksUnavailable(t *testing.T) {
	// tmux + 選択なし → アタッチ/削除は (×)、新規は通常表示
	out := RenderFooter(action.All(), backend.NewTmux(), "")
	if !strings.Contains(out, "(enter アタッチ ×)") {
		t.Fatalf("未選択でアタッチに × が付かない: %q", out)
	}
	if !strings.Contains(out, "(d 削除 ×)") {
		t.Fatalf("未選択で削除に × が付かない: %q", out)
	}
	if strings.Contains(out, "(n 新規 ×)") {
		t.Fatalf("新規に × が付いている: %q", out)
	}
}

func TestRenderExplainShowsCommand(t *testing.T) {
	a := action.Action{Key: "enter", Label: "アタッチ", Explain: "接続する。"}
	out := RenderExplain(a, "tmux attach -t main")
	if !strings.Contains(out, "接続する。") || !strings.Contains(out, "tmux attach -t main") {
		t.Fatalf("解説/コマンドが欠落: %q", out)
	}
}

func TestRenderMenuMarksCursorAndDisabled(t *testing.T) {
	items := []menuItem{
		{kind: kindAction, label: "アタッチ", enabled: true},
		{kind: kindAction, label: "リネーム", enabled: false},
		{kind: kindSep, label: "── 操作 ──"},
		{kind: kindOp, label: "分割(縦)", enabled: true},
	}
	out := RenderMenu(items, 0, true)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.HasPrefix(strings.TrimSpace(lines[0]), ">") {
		t.Fatalf("focus 時のカーソル行頭 > が無い: %q", lines[0])
	}
	if !strings.Contains(lines[1], "×") {
		t.Fatalf("無効項目の目印 × が無い: %q", lines[1])
	}
	if !strings.Contains(lines[2], "操作") {
		t.Fatalf("区切りが描画されない: %q", lines[2])
	}
	// 非フォーカス時はカーソル > を出さない
	if strings.Contains(RenderMenu(items, 0, false), ">") {
		t.Fatal("非フォーカスで > が出ている")
	}
}
