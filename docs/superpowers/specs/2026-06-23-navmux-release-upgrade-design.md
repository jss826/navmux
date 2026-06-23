# navmux リリースコマンド + `navmux upgrade` 設計

作成: 2026-06-23

## 目的

1. **開発者向け**: タグ付き GitHub Release を切る作業を Claude Code スラッシュコマンド `/release` に集約する。
2. **利用者向け**: インストール済みバイナリが自分を最新リリースに更新する `navmux upgrade` サブコマンドを追加する。

この 2 つは噛み合う: `/release` が生成・添付した全 OS バイナリと `SHA256SUMS` を、`navmux upgrade` が download・検証・自己置換して消費する。

```
[開発者] /release  ──tag + 全OSバイナリ + SHA256SUMS + gh release──▶  GitHub Release
                                                                          │
[利用者] navmux upgrade ──latest release 参照→DL→checksum検証→自己置換──────┘
```

## 確定した設計判断

| 軸 | 決定 |
|----|------|
| リリースコマンドの形態 | Claude Code スラッシュコマンド `.claude/commands/release.md`（Claude が実行） |
| バージョン番号 | 前タグ以降のコミットから semver を自動提案 + 承認/上書き |
| `navmux upgrade` の更新方式 | プレビルドバイナリを download し自己置換（**Go ツールチェーン不要**） |
| 自己置換の実装 | `github.com/minio/selfupdate` ライブラリ（Windows rename・rollback・checksum を担保） |
| 対応プラットフォーム | linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64（5 ターゲット） |

## コンポーネント

### 1. `.claude/commands/release.md`（新規・Claude 実行のスラッシュコマンド）

`/release [vX.Y.Z]` の手順:

1. **clean 確認** — working tree が dirty なら中止。
2. **バージョン提案** — `git describe --tags --abbrev=0` で前タグを取得し、それ以降のコミットを集計。
   - `BREAKING`/`!` を含む → major
   - `feat:` あり → minor
   - `fix:` のみ → patch
   - 提案を提示し、引数指定があればそれを優先、なければ承認/上書きを待つ。
3. **タグ作成 + push** — `git tag -a vX.Y.Z -m "release vX.Y.Z"` → `git push origin vX.Y.Z`。
   - push URL は SSH（`git@github.com-jss826:...`）。1Password SSH agent がロック時は `communication with agent failed` → アンロックで復帰（既知の罠）。
4. **クロスコンパイル** — 5 ターゲットを `GOOS`/`GOARCH` でビルド。各バイナリに
   `-ldflags "-X github.com/jss826/navmux/internal/app.version=vX.Y.Z"` でバージョンを埋め込む。
   - 出力名: `navmux_<goos>_<goarch>`（windows は `.exe` 付き）。
5. **チェックサム** — 全バイナリの SHA256 を `SHA256SUMS` に出力。
6. **Release 作成** — `gh release create vX.Y.Z --generate-notes <バイナリ群> SHA256SUMS`。

> 注: クロスコンパイル・checksum・gh release は Go コードではなくスラッシュコマンド内（Claude のツール実行）で行う。CI（GitHub Actions）は導入しない。

### 2. `internal/app/version.go`（改修・必須）

リリースバイナリは `go install` ではなく `go build` 由来のため、`debug.ReadBuildInfo().Main.Version` が
`(devel)` になる。これを補うため:

- パッケージに `var version string` を追加（`-ldflags -X` で注入可能）。
- `FormatVersion` は **注入された `version` を最優先**し、空なら従来どおり `bi.Main.Version`
  （`go install @vX.Y.Z` 経由のバージョン）→ それも空なら `(devel)` にフォールバック。
- 既存の vcs.revision/dirty 表示ロジックは維持。既存テストは壊さない（注入なし時は従来挙動）。

### 3. `internal/upgrade`（新規パッケージ）

CLAUDE.md の `runFunc` 流儀に倣い、外部 I/O（HTTP）を注入可能にして純関数部を単体テストする。

**純関数（単体テスト対象）:**
- `parseLatest(body []byte) (Release, error)` — GitHub API `releases/latest` の JSON から
  `tag_name` と `assets`（name, browser_download_url）を取り出す。
- `assetFor(assets []Asset, goos, goarch string) (Asset, bool)` — `runtime.GOOS/GOARCH` に対応する
  バイナリ資産を選ぶ。期待名は `navmux_<goos>_<goarch>`、`goos=="windows"` のときは `.exe` を付与する
  （例 `navmux_windows_amd64.exe`）。リリース step 4 の出力名と厳密一致させる。
- `isNewer(current, latest string) bool` — semver 比較。`current` が `(devel)`/空/不正なら「更新可」と判断。
- `checksumFor(sums []byte, assetName string) (string, bool)` — `SHA256SUMS` から該当行の hash を引く。

**統合部（フェイク HTTP でテスト）:**
- `Run(opts)` — latest 取得 → `isNewer` 判定 → 不要なら「最新です」で終了。必要なら対象 asset と
  `SHA256SUMS` を download → `checksumFor` で hash 特定 → `selfupdate.Apply(reader, selfupdate.Options{Checksum: ...})`
  で検証込み自己置換 → 「vX.Y.Z に更新しました」。
- HTTP は `httpGet func(url string) ([]byte, error)` 相当のフィールド注入でフェイク化する。

### 4. `cmd/navmux/main.go`（改修）

- `flag.Parse()` 後、`flag.Arg(0) == "upgrade"` なら `upgrade.Run(...)` を呼んで終了（TUI を起動しない）。
- 既存の `-version` フラグ分岐は維持。

### 5. README（改修）

インストール/アップグレード節に `navmux upgrade` を追記する（別 `docs:` コミット）。

## データフロー

```
/release vX.Y.Z
  └─ git tag + push ──▶ module proxy が @vX.Y.Z を解決
  └─ go build ×5 (+ldflags version) ──▶ SHA256SUMS ──▶ gh release create
                                                            │
navmux upgrade                                              │
  └─ GET releases/latest ◀────────────────────────────────┘
  └─ isNewer? ──no──▶ 「最新です」
            └─yes──▶ GET asset + SHA256SUMS ──▶ selfupdate.Apply(checksum 検証) ──▶ 置換完了
```

## エラーハンドリング

- `/release`: dirty tree / タグ重複 / push 失敗（SSH agent）/ gh 認証なし → 各段で中止しメッセージ。
- `navmux upgrade`: ネットワーク失敗 / 対応 asset なし / checksum 不一致 / 書込権限なし →
  `selfupdate` のロールバックで現行バイナリを保全しつつエラー終了。

## テスト

- `internal/upgrade` の純関数（parseLatest/assetFor/isNewer/checksumFor）を単体テスト。
- 統合 `Run` はフェイク HTTP で「最新時は何もしない」「新しい時に Apply を呼ぶ」を検証。
- `internal/app` の version 注入あり/なしの分岐をテスト。
- 品質ゲート（`go test ./...` / `go build ./...` / `go vet ./...`）を全通過させる。

## セキュリティ

- `navmux upgrade` は **新規の外部入力（ネットワーク download）+ 実行バイナリ置換**面を追加する。
- SHA256 検証（GitHub の `SHA256SUMS`、HTTPS 経由）で改ざんを緩和。TLS 信頼が前提。コード署名は MVP 非対象。
- CLAUDE.md §2 のシェルインジェクション非発生方式（`exec.Command(bin, args...)` 直接実行・シェル非経由）は崩さない。
  download は文字列連結のコマンド組み立てではない。
- **この差分は `/security-review` を実施する**（外部入力面の追加に該当）。

## 依存追加

- `github.com/minio/selfupdate` のみ。GitHub API アクセスは stdlib（`net/http` + `encoding/json`）で行う。

## スコープ外（YAGNI）

- GitHub Actions / CI パイプライン（クロスコンパイルはスラッシュコマンド内で行う）。
- コード署名・GPG 署名。
- `navmux upgrade --check`（dry-run）等のサブフラグ（必要になれば後付け）。
- 自動更新チェック（起動時の latest 問い合わせ）。
