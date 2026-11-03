# okf-export

Open Knowledge Format（OKF）のバンドルを、GitHub 風の見た目の静的サイトとして書き出すツールです。

- **ページごとの出力**：`.md` 1つにつき `.html` を1ページ出力し、CSV/TSV のページとフォルダごとの一覧ページも作ります。
- **1ファイルの出力（`-single`）**：全ページを1つの HTML に入れ、`#/tables/x.html` のような URL のハッシュで表示するページを切り替えます。
- **ライブプレビュー（`serve`）**：ファイルを保存すると、開いているブラウザが自動で再読み込みします。

どの方式でも CSS、画像、ダウンロード用のファイルを埋め込みます。CDN もフォントも使わないので、オフラインでも表示できます。ライト／ダークは OS の設定に合わせて切り替わり、スマホ幅にも対応しています。

## 使い方

```
go build -o okf-export .

./okf-export -in path/to/bundle -out site              # ページごと
./okf-export -in path/to/bundle -single -out site.html # 1ファイル
./okf-export serve -in path/to/bundle                  # http://127.0.0.1:8080/
./okf-export serve -in path/to/bundle -addr :8080      # 同じ LAN のスマホからも見る
```

| フラグ | 説明 |
| --- | --- |
| `-in` | OKF バンドルのディレクトリ（必須） |
| `-out` | 出力先のディレクトリ。`-single` のときは出力ファイル（必須） |
| `-single` | 1つの HTML ファイルに書き出す |
| `-addr` | `serve` の待ち受けアドレス（既定は `127.0.0.1:8080`） |
| `-name` | ヘッダーに出す名前（省略時は入力ディレクトリ名） |
| `-unsafe` | Markdown 中の生 HTML をそのまま出力する（省略時は除去） |

## 変換の内容

| 入力 | 出力 |
| --- | --- |
| `a/b.md` | `a/b.html` |
| `a/index.md` | `a/index.html`（フォルダ一覧の下に表示） |
| フォルダ | `<dir>/index.html`（ファイル一覧。`index.md` がなければ `README.md` を表示） |
| `a/x.csv`・`a/x.tsv` | `a/x.csv.html`（GitHub と同じく行番号付きの表。ダウンロードボタン付き） |
| `/x.md`・`../x.md`・`x/`・`x.csv` へのリンク | 出力ページへのリンク |
| 画像・その他のローカルファイル | data URI で埋め込み（画像以外は元のファイル名でダウンロード） |
| フロントマター | タイトル、説明、type、status、tags を見出し部分に表示。全項目は「Metadata」に表として表示 |
| `[^id]`（`sources` の id） | ページ末尾の Sources への番号付きリンク |
| 被リンク | ページ末尾に「Cited by」として表示 |

## ファイルの取り込み（Crossnote / Markdown Preview Enhanced 互換）

次のどれかを単独の行に書くと、ファイルの内容がその場所に取り込まれます。コードブロックの中に書いたものは取り込みません。

```
@import "data.csv"
<!-- @import "data.csv" -->
![](data.csv)
![[data.csv]]
```

| ファイル | 取り込まれ方 |
| --- | --- |
| `.csv` / `.tsv` | 表（1行目が見出し） |
| 画像 | 埋め込み画像 |
| `.md` | 本文をそのまま展開（フロントマターは除き、入れ子の取り込みも可。循環は検出してスキップ） |
| その他 | 拡張子を言語名にしたコードブロック |

`![[name]]` は相対パスで見つからない場合、バンドル全体からファイル名で探します。文の途中の `![](x.csv)` は、表のページへのリンクになります。

## 制限

- 表として描画するのは 1,000 行までです。それより後はダウンロードしたファイルで見てください。
- `@import` で取り込んだ `.md` の中の相対リンクは、取り込んだ側のファイルを基準に解決されます。
- オンラインのファイル（`@import "https://..."`）と、Crossnote の PDF・PlantUML・Graphviz・Mermaid の描画には対応していません。
- 解決できないリンクや取り込みは、そのまま残して標準エラーに警告を出します。

## インストール

```
go install github.com/Mokkori-Tom/okf-export@latest
```

## ライセンス

MIT。`assets/github-markdown.css` は [github-markdown-css](https://github.com/sindresorhus/github-markdown-css)（MIT, © Sindre Sorhus）を同梱しています。
