package ui

import (
	"github.com/jss826/navmux/internal/action"
	"github.com/jss826/navmux/internal/backend"
)

type itemKind int

const (
	kindAction  itemKind = iota // セッションアクション（attach/new/rename/kill）
	kindOp                      // mux 操作
	kindSep                     // 区切り（選択不可）
	kindCapture                 // 画面ダンプを取得してコピーする操作
)

// menuItem は右ペインの 1 行。display は「実行:」行/コピーに使う。
type menuItem struct {
	kind    itemKind
	label   string
	act     action.Kind     // kind==kindAction のとき有効
	command backend.Command // kind==kindOp のとき有効
	display string
	enabled bool
}

// canAttach は s にアタッチ可能か（生存していて名前があるか）を返す。
func canAttach(s backend.Session) bool {
	return s.Name != "" && !s.Dead && !s.Zombie
}

// buildMenu は選択中セッションに対する右ペインのメニューを組む（純関数）。
func buildMenu(b backend.Backend, sel backend.Session) []menuItem {
	name := sel.Name
	killLabel := "削除"
	killDisplay := b.KillCmd(name).Display
	if sel.Dead || sel.Zombie {
		killLabel = "掃除"
		killDisplay = "ソケット削除 + " + killDisplay
	}
	items := []menuItem{
		{kind: kindAction, act: action.Attach, label: "アタッチ", display: b.AttachCmd(name).Display, enabled: canAttach(sel)},
		{kind: kindAction, act: action.New, label: "新規セッション", display: b.NewCmd("<name>").Display, enabled: action.Runnable(b, action.New, name)},
		{kind: kindAction, act: action.Rename, label: "リネーム", enabled: action.Runnable(b, action.Rename, name)},
		{kind: kindAction, act: action.Kill, label: killLabel, display: killDisplay, enabled: action.Runnable(b, action.Kill, name)},
		{kind: kindSep, label: "── 操作 ──"},
	}
	if rc, ok := b.RenameHintCmd(name, "<new>"); ok {
		items[2].display = rc.Display
	}
	for _, op := range b.SessionOps(sel) {
		k := kindOp
		if op.Capture {
			k = kindCapture
		}
		items = append(items, menuItem{
			kind:    k,
			label:   op.Label,
			command: op.Command,
			display: op.Command.Display,
			enabled: op.Enabled,
		})
	}
	return items
}

// nextSelectable は cur から dir 方向へ、区切り/無効を飛ばした次の選択可能 index を返す。
// 端では cur のまま据え置く。
func nextSelectable(items []menuItem, cur, dir int) int {
	i := cur
	for {
		n := i + dir
		if n < 0 || n >= len(items) {
			return cur
		}
		i = n
		if items[i].kind != kindSep && items[i].enabled {
			return i
		}
	}
}

// currentDisplay は cur 位置の display を返す（範囲外は空文字）。
func currentDisplay(items []menuItem, cur int) string {
	if cur < 0 || cur >= len(items) {
		return ""
	}
	return items[cur].display
}
