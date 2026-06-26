package backend

import (
	"reflect"
	"testing"
)

func TestParseServerNames(t *testing.T) {
	in := []string{
		`C:\Users\soon7\AppData\Local\Zellij\zellij.exe --server C:\Users\soon7\AppData\Local\Temp\zellij\contract_version_1\nav`,
		`/usr/bin/zellij --server /tmp/zellij-1000/0.44.3/work`,
		`zellij.exe attach -c den2`,
	}
	got := parseServerNames(in)
	want := []string{"nav", "work"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseServerNames = %v, want %v", got, want)
	}
}
