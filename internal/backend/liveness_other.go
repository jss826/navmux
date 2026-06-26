//go:build !windows

package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// serverCommandLines は ps で全プロセスのコマンドラインを集める（シェル非経由）。
func serverCommandLines() ([]string, error) {
	out, err := exec.Command("ps", "-eo", "args=").Output()
	if err != nil {
		return nil, err
	}
	return splitNonEmptyLines(string(out)), nil
}

// socketRoot は zellij ソケット親ディレクトリ。Linux/macOS の実パスは zellij の
// バージョンで変わりうる（例 /tmp/zellij-<uid>）。実機スモークで要確認・調整。
func socketRoot() string {
	return filepath.Join(os.TempDir(), "zellij")
}

func splitNonEmptyLines(s string) []string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
