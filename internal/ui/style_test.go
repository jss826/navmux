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
	out := styleDashboard("t", "l", "m", "e", "f", "")
	if strings.Contains(out, "完了") {
		t.Fatalf("空 status なのに status 文字列が出ている:\n%s", out)
	}
}
