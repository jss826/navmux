# navmux リリースコマンド + `navmux upgrade` 実装プラン

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** タグ付き GitHub Release を切る `/release` スラッシュコマンドと、プレビルドバイナリを download・検証・自己置換する `navmux upgrade` サブコマンドを追加する。

**Architecture:** `/release`（Claude 実行のスラッシュコマンド）が tag push + 5 OS クロスコンパイル + `SHA256SUMS` + `gh release` を行い、`navmux upgrade`（Go サブコマンド）が GitHub API の latest release を参照して対応バイナリを download し、`minio/selfupdate` で checksum 検証込みに自己置換する。外部 I/O は注入可能にして純関数を単体テストする（CLAUDE.md の `runFunc` 流儀）。

**Tech Stack:** Go 1.25 / stdlib `net/http`・`encoding/json` / `github.com/minio/selfupdate` / `gh` CLI / Claude Code スラッシュコマンド

## Global Constraints

- module path: `github.com/jss826/navmux`、Go 1.25。
- 変更系 mux 操作は `Command{Argv, Display}` ビルダー + runner 一元化（本機能は mux 操作ではないので対象外だが、`exec.Command(bin, args...)` 直接実行・シェル非経由の原則は維持）。
- 品質ゲート: `go test ./...` 全 PASS / `go build ./...` exit 0 / `go vet ./...` 警告なし。
- 依存追加は `github.com/minio/selfupdate` のみ。GitHub API は stdlib で行う。
- バイナリ命名規約: `navmux_<goos>_<goarch>`、`goos=="windows"` のみ `.exe` 付与。`/release` の出力名と `assetFor` を厳密一致させる。
- リポジトリ: `jss826/navmux`。latest API URL は `https://api.github.com/repos/jss826/navmux/releases/latest`。
- この差分はリリース時に `/security-review` を実施する（外部入力・バイナリ置換面の追加）。

---

### Task 1: version.go に ldflags 注入版と `Version()` を追加

**Files:**
- Modify: `internal/app/version.go`
- Test: `internal/app/version_test.go`

**Interfaces:**
- Produces: `var version string`（`-ldflags -X github.com/jss826/navmux/internal/app.version=vX.Y.Z` で注入）、`func Version(bi *debug.BuildInfo, ok bool) string`（素のバージョン文字列）。`FormatVersion` は据え置きシグネチャ。

- [ ] **Step 1: 失敗テストを書く**

`internal/app/version_test.go` に追記:

```go
func TestVersion_InjectedTakesPriority(t *testing.T) {
	old := version
	version = "v0.2.0"
	defer func() { version = old }()

	bi := &debug.BuildInfo{}
	bi.Main.Version = "v9.9.9"
	if got := Version(bi, true); got != "v0.2.0" {
		t.Fatalf("注入版が優先されない: got %q", got)
	}
	if got := FormatVersion(bi, true); got != "navmux v0.2.0" {
		t.Fatalf("FormatVersion が注入版を使わない: got %q", got)
	}
}

func TestVersion_FallsBackToBuildInfo(t *testing.T) {
	old := version
	version = ""
	defer func() { version = old }()

	bi := &debug.BuildInfo{}
	bi.Main.Version = "v1.2.3"
	if got := Version(bi, true); got != "v1.2.3" {
		t.Fatalf("Main.Version へのフォールバック失敗: got %q", got)
	}
}

func TestVersion_DevelWhenUnknown(t *testing.T) {
	old := version
	version = ""
	defer func() { version = old }()

	if got := Version(nil, false); got != "(devel)" {
		t.Fatalf("不明時 (devel) でない: got %q", got)
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/app/ -run TestVersion_ -v`
Expected: FAIL（`version`/`Version` undefined のコンパイルエラー）

- [ ] **Step 3: 最小実装**

`internal/app/version.go` を次に置換:

```go
package app

import "runtime/debug"

// version はリリースビルドで -ldflags "-X .../internal/app.version=vX.Y.Z" により
// 注入される。go install 経由（注入なし）のときは空文字。
var version string

// Version は素のバージョン文字列を返す。注入版 → Main.Version → "(devel)" の順。
func Version(bi *debug.BuildInfo, ok bool) string {
	if version != "" {
		return version
	}
	if ok && bi != nil && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "(devel)"
}

// FormatVersion は -version 出力用の 1 行文字列を組む。
// bi は runtime/debug.ReadBuildInfo() の結果。
func FormatVersion(bi *debug.BuildInfo, ok bool) string {
	if version == "" && (!ok || bi == nil) {
		return "navmux (バージョン情報なし)"
	}
	out := "navmux " + Version(bi, ok)

	if ok && bi != nil {
		var revision, modified string
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				revision = s.Value
			case "vcs.modified":
				modified = s.Value
			}
		}
		if revision != "" {
			if len(revision) > 12 {
				revision = revision[:12]
			}
			out += " (" + revision
			if modified == "true" {
				out += "-dirty"
			}
			out += ")"
		}
	}
	return out
}
```

- [ ] **Step 4: テストを実行して緑を確認**

Run: `go test ./internal/app/ -v`
Expected: 既存 5 テスト + 新規 3 テスト 全 PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/version.go internal/app/version_test.go
git commit -m "feat: version に ldflags 注入版と Version() を追加（リリースバイナリ用）"
```

---

### Task 2: `internal/upgrade` の semver 比較 `isNewer`

**Files:**
- Create: `internal/upgrade/upgrade.go`
- Test: `internal/upgrade/upgrade_test.go`

**Interfaces:**
- Produces: `func isNewer(current, latest string) bool`（current が不明/不正なら true、latest が不正なら false。数値比較）。

- [ ] **Step 1: 失敗テストを書く**

`internal/upgrade/upgrade_test.go`:

```go
package upgrade

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		cur, latest string
		want        bool
	}{
		{"v0.1.0", "v0.2.0", true},
		{"v0.2.0", "v0.2.0", false},
		{"v0.2.1", "v0.2.0", false},
		{"v0.1.0", "v0.1.1", true},
		{"v0.9.0", "v0.10.0", true}, // 数値比較（辞書順なら誤判定）
		{"(devel)", "v0.1.0", true},
		{"", "v0.1.0", true},
		{"v0.1.0", "garbage", false}, // latest 不正は更新しない
	}
	for _, c := range cases {
		if got := isNewer(c.cur, c.latest); got != c.want {
			t.Errorf("isNewer(%q,%q)=%v want %v", c.cur, c.latest, got, c.want)
		}
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/upgrade/ -run TestIsNewer -v`
Expected: FAIL（`isNewer` undefined）

- [ ] **Step 3: 最小実装**

`internal/upgrade/upgrade.go`:

```go
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
```

- [ ] **Step 4: テストを実行して緑を確認**

Run: `go test ./internal/upgrade/ -run TestIsNewer -v`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/upgrade/upgrade.go internal/upgrade/upgrade_test.go
git commit -m "feat: upgrade に semver 比較 isNewer を追加"
```

---

### Task 3: GitHub Release JSON のパース `parseLatest`

**Files:**
- Modify: `internal/upgrade/upgrade.go`
- Test: `internal/upgrade/upgrade_test.go`

**Interfaces:**
- Produces: 型 `Asset{Name, URL string}` / `Release{TagName string, Assets []Asset}`、`func parseLatest(body []byte) (Release, error)`。

- [ ] **Step 1: 失敗テストを書く**

`internal/upgrade/upgrade_test.go` に追記:

```go
func TestParseLatest(t *testing.T) {
	body := []byte(`{
		"tag_name": "v0.2.0",
		"assets": [
			{"name": "navmux_linux_amd64", "browser_download_url": "https://x/navmux_linux_amd64"},
			{"name": "SHA256SUMS", "browser_download_url": "https://x/SHA256SUMS"}
		]
	}`)
	rel, err := parseLatest(body)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rel.TagName != "v0.2.0" {
		t.Fatalf("tag = %q", rel.TagName)
	}
	if len(rel.Assets) != 2 || rel.Assets[0].Name != "navmux_linux_amd64" ||
		rel.Assets[0].URL != "https://x/navmux_linux_amd64" {
		t.Fatalf("assets = %+v", rel.Assets)
	}
}

func TestParseLatest_Invalid(t *testing.T) {
	if _, err := parseLatest([]byte("not json")); err == nil {
		t.Fatal("不正 JSON でエラーにならない")
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/upgrade/ -run TestParseLatest -v`
Expected: FAIL（`parseLatest`/`Release`/`Asset` undefined）

- [ ] **Step 3: 最小実装**

`internal/upgrade/upgrade.go` に追記（import に `encoding/json` を追加）:

```go
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
```

- [ ] **Step 4: テストを実行して緑を確認**

Run: `go test ./internal/upgrade/ -run TestParseLatest -v`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/upgrade/upgrade.go internal/upgrade/upgrade_test.go
git commit -m "feat: upgrade に GitHub Release JSON パース parseLatest を追加"
```

---

### Task 4: 実行環境に対応する資産選択 `assetFor`

**Files:**
- Modify: `internal/upgrade/upgrade.go`
- Test: `internal/upgrade/upgrade_test.go`

**Interfaces:**
- Produces: `func assetFor(assets []Asset, goos, goarch string) (Asset, bool)`（期待名 `navmux_<goos>_<goarch>`、windows は `.exe` 付与）。

- [ ] **Step 1: 失敗テストを書く**

`internal/upgrade/upgrade_test.go` に追記:

```go
func TestAssetFor(t *testing.T) {
	assets := []Asset{
		{Name: "navmux_linux_amd64", URL: "u1"},
		{Name: "navmux_windows_amd64.exe", URL: "u2"},
		{Name: "SHA256SUMS", URL: "u3"},
	}
	if a, ok := assetFor(assets, "linux", "amd64"); !ok || a.URL != "u1" {
		t.Fatalf("linux/amd64: %+v ok=%v", a, ok)
	}
	if a, ok := assetFor(assets, "windows", "amd64"); !ok || a.URL != "u2" {
		t.Fatalf("windows/amd64 は .exe 付き名で一致すべき: %+v ok=%v", a, ok)
	}
	if _, ok := assetFor(assets, "darwin", "arm64"); ok {
		t.Fatal("非存在 asset で ok=true")
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/upgrade/ -run TestAssetFor -v`
Expected: FAIL（`assetFor` undefined）

- [ ] **Step 3: 最小実装**

`internal/upgrade/upgrade.go` に追記:

```go
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
```

- [ ] **Step 4: テストを実行して緑を確認**

Run: `go test ./internal/upgrade/ -run TestAssetFor -v`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/upgrade/upgrade.go internal/upgrade/upgrade_test.go
git commit -m "feat: upgrade に環境別資産選択 assetFor を追加"
```

---

### Task 5: `SHA256SUMS` からの hash 抽出 `checksumFor`

**Files:**
- Modify: `internal/upgrade/upgrade.go`
- Test: `internal/upgrade/upgrade_test.go`

**Interfaces:**
- Produces: `func checksumFor(sums []byte, assetName string) (string, bool)`（`sha256sum` 形式 `<hex>␣␣<name>` を行パース）。

- [ ] **Step 1: 失敗テストを書く**

`internal/upgrade/upgrade_test.go` に追記:

```go
func TestChecksumFor(t *testing.T) {
	sums := []byte(
		"aaaa1111  navmux_linux_amd64\n" +
			"bbbb2222  navmux_windows_amd64.exe\n")
	if h, ok := checksumFor(sums, "navmux_windows_amd64.exe"); !ok || h != "bbbb2222" {
		t.Fatalf("windows hash: %q ok=%v", h, ok)
	}
	if _, ok := checksumFor(sums, "navmux_darwin_arm64"); ok {
		t.Fatal("非存在で ok=true")
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/upgrade/ -run TestChecksumFor -v`
Expected: FAIL（`checksumFor` undefined）

- [ ] **Step 3: 最小実装**

`internal/upgrade/upgrade.go` に追記（import に `bufio`・`bytes` を追加）:

```go
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
```

- [ ] **Step 4: テストを実行して緑を確認**

Run: `go test ./internal/upgrade/ -run TestChecksumFor -v`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/upgrade/upgrade.go internal/upgrade/upgrade_test.go
git commit -m "feat: upgrade に SHA256SUMS 抽出 checksumFor を追加"
```

---

### Task 6: 統合 `Runner` + `Run`（注入 HTTP/Apply でテスト）

**Files:**
- Modify: `internal/upgrade/upgrade.go`、`go.mod`、`go.sum`
- Test: `internal/upgrade/upgrade_test.go`

**Interfaces:**
- Consumes: `parseLatest`, `assetFor`, `checksumFor`, `isNewer`。
- Produces: 型 `Runner{HTTPGet func(string)([]byte,error); Apply func(io.Reader,[]byte)error; GOOS,GOARCH,Current,APIURL string; Out io.Writer}`、`func (r Runner) Run() error`、`func NewRunner(current string) Runner`。

- [ ] **Step 1: minio/selfupdate を取得**

Run: `go get github.com/minio/selfupdate@latest && go mod tidy`
Expected: go.mod に `github.com/minio/selfupdate` が追加される（module proxy 失敗時は `GOPROXY=direct` で再取得）

- [ ] **Step 2: 失敗テストを書く**

`internal/upgrade/upgrade_test.go` に追記:

```go
import (
	"bytes"
	"io"
	"testing"
)

// fakeHTTP は URL→body のマップで HTTPGet を差し替える。
func fakeHTTP(m map[string][]byte) func(string) ([]byte, error) {
	return func(url string) ([]byte, error) {
		if b, ok := m[url]; ok {
			return b, nil
		}
		return nil, io.EOF
	}
}

func TestRun_UpToDate(t *testing.T) {
	api := "https://api/latest"
	body := []byte(`{"tag_name":"v0.1.0","assets":[]}`)
	applied := false
	r := Runner{
		HTTPGet: fakeHTTP(map[string][]byte{api: body}),
		Apply:   func(io.Reader, []byte) error { applied = true; return nil },
		GOOS:    "linux", GOARCH: "amd64",
		Current: "v0.1.0", APIURL: api, Out: io.Discard,
	}
	if err := r.Run(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if applied {
		t.Fatal("最新なのに Apply が呼ばれた")
	}
}

func TestRun_AppliesUpdate(t *testing.T) {
	api := "https://api/latest"
	body := []byte(`{"tag_name":"v0.2.0","assets":[
		{"name":"navmux_linux_amd64","browser_download_url":"https://x/bin"},
		{"name":"SHA256SUMS","browser_download_url":"https://x/sums"}
	]}`)
	sums := []byte("deadbeef  navmux_linux_amd64\n")
	r := Runner{
		HTTPGet: fakeHTTP(map[string][]byte{
			api:            body,
			"https://x/bin": []byte("BINARY"),
			"https://x/sums": sums,
		}),
		GOOS: "linux", GOARCH: "amd64",
		Current: "v0.1.0", APIURL: api, Out: io.Discard,
	}
	var gotChecksum []byte
	var gotBin []byte
	r.Apply = func(rd io.Reader, sum []byte) error {
		gotChecksum = sum
		gotBin, _ = io.ReadAll(rd)
		return nil
	}
	if err := r.Run(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(gotBin) != "BINARY" {
		t.Fatalf("Apply に渡ったバイナリ = %q", gotBin)
	}
	if string(gotChecksum) != string([]byte{0xde, 0xad, 0xbe, 0xef}) {
		t.Fatalf("checksum デコード不一致: %x", gotChecksum)
	}
}
```

- [ ] **Step 3: テストを実行して失敗を確認**

Run: `go test ./internal/upgrade/ -run TestRun -v`
Expected: FAIL（`Runner`/`Run` undefined）

- [ ] **Step 4: 最小実装**

`internal/upgrade/upgrade.go` に追記（import に `encoding/hex`・`fmt`・`io`・`net/http`・`os`・`runtime`・`time`・`github.com/minio/selfupdate` を追加）:

```go
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
	return io.ReadAll(resp.Body)
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
```

- [ ] **Step 5: テストを実行して緑を確認**

Run: `go test ./internal/upgrade/ -v`
Expected: 全 PASS（純関数 + Run 2 件）

- [ ] **Step 6: 品質ゲート**

Run: `go test ./... && go build ./... && go vet ./...`
Expected: 全 PASS / exit 0 / 警告なし

- [ ] **Step 7: コミット**

```bash
git add internal/upgrade/upgrade.go internal/upgrade/upgrade_test.go go.mod go.sum
git commit -m "feat: upgrade に統合 Runner.Run（minio/selfupdate で自己置換）を追加"
```

---

### Task 7: `navmux upgrade` サブコマンドを main に配線

**Files:**
- Modify: `cmd/navmux/main.go`

**Interfaces:**
- Consumes: `app.Version`, `upgrade.NewRunner`。

- [ ] **Step 1: 実装**

`cmd/navmux/main.go` の `flag.Parse()` 直後・`showVersion` 分岐の後に追記し、import に `github.com/jss826/navmux/internal/upgrade` を追加:

```go
	if flag.Arg(0) == "upgrade" {
		bi, ok := debug.ReadBuildInfo()
		if err := upgrade.NewRunner(app.Version(bi, ok)).Run(); err != nil {
			fmt.Fprintln(os.Stderr, "navmux upgrade:", err)
			os.Exit(1)
		}
		return
	}
```

完成形（参考）:

```go
func main() {
	showVersion := flag.Bool("version", false, "バージョンを表示して終了する")
	flag.Parse()
	if *showVersion {
		bi, ok := debug.ReadBuildInfo()
		fmt.Println(app.FormatVersion(bi, ok))
		return
	}

	if flag.Arg(0) == "upgrade" {
		bi, ok := debug.ReadBuildInfo()
		if err := upgrade.NewRunner(app.Version(bi, ok)).Run(); err != nil {
			fmt.Fprintln(os.Stderr, "navmux upgrade:", err)
			os.Exit(1)
		}
		return
	}

	current := env.CurrentMux(env.OSLookup)
	// ...（以降は既存のまま）
}
```

- [ ] **Step 2: ビルド・vet**

Run: `go build ./... && go vet ./...`
Expected: exit 0 / 警告なし

- [ ] **Step 3: 配線スモーク（ネットワーク到達確認）**

Run: `go run ./cmd/navmux upgrade`
Expected: 「最新です: ...」もしくは更新動作（v0.1.0 のローカル実行では `(devel)` 判定で更新を試みる可能性あり。ネットワーク経路と latest 参照が動くことだけ確認し、実際の置換は Task 9 後のリリース済み環境で検証）。

> 注: `go run` 由来は注入版なしのため `Current="(devel)"` となり `isNewer` が true を返す。置換まで走らせたくない場合はこのスモークを「latest JSON 取得が 200 で返る」ところまでに留める。実置換の検証はリリース後の実バイナリで行う。

- [ ] **Step 4: コミット**

```bash
git add cmd/navmux/main.go
git commit -m "feat: navmux upgrade サブコマンドを配線"
```

---

### Task 8: `/release` スラッシュコマンドを作成

**Files:**
- Create: `.claude/commands/release.md`

**Interfaces:** なし（Claude 実行のプロンプト）。

- [ ] **Step 1: コマンドを書く**

`.claude/commands/release.md`:

````markdown
---
description: タグ付き GitHub Release を切る（version 提案 + クロスコンパイル + checksum + gh release）
---

navmux のリリースを行う。引数 `$ARGUMENTS` にバージョン（例 `v0.2.0`）があれば優先、なければ提案する。

手順（各段で失敗したら中止して報告）:

1. **clean 確認**: `git status --short` が空でなければ中止。
2. **バージョン決定**:
   - 前タグ: `git describe --tags --abbrev=0`
   - それ以降のコミット: `git log --oneline <前タグ>..HEAD`
   - 判定: コミットに `BREAKING`/`!:` を含む→major / `feat:` あり→minor / `fix:` のみ→patch。
   - `$ARGUMENTS` 指定があればそれを採用。なければ提案を提示し承認を得る。
3. **タグ作成 + push**:
   - `git tag -a <ver> -m "release <ver>"`
   - `git push origin <ver>`
   - push が `communication with agent failed` で失敗したら 1Password SSH agent のアンロックを促す。
4. **クロスコンパイル**（5 ターゲット、各 ldflags でバージョン埋め込み）:
   ```
   GOOS=linux   GOARCH=amd64 go build -ldflags "-X github.com/jss826/navmux/internal/app.version=<ver>" -o dist/navmux_linux_amd64       ./cmd/navmux
   GOOS=linux   GOARCH=arm64 go build -ldflags "-X github.com/jss826/navmux/internal/app.version=<ver>" -o dist/navmux_linux_arm64       ./cmd/navmux
   GOOS=darwin  GOARCH=amd64 go build -ldflags "-X github.com/jss826/navmux/internal/app.version=<ver>" -o dist/navmux_darwin_amd64      ./cmd/navmux
   GOOS=darwin  GOARCH=arm64 go build -ldflags "-X github.com/jss826/navmux/internal/app.version=<ver>" -o dist/navmux_darwin_arm64      ./cmd/navmux
   GOOS=windows GOARCH=amd64 go build -ldflags "-X github.com/jss826/navmux/internal/app.version=<ver>" -o dist/navmux_windows_amd64.exe ./cmd/navmux
   ```
   （PowerShell では `$env:GOOS="linux"; $env:GOARCH="amd64"; go build ...` の形に読み替える）
5. **SHA256SUMS 生成**: `dist/` 内の全バイナリの SHA256 を `dist/SHA256SUMS` に `<hex>␣␣<basename>` 形式で出力（`navmux upgrade` の `checksumFor` が basename 一致で引く）。
6. **Release 作成**: `gh release create <ver> --generate-notes dist/navmux_linux_amd64 dist/navmux_linux_arm64 dist/navmux_darwin_amd64 dist/navmux_darwin_arm64 dist/navmux_windows_amd64.exe dist/SHA256SUMS`
7. **security-review**: 本リリースに upgrade 関連の差分が含まれる場合は `/security-review` を実施して報告する。
8. `dist/` は成果物なので `.gitignore` に含まれていることを確認（無ければ追加）。
````

- [ ] **Step 2: dist/ を gitignore に追加**

`.gitignore` に追記:

```
/dist/
```

- [ ] **Step 3: コミット**

```bash
git add .claude/commands/release.md .gitignore
git commit -m "feat: /release スラッシュコマンドを追加（tag+クロスコンパイル+gh release）"
```

---

### Task 9: README にアップグレード手順を追記

**Files:**
- Modify: `README.md`

**Interfaces:** なし。

- [ ] **Step 1: 実装**

`README.md` のインストール/アップグレード節に追記（既存の見出し構成に合わせる。該当節が無ければ「## アップグレード」を新設）:

```markdown
## アップグレード

ビルド済みバイナリを使っている場合は、navmux 自身で最新リリースに更新できる:

```
navmux upgrade
```

最新リリースを参照し、OS/CPU に合うバイナリを download・SHA256 検証して自己置換する（Go ツールチェーン不要）。
すでに最新なら何もしない。`go install` で入れた場合は従来どおり `go install github.com/jss826/navmux@latest` も使える。
```

- [ ] **Step 2: コミット**

```bash
git add README.md
git commit -m "docs: navmux upgrade のアップグレード手順を追記"
```

---

## 完了時チェック（品質ゲート + エビデンス）

- `go test ./...` 全 PASS / `go build ./...` exit 0 / `go vet ./...` 警告なし の stdout を引用して完了宣言する。
- `/security-review` を実施（upgrade による外部入力・バイナリ置換面の追加）。
- **手動スモーク（リリース後の実バイナリで）**: 旧バージョンの実バイナリで `navmux upgrade` を実行し、最新へ置換され `navmux -version` が新バージョンを表示することを確認（Windows の起動中 exe 置換含む）。`/release` は実際に v0.2.0 を切って end-to-end 検証する。

## Self-Review 結果

- **Spec coverage:** リリースコマンド=Task 8 / version 注入=Task 1 / isNewer・parseLatest・assetFor・checksumFor=Task 2-5 / Run 統合=Task 6 / main 配線=Task 7 / README=Task 9 / security-review=完了時チェック + Task 8 step 7。スコープ外（CI・--check・自動チェック）はタスク化せず＝意図どおり。
- **Placeholder scan:** プレースホルダなし。各ステップに実コード/実コマンドを記載。
- **Type consistency:** `Asset{Name,URL}`・`Release{TagName,Assets}`・`Runner{HTTPGet,Apply,GOOS,GOARCH,Current,APIURL,Out}`・`NewRunner(current)`・`app.Version(bi,ok)`・`isNewer/parseLatest/assetFor/checksumFor` のシグネチャはタスク間で一致。`assetFor`（環境別・.exe 付与）と `assetFor2`（名前完全一致・SHA256SUMS 用）を区別。バイナリ命名規約は Task 8 の出力名と Task 4/Task 6 の `assetFor`/`checksumFor` で厳密一致。
