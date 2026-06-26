# 設計: zellij ゾンビセッションの検出 + ワンキー掃除

- 日付: 2026-06-26
- 対象: navmux（zellij backend、主に Windows。Linux も射程）
- ステータス: 設計承認済み（実装プラン未作成）

## 背景 / 問題

`den2` というセッションが `navmux`（および `zellij` 直叩き）で見えているのに、アタッチしても何も始まらず固まる事象が観測された。切り分けの結果:

- `zellij list-sessions -n` は `den2` を `EXITED` マークなしで「生きているように」表示していた。
- 実プロセスを走査すると、`--server …\<name>` の **zellij server プロセスが存在しなかった**（クライアントだけがサーバー待ちでハングしていた）。
- ソケットファイル `…\Temp\zellij\contract_version_1\den2` が 8 時間前のまま残骸として残っていた。
- ソケットを削除すると `zellij` は `den2` を `(EXITED - attach to resurrect)` と正しく認識し、`delete-session` で完全に消えた。

### 根本原因の構造

1. zellij はセッションの生死をソケットの有無で判定する。
2. サーバープロセスが異常終了してもソケットファイルが残ると、`list-sessions` は `EXITED` を付けず「生きている」と誤表示する。
3. navmux は `list-sessions` の出力に `EXITED` 文字列があるかだけで生死を判定している（`parseZellijList`、`internal/backend/zellij.go`）。そのため `Dead=false`＝「生きている」と表示し、アタッチでハングする。

このゾンビ生成自体は、navmux が v0.3.1 で修正した「実端末を持たないデタッチ起動だと zellij server が即死する」のと同型の現象。`den2` は外部ツール `den`（zellij ラッパー）が生成したもので navmux 管理外だが、**navmux の一覧に出る以上、見抜いて掃除できるべき**というのが本設計の動機。

## 目標 / 非目標

### 目標
- `list-sessions` だけでは生死が確定できないゾンビ（サーバー不在 + ソケット残骸）を navmux が検出する。
- ゾンビを UI で減光表示し、ハングするアタッチ操作を出さない。
- ゾンビ／EXITED セッションを navmux からワンキーで掃除（ソケット削除 + `delete-session -f`）できる。

### 非目標
- アタッチのハングそのもののタイムアウト回避（今回は一覧表示の時点で見抜く方向を採る）。
- navmux 自身がゾンビを生成しない保証（v0.3.1 で対応済み。ExecProcess 経由生成）。
- tmux backend への対応（tmux はこのゾンビ概念を持たず常に `Dead=false`）。
- ソケット mtime ベースの発見的判定（誤検出を避けるためプロセス走査で確実判定する）。

## 設計

### ① 検出（ロジックは backend に寄せる）

`Session` に状態を 1 つ追加する:

```go
type Session struct {
    Name     string
    Attached bool
    Windows  int
    Dead     bool // zellij の EXITED（resurrect 可能な正規の死）
    Zombie   bool // サーバー不在なのに list に生きて見える応答なし状態
}
```

`Dead`（EXITED、再アタッチで復活可）と `Zombie`（アタッチでハング）は意味が異なるため分離する。

判定フロー（zellij backend、`List()` の後段）:
1. `zellij list-sessions -n` をパース（既存どおり。`EXITED` → `Dead=true`）。
2. `Dead` でないセッションについて、実行中の zellij server プロセスのコマンドラインを走査する。
3. `--server …\<name>`（パス末尾がセッション名）に一致する server プロセスが**実在しなければ `Zombie=true`**。

OS 依存はビルドタグで分離（既存 `internal/ui/exec_windows.go` / `exec_other.go` と同じ流儀）:
- `liveness_windows.go` … `tasklist /v` 等でプロセスのコマンドライン取得
- `liveness_other.go` … `ps` でコマンドライン取得

### テスト容易性

プロセス一覧の取得を注入可能な関数（`runFunc` と同じ要領）にし、純関数として単体テストする:

- 入力: server プロセスのコマンドライン一覧 + パース済みセッション一覧
- 出力: `Zombie` を埋めたセッション一覧

実バイナリ・実プロセスなしで「server があるセッションは生存 / ないセッションはゾンビ」を検証できる。OS 依存ファイルは「コマンドライン一覧を取得する」だけの薄い層に留める。

### ② UI 表示

- ゾンビは EXITED と同様に**減光**し、ラベルに状態ヒント（例 `応答なし`）を併記する。
- ゾンビのアタッチ操作は**無効化**（ハングする操作をそもそも出さない）。
- 既存の装飾方針（可＝シアン / 不可＝減光、記号は使わない）に乗せる。純コンテンツ層（`RenderList`）はプレーン維持。装飾は `RenderFooter` / `RenderMenu` 側。

### ③ ワンキー掃除

今回の手動手順を機能化する。ゾンビ／EXITED セッションに対し:

1. ソケットファイル `…\Temp\zellij\contract_version_1\<name>` を削除。
   **シェルを介さず Go の `os.Remove` で直接削除**する（既存の「シェルインジェクション非発生」方針を維持。`del`/`rm` をシェル経由で呼ばない）。
2. `zellij delete-session -f <name>`（resurrect キャッシュも消す）。

- **透明性原則**: 削除するソケットの絶対パスと `delete-session` コマンドを画面に表示・コピー可能にする。
- 実行は既存 Kill と同じ確認導線に乗せる。

#### 掃除がコマンドビルダー方針の例外になる点

navmux の規律は「変更系は `Command{Argv, Display}` ビルダーで返し、実行は runner 経由」。ソケット削除はシェルコマンドにならない（OS 依存のファイル操作）。これを `del`/`rm` でシェル経由実行すると規律違反かつインジェクション面が生じるため、**`os.Remove` による直接削除**を採る。`Display` には削除対象パスを人間可読で示し、透明性は担保する。`delete-session` は従来どおり `Command` ビルダーで生成する。

## コマンド対応表への影響（CLAUDE.md 正本を実装前に更新）

新規操作「掃除（ゾンビ/EXITED）」を追加:

| 操作 | tmux | zellij 0.44.3 |
|------|------|---------------|
| 掃除（ゾンビ/EXITED） | （対象外。tmux はゾンビ概念なし） | `os.Remove(<socket path>)` の後 `zellij delete-session -f <name>` |

`Display` は「ソケットパス削除 + `zellij delete-session -f <name>`」を厳密一致で示す。

## 影響範囲（変更対象ファイル）

- `internal/backend/backend.go` … `Session.Zombie` 追加
- `internal/backend/zellij.go` … `List()` 後段でゾンビ判定、掃除コマンド/操作の追加
- `internal/backend/liveness_windows.go`（新規）… Windows のプロセスコマンドライン取得
- `internal/backend/liveness_other.go`（新規）… 非 Windows のプロセスコマンドライン取得
- `internal/backend/zellij_test.go` … ゾンビ判定の純関数テスト、掃除コマンド生成テスト
- `internal/ui/render.go` / `menu.go` … ゾンビの減光表示・アタッチ無効化
- `internal/ui/*_test.go` … 装飾/無効化の検証テスト
- `CLAUDE.md` … コマンド対応表に「掃除」行を追加
- tmux backend は `Zombie=false` 固定。判定ロジックは持たない。

## 品質ゲート（検証フェーズで全通過）

```
go test ./...
go build ./...
go vet ./...
```

- zellij 実機スモークは手動・観察ベース（ゾンビを意図的に作って検出・掃除を確認）。TTY 必須のためユーザーのターミナルで実行。

## セキュリティ

- ソケット削除はユーザー入力（セッション名）を含むパス操作になる。`os.Remove` に渡す前に **セッション名がパス区切り・`..` を含まないこと**を確認する（zellij のセッション名規則上は安全側だが、防御的に検証）。シェルは介さない。
- 上記方針（シェル非経由・直接 `exec` / `os` 呼び出し）を維持する限り、追加のセキュリティレビューは不要。
