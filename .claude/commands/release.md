---
description: タグ付き GitHub Release を切る（version 提案 + クロスコンパイル + checksum + gh release）
---

navmux のリリースを行う。引数 `$ARGUMENTS` にバージョン（例 `v0.2.0`）があれば優先、なければ提案する。

手順（各段で失敗したら中止して報告）:

1. **clean 確認**: `git status --short` が空でなければ中止。
2. **バージョン決定**:
   - 前タグ: `git describe --tags --abbrev=0`
   - それ以降のコミット: `git log --oneline <前タグ>..HEAD`
   - 判定: コミットに `BREAKING`/`!:` を含む→major / `feat:` あり→minor / `fix:` のみ→patch。
   - `$ARGUMENTS` 指定があればそれを採用。なければ提案を提示し承認を得る。
3. **タグ作成 + push**:
   - `git tag -a <ver> -m "release <ver>"`
   - `git push origin <ver>`
   - push が `communication with agent failed` で失敗したら 1Password SSH agent のアンロックを促す。
4. **クロスコンパイル**（5 ターゲット、各 ldflags でバージョン埋め込み）:
   ```
   GOOS=linux   GOARCH=amd64 go build -ldflags "-X github.com/jss826/navmux/internal/app.version=<ver>" -o dist/navmux_linux_amd64       ./cmd/navmux
   GOOS=linux   GOARCH=arm64 go build -ldflags "-X github.com/jss826/navmux/internal/app.version=<ver>" -o dist/navmux_linux_arm64       ./cmd/navmux
   GOOS=darwin  GOARCH=amd64 go build -ldflags "-X github.com/jss826/navmux/internal/app.version=<ver>" -o dist/navmux_darwin_amd64      ./cmd/navmux
   GOOS=darwin  GOARCH=arm64 go build -ldflags "-X github.com/jss826/navmux/internal/app.version=<ver>" -o dist/navmux_darwin_arm64      ./cmd/navmux
   GOOS=windows GOARCH=amd64 go build -ldflags "-X github.com/jss826/navmux/internal/app.version=<ver>" -o dist/navmux_windows_amd64.exe ./cmd/navmux
   ```
   （PowerShell では `$env:GOOS="linux"; $env:GOARCH="amd64"; go build ...` の形に読み替える）
5. **SHA256SUMS 生成**: `dist/` 内の全バイナリの SHA256 を `dist/SHA256SUMS` に `<hex>␣␣<basename>` 形式で出力する。**必ず `dist/` の中で実行してパスを basename に保つ**（`navmux upgrade` の `checksumFor` は basename 一致で引くため、`dist/navmux_...` のようにパス前置されると全 upgrade が checksum 不一致で失敗する）:
   ```
   cd dist && sha256sum --text navmux_linux_amd64 navmux_linux_arm64 navmux_darwin_amd64 navmux_darwin_arm64 navmux_windows_amd64.exe > SHA256SUMS && cd ..
   ```
   `--text` を付けるのは、Windows(MSYS) の `sha256sum` が既定で binary モードになり name に `*` が付く（`<hex>␣*<name>`）ため。`checksumFor` は `*` 付きも受けるが、生成側も text 形式に揃える。
   （PowerShell では各ファイルに `Get-FileHash -Algorithm SHA256` を使い `<hex>␣␣<basename>` 形式で出力する。hex は小文字化する）
6. **Release 作成**: `gh release create <ver> --generate-notes dist/navmux_linux_amd64 dist/navmux_linux_arm64 dist/navmux_darwin_amd64 dist/navmux_darwin_arm64 dist/navmux_windows_amd64.exe dist/SHA256SUMS`
7. **security-review**: 本リリースに upgrade 関連の差分が含まれる場合は `/security-review` を実施して報告する。
8. `dist/` は成果物なので `.gitignore` に含まれていることを確認（無ければ追加）。
