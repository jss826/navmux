package env

import "os"

// CurrentMux は今いる multiplexer を返す（"tmux"/"zellij"/""）。
// 両方の環境変数があるときは zellij を優先する。
func CurrentMux(lookup func(string) string) string {
	if lookup("ZELLIJ") != "" {
		return "zellij"
	}
	if lookup("TMUX") != "" {
		return "tmux"
	}
	return ""
}

// OSLookup は本番用の環境変数 lookup。
func OSLookup(key string) string { return os.Getenv(key) }
