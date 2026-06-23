package upgrade

import "testing"

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
