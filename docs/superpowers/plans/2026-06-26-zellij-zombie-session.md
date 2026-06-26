# zellij ゾンビセッション検出 + ワンキー掃除 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `list-sessions` には生きて見えるのにサーバー死でアタッチがハングする zellij ゾンビセッションを、プロセス走査で確実検出して減光表示し、ソケット削除 + `delete-session` でワンキー掃除できるようにする。

**Architecture:** zellij backend の `List()` 後段に「実行中 zellij server プロセスのコマンドライン走査」を一段挟む。`--server …<path>\<name>` が実在しないセッションを `Session.Zombie=true` にする。判定本体は純関数（プロセス一覧を seam 注入してフェイク化）。OS 依存は薄い「コマンドライン一覧取得」層にビルドタグで閉じる。掃除はソケットを Go の `os.Remove` で直接消し（シェル非経由）、続けて既存 `KillCmd`（`delete-session -f`）を実行する。tmux は無関係（常に `Zombie=false`、`PurgeSocket` は no-op）。

**Tech Stack:** Go 1.25 / Bubble Tea / 標準ライブラリのみ（新規依存なし）。テストは `testing` + `io/fs`・`testing/fstest`。

## Global Constraints

- module: `github.com/jss826/navmux`、Go 1.25。
- 変更系は `Command{Argv, Display}` ビルダーで返し、実行は runner 経由（`internal/ui/exec.go`）。
- コマンド実行はシェルを介さず `exec.Command(bin, args...)` 直接。ソケット削除も `del`/`rm` を呼ばず `os.Remove`。
- `Command.Display` は `strings.Join(Argv, " ")`（`cmd()` ヘルパ）から導出。
- 純コンテンツ層（`RenderList`）はプレーン維持。装飾は `RenderFooter`/`RenderMenu` 側（可＝シアン / 不可＝減光、記号は使わない）。
- 品質ゲート: `go test ./...` 全 PASS / `go build ./...` exit 0 / `go vet ./...` 警告なし。

## File Structure

- `internal/backend/backend.go` … `Session.Zombie` 追加、`Backend` interface に `PurgeSocket(name string) error` 追加。
- `internal/backend/zombie.go`（新規）… OS 非依存の純関数（`parseServerNames` / `markZombies` / `validSessionName` / `findSocket`）。
- `internal/backend/zombie_test.go`（新規）… 上記純関数の単体テスト。
- `internal/backend/liveness_windows.go`（新規, `//go:build windows`）… `serverCommandLines()` / `socketRoot()`。
- `internal/backend/liveness_other.go`（新規, `//go:build !windows`）… 同上の非 Windows 実装。
- `internal/backend/zellij.go` … `procLister` seam、`List()` 後段でゾンビマーク、`PurgeSocket` 実装、`SessionOps` の Zombie 無効化。
- `internal/backend/tmux.go` … `PurgeSocket` no-op 実装。
- `internal/ui/render.go` … `RenderList` の Zombie 表示、`RenderFooter` に選択セッションの生死反映。
- `internal/ui/menu.go` … アタッチ可否ヘルパ `canAttach`、Dead/Zombie 時の Kill ラベル「掃除」。
- `internal/ui/model.go` … Kill 実行時に Dead/Zombie なら `PurgeSocket` 先行。
- `CLAUDE.md` … コマンド対応表に「掃除」行を追加。

---

### Task 1: ドキュメント正本（コマンド対応表）を先に更新

**Files:**
- Modify: `CLAUDE.md`（「コマンド対応表（正本）」の表）

**Interfaces:**
- Produces: 掃除操作の `Display` 規約（後続タスクの文字列と厳密一致させる基準）。

- [ ] **Step 1: コマンド対応表に「掃除」行を追加**

`CLAUDE.md` の表の「操作:他クライアント切断」行の直後に、以下の行を追加する:

```
| 掃除(ゾンビ/EXITED) | （対象外。tmux はゾンビ概念なし） | ソケットファイル削除(os.Remove) → zellij delete-session -f <name> |
```

さらに「zellij 0.44.3 の制約（レビュー観点）」の箇条書き末尾に追記:

```
- list-sessions はソケットの有無で生死判定するため、server が異常終了してソケット残骸が残ると EXITED が付かず「生きて見える」ゾンビになる → Session.Zombie で検出（実行中 server プロセスのコマンドライン走査で --server …\<name> 不在を判定）。掃除は PurgeSocket(os.Remove) → KillCmd(delete-session -f)。
```

- [ ] **Step 2: コミット**

```bash
git add CLAUDE.md
git commit -m "docs: コマンド対応表に zellij ゾンビ掃除を追加"
```

---

### Task 2: Session.Zombie とサーバー名抽出の純関数

**Files:**
- Modify: `internal/backend/backend.go`（`Session` に `Zombie` 追加）
- Create: `internal/backend/zombie.go`
- Create: `internal/backend/zombie_test.go`

**Interfaces:**
- Produces:
  - `Session.Zombie bool`
  - `func parseServerNames(cmdlines []string) []string` — 各コマンドライン文字列から `--server <path>` を探し、パス末尾（`/`・`\` 両対応）のセッション名を返す。

- [ ] **Step 1: 失敗するテストを書く**

`internal/backend/zombie_test.go`:

```go
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
```

- [ ] **Step 2: 赤を確認**

Run: `go test ./internal/backend/ -run TestParseServerNames`
Expected: FAIL（`undefined: parseServerNames`）

- [ ] **Step 3: 最小実装**

`internal/backend/backend.go` の `Session` に 1 フィールド追加:

```go
type Session struct {
	Name     string
	Attached bool // 今このセッションにアタッチ中か
	Windows  int  // tmux のみ。zellij は 0
	Dead     bool // zellij の EXITED セッション。tmux は常に false
	Zombie   bool // server 不在なのに list に生きて見える応答なし状態。tmux は常に false
}
```

`internal/backend/zombie.go` を新規作成:

```go
package backend

import (
	"path"
	"strings"
)

// parseServerNames は実行中プロセスのコマンドライン群から zellij server の
// セッション名を抽出する。`--server <path>` の <path> 末尾要素を名前とみなす。
// 注: パスにスペースを含む環境では Fields 分割で末尾が崩れうる（手動スモークで確認）。
func parseServerNames(cmdlines []string) []string {
	var names []string
	for _, line := range cmdlines {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "--server" && i+1 < len(fields) {
				p := strings.ReplaceAll(fields[i+1], "\\", "/")
				names = append(names, path.Base(p))
			}
		}
	}
	return names
}
```

- [ ] **Step 4: 緑を確認**

Run: `go test ./internal/backend/ -run TestParseServerNames`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/backend/backend.go internal/backend/zombie.go internal/backend/zombie_test.go
git commit -m "feat(backend): Session.Zombie と parseServerNames を追加"
```

---

### Task 3: ゾンビ判定純関数と List() への組み込み

**Files:**
- Modify: `internal/backend/zombie.go`（`markZombies` 追加）
- Modify: `internal/backend/zombie_test.go`（テスト追加）
- Modify: `internal/backend/zellij.go`（`procLister` seam、`List()` 後段）

**Interfaces:**
- Consumes: `parseServerNames`（Task 2）
- Produces:
  - `func markZombies(sessions []Session, serverNames []string) []Session`
  - `type procLister func() ([]string, error)`
  - `Zellij{run runFunc; proc procLister}`、`newZellijWithProc(run runFunc, proc procLister) *Zellij`

- [ ] **Step 1: 失敗するテストを書く**

`internal/backend/zombie_test.go` に追加:

```go
func TestMarkZombies(t *testing.T) {
	sessions := []Session{
		{Name: "nav", Attached: true},
		{Name: "work"},
		{Name: "den2"},
		{Name: "old", Dead: true},
	}
	got := markZombies(sessions, []string{"nav", "work"})
	want := []Session{
		{Name: "nav", Attached: true},
		{Name: "work"},
		{Name: "den2", Zombie: true},
		{Name: "old", Dead: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("markZombies = %+v, want %+v", got, want)
	}
}

func TestZellijListMarksZombie(t *testing.T) {
	run := func(args ...string) (string, error) {
		return "nav [Created 1m ago] (current)\nden2 [Created 8h ago]\n", nil
	}
	proc := func() ([]string, error) {
		return []string{`zellij --server /tmp/zellij/contract_version_1/nav`}, nil
	}
	z := newZellijWithProc(run, proc)
	got, err := z.List()
	if err != nil {
		t.Fatalf("List() err = %v", err)
	}
	want := []Session{
		{Name: "nav", Attached: true},
		{Name: "den2", Zombie: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %+v, want %+v", got, want)
	}
}
```

- [ ] **Step 2: 赤を確認**

Run: `go test ./internal/backend/ -run 'TestMarkZombies|TestZellijListMarksZombie'`
Expected: FAIL（`undefined: markZombies` / `undefined: newZellijWithProc`）

- [ ] **Step 3: 最小実装**

`internal/backend/zombie.go` に追加:

```go
// markZombies は server プロセス名集合に存在しない alive セッションを Zombie にする。
// Dead(EXITED) と Attached(current=自分が接続中＝生存) は対象外。
func markZombies(sessions []Session, serverNames []string) []Session {
	live := make(map[string]bool, len(serverNames))
	for _, n := range serverNames {
		live[n] = true
	}
	out := make([]Session, len(sessions))
	for i, s := range sessions {
		out[i] = s
		if !s.Dead && !s.Attached && !live[s.Name] {
			out[i].Zombie = true
		}
	}
	return out
}
```

`internal/backend/zellij.go` を変更。`procLister` 型と struct フィールドを追加し、`List()` 後段でマーク。`serverCommandLines` は Task 4 で定義するため、この Step では `NewZellij` の `proc` を `nil` のままにする:

```go
// procLister は実行中プロセスのコマンドライン一覧を返す。テストで差し替える。
type procLister func() ([]string, error)

// Zellij は zellij backend。
type Zellij struct {
	run  runFunc
	proc procLister
}

// NewZellij は実プロセスで動く zellij backend を返す（proc は Task 4 で注入）。
func NewZellij() *Zellij { return &Zellij{run: execRun(zellijBin), proc: nil} }

// newZellijWithRun はテスト用に runFunc を差し替える（proc は nil）。
func newZellijWithRun(run runFunc) *Zellij { return &Zellij{run: run} }

// newZellijWithProc はテスト用に run と proc を差し替える。
func newZellijWithProc(run runFunc, proc procLister) *Zellij { return &Zellij{run: run, proc: proc} }
```

`List()` の末尾を変更:

```go
func (z *Zellij) List() ([]Session, error) {
	out, err := z.run("list-sessions", "-n")
	if err != nil {
		if strings.Contains(out, "No active zellij sessions") {
			return nil, nil
		}
		return nil, err
	}
	sessions := parseZellijList(out)
	if z.proc != nil {
		if cmdlines, perr := z.proc(); perr == nil {
			sessions = markZombies(sessions, parseServerNames(cmdlines))
		}
	}
	return sessions, nil
}
```

- [ ] **Step 4: 緑を確認**

Run: `go test ./internal/backend/ -run 'TestMarkZombies|TestZellijListMarksZombie'`
Expected: PASS

- [ ] **Step 5: 全テストとビルド確認**

Run: `go test ./internal/backend/` then `go build ./...`
Expected: PASS / exit 0

- [ ] **Step 6: コミット**

```bash
git add internal/backend/zombie.go internal/backend/zombie_test.go internal/backend/zellij.go
git commit -m "feat(backend): markZombies と List() でのゾンビ判定（proc seam）"
```

---

### Task 4: OS 依存のプロセス取得とソケットルート

**Files:**
- Create: `internal/backend/liveness_windows.go`（`//go:build windows`）
- Create: `internal/backend/liveness_other.go`（`//go:build !windows`）
- Modify: `internal/backend/zellij.go`（`NewZellij` で `proc: serverCommandLines`）

**Interfaces:**
- Produces:
  - `func serverCommandLines() ([]string, error)`
  - `func socketRoot() string`

実プロセス/実 OS 依存のためユニットテストは設けず、`go build` と手動スモークで担保。解析責務は純関数 `parseServerNames`（Task 2）が持つため、この層は「行を集める」だけに限定。

- [ ] **Step 1: Windows 実装を書く**

`internal/backend/liveness_windows.go`:

```go
//go:build windows

package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// serverCommandLines は zellij.exe 系プロセスのコマンドラインを CIM 経由で集める（シェル非経由）。
func serverCommandLines() ([]string, error) {
	out, err := exec.Command(
		"powershell", "-NoProfile", "-NonInteractive", "-Command",
		`Get-CimInstance Win32_Process -Filter "Name='zellij.exe'" | ForEach-Object { $_.CommandLine }`,
	).Output()
	if err != nil {
		return nil, err
	}
	return splitNonEmptyLines(string(out)), nil
}

// socketRoot は Windows の zellij ソケット親ディレクトリ。
func socketRoot() string {
	return filepath.Join(os.TempDir(), "zellij")
}

func splitNonEmptyLines(s string) []string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
```

- [ ] **Step 2: 非 Windows 実装を書く**

`internal/backend/liveness_other.go`:

```go
//go:build !windows

package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// serverCommandLines は ps で全プロセスのコマンドラインを集める（シェル非経由）。
func serverCommandLines() ([]string, error) {
	out, err := exec.Command("ps", "-eo", "args=").Output()
	if err != nil {
		return nil, err
	}
	return splitNonEmptyLines(string(out)), nil
}

// socketRoot は zellij ソケット親ディレクトリ。Linux/macOS の実パスは zellij の
// バージョンで変わりうる（例 /tmp/zellij-<uid>）。実機スモークで要確認・調整。
func socketRoot() string {
	return filepath.Join(os.TempDir(), "zellij")
}

func splitNonEmptyLines(s string) []string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
```

- [ ] **Step 3: NewZellij を実 proc に戻す**

`internal/backend/zellij.go`:

```go
func NewZellij() *Zellij { return &Zellij{run: execRun(zellijBin), proc: serverCommandLines} }
```

- [ ] **Step 4: ビルドと vet**

Run: `go build ./...` then `go vet ./...`
Expected: exit 0 / 警告なし

（任意・クロス）Run: `GOOS=linux go build ./...`
Expected: exit 0

- [ ] **Step 5: コミット**

```bash
git add internal/backend/liveness_windows.go internal/backend/liveness_other.go internal/backend/zellij.go
git commit -m "feat(backend): server プロセス走査の OS 依存実装（windows/other）"
```

---

### Task 5: ソケット探索・名前検証と PurgeSocket

**Files:**
- Modify: `internal/backend/zombie.go`（`validSessionName` / `findSocket`）
- Modify: `internal/backend/zombie_test.go`（テスト）
- Modify: `internal/backend/backend.go`（interface に `PurgeSocket`）
- Modify: `internal/backend/zellij.go`（`PurgeSocket` 実装）
- Modify: `internal/backend/tmux.go`（`PurgeSocket` no-op）

**Interfaces:**
- Produces:
  - `func validSessionName(name string) bool`
  - `func findSocket(fsys fs.FS, name string) (string, bool)`
  - `Backend.PurgeSocket(name string) error`

- [ ] **Step 1: 失敗するテストを書く**

`internal/backend/zombie_test.go` に追加（import に `"testing/fstest"` を足す）:

```go
func TestValidSessionName(t *testing.T) {
	cases := map[string]bool{
		"nav": true, "den2": true,
		"": false, "..": false, "a/b": false, `a\b`: false, "a..b": false,
	}
	for name, want := range cases {
		if got := validSessionName(name); got != want {
			t.Fatalf("validSessionName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestFindSocket(t *testing.T) {
	fsys := fstest.MapFS{
		"contract_version_1/nav":  {Data: []byte("x")},
		"contract_version_1/den2": {Data: []byte("x")},
	}
	got, ok := findSocket(fsys, "den2")
	if !ok || got != "contract_version_1/den2" {
		t.Fatalf("findSocket den2 = %q,%v", got, ok)
	}
	if _, ok := findSocket(fsys, "missing"); ok {
		t.Fatal("findSocket missing は false のはず")
	}
}
```

- [ ] **Step 2: 赤を確認**

Run: `go test ./internal/backend/ -run 'TestValidSessionName|TestFindSocket'`
Expected: FAIL（`undefined: validSessionName` / `undefined: findSocket`）

- [ ] **Step 3: 最小実装**

`internal/backend/zombie.go` の import を `"io/fs"`・`"path"`・`"strings"` にし、追加:

```go
// validSessionName は os.Remove に渡す前のセッション名の防御的検証。
func validSessionName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return false
	}
	return true
}

// findSocket は fsys 配下を歩き、ベース名が name と完全一致する最初のファイルの相対パスを返す。
func findSocket(fsys fs.FS, name string) (string, bool) {
	var found string
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && path.Base(p) == name {
			found = p
			return fs.SkipAll
		}
		return nil
	})
	return found, found != ""
}
```

`internal/backend/backend.go` の `Backend` interface に追加（`CanRename()` の下）:

```go
	CanRename() bool

	// PurgeSocket は name のソケット残骸を削除する（ゾンビ/EXITED 掃除用）。残骸が無ければ nil。
	PurgeSocket(name string) error
```

`internal/backend/zellij.go` に実装（import に `"fmt"`・`"os"`・`"path/filepath"`）:

```go
func (z *Zellij) PurgeSocket(name string) error {
	if !validSessionName(name) {
		return fmt.Errorf("不正なセッション名: %q", name)
	}
	root := socketRoot()
	rel, ok := findSocket(os.DirFS(root), name)
	if !ok {
		return nil
	}
	return os.Remove(filepath.Join(root, filepath.FromSlash(rel)))
}
```

`internal/backend/tmux.go` に no-op:

```go
// PurgeSocket: tmux はソケット残骸ゾンビの概念を持たないため no-op。
func (t *Tmux) PurgeSocket(name string) error { return nil }
```

- [ ] **Step 4: 緑とビルド**

Run: `go test ./internal/backend/` then `go build ./...`
Expected: PASS / exit 0

- [ ] **Step 5: コミット**

```bash
git add internal/backend/zombie.go internal/backend/zombie_test.go internal/backend/backend.go internal/backend/zellij.go internal/backend/tmux.go
git commit -m "feat(backend): PurgeSocket（ソケット探索+os.Remove）と名前検証"
```

---

### Task 6: ゾンビの UI 表示とアタッチ/操作の無効化

**Files:**
- Modify: `internal/backend/zellij.go`（`SessionOps` の enabled を Zombie でも無効化）
- Modify: `internal/ui/render.go`（`RenderList` の Zombie 表示、`RenderFooter` のシグネチャ変更）
- Modify: `internal/ui/menu.go`（`canAttach`、buildMenu のアタッチ可否）
- Modify: `internal/ui/model.go`（`selectedSession`、`RenderFooter` 呼び出し）
- Modify: `internal/ui/render_test.go` / `internal/backend/zellij_test.go`（テスト）

**Interfaces:**
- Consumes: `Session.Zombie`
- Produces:
  - `func canAttach(s backend.Session) bool`（ui パッケージ）
  - `RenderFooter(actions []action.Action, b backend.Backend, sel backend.Session) string`（name → sel）
  - `func (m Model) selectedSession() backend.Session`

- [ ] **Step 1: 失敗するテストを書く**

`internal/backend/zellij_test.go` の `TestZellijSessionOps` に Zombie ケースを追加:

```go
	for _, o := range NewZellij().SessionOps(Session{Name: "foo", Zombie: true}) {
		if o.Enabled {
			t.Fatalf("Zombie で %s が有効になっている", o.Label)
		}
	}
```

`internal/ui/render_test.go` に追加:

```go
func TestRenderListZombie(t *testing.T) {
	out := RenderList([]backend.Session{{Name: "den2", Zombie: true}}, 0)
	if !strings.Contains(out, "den2") || !strings.Contains(out, "応答なし") {
		t.Fatalf("ゾンビ表示が無い: %q", out)
	}
}

func TestCanAttach(t *testing.T) {
	if canAttach(backend.Session{Name: "x", Zombie: true}) {
		t.Fatal("Zombie はアタッチ不可のはず")
	}
	if canAttach(backend.Session{Name: "x", Dead: true}) {
		t.Fatal("Dead はアタッチ不可のはず")
	}
	if !canAttach(backend.Session{Name: "x"}) {
		t.Fatal("生存はアタッチ可のはず")
	}
	if canAttach(backend.Session{Name: ""}) {
		t.Fatal("空名はアタッチ不可のはず")
	}
}
```

- [ ] **Step 2: 赤を確認**

Run: `go test ./internal/backend/ ./internal/ui/ -run 'TestZellijSessionOps|TestRenderListZombie|TestCanAttach'`
Expected: FAIL（`undefined: canAttach` ほか / Zombie で Enabled が true）

- [ ] **Step 3: 最小実装**

`internal/backend/zellij.go` の `SessionOps` の有効条件:

```go
	en := s.Name != "" && !s.Dead && !s.Zombie
```

`internal/ui/render.go` の `RenderList` extra 分岐に Zombie:

```go
		extra := ""
		if s.Dead {
			extra = " (EXITED)"
		} else if s.Zombie {
			extra = " (応答なし)"
		} else if s.Windows > 0 {
			extra = fmt.Sprintf(" [%d windows]", s.Windows)
		}
```

`RenderFooter` のシグネチャを sel に変更:

```go
func RenderFooter(actions []action.Action, b backend.Backend, sel backend.Session) string {
	var parts []string
	for _, a := range actions {
		label := fmt.Sprintf("%s %s", a.Key, a.Label)
		ok := action.Runnable(b, a.Kind, sel.Name)
		if a.Kind == action.Attach {
			ok = canAttach(sel)
		}
		if ok {
			parts = append(parts, runnableStyle.Render(label))
		} else {
			parts = append(parts, faintStyle.Render(label))
		}
	}
	for _, h := range []string{"←→ ペイン移動", "y コピー", "u 更新", "? 解説", "tab tmux/zellij", "q 終了"} {
		parts = append(parts, faintStyle.Render(h))
	}
	return strings.Join(parts, "   ")
}
```

`internal/ui/menu.go` に `canAttach` を追加し、buildMenu のアタッチ行を差し替え:

```go
// canAttach は s にアタッチ可能か（生存していて名前があるか）を返す。
func canAttach(s backend.Session) bool {
	return s.Name != "" && !s.Dead && !s.Zombie
}
```

```go
		{kind: kindAction, act: action.Attach, label: "アタッチ", display: b.AttachCmd(name).Display, enabled: canAttach(sel)},
```

`internal/ui/model.go` に `selectedSession` を追加し、`View()` の `RenderFooter` 呼び出しを変更:

```go
func (m Model) selectedSession() backend.Session {
	if m.cursor >= 0 && m.cursor < len(m.sessions) {
		return m.sessions[m.cursor]
	}
	return backend.Session{}
}
```

```go
		RenderFooter(action.All(), m.ActiveBackend(), m.selectedSession()),
```

- [ ] **Step 4: 緑とビルド（既存呼び出しの追従）**

`RenderFooter` の既存呼び出し・テストが name 文字列を渡している箇所はコンパイルエラーになる。`internal/ui/render_test.go` 等の該当箇所を `backend.Session{Name: "..."}` 形式に修正する。

Run: `go test ./...` then `go build ./...`
Expected: PASS / exit 0

- [ ] **Step 5: コミット**

```bash
git add internal/backend/zellij.go internal/backend/zellij_test.go internal/ui/render.go internal/ui/render_test.go internal/ui/menu.go internal/ui/model.go
git commit -m "feat(ui): ゾンビを (応答なし) 減光表示しアタッチ/操作を無効化"
```

---

### Task 7: ゾンビ/EXITED のワンキー掃除配線

**Files:**
- Modify: `internal/ui/menu.go`（Dead/Zombie 時の Kill ラベル「掃除」、display にソケット掃除明示）
- Modify: `internal/ui/model.go`（Kill 実行時に Dead/Zombie なら `PurgeSocket` 先行）
- Modify: `internal/ui/model_test.go`（テスト）

**Interfaces:**
- Consumes: `Backend.PurgeSocket`（Task 5）、`Session.Dead`/`Session.Zombie`、`selectedSession`（Task 6）
- Produces: `func (m Model) purgeIfDead(b backend.Backend, s backend.Session)`

- [ ] **Step 1: 失敗するテストを書く**

`internal/ui/model_test.go` に追加（埋め込みフェイクで `PurgeSocket` 呼び出しを記録）:

```go
type purgeSpyBackend struct {
	backend.Backend
	purged []string
}

func (b *purgeSpyBackend) PurgeSocket(name string) error {
	b.purged = append(b.purged, name)
	return nil
}

func TestKillZombiePurgesSocket(t *testing.T) {
	spy := &purgeSpyBackend{Backend: backend.NewZellij()}
	m := New([]backend.Backend{spy}, "zellij")
	m.sessions = []backend.Session{{Name: "den2", Zombie: true}}
	m.cursor = 0
	m.purgeIfDead(spy, m.selectedSession())
	if len(spy.purged) != 1 || spy.purged[0] != "den2" {
		t.Fatalf("PurgeSocket 呼び出し = %v, want [den2]", spy.purged)
	}
}

func TestKillAliveDoesNotPurge(t *testing.T) {
	spy := &purgeSpyBackend{Backend: backend.NewZellij()}
	m := New([]backend.Backend{spy}, "zellij")
	m.sessions = []backend.Session{{Name: "live"}}
	m.cursor = 0
	m.purgeIfDead(spy, m.selectedSession())
	if len(spy.purged) != 0 {
		t.Fatalf("生存で PurgeSocket が呼ばれた: %v", spy.purged)
	}
}
```

- [ ] **Step 2: 赤を確認**

Run: `go test ./internal/ui/ -run 'TestKillZombiePurgesSocket|TestKillAliveDoesNotPurge'`
Expected: FAIL（`undefined: m.purgeIfDead`）

- [ ] **Step 3: 最小実装**

`internal/ui/model.go` にヘルパを追加:

```go
// purgeIfDead は対象が Dead/Zombie のときソケット残骸を掃除する（ベストエフォート）。
// 生存セッションのソケットは消さない（稼働中サーバーを壊さない）。
func (m Model) purgeIfDead(b backend.Backend, s backend.Session) {
	if s.Dead || s.Zombie {
		_ = b.PurgeSocket(s.Name)
	}
}
```

`runOp` の `action.Kill` ケースで掃除を挟む:

```go
func (m Model) runOp(k action.Kind, arg string) tea.Cmd {
	b := m.ActiveBackend()
	if k == action.New {
		return m.newSessionCmd(b, arg)
	}
	sel := m.selectedName()
	selSession := m.selectedSession()
	return func() tea.Msg {
		var c backend.Command
		switch k {
		case action.Rename:
			rc, ok := b.RenameCmd(sel, arg)
			if !ok {
				return opDoneMsg{err: backend.ErrUnsupported}
			}
			c = rc
		case action.Kill:
			m.purgeIfDead(b, selSession)
			c = b.KillCmd(sel)
		default:
			return opDoneMsg{}
		}
		_, err := runCommand(c)
		return opDoneMsg{err: err}
	}
}
```

`internal/ui/menu.go` の buildMenu で Dead/Zombie 時に Kill 行を「掃除」化:

```go
	killLabel := "削除"
	killDisplay := b.KillCmd(name).Display
	if sel.Dead || sel.Zombie {
		killLabel = "掃除"
		killDisplay = "ソケット削除 + " + killDisplay
	}
	items := []menuItem{
		{kind: kindAction, act: action.Attach, label: "アタッチ", display: b.AttachCmd(name).Display, enabled: canAttach(sel)},
		{kind: kindAction, act: action.New, label: "新規セッション", display: b.NewCmd("<name>").Display, enabled: action.Runnable(b, action.New, name)},
		{kind: kindAction, act: action.Rename, label: "リネーム", enabled: action.Runnable(b, action.Rename, name)},
		{kind: kindAction, act: action.Kill, label: killLabel, display: killDisplay, enabled: action.Runnable(b, action.Kill, name)},
		{kind: kindSep, label: "── 操作 ──"},
	}
```

- [ ] **Step 4: 緑と全ゲート**

Run: `go test ./...` then `go build ./...` then `go vet ./...`
Expected: PASS / exit 0 / 警告なし

- [ ] **Step 5: コミット**

```bash
git add internal/ui/menu.go internal/ui/model.go internal/ui/model_test.go
git commit -m "feat(ui): ゾンビ/EXITED の削除をソケット掃除付き『掃除』に"
```

---

## 手動スモーク（実装後、ユーザーの TTY で）

`go build -o navmux.exe ./cmd/navmux` 後、`navmux.exe` を実端末で起動:

1. ゾンビを意図的に作る（`den` でセッション生成 → server を kill / ソケット残骸を残す）。
2. navmux 一覧で当該が `(応答なし)` 減光表示になり、アタッチが無効（減光）か。
3. その項目で「掃除」を実行し、ソケットが消えて一覧から消えるか。
4. 生存セッションが誤って `(応答なし)` にならないか（誤検出無し）。
5. Linux 実機（別途）で `socketRoot()` の実パスを確認し、必要なら調整。

---

## Self-Review

- **Spec coverage:** 検出（Task 2-4）/ UI 減光・アタッチ無効（Task 6）/ ワンキー掃除（Task 5,7）/ tmux 無関係（Task 5 no-op）/ コマンド対応表（Task 1）/ セキュリティ＝名前検証（Task 5 `validSessionName`）— spec 各節に対応タスクあり。
- **Placeholder scan:** 各 Step に実コード・実コマンド・期待結果。TBD/TODO なし（Linux `socketRoot` 実パス確認のみ手動スモークに明示委譲）。
- **Type consistency:** `Session.Zombie` / `procLister` / `newZellijWithProc` / `PurgeSocket(name) error` / `canAttach(Session) bool` / `RenderFooter(..., sel Session)` / `selectedSession()` / `purgeIfDead(b, s)` は定義タスクと利用タスクで一致。
- **既存呼び出しの追従:** `RenderFooter` のシグネチャ変更（name→sel）に伴う既存テスト/呼び出しの修正を Task 6 Step 4 に明示。
