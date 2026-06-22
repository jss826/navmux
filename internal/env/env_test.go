package env

import "testing"

func lookupFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestCurrentMux(t *testing.T) {
	cases := []struct {
		name string
		envv map[string]string
		want string
	}{
		{"zellij 内", map[string]string{"ZELLIJ": "0"}, "zellij"},
		{"tmux 内", map[string]string{"TMUX": "/tmp/tmux-1000/default,123,0"}, "tmux"},
		{"両方なら zellij 優先", map[string]string{"ZELLIJ": "0", "TMUX": "x"}, "zellij"},
		{"外", map[string]string{}, ""},
	}
	for _, c := range cases {
		if got := CurrentMux(lookupFrom(c.envv)); got != c.want {
			t.Fatalf("%s: CurrentMux = %q, want %q", c.name, got, c.want)
		}
	}
}
