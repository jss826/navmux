package backend

import (
	"path"
	"strings"
)

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
