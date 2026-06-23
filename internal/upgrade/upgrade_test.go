package upgrade

import (
	"io"
	"testing"
)

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
			api:             body,
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
