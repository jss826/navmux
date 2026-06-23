package app

import (
	"runtime/debug"
	"testing"
)

func TestFormatVersion_NoBuildInfo(t *testing.T) {
	if got := FormatVersion(nil, false); got != "navmux (バージョン情報なし)" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatVersion_TaggedNoVCS(t *testing.T) {
	bi := &debug.BuildInfo{}
	bi.Main.Version = "v1.2.3"
	if got := FormatVersion(bi, true); got != "navmux v1.2.3" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatVersion_EmptyVersionIsDevel(t *testing.T) {
	bi := &debug.BuildInfo{}
	bi.Main.Version = ""
	if got := FormatVersion(bi, true); got != "navmux (devel)" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatVersion_WithRevisionTruncated(t *testing.T) {
	bi := &debug.BuildInfo{}
	bi.Main.Version = "(devel)"
	bi.Settings = []debug.BuildSetting{
		{Key: "vcs.revision", Value: "0123456789abcdef0123"},
		{Key: "vcs.modified", Value: "false"},
	}
	if got := FormatVersion(bi, true); got != "navmux (devel) (0123456789ab)" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatVersion_DirtyRevision(t *testing.T) {
	bi := &debug.BuildInfo{}
	bi.Main.Version = "(devel)"
	bi.Settings = []debug.BuildSetting{
		{Key: "vcs.revision", Value: "abc123"},
		{Key: "vcs.modified", Value: "true"},
	}
	if got := FormatVersion(bi, true); got != "navmux (devel) (abc123-dirty)" {
		t.Fatalf("got %q", got)
	}
}

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
