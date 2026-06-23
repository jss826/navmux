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

// RenderFooter はアクションをキー併記で 1 行に並べ、実行可否を色で示す。
// 実行可 → 色付き / 実行不可（状態的・構造的どちらも） → 減光（グレーアウト）。記号は使わない。
func RenderFooter(actions []action.Action, b backend.Backend, name string) string {
	var parts []string
	for _, a := range actions {
		label := fmt.Sprintf("%s %s", a.Key, a.Label)
		if action.Runnable(b, a.Kind, name) {
			parts = append(parts, runnableStyle.Render(label))
		} else {
			parts = append(parts, faintStyle.Render(label))
		}
	}
	for _, h := range []string{"←→ ペイン移動", "y コピー", "u 更新", "? 解説", "tab tmux/zellij", "q 終了"} {
		parts = append(parts, faintStyle.Render(h))
	}
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
			label = faintStyle.Render(label) // 実行不可は減光（× 記号は使わない）
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
