# navmux

tmux / zellij のセッションを **メニュー駆動** で操作する TUI（Go 製）。

**設計原則は「何も覚えなくていい / 使ううちに覚える」。**

- 操作は基本 **矢印キー + Enter**。隠しキーやモード切替を覚える必要はない。
- メニュー方式だが **各アクションにキーを併記**（`enter アタッチ / n 新規 / d 削除`）。覚えたい人はそのまま移行できる。
- 実行されるコマンドを **画面に表示し、コピーできる**。覚えなくても困らないし、写経して学ぶこともできる。

## インストール

**プレビルドバイナリ**（Go 不要）か、**`go install` / ソースビルド**（Go 1.25 以降が必要）から選べます。

### プレビルドバイナリ（Go 不要）

[Releases ページ](https://github.com/jss826/navmux/releases/latest) から OS/CPU に合うバイナリを download します。Go ツールチェーンは不要で、入れたあとは [`navmux upgrade`](#アップグレード) で最新へ自己更新できます。

| OS | CPU | ファイル名 |
|----|-----|-----------|
| Linux | x86_64 | `navmux_linux_amd64` |
| Linux | ARM64 | `navmux_linux_arm64` |
| macOS | Intel | `navmux_darwin_amd64` |
| macOS | Apple Silicon | `navmux_darwin_arm64` |
| Windows | x86_64 | `navmux_windows_amd64.exe` |

**macOS / Linux**（例: Linux x86_64）:

```sh
ver=v0.2.0
curl -fsSLO https://github.com/jss826/navmux/releases/download/$ver/navmux_linux_amd64
curl -fsSLO https://github.com/jss826/navmux/releases/download/$ver/SHA256SUMS
sha256sum --ignore-missing -c SHA256SUMS   # → navmux_linux_amd64: OK を確認
chmod +x navmux_linux_amd64
mv navmux_linux_amd64 ~/.local/bin/navmux  # PATH の通ったディレクトリへ
```

**Windows**（PowerShell, x86_64）:

```powershell
$ver = "v0.2.0"
Invoke-WebRequest "https://github.com/jss826/navmux/releases/download/$ver/navmux_windows_amd64.exe" -OutFile navmux.exe
# SHA256 を検証し、Releases の SHA256SUMS の navmux_windows_amd64.exe 行と一致するか確認
(Get-FileHash navmux.exe -Algorithm SHA256).Hash.ToLower()
# PATH の通ったディレクトリ（例: %USERPROFILE%\go\bin）へ移動する
```

検証用の `SHA256SUMS` も同じ Releases に添付されています。

### go install（Go ユーザー向け）

`$GOBIN`（未設定なら `$GOPATH/bin`、既定 `~/go/bin`）に `navmux` バイナリが入ります。このディレクトリを `PATH` に通しておいてください。

```sh
# 最新を入れる
go install github.com/jss826/navmux/cmd/navmux@latest

# バージョンを固定して入れる
go install github.com/jss826/navmux/cmd/navmux@v0.1.0
```

> **`go install` は PATH を変更しません。** バイナリを上記ディレクトリに置くだけなので、`navmux` がコマンドとして見つからない場合はそのディレクトリを PATH に通してください。

#### Windows での PATH 設定

Go の公式インストーラ（MSI）が PATH に追加するのは Go ツールチェーン本体（`C:\Program Files\Go\bin`）だけで、`go install` の出力先 `%USERPROFILE%\go\bin` は**自動では追加されません**。新しい環境では一度だけ手動で通す必要があります。

PowerShell で以下を一度実行します（既に通っていれば何もしない安全版）:

```powershell
$go = "$(go env GOPATH)\bin"
$cur = [Environment]::GetEnvironmentVariable('Path','User')
if (($cur -split ';') -notcontains $go) {
  [Environment]::SetEnvironmentVariable('Path', "$cur;$go", 'User')
  "追加しました: $go（新しいターミナルから有効）"
} else { "既に PATH にあります: $go" }
```

設定は**新しく開いたターミナルから有効**になります（実行中のターミナルには即反映されません）。確認は `Get-Command navmux` または `where.exe navmux`。

#### macOS / Linux での PATH 設定

シェルの設定ファイル（`~/.bashrc` / `~/.zshrc` 等）に追記します:

```sh
export PATH="$PATH:$(go env GOPATH)/bin"
```

### ソースからビルド

```sh
git clone https://github.com/jss826/navmux.git
cd navmux
go build -o navmux ./cmd/navmux   # カレントに navmux を出力
# もしくは開発中の直接実行
go run ./cmd/navmux
```

### バージョン確認

```sh
navmux -version
# 例) navmux v0.1.0 (a1b2c3d4e5f6)
#     navmux (devel)               ← タグなしのローカルビルド
```

バージョン情報は `go install`／`go build` 時にビルドメタデータ（モジュールバージョンと VCS リビジョン）から自動で埋め込まれます。`-ldflags` の指定は不要です。

### 起動

起動すると、利用可能な multiplexer（tmux / zellij）が自動検出されます。いま multiplexer の中にいる場合は、その multiplexer が最初のタブに来ます。tmux も zellij も見つからない場合はエラー終了します。

## アップグレード

### ビルド済みバイナリで入れた場合

ビルド済みバイナリを使っている場合は、navmux 自身で最新リリースに更新できます:

```sh
navmux upgrade
```

最新リリースを参照し、OS/CPU に合うバイナリを download・SHA256 検証して自己置換します（Go ツールチェーン不要）。すでに最新なら何もしません。

### go install で入れた場合

同じコマンドを再実行するだけです。

```sh
go install github.com/jss826/navmux/cmd/navmux@latest   # 最新へ
go install github.com/jss826/navmux/cmd/navmux@v0.2.0   # 特定バージョンへ
```

`@latest` がキャッシュで古いバージョンを拾うときは、モジュールキャッシュを無視して取り直します。

```sh
GOFLAGS=-mod=mod GOPROXY=direct go install github.com/jss826/navmux/cmd/navmux@latest
```

### ソースから入れた場合

```sh
cd navmux
git pull
go build -o navmux ./cmd/navmux   # 再ビルド
```

アップグレード後は `navmux -version` で反映を確認してください。

## 使い方

起動するとセッション一覧が表示されます。カーソルを合わせて操作を選ぶだけです。

```
navmux — zellij

>   main
    work
  * dev          ← * は現在アタッチ中

enter アタッチ   n 新規   r リネーム   d 削除   ? 解説   tab tmux/zellij   q 終了
```

- 行頭 `>` がカーソル、`*` がアタッチ中のセッション。
- `?` で選択中アクションの **解説と実行コマンド** を表示し、`y` でそのコマンドをクリップボードへコピーできます。

### キー操作

| キー | 操作 |
|------|------|
| `↑` / `k`, `↓` / `j` | カーソル移動 |
| `enter` | 選択中セッションにアタッチ |
| `n` | 新規セッション作成（名前を入力 → detached で作成） |
| `r` | リネーム（zellij では非対応のためグレーアウト） |
| `d` | 削除（`y` で確定 / その他でキャンセル） |
| `?` | 解説・実行コマンドの表示トグル |
| `y` | 表示中の実行コマンドをコピー |
| `tab` | tmux / zellij タブの切替 |
| `q` / `Ctrl-C` | 終了 |

### アタッチの挙動

| 状況 | 方式 |
|------|------|
| multiplexer の **外** から | `attach` を子プロセスで起動し端末を渡す。detach すると navmux の一覧に戻る（`exec` 置換しないので Windows でも動く）。 |
| **tmux の中** から | `tmux switch-client -t <name>`（IPC で切替）。 |
| **zellij の中** から | セッション内切替が構造的に弱いため、MVP は「外から起動」を主ユースケースにする。 |

## 実行されるコマンド対応表

navmux は変更系操作を **シェルを介さず直接実行** します（`exec.Command(bin, args...)`）。画面に表示・コピーされるコマンド文字列は、実際に実行されるものと同一です。

| 操作 | tmux | zellij (0.44.3) |
|------|------|------------------|
| 一覧 | `tmux list-sessions -F …` | `zellij list-sessions -n` |
| 新規 (detached) | `tmux new-session -d -s <name>` | `zellij attach -b <name>` |
| リネーム | `tmux rename-session -t <old> <new>` | **非対応** |
| 削除 | `tmux kill-session -t <name>` | `zellij delete-session -f <name>` |
| アタッチ（外から） | `tmux attach -t <name>` | `zellij attach <name>` |
| 切替（中から） | `tmux switch-client -t <name>` | **非対応**（外から attach にフォールバック） |

### zellij 0.44.3 の制約

- **detached のリネーム不可** → `r` はグレーアウト表示。
- **セッション内切替不可** → 外から attach にフォールバック。
- ウィンドウ数を CLI 公開しないため、一覧にウィンドウ数は出ない。
- 終了済み（EXITED）セッションは一覧に表示され、削除・再アタッチの対象にできる。

## プラットフォーム

- **zellij**: Windows / Linux / macOS で動作（実機検証は zellij 0.44.3）。
- **tmux**: Windows ネイティブ非対応。tmux 向けのコマンド生成・パースは単体テストで担保し、実機検証は Linux で行う。

## 開発

```sh
go test ./...     # 全テスト
go build ./...    # ビルド
go vet ./...      # 静的解析
```

詳細な設計規律・開発プロセスは [CLAUDE.md](./CLAUDE.md) を参照してください。

## ライセンス

[MIT License](./LICENSE)
