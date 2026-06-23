# navmux アクション拡張 + 状態可視化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ダッシュボード UI に、フォーカス可視化・フッターの実行可否ハイライト・コンソール出力コピー・他クライアント切断を加える。

**Architecture:** 変更系は実行せず `backend.Command{Argv, Display}` を返すビルダーに寄せる既存方針を維持。実行可否は `action.Runnable` 純関数に一元化し、フッターとメニューの両方が参照する。コンソール取得は mux CLI（zellij `dump-screen` / tmux `capture-pane`）の stdout をクリップボードへ。装飾（フォーカス枠）は `style.go` の装飾層のみで完結させ、純コンテンツ層（`RenderList`/`RenderMenu`）は不変に保つ。

**Tech Stack:** Go 1.25 / Bubble Tea / lipgloss / atotto/clipboard。検証済み実機 zellij 0.44.3。

## Global Constraints

- module: `github.com/jss826/navmux`。
- 変更系は `exec.Command(bin, args...)` を**シェルを介さず**直接実行（インジェクション回避）。
- `Command.Display` は `strings.Join(Argv, " ")`（`cmd()` ヘルパ）で導出。表示・コピー・実行が 1 定義から出る。
- コマンド文字列の正本は CLAUDE.md「コマンド対応表」。`Display` を表に byte 一致させる。
- 純コンテンツ層はプレーン文字列を維持（テストが substring 一致）。ANSI 装飾は装飾層（`styleDashboard`）のみ。
- 品質ゲート: `go test ./...` 全 PASS / `go build ./...` exit 0 / `go vet ./...` 警告なし。
- ブランチは main 直接 commit（ユーザー選択）。push は明示指示まで行わない。

### コマンド対応表（本計画で追加する正本）

| 操作 | tmux | zellij 0.44.3 |
|------|------|---------------|
| 操作:画面コピー | `tmux capture-pane -t <name> -p` | `zellij -s <name> action dump-screen` |
| 操作:全履歴コピー | `tmux capture-pane -t <name> -p -S -` | `zellij -s <name> action dump-screen -f` |
| 操作:他クライアント切断 | `tmux detach-client -a -t <name>` | **非対応** → グレーアウト + ラベルにキーヒント `Ctrl o w Ctrl x` |

## File Structure

- `internal/action/action.go` — `Runnable` 純関数を追加（実行可否の単一の真実）。
- `internal/action/action_test.go` — `Runnable` のテスト。
- `internal/ui/render.go` — `RenderFooter` を `Runnable` 駆動に変更。
- `internal/ui/render_test.go` — フッターテストを新シグネチャへ。
- `internal/ui/menu.go` — `buildMenu` を `Runnable` 駆動に統一、`kindCapture` 追加、capture 項目を組む。
- `internal/ui/menu_test.go` — op 数の更新。
- `internal/ui/style.go` — `activePaneStyle` 追加、`styleDashboard` に `focus` 引数。
- `internal/ui/style_test.go` — `focus` 引数追従 + フォーカス差分テスト。
- `internal/ui/model.go` — `View` の配線、capture 実行経路、`captureDoneMsg`、`countLines`、seam 変数。
- `internal/ui/model_test.go` — capture フローのテスト。
- `internal/ui/exec.go` — stdout 専用 runner `runCapture`。
- `internal/backend/backend.go` — `OpPreset.Capture` フィールド追加。
- `internal/backend/tmux.go` / `zellij.go` — detach / capture の op を追加。
- `internal/backend/tmux_test.go` / `zellij_test.go` — 新 op の Display byte 一致テスト。
- `CLAUDE.md` / `README` — コマンド対応表・キー操作表の更新。

---

## Task 1: action.Runnable（実行可否の単一の真実）

**Files:**
- Modify: `internal/action/action.go`
- Test: `internal/action/action_test.go`

**Interfaces:**
- Consumes: `backend.Backend`（`CanRename() bool`）。
- Produces: `func Runnable(b backend.Backend, k Kind, name string) bool`
  - `New` → 常に true
  - `Attach` / `Kill` → `name != ""`
  - `Rename` → `b.CanRename() && name != ""`

- [ ] **Step 1: 失敗テストを書く**

`internal/action/action_test.go` に追記:

```go
func TestRunnable(t *testing.T) {
	tx := backend.NewTmux()  // CanRename() == true
	z := backend.NewZellij() // CanRename() == false

	cases := []struct {
		name string
		b    backend.Backend
		k    Kind
		sel  string
		want bool
	}{
		{"new は常に可(選択なし)", tx, New, "", true},
		{"attach は選択必要", tx, Attach, "", false},
		{"attach 選択あり", tx, Attach, "foo", true},
		{"kill は選択必要", tx, Kill, "", false},
		{"kill 選択あり", tx, Kill, "foo", true},
		{"rename tmux 選択あり", tx, Rename, "foo", true},
		{"rename tmux 選択なし", tx, Rename, "", false},
		{"rename zellij は不可", z, Rename, "foo", false},
	}
	for _, c := range cases {
		if got := Runnable(c.b, c.k, c.sel); got != c.want {
			t.Fatalf("%s: Runnable = %v want %v", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 失敗を確認**

Run: `go test ./internal/action/ -run TestRunnable`
Expected: FAIL（`undefined: Runnable`）

- [ ] **Step 3: 最小実装**

`internal/action/action.go` の末尾に追記:

```go
// Runnable はそのアクションが今この瞬間に実行可能かを返す（フッター/メニュー共通）。
func Runnable(b backend.Backend, k Kind, name string) bool {
	switch k {
	case New:
		return true
	case Attach, Kill:
		return name != ""
	case Rename:
		return b.CanRename() && name != ""
	}
	return false
}
```

- [ ] **Step 4: 緑を確認**

Run: `go test ./internal/action/`
Expected: PASS

- [ ] **Step 5: commit**

```bash
git add internal/action/action.go internal/action/action_test.go
git commit -m "feat(action): 実行可否を判定する Runnable 純関数を追加"
```

---

## Task 2: フッター実行可否ハイライト + メニュー判定の統一

**Files:**
- Modify: `internal/ui/render.go`（`RenderFooter`）
- Modify: `internal/ui/menu.go`（`buildMenu` を `Runnable` 駆動へ）
- Modify: `internal/ui/model.go`（`View` の `RenderFooter` 呼び出し）
- Test: `internal/ui/render_test.go`

**Interfaces:**
- Consumes: `action.Runnable`（Task 1）、`backend.Backend`。
- Produces: `func RenderFooter(actions []action.Action, b backend.Backend, name string) string`
  - 実行可 → `key label`
  - 状態的に不可 → `(key label ×)`
  - 構造的に非対応（rename × zellij）→ `(key label=非対応)`

- [ ] **Step 1: 失敗テストを書く**

`internal/ui/render_test.go` の既存 `TestRenderFooterShowsKeys` と `TestRenderFooterGreysRenameWhenUnsupported` を以下で**置き換え**、`TestRenderFooterMarksUnavailable` を追加:

```go
func TestRenderFooterShowsKeys(t *testing.T) {
	// tmux + 選択あり → 全アクション実行可（× や 非対応 が付かない）
	out := RenderFooter(action.All(), backend.NewTmux(), "main")
	for _, want := range []string{"enter", "アタッチ", "n", "d"} {
		if !strings.Contains(out, want) {
			t.Fatalf("footer に %q が無い: %q", want, out)
		}
	}
	if strings.Contains(out, "×") || strings.Contains(out, "非対応") {
		t.Fatalf("選択ありでは不可マークが出ないはず: %q", out)
	}
}

func TestRenderFooterGreysRenameWhenUnsupported(t *testing.T) {
	out := RenderFooter(action.All(), backend.NewZellij(), "main")
	if !strings.Contains(out, "非対応") {
		t.Fatalf("リネーム非対応の目印が無い: %q", out)
	}
}

func TestRenderFooterMarksUnavailable(t *testing.T) {
	// tmux + 選択なし → アタッチ/削除は (×)、新規は通常表示
	out := RenderFooter(action.All(), backend.NewTmux(), "")
	if !strings.Contains(out, "(enter アタッチ ×)") {
		t.Fatalf("未選択でアタッチに × が付かない: %q", out)
	}
	if !strings.Contains(out, "(d 削除 ×)") {
		t.Fatalf("未選択で削除に × が付かない: %q", out)
	}
	if strings.Contains(out, "(n 新規 ×)") {
		t.Fatalf("新規に × が付いている: %q", out)
	}
}
```

- [ ] **Step 2: 失敗を確認**

Run: `go test ./internal/ui/ -run TestRenderFooter`
Expected: FAIL（引数不一致でコンパイルエラー → ビルド不能で FAIL）

- [ ] **Step 3: RenderFooter を実装**

`internal/ui/render.go` の `RenderFooter` を置き換え:

```go
// RenderFooter はアクションをキー併記で 1 行に並べ、実行可否で表示を変える。
//   実行可 → "key label" / 状態的に不可 → "(key label ×)" / 構造的に非対応 → "(key label=非対応)"
func RenderFooter(actions []action.Action, b backend.Backend, name string) string {
	var parts []string
	for _, a := range actions {
		if a.Kind == action.Rename && !b.CanRename() {
			parts = append(parts, fmt.Sprintf("(%s %s=非対応)", a.Key, a.Label))
			continue
		}
		if action.Runnable(b, a.Kind, name) {
			parts = append(parts, fmt.Sprintf("%s %s", a.Key, a.Label))
		} else {
			parts = append(parts, fmt.Sprintf("(%s %s ×)", a.Key, a.Label))
		}
	}
	parts = append(parts, "←→ ペイン移動", "y コピー", "? 解説", "tab tmux/zellij", "q 終了")
	return strings.Join(parts, "   ")
}
```

- [ ] **Step 4: buildMenu を Runnable 駆動に統一**

`internal/ui/menu.go` の `buildMenu` の `items` 初期化を置き換え（動作は現状と等価。判定の二重持ちを解消）:

```go
	items := []menuItem{
		{kind: kindAction, act: action.Attach, label: "アタッチ", display: b.AttachCmd(name).Display, enabled: action.Runnable(b, action.Attach, name)},
		{kind: kindAction, act: action.New, label: "新規セッション", display: b.NewCmd("<name>").Display, enabled: action.Runnable(b, action.New, name)},
		{kind: kindAction, act: action.Rename, label: "リネーム", enabled: action.Runnable(b, action.Rename, name)},
		{kind: kindAction, act: action.Kill, label: "削除", display: b.KillCmd(name).Display, enabled: action.Runnable(b, action.Kill, name)},
		{kind: kindSep, label: "── 操作 ──"},
	}
```

- [ ] **Step 5: View の呼び出しを更新**

`internal/ui/model.go` の `View` 内 `RenderFooter(action.All(), m.ActiveBackend().CanRename())` を置き換え:

```go
		RenderFooter(action.All(), m.ActiveBackend(), m.selectedName()),
```

- [ ] **Step 6: 緑を確認**

Run: `go test ./internal/ui/`
Expected: PASS（既存 menu/model テストも引き続き PASS）

- [ ] **Step 7: commit**

```bash
git add internal/ui/render.go internal/ui/render_test.go internal/ui/menu.go internal/ui/model.go
git commit -m "feat(ui): フッターを実行可否でハイライトし判定を Runnable に統一"
```

---

## Task 3: フォーカス可視化（Sessions / Actions の枠）

**Files:**
- Modify: `internal/ui/style.go`
- Modify: `internal/ui/model.go`（`View` の `styleDashboard` 呼び出し）
- Test: `internal/ui/style_test.go`

**Interfaces:**
- Produces: `func styleDashboard(title, list, menu, execLine, footer, status string, focus int) string`
  - `focus==0` → 左ペインを `activePaneStyle`、`focus==1` → 右ペインを `activePaneStyle`。

- [ ] **Step 1: 失敗テストを書く**

`internal/ui/style_test.go` の `TestStyleDashboardPreservesContent` と `TestStyleDashboardOmitsEmptyStatus` の `styleDashboard(...)` 呼び出しに `focus` 引数を追加し、差分テストを追加:

```go
func TestStyleDashboardPreservesContent(t *testing.T) {
	out := styleDashboard(
		"navmux — tmux",
		"> * main\n",
		"> アタッチ\n",
		"tmux attach -t main",
		"enter アタッチ   q 終了",
		"完了",
		0,
	)
	for _, want := range []string{
		"navmux — tmux",
		"main",
		"アタッチ",
		"tmux attach -t main",
		"q 終了",
		"完了",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("styleDashboard 出力に %q が残っていない:\n%s", want, out)
		}
	}
}

func TestStyleDashboardOmitsEmptyStatus(t *testing.T) {
	out := styleDashboard("t", "l", "m", "e", "f", "", 0)
	if strings.Contains(out, "完了") {
		t.Fatalf("空 status なのに status 文字列が出ている:\n%s", out)
	}
}

func TestStyleDashboardFocusChangesOutput(t *testing.T) {
	left := styleDashboard("t", "l", "m", "e", "f", "", 0)
	right := styleDashboard("t", "l", "m", "e", "f", "", 1)
	if left == right {
		t.Fatal("focus 0/1 で出力が同一（フォーカス枠が反映されていない）")
	}
}
```

- [ ] **Step 2: 失敗を確認**

Run: `go test ./internal/ui/ -run TestStyleDashboard`
Expected: FAIL（引数不一致でコンパイルエラー）

- [ ] **Step 3: style.go を実装**

`internal/ui/style.go` を置き換え:

```go
package ui

import "github.com/charmbracelet/lipgloss"

var (
	paneStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	activePaneStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).BorderForeground(lipgloss.Color("14"))
	titleStyle      = lipgloss.NewStyle().Bold(true)
	execStyle       = lipgloss.NewStyle().Faint(true)
	footerHint      = lipgloss.NewStyle().Faint(true)
)

// styleDashboard は純コンテンツ（list/menu 等）を受け取り、枠付き 2 ペインに整形する。
// focus==0 で左（Sessions）、focus==1 で右（Actions）の枠を強調する。
// 純コンテンツ層には触れない（テストは純コンテンツ側で行う）。
func styleDashboard(title, list, menu, execLine, footer, status string, focus int) string {
	leftStyle, rightStyle := paneStyle, paneStyle
	if focus == 0 {
		leftStyle = activePaneStyle
	} else {
		rightStyle = activePaneStyle
	}
	left := leftStyle.Render("Sessions\n" + list)
	right := rightStyle.Render("Actions\n" + menu)
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

- [ ] **Step 4: View の呼び出しを更新**

`internal/ui/model.go` の `View` 内 `styleDashboard(...)` 呼び出しの最後に `m.focus` を追加（`RenderFooter` 引数は Task 2 で更新済み）:

```go
	out := styleDashboard(
		title,
		RenderList(m.sessions, m.cursor),
		RenderMenu(items, m.menuCursor, m.focus == 1),
		currentDisplay(items, m.menuCursor),
		RenderFooter(action.All(), m.ActiveBackend(), m.selectedName()),
		m.status,
		m.focus,
	)
```

- [ ] **Step 5: 緑を確認**

Run: `go test ./internal/ui/`
Expected: PASS

- [ ] **Step 6: commit**

```bash
git add internal/ui/style.go internal/ui/style_test.go internal/ui/model.go
git commit -m "feat(ui): フォーカス中ペインの枠を強調して可視化"
```

---

## Task 4: 他クライアント切断 op（tmux 実行 / zellij キーヒント）

**Files:**
- Modify: `internal/backend/tmux.go`（`SessionOps`）
- Modify: `internal/backend/zellij.go`（`SessionOps`）
- Test: `internal/backend/tmux_test.go`, `internal/backend/zellij_test.go`
- Modify: `internal/ui/menu_test.go`（op 数の更新）

**Interfaces:**
- Consumes: `backend.OpPreset`（既存）、`cmd()` ヘルパ。
- Produces: tmux/zellij の `SessionOps` 末尾に「他クライアント切断」を1項目追加。
  - tmux: `Command.Display == "tmux detach-client -a -t <name>"`, `Enabled == name!=""`
  - zellij: `Enabled == false`, `Label` にキーヒント `Ctrl o w Ctrl x` を含む。

- [ ] **Step 1: 失敗テストを書く**

`internal/backend/tmux_test.go` に追記:

```go
func TestTmuxSessionOpsHasDetachOthers(t *testing.T) {
	ops := NewTmux().SessionOps(Session{Name: "foo"})
	found := false
	for _, op := range ops {
		if op.Command.Display == "tmux detach-client -a -t foo" {
			found = true
			if !op.Enabled {
				t.Fatal("選択ありで他クライアント切断が無効")
			}
		}
	}
	if !found {
		t.Fatalf("tmux に他クライアント切断 op が無い: %+v", ops)
	}
}
```

`internal/backend/zellij_test.go` に追記:

```go
func TestZellijSessionOpsDetachOthersIsHint(t *testing.T) {
	ops := NewZellij().SessionOps(Session{Name: "foo"})
	found := false
	for _, op := range ops {
		if strings.Contains(op.Label, "他クライアント切断") {
			found = true
			if op.Enabled {
				t.Fatal("zellij は他クライアント切断 CLI 非対応のはず（Enabled=false）")
			}
			if !strings.Contains(op.Label, "Ctrl o w Ctrl x") {
				t.Fatalf("キーヒントがラベルに無い: %q", op.Label)
			}
		}
	}
	if !found {
		t.Fatalf("zellij に他クライアント切断 op が無い: %+v", ops)
	}
}
```

`internal/backend/zellij_test.go` の import に `"strings"` を追加（未追加の場合）。

- [ ] **Step 2: 失敗を確認**

Run: `go test ./internal/backend/ -run "DetachOthers"`
Expected: FAIL（op が無い）

- [ ] **Step 3: tmux 実装**

`internal/backend/tmux.go` の `SessionOps` の戻り `[]OpPreset{...}` 末尾（`"閉じる"` の後）に追記:

```go
		{Label: "他クライアント切断", Command: cmd(tmuxBin, "detach-client", "-a", "-t", n), Enabled: en},
```

- [ ] **Step 4: zellij 実装**

`internal/backend/zellij.go` の `SessionOps` の戻り `[]OpPreset{...}` 末尾（`"閉じる"` の後）に追記:

```go
		{Label: "他クライアント切断  Ctrl o w Ctrl x", Command: Command{Display: "Ctrl o w Ctrl x（手動）"}, Enabled: false},
```

- [ ] **Step 5: menu_test の op 数を更新**

`internal/ui/menu_test.go` の `TestBuildMenuTmux` の op 期待値を 5 → 6 に更新:

```go
	if ops != 6 {
		t.Fatalf("op の数 = %d want 6", ops)
	}
```

- [ ] **Step 6: 緑を確認**

Run: `go test ./internal/backend/ ./internal/ui/`
Expected: PASS

- [ ] **Step 7: commit**

```bash
git add internal/backend/tmux.go internal/backend/zellij.go internal/backend/tmux_test.go internal/backend/zellij_test.go internal/ui/menu_test.go
git commit -m "feat(backend): 他クライアント切断 op を追加（tmux 実行 / zellij はキーヒント）"
```

---

## Task 5: コンソール出力コピー（画面 / 全履歴）

**Files:**
- Modify: `internal/backend/backend.go`（`OpPreset.Capture`）
- Modify: `internal/backend/tmux.go`, `internal/backend/zellij.go`（capture op 追加）
- Test: `internal/backend/tmux_test.go`, `internal/backend/zellij_test.go`
- Modify: `internal/ui/menu.go`（`kindCapture` + buildMenu の振り分け）
- Modify: `internal/ui/exec.go`（`runCapture`）
- Modify: `internal/ui/model.go`（seam 変数 / `countLines` / `captureDoneMsg` / runMenuItem の capture 分岐 / Update）
- Test: `internal/ui/model_test.go`

**Interfaces:**
- Consumes: `backend.Command`、`clipboard.WriteAll`。
- Produces:
  - `backend.OpPreset` に `Capture bool` フィールド。
  - tmux/zellij `SessionOps` に「画面コピー」「全履歴コピー」（`Capture: true`）。
    - zellij: `zellij -s <name> action dump-screen` / `... dump-screen -f`
    - tmux: `tmux capture-pane -t <name> -p` / `... -p -S -`
  - ui: `const kindCapture`、`func countLines(out string) int`、`var captureRunner func(backend.Command)(string,error)`、`var clipboardWrite func(string) error`、`type captureDoneMsg struct{ lines int; err error }`。

- [ ] **Step 1: backend テストを書く（失敗）**

`internal/backend/tmux_test.go` に追記:

```go
func TestTmuxCaptureOps(t *testing.T) {
	ops := NewTmux().SessionOps(Session{Name: "foo"})
	want := map[string]bool{
		"tmux capture-pane -t foo -p":      false,
		"tmux capture-pane -t foo -p -S -": false,
	}
	for _, op := range ops {
		if _, ok := want[op.Command.Display]; ok {
			want[op.Command.Display] = true
			if !op.Capture {
				t.Fatalf("capture op の Capture=false: %q", op.Command.Display)
			}
			if !op.Enabled {
				t.Fatalf("選択ありで capture が無効: %q", op.Command.Display)
			}
		}
	}
	for disp, seen := range want {
		if !seen {
			t.Fatalf("capture op が無い: %q", disp)
		}
	}
}
```

`internal/backend/zellij_test.go` に追記:

```go
func TestZellijCaptureOps(t *testing.T) {
	ops := NewZellij().SessionOps(Session{Name: "foo"})
	want := map[string]bool{
		"zellij -s foo action dump-screen":    false,
		"zellij -s foo action dump-screen -f": false,
	}
	for _, op := range ops {
		if _, ok := want[op.Command.Display]; ok {
			want[op.Command.Display] = true
			if !op.Capture {
				t.Fatalf("capture op の Capture=false: %q", op.Command.Display)
			}
		}
	}
	for disp, seen := range want {
		if !seen {
			t.Fatalf("capture op が無い: %q", disp)
		}
	}
}
```

- [ ] **Step 2: 失敗を確認**

Run: `go test ./internal/backend/ -run Capture`
Expected: FAIL（`op.Capture` 未定義でコンパイルエラー）

- [ ] **Step 3: OpPreset に Capture を追加**

`internal/backend/backend.go` の `OpPreset` にフィールド追加:

```go
// OpPreset は右ペインに並べる 1 つの mux 操作。
type OpPreset struct {
	Label   string  // 例 "分割(縦)"
	Command Command // 実行/表示用
	Enabled bool    // false ならグレーアウト（実行不可）
	Capture bool    // true なら stdout を取得してクリップボードに入れる操作
}
```

- [ ] **Step 4: backend に capture op を追加**

`internal/backend/tmux.go` の `SessionOps` 末尾（他クライアント切断の後）に追記:

```go
		{Label: "画面コピー", Command: cmd(tmuxBin, "capture-pane", "-t", n, "-p"), Enabled: en, Capture: true},
		{Label: "全履歴コピー", Command: cmd(tmuxBin, "capture-pane", "-t", n, "-p", "-S", "-"), Enabled: en, Capture: true},
```

`internal/backend/zellij.go` の `SessionOps` 末尾（他クライアント切断の後）に追記:

```go
		{Label: "画面コピー", Command: cmd(zellijBin, "-s", n, "action", "dump-screen"), Enabled: en, Capture: true},
		{Label: "全履歴コピー", Command: cmd(zellijBin, "-s", n, "action", "dump-screen", "-f"), Enabled: en, Capture: true},
```

- [ ] **Step 5: backend の緑を確認**

Run: `go test ./internal/backend/`
Expected: PASS

- [ ] **Step 6: countLines のテストを書く（失敗）**

`internal/ui/model_test.go` に追記:

```go
func TestCountLines(t *testing.T) {
	cases := map[string]int{
		"":         0,
		"\n":       0,
		"x":        1,
		"a\nb\n":   2,
		"a\nb\nc":  3,
	}
	for in, want := range cases {
		if got := countLines(in); got != want {
			t.Fatalf("countLines(%q) = %d want %d", in, got, want)
		}
	}
}
```

- [ ] **Step 7: 失敗を確認**

Run: `go test ./internal/ui/ -run TestCountLines`
Expected: FAIL（`undefined: countLines`）

- [ ] **Step 8: exec.go に runCapture を追加**

`internal/ui/exec.go` に追記（stdout のみ。stderr 混入を避ける）:

```go
// runCapture は stdout のみを取得する（画面ダンプのコピー用）。
func runCapture(c backend.Command) (string, error) {
	if len(c.Argv) == 0 {
		return "", nil
	}
	out, err := exec.Command(c.Argv[0], c.Argv[1:]...).Output()
	return string(out), err
}
```

- [ ] **Step 9: model.go に capture 配線を追加**

`internal/ui/model.go` の import に `"fmt"` を追加。`opDoneMsg` の定義の近くに追記:

```go
// captureDoneMsg は画面コピー操作の完了。
type captureDoneMsg struct {
	lines int
	err   error
}

// seam（テストで差し替え可能）。
var (
	captureRunner  = runCapture
	clipboardWrite = clipboard.WriteAll
)

// countLines はキャプチャ文字列の行数を返す（末尾改行は無視）。
func countLines(out string) int {
	s := strings.TrimRight(out, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
```

`internal/ui/model.go` の import に `"strings"` を追加。

- [ ] **Step 10: kindCapture と buildMenu 振り分け**

`internal/ui/menu.go` の `itemKind` const に追加:

```go
const (
	kindAction itemKind = iota // セッションアクション（attach/new/rename/kill）
	kindOp                     // mux 操作
	kindSep                    // 区切り（選択不可）
	kindCapture                // 画面ダンプを取得してコピーする操作
)
```

同ファイルの `buildMenu` の op ループを置き換え:

```go
	for _, op := range b.SessionOps(sel) {
		k := kindOp
		if op.Capture {
			k = kindCapture
		}
		items = append(items, menuItem{
			kind:    k,
			label:   op.Label,
			command: op.Command,
			display: op.Command.Display,
			enabled: op.Enabled,
		})
	}
```

- [ ] **Step 11: runMenuItem に capture 分岐 + Update ハンドラ**

`internal/ui/model.go` の `runMenuItem` の `switch it.kind` に `kindCapture` を追加（`case kindOp:` の後）:

```go
	case kindCapture:
		c := it.command
		return m, func() tea.Msg {
			out, err := captureRunner(c)
			if err != nil {
				return captureDoneMsg{err: err}
			}
			if werr := clipboardWrite(out); werr != nil {
				return captureDoneMsg{err: werr}
			}
			return captureDoneMsg{lines: countLines(out)}
		}
```

同ファイルの `Update` の `switch msg := msg.(type)` に `opDoneMsg` ケースの後へ追加:

```go
	case captureDoneMsg:
		if msg.err != nil {
			m.status = "コピー失敗: " + msg.err.Error()
		} else {
			m.status = fmt.Sprintf("%d 行コピーしました", msg.lines)
		}
		return m, nil
```

- [ ] **Step 12: capture フローのテストを書く**

`internal/ui/model_test.go` に追記:

```go
func TestCaptureCopiesAndCountsLines(t *testing.T) {
	oldClip, oldCap := clipboardWrite, captureRunner
	defer func() { clipboardWrite, captureRunner = oldClip, oldCap }()

	var copied string
	clipboardWrite = func(s string) error { copied = s; return nil }
	captureRunner = func(c backend.Command) (string, error) { return "a\nb\nc\n", nil }

	m := New([]backend.Backend{backend.NewZellij()}, "")
	m.sessions = []backend.Session{{Name: "navmux"}}
	m.focus = 1

	items := m.menu()
	idx := -1
	for i, it := range items {
		if it.kind == kindCapture {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("kindCapture 項目が無い")
	}
	m.menuCursor = idx

	_, cmd := m.runMenuItem()
	if cmd == nil {
		t.Fatal("capture 実行で cmd が nil")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)

	if copied != "a\nb\nc\n" {
		t.Fatalf("クリップボード = %q", copied)
	}
	if m.status != "3 行コピーしました" {
		t.Fatalf("status = %q", m.status)
	}
}
```

- [ ] **Step 13: 緑を確認**

Run: `go test ./internal/ui/`
Expected: PASS

- [ ] **Step 14: 全体ゲート**

Run: `go test ./...` then `go build ./...` then `go vet ./...`
Expected: 全 PASS / exit 0 / 警告なし

- [ ] **Step 15: commit**

```bash
git add internal/backend/backend.go internal/backend/tmux.go internal/backend/zellij.go internal/backend/tmux_test.go internal/backend/zellij_test.go internal/ui/menu.go internal/ui/exec.go internal/ui/model.go internal/ui/model_test.go
git commit -m "feat: コンソール画面/全履歴をクリップボードへコピーする操作を追加"
```

---

## Task 6: ドキュメント更新

**Files:**
- Modify: `CLAUDE.md`（コマンド対応表）
- Modify: `README.md`（キー操作 / 操作メニュー表。存在する表のみ）

**Interfaces:** なし（ドキュメントのみ）。

- [ ] **Step 1: CLAUDE.md のコマンド対応表に3行追加**

`CLAUDE.md`「コマンド対応表（正本）」の表末尾（`操作:閉じる` の行の後）に追加:

```markdown
| 操作:画面コピー | `tmux capture-pane -t <name> -p` | `zellij -s <name> action dump-screen` |
| 操作:全履歴コピー | `tmux capture-pane -t <name> -p -S -` | `zellij -s <name> action dump-screen -f` |
| 操作:他クライアント切断 | `tmux detach-client -a -t <name>` | **非対応** → グレーアウト + ラベルにキーヒント `Ctrl o w Ctrl x` |
```

- [ ] **Step 2: CLAUDE.md の zellij 制約に追記**

`CLAUDE.md`「zellij 0.44.3 の制約（レビュー観点）」の箇条書き末尾に追加:

```markdown
- 他クライアント切断は CLI 非公開（`list-clients` はあるが detach-by-id 無し）→ `SessionOps` で `Enabled=false`、ラベルに手動キーヒント `Ctrl o w Ctrl x` を併記。
- 画面ダンプは `action dump-screen`（`--path` 省略で STDOUT、`-f` で全スクロールバック）。フォーカス中ペインのみ。
```

- [ ] **Step 3: README のキー操作表を更新**

`README.md` 内にキー操作・操作メニューの記述があれば、`y コピー`（コマンド/画面）・右ペインの「画面コピー / 全履歴コピー / 他クライアント切断」を追記する。該当記述が無ければこの Step はスキップし、その旨を報告する。

- [ ] **Step 4: commit**

```bash
git add CLAUDE.md README.md
git commit -m "docs: コマンド対応表に画面コピー/全履歴/他クライアント切断を追加"
```

---

## Self-Review（計画作成者によるチェック結果）

**1. Spec coverage:**
- フォーカス可視化 → Task 3 ✅
- フッター実行可否ハイライト（判定集約）→ Task 1 + Task 2 ✅
- コンソール出力コピー（画面/全履歴 + 行数表示）→ Task 5 ✅
- 他クライアント切断（tmux 実行 / zellij ヒント）→ Task 4 ✅
- コマンド対応表・ドキュメント → Task 6 ✅
- テスト方針（Runnable / Display byte 一致 / countLines / capture 経路 / 装飾の純層不変）→ 各 Task に内包 ✅

**2. Placeholder scan:** TBD/TODO 無し。各 step に実コードを記載。README は条件付き（表があれば更新、無ければスキップを報告）で明示。

**3. Type consistency:**
- `action.Runnable(b, k, name)` … Task 1 定義、Task 2 で footer/menu が使用（シグネチャ一致）。
- `styleDashboard(..., focus int)` … Task 3 で 7 引数化、同 Task で View 更新。
- `OpPreset.Capture` … Task 5 で追加、同 Task の tmux/zellij/menu/test で使用。
- `kindCapture` / `countLines` / `captureRunner` / `clipboardWrite` / `captureDoneMsg` … いずれも Task 5 内で定義・使用。

**注意（実装者向け）:** Task は順序依存（1→2→3→4→5→6）。特に Task 2 は Task 1 の `Runnable` を、Task 5 は Task 4 で増えた op の後ろに capture op を足す前提。順番に実装すること。
