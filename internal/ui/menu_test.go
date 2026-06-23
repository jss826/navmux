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
	// 操作（kindOp）が 5 つ含まれ display が埋まっている
	ops := 0
	for _, it := range items {
		if it.kind == kindOp {
			ops++
			if it.display == "" {
				t.Fatalf("op の display が空: %+v", it)
			}
		}
	}
	if ops != 5 {
		t.Fatalf("op の数 = %d want 5", ops)
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
