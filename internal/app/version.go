package app

import "runtime/debug"

// FormatVersion は -version 出力用の 1 行文字列を組む。
// bi は runtime/debug.ReadBuildInfo() の結果。go install で付くモジュール
// バージョン、source ビルドで付く VCS リビジョン（短縮 12 桁 + dirty）を反映する。
func FormatVersion(bi *debug.BuildInfo, ok bool) string {
	if !ok || bi == nil {
		return "navmux (バージョン情報なし)"
	}
	version := bi.Main.Version
	if version == "" {
		version = "(devel)"
	}
	out := "navmux " + version

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
	return out
}
