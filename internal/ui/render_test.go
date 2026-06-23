package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

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
	// tmux + 選択あり → 全アクション実行可。記号(× / 非対応)は出ない。
	out := RenderFooter(action.All(), backend.NewTmux(), "main")
	for _, want := range []string{"enter", "アタッチ", "n", "d"} {
		if !strings.Contains(out, want) {
			t.Fatalf("footer に %q が無い: %q", want, out)
		}
	}
	if strings.Contains(out, "×") || strings.Contains(out, "非対応") {
		t.Fatalf("記号方式は廃止のはず: %q", out)
	}
}

// 実行可否で footer のスタイル（色/減光）が変わり、記号は使わない。
func TestRenderFooterStylesRunnableVsDisabled(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)

	runnable := RenderFooter(action.All(), backend.NewTmux(), "main") // 全可
	disabled := RenderFooter(action.All(), backend.NewTmux(), "")     // attach/rename/kill 不可
	if runnable == disabled {
		t.Fatal("実行可否で footer のスタイルが変わっていない")
	}
	if strings.Contains(disabled, "×") || strings.Contains(disabled, "非対応") {
		t.Fatalf("記号方式(× / 非対応)が残っている: %q", disabled)
	}
	if !strings.Contains(disabled, "アタッチ") {
		t.Fatalf("ラベルが欠落: %q", disabled)
	}
}

// zellij の rename 非対応は減光で示す（tmux の rename 可とスタイルが異なる）。
func TestRenderFooterFaintsZellijRename(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)

	zj := RenderFooter(action.All(), backend.NewZellij(), "main")
	tm := RenderFooter(action.All(), backend.NewTmux(), "main")
	if zj == tm {
		t.Fatal("zellij(rename不可) と tmux(rename可) で footer スタイルが同一")
	}
	if strings.Contains(zj, "非対応") || strings.Contains(zj, "×") {
		t.Fatalf("記号が残っている: %q", zj)
	}
}

func TestRenderExplainShowsCommand(t *testing.T) {
	a := action.Action{Key: "enter", Label: "アタッチ", Explain: "接続する。"}
	out := RenderExplain(a, "tmux attach -t main")
	if !strings.Contains(out, "接続する。") || !strings.Contains(out, "tmux attach -t main") {
		t.Fatalf("解説/コマンドが欠落: %q", out)
	}
}

func TestRenderFooterShowsRefresh(t *testing.T) {
	out := RenderFooter(action.All(), backend.NewTmux(), "main")
	if !strings.Contains(out, "u 更新") {
		t.Fatalf("footer に u 更新 が無い: %q", out)
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
	if strings.Contains(out, "×") {
		t.Fatalf("× 記号は廃止のはず: %q", out)
	}
	if !strings.Contains(lines[2], "操作") {
		t.Fatalf("区切りが描画されない: %q", lines[2])
	}
	// 非フォーカス時はカーソル > を出さない
	if strings.Contains(RenderMenu(items, 0, false), ">") {
		t.Fatal("非フォーカスで > が出ている")
	}
}

// 無効なメニュー項目は減光で示す（有効項目とスタイルが異なる）。
func TestRenderMenuFaintsDisabled(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)

	en := RenderMenu([]menuItem{{kind: kindAction, label: "リネーム", enabled: true}}, -1, false)
	dis := RenderMenu([]menuItem{{kind: kindAction, label: "リネーム", enabled: false}}, -1, false)
	if en == dis {
		t.Fatal("enabled/disabled でメニュー行スタイルが変わっていない")
	}
}
