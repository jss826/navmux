package action

import "github.com/jss826/navmux/internal/backend"

// Kind はユーザー操作の種類。
type Kind int

const (
	Attach Kind = iota
	New
	Rename
	Kill
)

// Action はフッターに出す 1 操作のメタ情報。
type Action struct {
	Kind    Kind
	Key     string // 表示するキー（例 "enter", "n"）
	Label   string // 例 "アタッチ"
	Explain string // 1〜2 行の解説
}

// All はフッター表示順のアクション一覧。
func All() []Action {
	return []Action{
		{Attach, "enter", "アタッチ", "選択中のセッションに接続する。外からは新しい子プロセスで開き、detach すると navmux に戻る。"},
		{New, "n", "新規", "名前を入力して detached（バックグラウンド）のセッションを新規作成する。"},
		{Rename, "r", "リネーム", "選択中セッションの名前を変更する。※ zellij は detached のリネーム非対応。"},
		{Kill, "d", "削除", "選択中セッションを削除する（確認あり）。"},
	}
}

// CommandFor はアクションが実行する人間向けコマンド文字列を返す。
// 純 UI 操作やサポート外は ok=false。
func CommandFor(b backend.Backend, k Kind, name, newName string) (string, bool) {
	switch k {
	case Attach:
		return b.AttachCmd(name).Display, true
	case New:
		return b.NewCmd(name).Display, true
	case Rename:
		c, ok := b.RenameCmd(name, newName)
		if !ok {
			return "", false
		}
		return c.Display, true
	case Kill:
		return b.KillCmd(name).Display, true
	}
	return "", false
}

// Runnable はそのアクションが今この瞬間に実行可能かを返す（フッター/メニュー共通）。
func Runnable(b backend.Backend, k Kind, name string) bool {
	switch k {
	case New:
		return true
	case Attach, Kill:
		return name != ""
	case Rename:
		return b.CanRename() && name != ""
	}
	return false
}
