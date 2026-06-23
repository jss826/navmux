// Package upgrade は navmux 自身を最新リリースへ更新する。
package upgrade

import (
	"bufio"
	"bytes"
	"encoding/json"
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

// Asset は Release に添付された 1 ファイル。
type Asset struct {
	Name string
	URL  string
}

// Release は GitHub の 1 リリース。
type Release struct {
	TagName string
	Assets  []Asset
}

// parseLatest は GitHub API releases/latest の JSON を Release に変換する。
func parseLatest(body []byte) (Release, error) {
	var raw struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Release{}, err
	}
	rel := Release{TagName: raw.TagName}
	for _, a := range raw.Assets {
		rel.Assets = append(rel.Assets, Asset{Name: a.Name, URL: a.URL})
	}
	return rel, nil
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

// assetFor は goos/goarch に対応するバイナリ資産を返す。
// 期待名は navmux_<goos>_<goarch>、windows のみ .exe を付与する。
func assetFor(assets []Asset, goos, goarch string) (Asset, bool) {
	name := "navmux_" + goos + "_" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	for _, a := range assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// checksumFor は SHA256SUMS（"<hex>␣␣<name>" 行）から assetName の hex を引く。
func checksumFor(sums []byte, assetName string) (string, bool) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[1] == assetName {
			return fields[0], true
		}
	}
	return "", false
}
