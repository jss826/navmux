package ui

import (
	"fmt"
	"strings"

	"github.com/jss826/navmux/internal/action"
	"github.com/jss826/navmux/internal/backend"
)

// RenderList は一覧を文字列化する。カーソル行は ">"、attached は "*"。
func RenderList(sessions []backend.Session, cursor int) string {
	if len(sessions) == 0 {
		return "  (セッションなし)\n"
	}
	var b strings.Builder
	for i, s := range sessions {
		cursorMark := " "
		if i == cursor {
			cursorMark = ">"
		}
		attachMark := " "
		if s.Attached {
			attachMark = "*"
		}
		extra := ""
		if s.Dead {
			extra = " (EXITED)"
		} else if s.Windows > 0 {
			extra = fmt.Sprintf(" [%d windows]", s.Windows)
		}
		fmt.Fprintf(&b, "%s %s %s%s\n", cursorMark, attachMark, s.Name, extra)
	}
	return b.String()
}

// RenderFooter はアクションをキー併記で 1 行に並べ、実行可否で表示を変える。
//
//	実行可 → "key label" / 状態的に不可 → "(key label ×)" / 構造的に非対応 → "(key label=非対応)"
func RenderFooter(actions []action.Action, b backend.Backend, name string) string {
	var parts []string
	for _, a := range actions {
		if a.Kind == action.Rename && !b.CanRename() {
			parts = append(parts, fmt.Sprintf("(%s %s=非対応)", a.Key, a.Label))
			continue
		}
		if action.Runnable(b, a.Kind, name) {
			parts = append(parts, fmt.Sprintf("%s %s", a.Key, a.Label))
		} else {
			parts = append(parts, fmt.Sprintf("(%s %s ×)", a.Key, a.Label))
		}
	}
	parts = append(parts, "←→ ペイン移動", "y コピー", "? 解説", "tab tmux/zellij", "q 終了")
	return strings.Join(parts, "   ")
}

// RenderMenu は右ペインのメニューを純テキストで描く。focused 時のみカーソル > を出す。
func RenderMenu(items []menuItem, cur int, focused bool) string {
	var b strings.Builder
	for i, it := range items {
		if it.kind == kindSep {
			fmt.Fprintf(&b, "  %s\n", it.label)
			continue
		}
		mark := " "
		if focused && i == cur {
			mark = ">"
		}
		label := it.label
		if !it.enabled {
			label += " (×)"
		}
		fmt.Fprintf(&b, "%s %s\n", mark, label)
	}
	return b.String()
}

// RenderExplain は解説と実コマンドを表示する。
func RenderExplain(a action.Action, commandDisplay string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %s\n", a.Key, a.Label)
	fmt.Fprintf(&b, "%s\n", a.Explain)
	if commandDisplay != "" {
		fmt.Fprintf(&b, "\n実行コマンド: %s\n", commandDisplay)
		fmt.Fprintf(&b, "(y でコピー)\n")
	}
	return b.String()
}
