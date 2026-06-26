//go:build windows

package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// serverCommandLines は zellij.exe 系プロセスのコマンドラインを CIM 経由で集める（シェル非経由）。
func serverCommandLines() ([]string, error) {
	out, err := exec.Command(
		"powershell", "-NoProfile", "-NonInteractive", "-Command",
		`Get-CimInstance Win32_Process -Filter "Name='zellij.exe'" | ForEach-Object { $_.CommandLine }`,
	).Output()
	if err != nil {
		return nil, err
	}
	return splitNonEmptyLines(string(out)), nil
}

// socketRoot は Windows の zellij ソケット親ディレクトリ。
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
