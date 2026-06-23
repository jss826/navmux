package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jss826/navmux/internal/backend"
)

func TestTabSwitchesBackend(t *testing.T) {
	m := New([]backend.Backend{backend.NewTmux(), backend.NewZellij()}, "")
	if m.ActiveBackend().Name() != "tmux" {
		t.Fatalf("初期 active = %s", m.ActiveBackend().Name())
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.ActiveBackend().Name() != "zellij" {
		t.Fatalf("tab 後 active = %s", m.ActiveBackend().Name())
	}
}

func TestQuitKey(t *testing.T) {
	m := New([]backend.Backend{backend.NewTmux()}, "")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("q で quit cmd が返らない")
	}
}

func TestToggleExplain(t *testing.T) {
	m := New([]backend.Backend{backend.NewTmux()}, "")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = next.(Model)
	if !m.showExplain {
		t.Fatalf("? で解説が開かない")
	}
}

func TestRightArrowFocusesMenu(t *testing.T) {
	m := New([]backend.Backend{backend.NewTmux()}, "")
	if m.focus != 0 {
		t.Fatalf("初期 focus = %d, want 0", m.focus)
	}
	// → でフォーカスが 1 になる
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if m.focus != 1 {
		t.Fatalf("→ 後 focus = %d, want 1", m.focus)
	}
	// ← でフォーカスが 0 に戻る
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(Model)
	if m.focus != 0 {
		t.Fatalf("← 後 focus = %d, want 0", m.focus)
	}
}

func TestRenameGuardNoSelection(t *testing.T) {
	// セッションが空の状態で r を押すと modePrompt に入らず status が設定される
	m := New([]backend.Backend{backend.NewTmux()}, "")
	// セッション無し（初期状態は空）
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = next.(Model)
	if m.mode == modePrompt {
		t.Fatalf("セッション未選択で modePrompt に入ってしまった")
	}
	if m.status == "" {
		t.Fatalf("セッション未選択で r を押したのに status が空")
	}
}

func TestViewExplainOutOfRangeMenuCursorNoPanic(t *testing.T) {
	// showExplain + 右ペインフォーカスで menuCursor が範囲外でも panic しない。
	// 現状は不変条件で範囲外にならないが、View() の素引きを兄弟アクセサと同じくガードする回帰テスト。
	m := New([]backend.Backend{backend.NewTmux()}, "")
	m.focus = 1
	m.showExplain = true
	m.menuCursor = 9999 // 故意に範囲外
	_ = m.View()        // panic しなければ成功
}

func TestCopyNoSessionListPane(t *testing.T) {
	// セッションなし・focus=0（左ペイン）で y を押すと不正コマンドをコピーしない
	m := New([]backend.Backend{backend.NewTmux()}, "")
	(&m).copyCurrentCommand()
	if m.status != "コピーできるコマンドがありません" {
		t.Fatalf("未選択コピーの status = %q", m.status)
	}
}
