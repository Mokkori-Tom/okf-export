# mdBook との比較

同じ OKF バンドルを okf-export と [mdBook](https://rust-lang.github.io/mdBook/) で書き出し、同じページのスクリーンショットを横に並べた `report.html` を作ります。見た目や操作性が mdBook からどれだけ離れているかを確かめるためのものです。

```sh
tools/compare-mdbook/compare.sh path/to/bundle compare-out
# → compare-out/report.html
```

必要なもの:

- `mdbook`（`cargo install mdbook` か、公式のリリースのバイナリ）
- `node` と `playwright` パッケージ、それ用の Chromium（`npm i playwright && npx playwright install chromium`）
- `okf-export`（PATH に無ければこのリポジトリから `go build` します）

やること:

1. バンドルを `compare-out/book/src` に写します。mdBook はフロントマターをそのまま表示するので、`.md` のフロントマターは除きます。
2. `okf-export summary` で、okf-export のサイドバーと同じ並びの `SUMMARY.md` を作ります。`index.md` の無いフォルダは、mdBook では下書きの章（`[名前]()`）になります。
3. `mdbook build` と `okf-export -out` で書き出します。どちらも `x.md` は `x.html` になるので、同じパスのページを比べます。
4. デスクトップ（1280px）とスマホ（390px）、Light と Navy のテーマで撮影し、`report.html` に並べます。
