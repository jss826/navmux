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
