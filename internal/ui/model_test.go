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

func TestCaptureCopiesAndCountsLines(t *testing.T) {
	oldClip, oldCap := clipboardWrite, captureRunner
	defer func() { clipboardWrite, captureRunner = oldClip, oldCap }()

	var copied string
	clipboardWrite = func(s string) error { copied = s; return nil }
	captureRunner = func(c backend.Command) (string, error) { return "a\nb\nc\n", nil }

	m := New([]backend.Backend{backend.NewZellij()}, "")
	m.sessions = []backend.Session{{Name: "navmux"}}
	m.focus = 1

	items := m.menu()
	idx := -1
	for i, it := range items {
		if it.kind == kindCapture {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("kindCapture 項目が無い")
	}
	m.menuCursor = idx

	_, cmd := m.runMenuItem()
	if cmd == nil {
		t.Fatal("capture 実行で cmd が nil")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)

	if copied != "a\nb\nc\n" {
		t.Fatalf("クリップボード = %q", copied)
	}
	if m.status != "3 行コピーしました" {
		t.Fatalf("status = %q", m.status)
	}
}

func TestCountLines(t *testing.T) {
	cases := map[string]int{
		"":        0,
		"\n":      0,
		"x":       1,
		"a\nb\n":  2,
		"a\nb\nc": 3,
	}
	for in, want := range cases {
		if got := countLines(in); got != want {
			t.Fatalf("countLines(%q) = %d want %d", in, got, want)
		}
	}
}

func TestCopyNoSessionListPane(t *testing.T) {
	// セッションなし・focus=0（左ペイン）で y を押すと不正コマンドをコピーしない
	m := New([]backend.Backend{backend.NewTmux()}, "")
	(&m).copyCurrentCommand()
	if m.status != "コピーできるコマンドがありません" {
		t.Fatalf("未選択コピーの status = %q", m.status)
	}
}

func TestURefreshes(t *testing.T) {
	m := New([]backend.Backend{backend.NewTmux()}, "")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if cmd == nil {
		t.Fatal("u で refresh cmd が返らない")
	}
}

type purgeSpyBackend struct {
	backend.Backend
	purged []string
}

func (b *purgeSpyBackend) PurgeSocket(name string) error {
	b.purged = append(b.purged, name)
	return nil
}

func TestKillZombiePurgesSocket(t *testing.T) {
	spy := &purgeSpyBackend{Backend: backend.NewZellij()}
	m := New([]backend.Backend{spy}, "zellij")
	m.sessions = []backend.Session{{Name: "den2", Zombie: true}}
	m.cursor = 0
	m.purgeIfDead(spy, m.selectedSession())
	if len(spy.purged) != 1 || spy.purged[0] != "den2" {
		t.Fatalf("PurgeSocket 呼び出し = %v, want [den2]", spy.purged)
	}
}

func TestKillAliveDoesNotPurge(t *testing.T) {
	spy := &purgeSpyBackend{Backend: backend.NewZellij()}
	m := New([]backend.Backend{spy}, "zellij")
	m.sessions = []backend.Session{{Name: "live"}}
	m.cursor = 0
	m.purgeIfDead(spy, m.selectedSession())
	if len(spy.purged) != 0 {
		t.Fatalf("生存で PurgeSocket が呼ばれた: %v", spy.purged)
	}
}
