package ui

import (
	"testing"

	"github.com/jss826/navmux/internal/backend"
)

func TestBuildMenuTmux(t *testing.T) {
	items := buildMenu(backend.NewTmux(), backend.Session{Name: "main"})
	// 先頭はアタッチ（選択ありで有効・display あり）
	if items[0].label != "アタッチ" || !items[0].enabled || items[0].display != "tmux attach -t main" {
		t.Fatalf("先頭 item 不正: %+v", items[0])
	}
	// 区切りが 1 つ含まれる
	seps := 0
	for _, it := range items {
		if it.kind == kindSep {
			seps++
		}
	}
	if seps != 1 {
		t.Fatalf("区切りの数 = %d want 1", seps)
	}
	// 操作（kindOp）が 6 つ含まれ display が埋まっている
	ops := 0
	for _, it := range items {
		if it.kind == kindOp {
			ops++
			if it.display == "" {
				t.Fatalf("op の display が空: %+v", it)
			}
		}
	}
	if ops != 6 {
		t.Fatalf("op の数 = %d want 6", ops)
	}
}

func TestBuildMenuRenameDisabledOnZellij(t *testing.T) {
	items := buildMenu(backend.NewZellij(), backend.Session{Name: "x"})
	for _, it := range items {
		if it.kind == kindAction && it.act == 2 { // action.Rename
			if it.enabled {
				t.Fatal("zellij でリネームが有効になっている")
			}
		}
	}
}

func TestBuildMenuNewAlwaysEnabled(t *testing.T) {
	// セッション未選択（空）でも「新規セッション」は有効
	items := buildMenu(backend.NewTmux(), backend.Session{})
	for _, it := range items {
		if it.kind == kindAction && it.act == 1 { // action.New
			if !it.enabled {
				t.Fatal("新規セッションは常に有効のはず")
			}
		}
	}
}

func TestNextSelectableSkipsSepAndDisabled(t *testing.T) {
	items := []menuItem{
		{kind: kindAction, label: "a", enabled: true},  // 0
		{kind: kindAction, label: "b", enabled: false}, // 1 無効
		{kind: kindSep, label: "--"},                   // 2 区切り
		{kind: kindOp, label: "c", enabled: true},      // 3
	}
	if got := nextSelectable(items, 0, +1); got != 3 {
		t.Fatalf("0 から +1 = %d want 3（1 と 2 をスキップ）", got)
	}
	if got := nextSelectable(items, 3, -1); got != 0 {
		t.Fatalf("3 から -1 = %d want 0", got)
	}
	if got := nextSelectable(items, 3, +1); got != 3 {
		t.Fatalf("末尾で +1 は据え置き = %d want 3", got)
	}
}

func TestCurrentDisplay(t *testing.T) {
	items := []menuItem{{display: "X"}, {display: "Y"}}
	if currentDisplay(items, 1) != "Y" {
		t.Fatal("index 1 の display は Y")
	}
	if currentDisplay(items, 9) != "" {
		t.Fatal("範囲外は空文字")
	}
}
