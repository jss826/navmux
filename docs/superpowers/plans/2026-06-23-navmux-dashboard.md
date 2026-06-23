# navmux 常駐ナビ化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** navmux を 2 ペインのダッシュボードに進化させ、メニュー選択で mux 操作を選択中セッションに実行できるようにし、zellij 新規生成の誤「完了」バグを修正する。

**Architecture:** 既存の「変更系は `Command{Argv, Display}` ビルダーを返し runner で実行」規律を維持。操作は backend が `SessionOps` で出すリストを UI が描画・実行するだけ（UI は具体 mux を知らない）。render は純コンテンツ層（テスト対象）と lipgloss 装飾層（スモークのみ）に分離。セッション生成だけは Windows でコンソール割当が要るため専用経路 `spawnDetached` ＋実在確認で実行する。

**Tech Stack:** Go 1.25, Bubble Tea (bubbletea / bubbles/textinput), lipgloss（新規導入）, atotto/clipboard。

## Global Constraints

- module: `github.com/jss826/navmux`、Go 1.25。
- 変更系は実行せず `Command{Argv, Display}` を返すビルダーに統一。`Display = strings.Join(Argv, " ")`（`cmd()` ヘルパ）。
- シェルを介さず `exec.Command(bin, args...)` で直接実行（シェルインジェクションを発生させない）。
- 純コンテンツ層の描画はプレーン文字列（テストが substring 一致）。lipgloss 装飾は装飾層に隔離し純層を汚さない。
- 品質ゲート（各タスクの最後 / 完了前に全実行）: `go test ./...` 全 PASS、`go build ./...` exit 0、`go vet ./...` 警告なし。
- コミットメッセージは `feat:` / `fix:` / `docs:` / `chore:` プレフィックス（日本語可）。末尾に `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`。
- push は明示指示があるまで行わない。ブランチは main 直接（ユーザー選択済み）。
- コマンドに触れる変更は CLAUDE.md「コマンド対応表」を先に更新し `Display` と厳密一致させる。

---

## Phase 1: 新規生成バグ修正（独立して出荷可能）

### Task 1: `containsSession` 純関数

**Files:**
- Modify: `internal/ui/exec.go`
- Test: `internal/ui/exec_test.go`（新規）

**Interfaces:**
- Produces: `func containsSession(sessions []backend.Session, name string) bool`

- [ ] **Step 1: 失敗テストを書く**

`internal/ui/exec_test.go`:
```go
package ui

import (
	"testing"

	"github.com/jss826/navmux/internal/backend"
)

func TestContainsSession(t *testing.T) {
	ss := []backend.Session{{Name: "a"}, {Name: "b"}}
	if !containsSession(ss, "b") {
		t.Fatal("b が含まれるはず")
	}
	if containsSession(ss, "x") {
		t.Fatal("x は含まれないはず")
	}
	if containsSession(nil, "a") {
		t.Fatal("nil で false のはず")
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/ui/ -run TestContainsSession`
Expected: FAIL（`undefined: containsSession`）

- [ ] **Step 3: 最小実装**

`internal/ui/exec.go` の末尾に追加:
```go
// containsSession は name のセッションが一覧に存在するか返す。
func containsSession(sessions []backend.Session, name string) bool {
	for _, s := range sessions {
		if s.Name == name {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: テストを実行して PASS を確認**

Run: `go test ./internal/ui/ -run TestContainsSession`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/ui/exec.go internal/ui/exec_test.go
git commit -m "feat: containsSession（一覧に対象セッションが在るか判定する純関数）"
```

---

### Task 2: `spawnDetached` ＋ `newSession`（生成の実在確認）と runOp 配線

**Files:**
- Modify: `internal/ui/exec.go`
- Create: `internal/ui/exec_windows.go`
- Create: `internal/ui/exec_other.go`
- Modify: `internal/ui/model.go:189-211`（`runOp` の `action.New` 分岐）

**Interfaces:**
- Consumes: `containsSession`（Task 1）, `backend.Backend.List()`, `backend.Command`
- Produces:
  - `func spawnDetached(c backend.Command) error`（OS 別実装）
  - `func newSession(b backend.Backend, c backend.Command, name string) error`
  - `var errNotCreated error`

- [ ] **Step 1: OS 別 `spawnDetached` を作る（Windows）**

`internal/ui/exec_windows.go`:
```go
//go:build windows

package ui

import (
	"os/exec"
	"syscall"

	"github.com/jss826/navmux/internal/backend"
)

// CREATE_NEW_CONSOLE: 子プロセスに新しいコンソールを割り当てる。
// zellij の detached セッションは中のシェルが生存するためにコンソールを要するため、
// パイプ実行（CombinedOutput）では即終了してしまう。新コンソールを与えて投げっぱなしにする。
const createNewConsole = 0x00000010

func spawnDetached(c backend.Command) error {
	if len(c.Argv) == 0 {
		return nil
	}
	cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole, HideWindow: true}
	return cmd.Start() // Wait しない（独立プロセスとして生かす）
}
```

- [ ] **Step 2: OS 別 `spawnDetached` を作る（非 Windows）**

`internal/ui/exec_other.go`:
```go
//go:build !windows

package ui

import "github.com/jss826/navmux/internal/backend"

// 非 Windows では background が自前で daemonize するため従来実行でよい。
func spawnDetached(c backend.Command) error {
	_, err := runCommand(c)
	return err
}
```

- [ ] **Step 3: `newSession` を追加**

`internal/ui/exec.go`：先頭の import を以下に更新し、関数を追加する。
```go
import (
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/jss826/navmux/internal/backend"
)
```
```go
// errNotCreated は生成コマンドが exit 0 でも実在確認に失敗したときに返す。
var errNotCreated = errors.New("作成に失敗した可能性があります（一覧に現れませんでした）")

// newSession はセッション生成を spawnDetached で実行し、List() に現れるまで
// 短くポーリングして実在を確認する。exit 0 を信用しない。
func newSession(b backend.Backend, c backend.Command, name string) error {
	if err := spawnDetached(c); err != nil {
		return err
	}
	for i := 0; i < 15; i++ {
		if ss, err := b.List(); err == nil && containsSession(ss, name) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errNotCreated
}
```

- [ ] **Step 4: `runOp` の New 分岐を `newSession` 経路へ**

`internal/ui/model.go` の `runOp` 内、`case action.New:` を以下に置換:
```go
		case action.New:
			return opDoneMsg{err: newSession(b, b.NewCmd(arg), arg)}
```
（他の分岐 Rename/Kill と末尾の `runCommand(c)` はそのまま。New は early return する。）

- [ ] **Step 5: ビルドとテスト（全 OS でコンパイルできること）**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: build exit 0 / vet 警告なし / 既存テスト含め全 PASS

- [ ] **Step 6: zellij 実機スモーク（ユーザーのターミナルで観察）**

navmux を zellij 0.44.3 内で起動 → `n` で新規セッション作成 → 「完了」表示後に一覧へ新セッションが**実際に出る**こと、`zellij list-sessions -n` でも残存することを確認する。出ない場合は「作成に失敗した可能性があります」が出ること。

- [ ] **Step 7: コミット**

```bash
git add internal/ui/exec.go internal/ui/exec_windows.go internal/ui/exec_other.go internal/ui/model.go
git commit -m "fix: 新規セッション生成で新コンソール割当+実在確認（誤「完了」を解消）"
```

---

## Phase 2: backend に mux 操作ビルダーを追加

### Task 3: CLAUDE.md コマンド対応表に操作行を追記

**Files:**
- Modify: `CLAUDE.md`（「コマンド対応表（正本）」の表）

- [ ] **Step 1: 表に操作行を追記**

CLAUDE.md の対応表の末尾（切替の行の下）に以下を追加する:
```markdown
| 操作:新規ウィンドウ/タブ | `tmux new-window -t <name>` | `zellij -s <name> action new-tab` |
| 操作:分割(縦) | `tmux split-window -h -t <name>` | `zellij -s <name> action new-pane -d right` |
| 操作:分割(横) | `tmux split-window -v -t <name>` | `zellij -s <name> action new-pane -d down` |
| 操作:次ウィンドウ/タブ | `tmux next-window -t <name>` | `zellij -s <name> action go-to-next-tab` |
| 操作:閉じる | `tmux kill-window -t <name>` | `zellij -s <name> action close-pane` |
```

- [ ] **Step 2: コミット**

```bash
git add CLAUDE.md
git commit -m "docs: コマンド対応表に mux 操作（new-window/split/next/close 等）を追記"
```

---

### Task 4: `OpPreset` 型 + interface 拡張 + Tmux `SessionOps`

**Files:**
- Modify: `internal/backend/backend.go`（`OpPreset` 型と interface メソッド追加）
- Modify: `internal/backend/tmux.go`（`SessionOps` 実装）
- Test: `internal/backend/tmux_test.go`

**Interfaces:**
- Produces:
  - `type OpPreset struct { Label string; Command Command; Enabled bool }`
  - `Backend.SessionOps(s Session) []OpPreset`（interface に追加）
  - `func (t *Tmux) SessionOps(s Session) []OpPreset`

- [ ] **Step 1: 失敗テストを書く**

`internal/backend/tmux_test.go` に追加:
```go
func TestTmuxSessionOps(t *testing.T) {
	ops := NewTmux().SessionOps(Session{Name: "foo"})
	want := map[string]string{
		"新規ウィンドウ": "tmux new-window -t foo",
		"分割(縦)":   "tmux split-window -h -t foo",
		"分割(横)":   "tmux split-window -v -t foo",
		"次ウィンドウ":  "tmux next-window -t foo",
		"閉じる":     "tmux kill-window -t foo",
	}
	got := map[string]string{}
	for _, o := range ops {
		got[o.Label] = o.Command.Display
		if !o.Enabled {
			t.Fatalf("%s が無効になっている", o.Label)
		}
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: got %q want %q", k, got[k], v)
		}
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/backend/ -run TestTmuxSessionOps`
Expected: FAIL（`OpPreset` / `SessionOps` 未定義でコンパイルエラー）

- [ ] **Step 3: 型と interface を追加**

`internal/backend/backend.go`：`Command` 型定義の下に追加:
```go
// OpPreset は右ペインに並べる 1 つの mux 操作。
type OpPreset struct {
	Label   string  // 例 "分割(縦)"
	Command Command // 実行/表示用
	Enabled bool    // false ならグレーアウト（実行不可）
}
```
`Backend` interface に 1 行追加（`CanRename() bool` の上）:
```go
	// SessionOps は対象セッションに実行できる mux 操作の一覧（backend 固有）。
	SessionOps(s Session) []OpPreset
```

- [ ] **Step 4: Tmux に実装**

`internal/backend/tmux.go`：`CanRename` の下に追加:
```go
func (t *Tmux) SessionOps(s Session) []OpPreset {
	en := s.Name != ""
	n := s.Name
	return []OpPreset{
		{Label: "新規ウィンドウ", Command: cmd(tmuxBin, "new-window", "-t", n), Enabled: en},
		{Label: "分割(縦)", Command: cmd(tmuxBin, "split-window", "-h", "-t", n), Enabled: en},
		{Label: "分割(横)", Command: cmd(tmuxBin, "split-window", "-v", "-t", n), Enabled: en},
		{Label: "次ウィンドウ", Command: cmd(tmuxBin, "next-window", "-t", n), Enabled: en},
		{Label: "閉じる", Command: cmd(tmuxBin, "kill-window", "-t", n), Enabled: en},
	}
}
```

- [ ] **Step 5: テストを実行して PASS を確認**

Run: `go test ./internal/backend/ -run TestTmuxSessionOps`
Expected: PASS（※この時点で Zellij は未実装のため `go build ./...` は失敗してよい。次タスクで解消）

- [ ] **Step 6: コミット**

```bash
git add internal/backend/backend.go internal/backend/tmux.go internal/backend/tmux_test.go
git commit -m "feat: backend に OpPreset/SessionOps を追加し tmux 操作ビルダーを実装"
```

---

### Task 5: Zellij `SessionOps`

**Files:**
- Modify: `internal/backend/zellij.go`
- Test: `internal/backend/zellij_test.go`

**Interfaces:**
- Consumes: `OpPreset`, `Session`（Task 4）
- Produces: `func (z *Zellij) SessionOps(s Session) []OpPreset`

- [ ] **Step 1: 失敗テストを書く**

`internal/backend/zellij_test.go` に追加:
```go
func TestZellijSessionOps(t *testing.T) {
	ops := NewZellij().SessionOps(Session{Name: "foo"})
	want := map[string]string{
		"新規タブ":  "zellij -s foo action new-tab",
		"分割(縦)": "zellij -s foo action new-pane -d right",
		"分割(横)": "zellij -s foo action new-pane -d down",
		"次タブ":   "zellij -s foo action go-to-next-tab",
		"閉じる":   "zellij -s foo action close-pane",
	}
	got := map[string]string{}
	for _, o := range ops {
		got[o.Label] = o.Command.Display
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: got %q want %q", k, got[k], v)
		}
	}
	// EXITED セッションでは無効
	for _, o := range NewZellij().SessionOps(Session{Name: "foo", Dead: true}) {
		if o.Enabled {
			t.Fatalf("EXITED で %s が有効になっている", o.Label)
		}
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/backend/ -run TestZellijSessionOps`
Expected: FAIL（`SessionOps` 未定義）

- [ ] **Step 3: Zellij に実装**

`internal/backend/zellij.go`：`CanRename` の下に追加:
```go
func (z *Zellij) SessionOps(s Session) []OpPreset {
	en := s.Name != "" && !s.Dead
	n := s.Name
	return []OpPreset{
		{Label: "新規タブ", Command: cmd(zellijBin, "-s", n, "action", "new-tab"), Enabled: en},
		{Label: "分割(縦)", Command: cmd(zellijBin, "-s", n, "action", "new-pane", "-d", "right"), Enabled: en},
		{Label: "分割(横)", Command: cmd(zellijBin, "-s", n, "action", "new-pane", "-d", "down"), Enabled: en},
		{Label: "次タブ", Command: cmd(zellijBin, "-s", n, "action", "go-to-next-tab"), Enabled: en},
		{Label: "閉じる", Command: cmd(zellijBin, "-s", n, "action", "close-pane"), Enabled: en},
	}
}
```

- [ ] **Step 4: テストとビルドを実行して PASS を確認**

Run: `go test ./... && go build ./... && go vet ./...`
Expected: 全 PASS / build exit 0 / vet 警告なし（interface を両 backend が満たした）

- [ ] **Step 5: コミット**

```bash
git add internal/backend/zellij.go internal/backend/zellij_test.go
git commit -m "feat: zellij 操作ビルダー SessionOps を実装（EXITED は無効）"
```

---

## Phase 3: メニュー（文脈連動）と既出バグ修正

### Task 6: `menuItem` と `buildMenu` 純関数

**Files:**
- Create: `internal/ui/menu.go`
- Test: `internal/ui/menu_test.go`

**Interfaces:**
- Consumes: `action.Kind`, `backend.Backend`, `backend.Session`, `backend.Command`
- Produces:
  - `type itemKind int`（`kindAction`, `kindOp`, `kindSep`）
  - `type menuItem struct { kind itemKind; label string; act action.Kind; command backend.Command; display string; enabled bool }`
  - `func buildMenu(b backend.Backend, sel backend.Session) []menuItem`

- [ ] **Step 1: 失敗テストを書く**

`internal/ui/menu_test.go`:
```go
package ui

import (
	"testing"

	"github.com/jss826/navmux/internal/backend"
)

func TestBuildMenuTmux(t *testing.T) {
	items := buildMenu(backend.NewTmux(), backend.Session{Name: "main"})
	// 先頭はアタッチ（選択ありで有効・display あり）
	if items[0].label != "アタッチ" || !items[0].enabled || items[0].display != "tmux attach -t main" {
		t.Fatalf("先頭 item 不正: %+v", items[0])
	}
	// 区切りが 1 つ含まれる
	seps := 0
	for _, it := range items {
		if it.kind == kindSep {
			seps++
		}
	}
	if seps != 1 {
		t.Fatalf("区切りの数 = %d want 1", seps)
	}
	// 操作（kindOp）が 5 つ含まれ display が埋まっている
	ops := 0
	for _, it := range items {
		if it.kind == kindOp {
			ops++
			if it.display == "" {
				t.Fatalf("op の display が空: %+v", it)
			}
		}
	}
	if ops != 5 {
		t.Fatalf("op の数 = %d want 5", ops)
	}
}

func TestBuildMenuRenameDisabledOnZellij(t *testing.T) {
	items := buildMenu(backend.NewZellij(), backend.Session{Name: "x"})
	for _, it := range items {
		if it.kind == kindAction && it.act == 2 { // action.Rename
			if it.enabled {
				t.Fatal("zellij でリネームが有効になっている")
			}
		}
	}
}

func TestBuildMenuNewAlwaysEnabled(t *testing.T) {
	// セッション未選択（空）でも「新規セッション」は有効
	items := buildMenu(backend.NewTmux(), backend.Session{})
	for _, it := range items {
		if it.kind == kindAction && it.act == 1 { // action.New
			if !it.enabled {
				t.Fatal("新規セッションは常に有効のはず")
			}
		}
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/ui/ -run TestBuildMenu`
Expected: FAIL（`buildMenu` 未定義）

- [ ] **Step 3: 最小実装**

`internal/ui/menu.go`:
```go
package ui

import (
	"github.com/jss826/navmux/internal/action"
	"github.com/jss826/navmux/internal/backend"
)

type itemKind int

const (
	kindAction itemKind = iota // セッションアクション（attach/new/rename/kill）
	kindOp                     // mux 操作
	kindSep                    // 区切り（選択不可）
)

// menuItem は右ペインの 1 行。display は「実行:」行/コピーに使う。
type menuItem struct {
	kind    itemKind
	label   string
	act     action.Kind     // kind==kindAction のとき有効
	command backend.Command // kind==kindOp のとき有効
	display string
	enabled bool
}

// buildMenu は選択中セッションに対する右ペインのメニューを組む（純関数）。
func buildMenu(b backend.Backend, sel backend.Session) []menuItem {
	name := sel.Name
	items := []menuItem{
		{kind: kindAction, act: action.Attach, label: "アタッチ", display: b.AttachCmd(name).Display, enabled: name != ""},
		{kind: kindAction, act: action.New, label: "新規セッション", display: b.NewCmd("<name>").Display, enabled: true},
		{kind: kindAction, act: action.Rename, label: "リネーム", enabled: b.CanRename() && name != ""},
		{kind: kindAction, act: action.Kill, label: "削除", display: b.KillCmd(name).Display, enabled: name != ""},
		{kind: kindSep, label: "── 操作 ──"},
	}
	if rc, ok := b.RenameCmd(name, "<new>"); ok {
		items[2].display = rc.Display
	}
	for _, op := range b.SessionOps(sel) {
		items = append(items, menuItem{
			kind:    kindOp,
			label:   op.Label,
			command: op.Command,
			display: op.Command.Display,
			enabled: op.Enabled,
		})
	}
	return items
}
```

- [ ] **Step 4: テストを実行して PASS を確認**

Run: `go test ./internal/ui/ -run TestBuildMenu`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/ui/menu.go internal/ui/menu_test.go
git commit -m "feat: 右ペインメニューを組む buildMenu（純関数）"
```

---

### Task 7: `nextSelectable` と `currentDisplay` 純関数

**Files:**
- Modify: `internal/ui/menu.go`
- Test: `internal/ui/menu_test.go`

**Interfaces:**
- Consumes: `menuItem`（Task 6）
- Produces:
  - `func nextSelectable(items []menuItem, cur, dir int) int`（端で止まり、区切り/無効をスキップ）
  - `func currentDisplay(items []menuItem, cur int) string`

- [ ] **Step 1: 失敗テストを書く**

`internal/ui/menu_test.go` に追加:
```go
func TestNextSelectableSkipsSepAndDisabled(t *testing.T) {
	items := []menuItem{
		{kind: kindAction, label: "a", enabled: true},  // 0
		{kind: kindAction, label: "b", enabled: false}, // 1 無効
		{kind: kindSep, label: "--"},                   // 2 区切り
		{kind: kindOp, label: "c", enabled: true},      // 3
	}
	if got := nextSelectable(items, 0, +1); got != 3 {
		t.Fatalf("0 から +1 = %d want 3（1 と 2 をスキップ）", got)
	}
	if got := nextSelectable(items, 3, -1); got != 0 {
		t.Fatalf("3 から -1 = %d want 0", got)
	}
	if got := nextSelectable(items, 3, +1); got != 3 {
		t.Fatalf("末尾で +1 は据え置き = %d want 3", got)
	}
}

func TestCurrentDisplay(t *testing.T) {
	items := []menuItem{{display: "X"}, {display: "Y"}}
	if currentDisplay(items, 1) != "Y" {
		t.Fatal("index 1 の display は Y")
	}
	if currentDisplay(items, 9) != "" {
		t.Fatal("範囲外は空文字")
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/ui/ -run "TestNextSelectable|TestCurrentDisplay"`
Expected: FAIL（未定義）

- [ ] **Step 3: 最小実装**

`internal/ui/menu.go` に追加:
```go
// nextSelectable は cur から dir 方向へ、区切り/無効を飛ばした次の選択可能 index を返す。
// 端では cur のまま据え置く。
func nextSelectable(items []menuItem, cur, dir int) int {
	i := cur
	for {
		n := i + dir
		if n < 0 || n >= len(items) {
			return cur
		}
		i = n
		if items[i].kind != kindSep && items[i].enabled {
			return i
		}
	}
}

// currentDisplay は cur 位置の display を返す（範囲外は空文字）。
func currentDisplay(items []menuItem, cur int) string {
	if cur < 0 || cur >= len(items) {
		return ""
	}
	return items[cur].display
}
```

- [ ] **Step 4: テストを実行して PASS を確認**

Run: `go test ./internal/ui/ -run "TestNextSelectable|TestCurrentDisplay"`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/ui/menu.go internal/ui/menu_test.go
git commit -m "feat: メニューカーソル移動 nextSelectable と currentDisplay"
```

---

### Task 8: `RenderMenu` 純コンテンツ描画

**Files:**
- Modify: `internal/ui/render.go`
- Test: `internal/ui/render_test.go`

**Interfaces:**
- Consumes: `menuItem`（Task 6）
- Produces: `func RenderMenu(items []menuItem, cur int, focused bool) string`

- [ ] **Step 1: 失敗テストを書く**

`internal/ui/render_test.go` に追加:
```go
func TestRenderMenuMarksCursorAndDisabled(t *testing.T) {
	items := []menuItem{
		{kind: kindAction, label: "アタッチ", enabled: true},
		{kind: kindAction, label: "リネーム", enabled: false},
		{kind: kindSep, label: "── 操作 ──"},
		{kind: kindOp, label: "分割(縦)", enabled: true},
	}
	out := RenderMenu(items, 0, true)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.HasPrefix(strings.TrimSpace(lines[0]), ">") {
		t.Fatalf("focus 時のカーソル行頭 > が無い: %q", lines[0])
	}
	if !strings.Contains(lines[1], "×") {
		t.Fatalf("無効項目の目印 × が無い: %q", lines[1])
	}
	if !strings.Contains(lines[2], "操作") {
		t.Fatalf("区切りが描画されない: %q", lines[2])
	}
	// 非フォーカス時はカーソル > を出さない
	if strings.Contains(RenderMenu(items, 0, false), ">") {
		t.Fatal("非フォーカスで > が出ている")
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/ui/ -run TestRenderMenu`
Expected: FAIL（`RenderMenu` 未定義）

- [ ] **Step 3: 最小実装**

`internal/ui/render.go` に追加:
```go
// RenderMenu は右ペインのメニューを純テキストで描く。focused 時のみカーソル > を出す。
func RenderMenu(items []menuItem, cur int, focused bool) string {
	var b strings.Builder
	for i, it := range items {
		if it.kind == kindSep {
			fmt.Fprintf(&b, "  %s\n", it.label)
			continue
		}
		mark := " "
		if focused && i == cur {
			mark = ">"
		}
		label := it.label
		if !it.enabled {
			label += " (×)"
		}
		fmt.Fprintf(&b, "%s %s\n", mark, label)
	}
	return b.String()
}
```

- [ ] **Step 4: テストを実行して PASS を確認**

Run: `go test ./internal/ui/ -run TestRenderMenu`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/ui/render.go internal/ui/render_test.go
git commit -m "feat: 右ペインメニューの純コンテンツ描画 RenderMenu"
```

---

### Task 9: model 配線（focus・メニュー実行・リネームガード・文脈連動 View）

**Files:**
- Modify: `internal/ui/model.go`
- Test: `internal/ui/model_test.go`

**Interfaces:**
- Consumes: `buildMenu`, `nextSelectable`, `currentDisplay`（Task 6,7）, `RenderMenu`, `RenderList`, `RenderFooter`（Task 8 / 既存）
- Produces（Model に追加するフィールド/メソッド）:
  - フィールド `focus int`（0=list, 1=menu）, `menuCursor int`
  - `func (m Model) menu() []menuItem`
  - `func (m Model) startRename() (tea.Model, tea.Cmd)`
  - `func (m Model) runMenuItem(it menuItem) (tea.Model, tea.Cmd)`

- [ ] **Step 1: 失敗テストを書く**

`internal/ui/model_test.go` に追加:
```go
func TestRightArrowFocusesMenu(t *testing.T) {
	m := New([]backend.Backend{backend.NewTmux()}, "")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if m.focus != 1 {
		t.Fatalf("→ で menu フォーカスにならない: focus=%d", m.focus)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(Model)
	if m.focus != 0 {
		t.Fatalf("← で list フォーカスに戻らない: focus=%d", m.focus)
	}
}

func TestRenameGuardNoSelection(t *testing.T) {
	m := New([]backend.Backend{backend.NewTmux()}, "") // セッション無し
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = next.(Model)
	if m.mode == modePrompt {
		t.Fatal("未選択で r がプロンプトに入ってしまった")
	}
	if m.status == "" {
		t.Fatal("未選択 r で案内 status が出ていない")
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/ui/ -run "TestRightArrowFocusesMenu|TestRenameGuardNoSelection"`
Expected: FAIL（`focus` 未定義 / r ガード未実装）

- [ ] **Step 3: Model にフィールドとヘルパを追加**

`internal/ui/model.go` の `Model` struct に追加:
```go
	focus      int // 0=list, 1=menu
	menuCursor int
```
`New` の返却 `Model{...}` に `menuCursor: 1`（「新規セッション」= 常に有効）を追加。

`selectedName` の下にヘルパを追加:
```go
// menu は現在の選択セッションに対するメニュー。
func (m Model) menu() []menuItem {
	var sel backend.Session
	if m.cursor >= 0 && m.cursor < len(m.sessions) {
		sel = m.sessions[m.cursor]
	}
	return buildMenu(m.ActiveBackend(), sel)
}

// startRename はガード付きでリネーム入力に入る。
func (m Model) startRename() (tea.Model, tea.Cmd) {
	if !m.ActiveBackend().CanRename() {
		m.status = "この multiplexer はリネーム非対応"
		return m, nil
	}
	if m.selectedName() == "" {
		m.status = "セッションが選択されていません"
		return m, nil
	}
	m.pending = action.Rename
	m.mode = modePrompt
	m.input.Focus()
	return m, textinput.Blink
}

// runMenuItem はカーソル中のメニュー項目を実行する。
func (m Model) runMenuItem(it menuItem) (tea.Model, tea.Cmd) {
	if !it.enabled {
		return m, nil
	}
	switch it.kind {
	case kindOp:
		c := it.command
		return m, func() tea.Msg { _, err := runCommand(c); return opDoneMsg{err: err} }
	case kindAction:
		switch it.act {
		case action.Attach:
			return m, m.attachSelected()
		case action.New:
			m.pending = action.New
			m.mode = modePrompt
			m.input.Focus()
			return m, textinput.Blink
		case action.Rename:
			return m.startRename()
		case action.Kill:
			if m.selectedName() == "" {
				return m, nil
			}
			m.mode = modeConfirm
			return m, nil
		}
	}
	return m, nil
}
```

- [ ] **Step 4: 通常モードのキー処理を更新**

`handleKey` の「通常モード」`switch msg.String()` を以下に置換（既存の up/down/tab/?/enter/n/r/d/y を統合）:
```go
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "left":
		m.focus = 0
	case "right":
		m.focus = 1
		items := m.menu()
		if m.menuCursor >= len(items) || items[m.menuCursor].kind == kindSep || !items[m.menuCursor].enabled {
			m.menuCursor = nextSelectable(items, -1, +1)
		}
	case "up", "k":
		if m.focus == 1 {
			m.menuCursor = nextSelectable(m.menu(), m.menuCursor, -1)
		} else if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.focus == 1 {
			m.menuCursor = nextSelectable(m.menu(), m.menuCursor, +1)
		} else if m.cursor < len(m.sessions)-1 {
			m.cursor++
		}
	case "tab":
		m.active = (m.active + 1) % len(m.backends)
		m.cursor = 0
		return m, m.refresh()
	case "?":
		m.showExplain = !m.showExplain
	case "enter":
		if m.focus == 1 {
			items := m.menu()
			if m.menuCursor >= 0 && m.menuCursor < len(items) {
				return m.runMenuItem(items[m.menuCursor])
			}
			return m, nil
		}
		return m, m.attachSelected()
	case "n":
		m.pending = action.New
		m.mode = modePrompt
		m.input.Focus()
		return m, textinput.Blink
	case "r":
		return m.startRename()
	case "d":
		if m.selectedName() == "" {
			return m, nil
		}
		m.mode = modeConfirm
	case "y":
		m.copyCurrentCommand()
	}
	return m, nil
```

- [ ] **Step 5: 文脈連動の copy / View に置換**

`copyCurrentCommand` を以下に置換（カーソル連動）:
```go
func (m *Model) copyCurrentCommand() {
	disp := currentDisplay(m.menu(), m.menuCursor)
	if disp == "" {
		m.status = "コピーできるコマンドがありません"
		return
	}
	if err := clipboard.WriteAll(disp); err != nil {
		m.status = "コピー失敗: " + err.Error()
		return
	}
	m.status = "コピーしました: " + disp
}
```
`View` を以下に置換（純コンテンツの縦積み。2 ペイン横並び化は Phase 5 で行う）:
```go
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	items := m.menu()
	out := "navmux — " + m.ActiveBackend().Name() + "\n\n"
	out += "[Sessions]\n" + RenderList(m.sessions, m.cursor) + "\n"
	out += "[Actions]\n" + RenderMenu(items, m.menuCursor, m.focus == 1) + "\n"
	out += "実行: " + currentDisplay(items, m.menuCursor) + "\n"

	if m.mode == modePrompt {
		out += "\n名前: " + m.input.View() + "\n(enter 確定 / esc キャンセル)\n"
	}
	if m.mode == modeConfirm {
		out += "\n削除しますか? " + m.selectedName() + " [y/N]\n"
	}
	out += "\n" + RenderFooter(action.All(), m.ActiveBackend().CanRename()) + "\n"
	out += "↑↓ 選択   ←→ ペイン移動   enter 実行   tab mux   y コピー   q 終了\n"
	if m.status != "" {
		out += "\n" + m.status + "\n"
	}
	return out
}
```
不要になった旧 `showExplain` ブロックの `RenderExplain(action.All()[0], ...)` 呼び出しは削除する（attach 固定をやめる）。`showExplain` を残す場合は `RenderExplain` にカーソル項目の label/display を渡す形に直す:
```go
	if m.showExplain {
		it := items[m.menuCursor]
		a := action.Action{Key: "", Label: it.label, Explain: "選択中セッションに対して実行します。"}
		out += "\n" + RenderExplain(a, it.display) + "\n"
	}
```
（`RenderExplain` の既存シグネチャ `(a action.Action, commandDisplay string)` を流用。）

- [ ] **Step 6: テスト・ビルド・vet を実行**

Run: `go test ./... && go build ./... && go vet ./...`
Expected: 全 PASS / build exit 0 / vet 警告なし（既存 `TestToggleExplain` も維持されること）

- [ ] **Step 7: zellij 実機スモーク（ユーザーのターミナル）**

navmux 起動 → `→` で右ペイン、`↑↓` で操作選択（区切り/無効をスキップ）、`enter` で操作実行、`y` でカーソル項目のコマンドがコピーされること、未選択で `r` がガードされることを確認。

- [ ] **Step 8: コミット**

```bash
git add internal/ui/model.go internal/ui/model_test.go
git commit -m "feat: 2ペインのフォーカス/メニュー実行/リネームガード/文脈連動コピー"
```

---

## Phase 4: 視認性（lipgloss 装飾層）

### Task 10: lipgloss 導入と 2 ペイン横並び・色付け（装飾層）

**Files:**
- Modify: `go.mod` / `go.sum`（`go get github.com/charmbracelet/lipgloss`）
- Create: `internal/ui/style.go`
- Modify: `internal/ui/model.go`（`View` の組み立てを装飾層経由に）

**Interfaces:**
- Consumes: `RenderList`, `RenderMenu`, `currentDisplay`（純コンテンツ層）
- Produces: `func styleDashboard(title, list, menu, execLine, footer, status string) string`

- [ ] **Step 1: lipgloss を取得**

Run: `go get github.com/charmbracelet/lipgloss@latest && go mod tidy`
Expected: go.mod に lipgloss が追加される

- [ ] **Step 2: 装飾層を作る**

`internal/ui/style.go`:
```go
package ui

import "github.com/charmbracelet/lipgloss"

var (
	paneStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	titleStyle = lipgloss.NewStyle().Bold(true)
	execStyle  = lipgloss.NewStyle().Faint(true)
	footerHint = lipgloss.NewStyle().Faint(true)
)

// styleDashboard は純コンテンツ（list/menu 等）を受け取り、枠付き 2 ペインに整形する。
// 純コンテンツ層には触れない（テストは純コンテンツ側で行う）。
func styleDashboard(title, list, menu, execLine, footer, status string) string {
	left := paneStyle.Render("Sessions\n" + list)
	right := paneStyle.Render("Actions\n" + menu)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	out := titleStyle.Render(title) + "\n" + body + "\n" +
		execStyle.Render("実行: "+execLine) + "\n" +
		footerHint.Render(footer)
	if status != "" {
		out += "\n" + status
	}
	return out
}
```

- [ ] **Step 3: View を装飾層経由に変更**

`internal/ui/model.go` の `View` の通常表示部分（prompt/confirm を除く土台）を `styleDashboard` で組む:
```go
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	items := m.menu()
	title := "navmux — " + m.ActiveBackend().Name()
	footer := "↑↓ 選択   ←→ ペイン移動   enter 実行   tab mux   y コピー   ? 解説   q 終了"
	out := styleDashboard(
		title,
		RenderList(m.sessions, m.cursor),
		RenderMenu(items, m.menuCursor, m.focus == 1),
		currentDisplay(items, m.menuCursor),
		footer,
		m.status,
	)
	if m.mode == modePrompt {
		out += "\n名前: " + m.input.View() + "\n(enter 確定 / esc キャンセル)"
	}
	if m.mode == modeConfirm {
		out += "\n削除しますか? " + m.selectedName() + " [y/N]"
	}
	return out
}
```

- [ ] **Step 4: テスト・ビルド・vet**

Run: `go test ./... && go build ./... && go vet ./...`
Expected: 全 PASS（純コンテンツ層テストは ANSI に依存しないので維持）/ build exit 0 / vet 警告なし

- [ ] **Step 5: zellij 実機スモーク（ユーザーのターミナル）**

枠・色付きで 2 ペインが横並びに出ること、日本語ラベルの折返し/桁ズレが許容範囲か、操作・コピー・新規生成が引き続き動くことを観察。

- [ ] **Step 6: コミット**

```bash
git add go.mod go.sum internal/ui/style.go internal/ui/model.go
git commit -m "feat: lipgloss で2ペイン枠・色付け（装飾層を純コンテンツ層から分離）"
```

---

## 完了時チェック（品質ゲート全実行＋エビデンス引用）

- `go test ./...` 全 PASS / `go build ./...` exit 0 / `go vet ./...` 警告なしの stdout を引用して完了宣言する。
- zellij 0.44.3 実機スモーク（新規生成の実在確認・操作実行・コピー・2 ペイン表示）をユーザーのターミナルで観察し結果を残す。
- README のキー操作表（`←→ ペイン移動` / `操作メニュー` / `y コピー`）を必要に応じ更新（別コミット `docs:`）。

## Self-Review 結果

- **Spec coverage:** 視認性=Task 10 / 2 ペイン=Task 9,10 / メニュー実行=Task 6-9 / mux 操作送出=Task 3-5,9 / 生成バグ=Task 1,2 / 解説・コピー文脈連動=Task 9 / リネームガード=Task 9。設定ファイル・send-keys は非目標（spec §2）でタスク無し＝意図どおり。
- **Placeholder scan:** プレースホルダ無し。各コード/テストは実体を記載。
- **Type consistency:** `menuItem`/`itemKind`/`OpPreset`/`SessionOps(s Session)`/`buildMenu`/`nextSelectable`/`currentDisplay`/`RenderMenu`/`newSession`/`containsSession`/`spawnDetached` のシグネチャはタスク間で一致。`action.Kind` の数値（New=1, Rename=2）はテストで使用（`internal/action/action.go` の iota 定義に一致）。
