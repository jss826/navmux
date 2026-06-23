// Package upgrade は navmux 自身を最新リリースへ更新する。
package upgrade

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/minio/selfupdate"
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

// checksumFor は SHA256SUMS から assetName の hex を引く。
// text モード "<hex>␣␣<name>" と binary モード "<hex>␣*<name>" の両方を受ける
// （GNU sha256sum は Windows 既定が binary モードで name に '*' が付くため）。
func checksumFor(sums []byte, assetName string) (string, bool) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == assetName {
			return fields[0], true
		}
	}
	return "", false
}

// Runner は upgrade の実行コンテキスト。外部 I/O は注入してテスト可能にする。
type Runner struct {
	HTTPGet func(url string) ([]byte, error)
	Apply   func(r io.Reader, checksum []byte) error
	GOOS    string
	GOARCH  string
	Current string
	APIURL  string
	Out     io.Writer
}

// NewRunner は本番用の Runner を組む。
func NewRunner(current string) Runner {
	return Runner{
		HTTPGet: httpGet,
		Apply: func(r io.Reader, checksum []byte) error {
			return selfupdate.Apply(r, selfupdate.Options{Checksum: checksum})
		},
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
		Current: current,
		APIURL:  "https://api.github.com/repos/jss826/navmux/releases/latest",
		Out:     os.Stdout,
	}
}

const maxDownloadBytes = 100 << 20 // 100 MiB: release binaries are ~10–15 MiB

// readCapped reads from r up to max bytes. If the source has more than max
// bytes, it returns an error instead of buffering an unbounded amount.
func readCapped(r io.Reader, max int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("レスポンスが上限 %d バイトを超えました", max)
	}
	return b, nil
}

// requireHTTPS は URL のスキームが https:// であることを検証する。
func requireHTTPS(url string) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("資産 URL が https ではありません: %s", url)
	}
	return nil
}

func httpGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "navmux-upgrade")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	return readCapped(resp.Body, maxDownloadBytes)
}

// Run は latest を参照し、必要なら download→検証→自己置換する。
func (r Runner) Run() error {
	body, err := r.HTTPGet(r.APIURL)
	if err != nil {
		return fmt.Errorf("リリース情報の取得に失敗: %w", err)
	}
	rel, err := parseLatest(body)
	if err != nil {
		return fmt.Errorf("リリース情報の解析に失敗: %w", err)
	}
	if !isNewer(r.Current, rel.TagName) {
		fmt.Fprintf(r.Out, "最新です: %s\n", r.Current)
		return nil
	}

	asset, ok := assetFor(rel.Assets, r.GOOS, r.GOARCH)
	if !ok {
		return fmt.Errorf("%s/%s 用のバイナリが %s に見つかりません", r.GOOS, r.GOARCH, rel.TagName)
	}
	sumsAsset, ok := assetFor2(rel.Assets, "SHA256SUMS")
	if !ok {
		return fmt.Errorf("SHA256SUMS が %s に見つかりません", rel.TagName)
	}
	if err := requireHTTPS(sumsAsset.URL); err != nil {
		return err
	}
	sums, err := r.HTTPGet(sumsAsset.URL)
	if err != nil {
		return fmt.Errorf("SHA256SUMS の取得に失敗: %w", err)
	}
	hexsum, ok := checksumFor(sums, asset.Name)
	if !ok {
		return fmt.Errorf("%s の checksum が見つかりません", asset.Name)
	}
	checksum, err := hex.DecodeString(hexsum)
	if err != nil {
		return fmt.Errorf("checksum のデコードに失敗: %w", err)
	}

	if err := requireHTTPS(asset.URL); err != nil {
		return err
	}
	bin, err := r.HTTPGet(asset.URL)
	if err != nil {
		return fmt.Errorf("バイナリの取得に失敗: %w", err)
	}
	if err := r.Apply(bytes.NewReader(bin), checksum); err != nil {
		return fmt.Errorf("自己置換に失敗: %w", err)
	}
	fmt.Fprintf(r.Out, "更新しました: %s → %s\n", r.Current, rel.TagName)
	return nil
}

// assetFor2 は名前完全一致で資産を引く（SHA256SUMS 用）。
func assetFor2(assets []Asset, name string) (Asset, bool) {
	for _, a := range assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}
