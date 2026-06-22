// Package app は navmux の起動配線（backend 選択など）を担う。
package app

import "github.com/jss826/navmux/internal/backend"

// OrderedBackends は all のうち Available() な backend だけを残し、
// current に一致するものを先頭へ移動して返す（残りは入力順を保つ）。
// current が空・未一致・利用不可の場合は単に利用可能分を入力順で返す。
func OrderedBackends(all []backend.Backend, current string) []backend.Backend {
	avail := make([]backend.Backend, 0, len(all))
	for _, b := range all {
		if b.Available() {
			avail = append(avail, b)
		}
	}

	for i, b := range avail {
		if b.Name() == current {
			ordered := make([]backend.Backend, 0, len(avail))
			ordered = append(ordered, b)
			ordered = append(ordered, avail[:i]...)
			ordered = append(ordered, avail[i+1:]...)
			return ordered
		}
	}
	return avail
}
