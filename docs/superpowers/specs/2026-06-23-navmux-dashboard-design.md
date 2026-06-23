# navmux ブラッシュアップ — 常駐ナビ化 設計

作成日: 2026-06-23
対象: navmux（tmux / zellij をメニュー駆動で操作する TUI）

## 1. 目的（このブラッシュアップで実現すること）

navmux を「一覧 → アタッチで離脱」する一過性ツールから、**開きっぱなしのナビ（ダッシュボード）** へ進化させる。あわせて視認性を上げ、既知のバグを潰す。

具体的には 5 つ:

1. **視認性向上** — lipgloss による枠・色付け。
2. **2 ペインダッシュボード** — 左=セッション一覧 / 右=操作メニュー。
3. **メニュー実行** — カーソルで操作を選び Enter で実行（コマンド入力不要で新規セッション作成等ができる）。
4. **mux 操作コマンドの送出** — 選択中セッションを `-t`/`-s` で名指しして tmux/zellij の操作（新規ウィンドウ・分割・タブ移動・閉じる等）を実行。
5. **新規セッション生成バグの修正**（下記 §6）。

加えて、調査中に判明した既知バグ（解説/コピーが attach 固定、リネームの選択ガード欠如）を同梱で解消する。

## 2. 非目標（今回やらないこと）

- **設定ファイルによるカスタム操作の追加**。送出が「mux 操作のみ」になり語彙が固定的なため、価値が下がった。拡張点だけ残し、パース/パス解決/テストは後回し。
- **send-keys 系の任意シェル文字列送出**（`git status` 等をシェルに打鍵）。今回は mux 操作のみ。別 spec で追加余地を残す。
- tmux の Windows ネイティブ対応（従来どおり Linux で別途検証）。

## 3. 設計原則の踏襲

- 変更系は実行せず `Command{Argv, Display}` を返すビルダーに統一（mux 操作も同様）。`Display = strings.Join(Argv, " ")`。
- UI 層は具体 multiplexer を知らない。操作は backend が出すリストを描くだけ。
- ロジックは backend/action 側の純関数に寄せ、UI（Bubble Tea）は薄く保つ。
- シェルを介さず `exec.Command(bin, args...)` で直接実行（シェルインジェクションを発生させない）。

## 4. アーキテクチャ

### 4.1 backend 層 — 操作ビルダーの追加

`Backend` interface に追加:

```go
// SessionOps は対象セッションに対して実行できる mux 操作の一覧。
// UI はこれを描画・実行するだけで、具体コマンドを知らない。
SessionOps(name string) []OpPreset
```

```go
// OpPreset は右ペインに並べる 1 操作。
type OpPreset struct {
    Label   string  // 例 "分割(縦)"
    Command Command // Argv/Display を内包
    Enabled bool    // false ならグレーアウト（実行不可）
}
```

- 操作は backend 固有（tmux=ウィンドウ、zellij=タブ/ペイン）。
- 既定セット（対象セッション名を `<s>` とする）:

| ラベル | tmux | zellij 0.44.3 |
|--------|------|----------------|
| 新規ウィンドウ/タブ | `tmux new-window -t <s>` | `zellij -s <s> action new-tab` |
| 分割(縦) | `tmux split-window -h -t <s>` | `zellij -s <s> action new-pane -d right` |
| 分割(横) | `tmux split-window -v -t <s>` | `zellij -s <s> action new-pane -d down` |
| 次ウィンドウ/タブ | `tmux next-window -t <s>` | `zellij -s <s> action go-to-next-tab` |
| 閉じる | `tmux kill-window -t <s>` | `zellij -s <s> action close-pane` |

- EXITED（`Dead`）セッションでは操作は `Enabled=false`。

### 4.2 実行経路と新規生成バグ修正

セッション**生成**コマンド（`action.New`）だけ専用経路で実行する:

```
spawnDetached(c Command) error  // セッション生成専用
  - Windows: 新コンソールを割り当てて起動
             SysProcAttr{CreationFlags: CREATE_NEW_CONSOLE}
             CombinedOutput で待たない（投げっぱなし）
  - Unix:    従来どおり（zellij/tmux の background が自前で daemonize）
```

- **終了コードを信用しない**。生成後に `List()` で対象名の実在を確認し、出たら「完了」、出なければ「作成に失敗した可能性があります」を表示する（verification-before-completion）。
- mux 操作（§4.1）と rename/kill/switch は既存セッション対象で即返るため、従来の `runCommand`（`CombinedOutput`）のままでよい。コンソール割り当てが必要なのは**セッション生成のみ**。

根拠（§6 参照）: zellij の detached セッションは中で起動するシェルが生存するためにコンソール（ConPTY）を要する。navmux はパイプ実行でコンソールを与えないため、Windows ではシェルが即 EOF 終了しセッションが消える一方、zellij は exit 0 を返す。これが誤「完了」の原因。

### 4.3 UI / レイアウト（2 ペイン・lipgloss）

```
navmux                tmux ● zellij ○
┌ Sessions ─────────┐ ┌ Actions ──────────────┐
│> * main   running │ │> アタッチ              │
│    work   running │ │  新規セッション         │
│    old    EXITED  │ │  リネーム (×)          │
│                   │ │  削除                  │
│                   │ │  ── 操作 ──            │
│                   │ │  新規ウィンドウ         │
│                   │ │  分割(縦) / 分割(横)    │
│                   │ │  次ウィンドウ / 閉じる   │
└───────────────────┘ └───────────────────────┘
実行: tmux attach -t main                  [y コピー]
────────────────────────────────────────────────────
↑↓ 選択  ←→ ペイン移動  enter 実行  tab mux  ? 解説  q 終了
```

- 右ペインは 1 本のメニュー: セッションアクション（アタッチ/新規/リネーム/削除）＋区切り＋ mux 操作（§4.1）。すべてカーソル選択 → Enter 実行。
- 非対応項目（zellij のリネーム等）はグレーアウト。
- **「実行:」行と `?` 解説はカーソル中のメニュー項目に連動**（attach 固定をやめる）。`y` でその行のコマンドをコピー。フッターに `y コピー` を併記。
- キー: `↑↓`=メニュー内移動、`←→`=左右ペイン移動、`enter`=実行、`tab`=mux 切替、`?`=解説、`q`=終了。

### 4.4 render の純/装飾分離

- **(1) 純コンテンツ層**: プレーン文字列を組む純関数（現行 `RenderList` 等の発展）。単体テストはここに substring 一致で行う。
- **(2) 装飾層**: lipgloss で枠・色を付ける薄いラッパ。単体テスト対象外（実機スモークで確認）。

これにより「視認性向上」と「テストが ANSI で壊れない」を両立する。

## 5. データフロー

1. 起動 → `env.CurrentMux` で現在 mux 判定 → backend 配線（既存）。
2. `Init`/`refresh` → active backend の `List()` → 左ペイン更新。
3. `←→` でフォーカスペイン切替、`↑↓` でカーソル移動。カーソル位置に応じ「実行:」行を更新。
4. `enter`:
   - セッションアクション → 既存（attach は ExecProcess / switch、new は §4.2、rename/kill は runCommand）。
   - mux 操作 → 選択中セッション名を埋めた `Command` を `runCommand` で実行 → `refresh`。
5. 操作完了後は一覧を refresh して状態を反映。

## 6. 調査済みエビデンス（バグ根本原因）

実機 zellij 0.44.3（`C:\Users\soon7\AppData\Local\Zellij\zellij.exe`）で navmux と同じ呼び出しを再現:

| 実行方法 | 終了コード | セッション生成 |
|----------|-----------|----------------|
| パイプ実行・コンソールなし（= 現 navmux の `CombinedOutput`） | 0 | 残らない（即死） |
| 新コンソール割り当て（`Start-Process -WindowStyle Hidden` 相当） | — | 残る ✅ |

- パイプ実行時、stdout に新セッションのシェル（PowerShell）の起動バナーが漏れ、その後即終了 → セッション消滅。
- 新コンソール割り当てでは `navmux_probe_003` が `list-sessions` に残存し永続を確認。
- → 修正方針 §4.2 はこのエビデンスに基づく。

## 7. エラー処理

- 生成: exit 0 でも実在確認に失敗したら失敗扱いの status を表示。
- 操作: `runCommand` のエラーは status 行に表示（既存パターン）。
- 非対応操作: `Enabled=false` で実行不可（そもそも Enter で発火させない）。

## 8. テスト方針（① 品質ゲート: `go test ./...` / `go build ./...` / `go vet ./...` 全通過）

- backend: `SessionOps` の各 `Command.Display` を §4.1 の表と厳密一致で純関数テスト（注入 `runFunc` は不要、ビルダーは純粋）。
- action/UI: メニュー構築・カーソル連動の「実行:」行・グレーアウト判定を純コンテンツ層で substring テスト。
- リネーム選択ガード: 未選択時に発火しないことをテスト。
- 生成経路 `spawnDetached`: OS 依存のため単体テストは最小（純粋に分離できる部分のみ）。**実機検証は zellij 0.44.3 スモーク**で担保（新規生成 → `list-sessions` 実在確認 → 永続確認）。

## 9. コマンド対応表（CLAUDE.md 正本への追記）

実装の最初に CLAUDE.md「コマンド対応表」へ §4.1 の操作行を追記し、`Display` と厳密一致させる（表示・コピー・実行が 1 定義から出るため）。

## 10. 実装フェーズ（提案）

1. **バグ修正先行**: `spawnDetached` ＋ 生成後実在確認（誤「完了」解消）。
2. **backend `SessionOps`** ＋ 既定 5 操作 ＋ コマンド対応表追記。
3. **UI メニュー文脈連動**: 「実行:」/`?`/`y` をカーソル連動化、リネーム選択ガード。
4. **2 ペインレイアウト**（純コンテンツ層）。
5. **lipgloss 装飾層**（視認性）。
6. 各フェーズで品質ゲート＋ zellij 実機スモーク。

## 11. リスク・要検証

- zellij の各 `action`（new-tab/new-pane/close-pane 等）が detached/非フォーカスセッションに対しどう効くかは実機スモークで確認（フォーカス中タブ/ペインに作用する想定）。
- 新コンソール割り当て時に可視ウィンドウが一瞬出ないか（hidden で抑制できるか）を確認。
- lipgloss 導入後、既存テストの substring 前提が純コンテンツ層に正しく隔離されているか確認。
