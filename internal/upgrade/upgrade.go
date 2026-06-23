// Package upgrade は navmux 自身を最新リリースへ更新する。
package upgrade

import (
	"strconv"
	"strings"
)

// parseSemver は "vX.Y.Z" を [3]int に分解する。先頭 v は任意。
func parseSemver(s string) ([3]int, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		// "-rc1" 等のサフィックスは数値部だけ採る
		num := p
		if j := strings.IndexFunc(p, func(r rune) bool { return r < '0' || r > '9' }); j >= 0 {
			num = p[:j]
		}
		n, err := strconv.Atoi(num)
		if err != nil {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

// isNewer は latest が current より新しいかを返す。
// current が解釈不能（(devel)/空/不正）なら更新可とみなす。latest が不正なら false。
func isNewer(current, latest string) bool {
	lv, ok := parseSemver(latest)
	if !ok {
		return false
	}
	cv, ok := parseSemver(current)
	if !ok {
		return true
	}
	for i := 0; i < 3; i++ {
		if lv[i] != cv[i] {
			return lv[i] > cv[i]
		}
	}
	return false
}
