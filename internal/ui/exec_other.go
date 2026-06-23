//go:build !windows

package ui

import "github.com/jss826/navmux/internal/backend"

// 非 Windows では background が自前で daemonize するため従来実行でよい。
func spawnDetached(c backend.Command) error {
	_, err := runCommand(c)
	return err
}
