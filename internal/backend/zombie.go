package backend

import (
	"io/fs"
	"path"
	"strings"
)

// validSessionName は os.Remove に渡す前のセッション名の防御的検証。
func validSessionName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return false
	}
	return true
}

// findSocket は fsys 配下を歩き、ベース名が name と完全一致する最初のファイルの相対パスを返す。
func findSocket(fsys fs.FS, name string) (string, bool) {
	var found string
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && path.Base(p) == name {
			found = p
			return fs.SkipAll
		}
		return nil
	})
	return found, found != ""
}

// markZombies は server プロセス名集合に存在しない alive セッションを Zombie にする。
// Dead(EXITED) と Attached(current=自分が接続中＝生存) は対象外。
func markZombies(sessions []Session, serverNames []string) []Session {
	live := make(map[string]bool, len(serverNames))
	for _, n := range serverNames {
		live[n] = true
	}
	out := make([]Session, len(sessions))
	for i, s := range sessions {
		out[i] = s
		if !s.Dead && !s.Attached && !live[s.Name] {
			out[i].Zombie = true
		}
	}
	return out
}

// parseServerNames は実行中プロセスのコマンドライン群から zellij server の
// セッション名を抽出する。`--server <path>` の <path> 末尾要素を名前とみなす。
// 注: パスにスペースを含む環境では Fields 分割で末尾が崩れうる（手動スモークで確認）。
func parseServerNames(cmdlines []string) []string {
	var names []string
	for _, line := range cmdlines {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "--server" && i+1 < len(fields) {
				p := strings.ReplaceAll(fields[i+1], "\\", "/")
				names = append(names, path.Base(p))
			}
		}
	}
	return names
}
