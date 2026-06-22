# navmux — プロジェクト指示

tmux / zellij のセッションを **メニュー駆動** で操作する TUI（Go）。「何も覚えなくていい / 使ううちに覚える」が設計原則。

## 設計原則

- 操作は基本 **矢印キー + Enter**。隠しキー・モード切替に依存しない。
- メニュー方式だが **各アクションにキーを併記**（例 `enter アタッチ / n 新規 / d 削除`）。覚えたい人への移行路を残す。
- 実行されるコマンドを **画面に表示し、コピーできる**（覚えなくても困らない / 写経もできる）。

## アーキテクチャ規律

- multiplexer は `internal/backend` の `Backend` interface で抽象化。UI 層は具体 multiplexer を知らない。
- **変更系操作は実行せず `Command{Argv, Display}` を返すビルダー**に統一する（`AttachCmd` / `NewCmd` / `RenameCmd` / `KillCmd` / `SwitchCmd`）。実行は runner 経由に一元化する。
- `Command.Display` は `strings.Join(Argv, " ")` で導出する（`cmd()` ヘルパ）。コピー/表示はこの文字列を使う。
- 一覧パースは注入可能な `runFunc` でフェイク化し、純関数（`parseTmuxList` / `parseZellijList`）として単体テストする。
- ロジックは backend / action 側に寄せ、UI（Bubble Tea）は薄く保つ。

## コマンド対応表（正本）

**コマンド文字列の正本はこの表**。`Display` はこの表に厳密一致させる（表示・コピー・実行が 1 定義から出るため、ここがズレると 3 箇所同時にズレる）。コマンドに触れる変更はこの表を先に更新する。

| 操作 | tmux | zellij 0.44.3 |
|------|------|---------------|
| 一覧 | `tmux list-sessions -F "#{session_name}\|#{?session_attached,1,0}\|#{session_windows}"` | `zellij list-sessions -n`（`name [Created ...] (current)/(EXITED ...)` を parse） |
| 新規(detached) | `tmux new-session -d -s <name>` | `zellij attach -b <name>`（`-b`/`--create-background`） |
| リネーム | `tmux rename-session -t <old> <new>` | **非対応** → `RenameCmd` は `(Command{}, false)` |
| 削除 | `tmux kill-session -t <name>` | `zellij delete-session -f <name>`（`-f` で起動中なら kill してから削除） |
| アタッチ(外から) | `tmux attach -t <name>` | `zellij attach <name>` |
| 切替(中から) | `tmux switch-client -t <name>` | **非対応** → `SwitchCmd` は false |

### zellij 0.44.3 の制約（レビュー観点）

- detached のリネーム不可 → `RenameCmd` は `(Command{}, false)`、UI は `CanRename()` で r をグレーアウト。
- セッション内切替不可 → `SwitchCmd` は false（外から attach にフォールバック）。
- ウィンドウ数を CLI 公開しない → `Session.Windows = 0`。
- EXITED セッションは `Session.Dead = true` で一覧表示し、削除/再アタッチの対象にできる。

## アタッチの実装方式

| 状況 | 方式 |
|------|------|
| multiplexer の **外** から | `attach` を子プロセスで起動し TTY を渡して待つ。detach したら navmux のリストに戻る。`exec` 置換しないので Windows でも動く。 |
| **tmux の中** から | `tmux switch-client -t <name>`（IPC で切替）。 |
| **zellij の中** から | 切替が構造的に弱いため、MVP は「外から起動」を主ユースケースにする。 |

## 開発プロセス（flow build プロファイル）

新規実装・1 Issue 実装は flow の build プロファイル（Phase 1-7）に沿って進める。

1. **要件確認** — 影響範囲（変更対象ファイル）を確定。
2. **設計** — コマンドに触れる変更は本ファイルの「コマンド対応表」を先に更新。
3. **実装** — **TDD 必須**（`superpowers:test-driven-development`）。各タスクは「失敗テスト → 実行で赤確認 → 最小実装 → 緑確認 → commit」のバイトサイズ。**各タスクのチェックポイントで止めて報告する**。
4. **テスト** — テスト先行。実バイナリ無しで成立する単体テストを基本にする（コマンド生成・パースはフェイク `runFunc`）。
5. **検証** — 下記「① 品質ゲート」を全実行し、**stdout のエビデンスを引用**してから完了を宣言する（`superpowers:verification-before-completion`）。
6. **出荷** — タスク毎に逐次 commit。`/code-review` を実施。
7. **振り返り** — プロセス・ルールの改善点を確認。

### ブランチ / コミット / マージ

- ブランチ: 既定は `feat/<説明>` 等。**ただしユーザーが「main 直接」を選んだ場合は main へ直接 commit**（その場合 squash merge フェーズは行わない）。
- コミットメッセージ: `feat:` / `fix:` / `docs:` / `chore:` プレフィックス。日本語可。
- **push は明示指示があるまで行わない**（破壊操作はゲート）。

## プロジェクトスロット（flow 委譲スロット）

### ① 品質ゲート（検証フェーズで全通過させる）

```
go test ./...     # 全 PASS
go build ./...    # exit 0
go vet ./...      # 警告なし
```

- zellij 実機スモークは**手動・観察ベース**（一覧/新規/削除/アタッチ(detach 復帰)/コピー）。CI 自動化対象外。
- tmux は Windows ネイティブ非対応。tmux backend はビルダー/パースの単体テストで担保し、実機検証は Linux で別途。

### ② セキュリティ対象差分（`/security-review` を必須とする差分）

- MVP は認証・セッション・テナント境界・SQL・外部入力フォームを持たない。
- 留意点は `exec.Command` にセッション名（ユーザー入力）を渡す箇所。**シェルを介さず `exec.Command(bin, args...)` で直接実行**する限りシェルインジェクションは発生しない（この方式を維持すること）。
- 上記方式を崩す変更（シェル経由実行・文字列連結でのコマンド組み立て）を入れる場合のみ `/security-review` を実施する。

### ③ 実行上の罠

- **Bash ツール制約**: `echo` / `cat` / `head` / `tail` / `sed` / `awk` / `find` / `grep` / `pwd` 等はトップレベルでブロックされる → ファイルは Read/Write/Edit、検索は Grep/Glob を使う。
- `go test ./...` / `go build ./...` は短時間なので**前景実行**でよい。長時間化したら `run_in_background`。
- `cmd | tail` のトップレベルパイプは ConPTY 経由でハングし得る → 出力を絞るなら Read で出力ファイル末尾を読むか `go test` 側で絞る。
- 環境: シェルは PowerShell が主。zellij は `C:\Users\soon7\AppData\Local\Zellij\zellij.exe`（0.44.3）でスモーク可。tmux は無い。

### ④ フェーズ拡張（追加チェック）

- **設計フェーズ**: コマンドに触れる変更は「コマンド対応表」を先に更新し、実装の `Display` 文字列と厳密一致させる。
- **レビュー観点**: 上記「zellij 0.44.3 の制約」を破っていないか確認する。
- **MVP 制約**: プレーン文字列描画を維持（lipgloss の ANSI 装飾は MVP 後）。テストが substring 一致のため。

## 技術スタック

- Go 1.25（module: `github.com/jss826/navmux`）
- Bubble Tea（bubbletea / bubbles/textinput）+ atotto/clipboard
- 検証済み実機: zellij 0.44.3
