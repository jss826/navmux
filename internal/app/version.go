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
