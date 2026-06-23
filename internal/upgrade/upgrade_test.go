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
