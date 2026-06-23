package ui

import (
	"testing"

	"github.com/jss826/navmux/internal/backend"
)

func TestContainsSession(t *testing.T) {
	ss := []backend.Session{{Name: "a"}, {Name: "b"}}
	if !containsSession(ss, "b") {
		t.Fatal("b が含まれるはず")
	}
	if containsSession(ss, "x") {
		t.Fatal("x は含まれないはず")
	}
	if containsSession(nil, "a") {
		t.Fatal("nil で false のはず")
	}
}
