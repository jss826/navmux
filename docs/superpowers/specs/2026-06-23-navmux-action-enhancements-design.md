# navmux アクション拡張 + 状態可視化 設計

作成日: 2026-06-23

## 目的

ダッシュボード UI に対し、以下4点を加える。

1. **フォーカス可視化** — Sessions / Actions のどちらのペインにいるかを枠で区別する。
2. **フッターの実行可否ハイライト** — 今この瞬間に実行可能なアクションと、不可能なものを表示で区別する。
3. **コンソール出力コピー** — 選択セッションの画面内容をクリップボードへコピーする mux 操作。
4. **他クライアント切断** — 同一セッションを複数クライアントで閲覧した際、自分以外を切断してサイズ最適化する mux 操作。

設計原則（CLAUDE.md）に従い、変更系は実行せず `Command{Argv, Display}` を返すビルダーに寄せ、ロジックは backend / action 側に置き UI は薄く保つ。コマンドに触れる変更なので「コマンド対応表」を本設計で先に確定する。

## 前提・調査結果（裏取り済み）

- navmux は**コンソール出力を保持していない**。attach 時に TTY を子プロセスへ渡すだけ。画面内容は mux の CLI から取得する。
- zellij 0.44.3 実機確認:
  - `zellij -s <name> action dump-screen` … viewport を **STDOUT** に出力（`--path` 省略時）。`-f` で全スクロールバック、`-a` で ANSI 保持。**フォーカス中ペインのみ**。
  - `action list-clients` は存在するが、**他クライアントを detach する CLI は無い**（`detach` は自分のみ）。→ zellij では「他クライアント切断」は非対応。
- tmux:
  - `tmux capture-pane -t <name> -p` で画面を STDOUT へ。`-S -` で全履歴。
  - `tmux detach-client -a -t <name>` で自分以外を全 detach。

## コマンド対応表（追加分・正本）

| 操作 | tmux | zellij 0.44.3 |
|------|------|---------------|
| 操作:画面コピー | `tmux capture-pane -t <name> -p` | `zellij -s <name> action dump-screen` |
| 操作:全履歴コピー | `tmux capture-pane -t <name> -p -S -` | `zellij -s <name> action dump-screen -f` |
| 操作:他クライアント切断 | `tmux detach-client -a -t <name>` | **非対応** → グレーアウト + キーヒント `Ctrl o w Ctrl x` |

実装時、本表を CLAUDE.md「コマンド対応表」へ反映し、各 `Command.Display` を byte 一致させる。

## 設計詳細

### 1. フォーカス可視化（装飾層のみ）

- `styleDashboard` のシグネチャに `focus int` を追加。
- `style.go` に `activePaneStyle`（`paneStyle` + 枠色 BrightCyan + タイトル強調）を追加。
- `focus==0` → 左（Sessions）に active 適用、`focus==1` → 右（Actions）に適用。
- 純コンテンツ層（`RenderList`/`RenderMenu`）は不変。substring 一致テストに影響なし。

### 2. フッター実行可否ハイライト（判定の集約）

- **可否判定を1関数に集約**: `action.Runnable(b backend.Backend, k Kind, name string) bool`
  - `Attach` / `Kill` … `name != ""`
  - `Rename` … `b.CanRename() && name != ""`
  - `New` … 常に true
- `RenderFooter` はアクションごとに `Runnable` を見て表示を分岐:
  - 実行可 → `enter アタッチ`（通常）
  - 状態的に不可（未選択など）→ `(enter アタッチ ×)`
  - 構造的に非対応（zellij rename）→ `(r リネーム=非対応)`（現状維持）
- `buildMenu` の `enabled` も同じ `action.Runnable` を使い、判定の二重持ちを解消する。
- 表示はテキストマーカー方式（ANSI 色付けはしない）。menu の `(×)` と一貫し、substring テストで検証可能。

### 3. コンソール出力コピー（新 itemKind `kindCapture`）

- `backend.OpPreset` に `Capture bool` を追加（true = stdout をキャプチャして使う op）。
- backend の `SessionOps` に2項目を追加: 「画面コピー」「全履歴コピー」（`Capture: true`, `Enabled: name!="" && !Dead`）。
- `menu.go`: `Capture` の OpPreset は `kindCapture` 項目として組む。
- 実行経路（`runMenuItem`）: `kindCapture` のとき stdout 専用 runner で実行し、`clipboard.WriteAll(out)`、ステータスに `N 行コピーしました`（`N = 改行数`）を出す。
  - **stdout のみ取得**するため、capture 用は `CombinedOutput` ではなく stdout 専用の実行関数を使う（stderr 混入回避）。`internal/ui/exec*.go` に capture 用 runner を足す。
- 行数の純関数 `countLines(out string) int` を `internal/ui` に置き単体テスト。
- 「直近 N 行指定」は zellij が非対応のため非目標（viewport / 全履歴の2択のみ）。

### 4. 他クライアント切断

- tmux: `SessionOps` に `OpPreset{Label:"他クライアント切断", Command: cmd("tmux","detach-client","-a","-t",name), Enabled: name!="" && !Dead}`。
- zellij: `OpPreset{Label:"他クライアント切断", Command: Command{Argv: nil, Display: "Ctrl o w Ctrl x（手動）"}, Enabled: false}`。
  - グレーアウト表示。`Display` にキーヒントを載せ、`y` でヒント文字列をコピー可。
  - ※ この並びが既定 zellij で「他クライアント切断」に当たるかは未検証。ユーザー提供文字列をそのまま表示する。

## テスト方針（CLAUDE.md 品質ゲート準拠）

- `action.Runnable` 純関数: 各 backend × 選択有無 × 各 Kind。
- backend 新 `Command.Display` がコマンド対応表と byte 一致（tmux / zellij）。
- zellij「他クライアント切断」が `Enabled=false` かつ `Display` がキーヒント。
- `countLines` 純関数。
- capture 経路: フェイク `runFunc` で stdout → クリップボード（clipboard 注入 or ステータス文字列で検証）。
- 装飾（フォーカス枠・フッターマーカー）は純コンテンツ層を壊さないこと。
- `go test ./...` / `go build ./...` / `go vet ./...` 全通過。

## 非目標（YAGNI）

- 任意 N 行指定のキャプチャ。
- send-keys 等の任意シェル送出・設定ファイル。
- zellij 向け「他クライアント切断」の CLI 実行（CLI 非公開のため手動ヒントのみ）。
- フッターの ANSI 色付け（テキストマーカーで区別）。

## セキュリティ

`exec.Command(bin, args...)` 直接実行を維持。シェルを介さないためインジェクションなし。capture も同方式。`/security-review` 必須差分には当たらない。
