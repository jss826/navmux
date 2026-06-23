package ui

import (
	"strings"
	"testing"
)

// styleDashboard は装飾層。ANSI の正確な中身は検査せず、
// 純コンテンツ（title/list/menu/exec/footer/status）が出力に残ることだけを保証する。
func TestStyleDashboardPreservesContent(t *testing.T) {
	out := styleDashboard(
		"navmux — tmux",
		"> * main\n",
		"> アタッチ\n",
		"tmux attach -t main",
		"enter アタッチ   q 終了",
		"完了",
		0,
	)
	for _, want := range []string{
		"navmux — tmux",
		"main",
		"アタッチ",
		"tmux attach -t main",
		"q 終了",
		"完了",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("styleDashboard 出力に %q が残っていない:\n%s", want, out)
		}
	}
}

// status が空なら status 行は出さない。
func TestStyleDashboardOmitsEmptyStatus(t *testing.T) {
	out := styleDashboard("t", "l", "m", "e", "f", "", 0)
	if strings.Contains(out, "完了") {
		t.Fatalf("空 status なのに status 文字列が出ている:\n%s", out)
	}
}

// focus 0/1 で出力が異なる（フォーカス枠が反映される）。
func TestStyleDashboardFocusChangesOutput(t *testing.T) {
	left := styleDashboard("t", "l", "m", "e", "f", "", 0)
	right := styleDashboard("t", "l", "m", "e", "f", "", 1)
	if left == right {
		t.Fatal("focus 0/1 で出力が同一（フォーカス枠が反映されていない）")
	}
	// focus が異なると Sessions/Actions ペインの枠色コード（BrightCyan = color 14）が異なるはず
	// left なら Sessions に色が入り、right なら Actions に色が入るので、出力は異なる
}
