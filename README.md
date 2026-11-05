# okf-export

Open Knowledge Format（OKF）の Markdown 文書を、[mdBook](https://rust-lang.github.io/mdBook/) のような静的ドキュメントサイトとして書き出すツールです。Go の単一バイナリで動きます。

- **ページごとの出力**：`.md` 1つにつき `.html` を1ページ出力し、フォルダ・CSV・画像などのページも作ります。
- **1ファイルの出力（`-single`）**：全ページを1つの HTML に入れ、`#/tables/x.html` のような URL のハッシュで表示するページを切り替えます。
- **ライブプレビュー（`serve`）**：ファイルを保存すると、開いているブラウザが自動で再読み込みします。

画面は mdBook と同じ構成です。左にディレクトリ構造から作った目次（サイドバー）、上にメニューバー（目次の開閉・テーマ・検索・印刷）、左右に前後のページへの矢印があります。CSS・画像・検索の索引はすべて出力に含まれ、CDN も Web フォントも使わないので、オフラインでも、ファイルを直接開いても表示できます。

## 使い方

```
go build -o okf-export .

./okf-export -in path/to/bundle -out site              # ページごと（site/index.html を開く）
./okf-export -in path/to/bundle -single -out site.html # 1ファイル
./okf-export serve -in path/to/bundle                  # http://127.0.0.1:8080/
./okf-export serve -in path/to/bundle -addr :8080      # 同じ LAN のスマホからも見る
./okf-export summary -in path/to/bundle                # 目次を mdBook の SUMMARY.md として出力
```

| フラグ | 説明 |
| --- | --- |
| `-in` | OKF バンドルのディレクトリ（必須） |
| `-out` | 出力先のディレクトリ。`-single` のときは出力ファイル（必須） |
| `-single` | 1つの HTML ファイルに書き出す |
| `-addr` | `serve` の待ち受けアドレス（既定は `127.0.0.1:8080`） |
| `-name` | メニューバーに出す名前（省略時は入力ディレクトリ名） |
| `-unsafe` | Markdown 中の生 HTML をそのまま出力する（省略時は除去） |

## 画面と操作（mdBook と同じ）

| 部分 | 内容 |
| --- | --- |
| サイドバー | バンドルの目次。先頭がトップページ（ルートの `index.md`）、そのあとフォルダごとに、文書（ファイル名順）とサブフォルダ（名前順）を並べます。見出しはフロントマターの `title`（なければ本文の最初の `#` 見出し、なければファイル名）、フォルダはフォルダ名です。今のページが色付きになり、`❱` で節を折りたためます。幅は右端をドラッグして変えられ、開閉と幅は次のページでも保たれます。 |
| メニューバー | 目次の開閉、テーマ、検索、バンドル名（トップページへのリンク）、印刷 |
| テーマ | Light・Rust・Coal・Navy・Ayu の5つ（mdBook と同じ配色）。初めは OS の設定に合わせて Light か Navy。選んだテーマは保存されます。 |
| 前後のページ | 画面の左右の `‹` `›`（狭い画面ではページの下）と、キーボードの `←` `→`。順番はサイドバーと同じです。 |
| 検索 | `S` か `/`、または虫めがねで開きます。下の「検索」を参照。 |
| ページの道具 | 本文の右上に、元のファイルの表示（`<>`）、コピー、ダウンロード、見出しの一覧（アウトライン）。コードブロックにはコピーボタン、見出しにはそこへのリンクが付きます。 |
| 印刷 | 今のページを、サイドバーなどを除いて印刷します。 |
| スマホ | サイドバーはページの上に重なって開き、ページを選ぶと閉じます。 |

## 検索（日本語などの CJK に対応）

全ページの本文を見出しごとに区切った索引を出力に含め、ブラウザの中で検索します。

- 部分一致で探すので、日本語・中国語・韓国語も単語に区切らずに検索できます。空白で区切ると AND 検索です。
- 全角と半角（`ＡＢＣ` と `ABC`、`ｶﾅ` と `カナ`）、大文字と小文字を区別しません（Unicode の NFKC 正規化）。
- 結果は「ページのタイトル » 見出し」とその前後の文で表示し、見出しの位置へ移動します。タイトルや見出しに含むものが上に来ます。
- 結果から開いたページでは、探した語に色が付きます（`Esc` で消えます）。
- `↓` `↑` で結果を選び、`Enter` で開きます。
- 索引に入れるのは1ページにつき本文の 100,000 バイトまでです。

## OKF 文書の表示

| 入力 | 出力 |
| --- | --- |
| `a/b.md` | `a/b.html` |
| フォルダ・`a/index.md` | `a/index.html`（そのフォルダの `index.md` を表示。なければ同じ形の一覧を自動で作る） |
| フロントマター | ページの上に、`title`（本文が同じ `#` 見出しで始まるときは省略）、`description`、`type`・`status`・`tags` のラベル。`sources` は本文の後の Sources に。それ以外の項目（`resource` など）は折りたたんだ「Metadata」の表に。`description` は `<meta name="description">` にも入ります。 |
| `[^id]`（`sources` の id） | ページ末尾の Sources への番号付きリンク |
| 被リンク | ページ末尾の「Cited by」（`okf backlinks` と同じく、`index.md`・`log.md` からのリンクは数えない） |
| ルートの `index.md` の `okf_version` | トップページに `OKF 0.2` のようなラベル |
| `a/x.csv`・`a/x.tsv` | `a/x.csv.html`（行番号付きの表） |
| その他のテキストファイル（`.txt`・`.yaml`・`.go` など 1MB まで） | `a/x.txt.html`（行番号と色分け付き） |
| 画像 | `a/x.png.html`（画像、ファイルサイズ、縦横のピクセル数） |
| PDF | `a/x.pdf.html`（ブラウザの PDF ビューアでページ内に表示。表示できないブラウザ（スマホなど）ではダウンロードボタン） |
| その他のファイル（zip・docx など、1MB を超えるテキスト） | `a/x.zip.html`（「表示できません」とダウンロードボタン） |
| 本文の画像 | 埋め込み。クリックするとその画像のページが開く（外部の画像は元の URL を新しいタブで） |

### OKF の index.md

OKF では `index.md` はフォルダの目次で、フロントマターを持ちません。持てるのはバンドルのルートの `index.md` だけで、書けるのは `okf_version` だけです（仕様 §6・§8・§12）。

- フォルダのページは、そのフォルダの `index.md` をそのまま表示します。見出しで分けた `* [タイトル](リンク) - 説明` の一覧が、そのまま目次になります（`okf index` で作ったものも、手で書いたものも同じ）。
- `index.md` が無いフォルダでは、同じ形の一覧を自動で作ります（§8 で認められている動き）。文書はタイトルと `description`、その他のファイルとサブフォルダは名前を、Concepts・Files・Subdirectories に分けて並べます。
- ページのタイトルはフォルダ名（ルートはバンドル名）です。`index.md` の `title` などは使わず、仕様にないフロントマターには警告を出します。

## Markdown の記法

GitHub Flavored Markdown（表・取り消し線・タスクリスト・自動リンク）と脚注に加えて、[Crossnote](https://github.com/shd101wyy/crossnote)（Markdown Preview Enhanced）の記法のうち次のものに対応しています。

| 記法 | 例 |
| --- | --- |
| Wiki リンク | `[[setup]]`・`[[setup\|表示名]]`・`[[setup#見出し]]`・`[[#見出し]]`・`[[/tables/x]]`。探し方は OKF（Obsidian）と同じで、同じフォルダ、ルートからのパス、バンドル全体のファイル名の順。見つからなければ文字だけ残して警告します。 |
| 埋め込み | `![[image.png]]`（文中でも可） |
| ハイライト | `==強調したい語==` → `<mark>` |
| 目次 | `[TOC]` だけの段落 → そのページの見出しの一覧 |
| 属性 | `![](a.png){height=800px}`・`{width=300 .class #id}`・見出しの末尾の `{#id}` |
| ファイルの取り込み | 単独の行の `@import "data.csv"`・`<!-- @import "x.md" -->`・`![](data.csv)`・`![[data.csv]]` |
| GitHub のアラート | `> [!NOTE]`・`[!TIP]`・`[!IMPORTANT]`・`[!WARNING]`・`[!CAUTION]` |
| `<img>` タグ | `<img src="a.png" width="300">`（GitHub での大きさの指定。`-unsafe` なしで効きます） |

- 見出しの id は mdBook と同じく、日本語などの文字をそのまま残します（`## 設定ファイル` → `#設定ファイル`）。
- 画像の `width`・`height` は CSS として出力します（数字だけなら px）。縦横比は保ち、`height` だけの画像が画面の幅に収まらないときは高さも同じ比率で縮めます。
- `<img>` は、`<img>` だけの HTML（`<p>`・`<div>` で囲んだものも可）に限ります。使う属性は `alt`・`title`・`width`・`height`・`align` だけです。

### ファイルの取り込み

| ファイル | 取り込まれ方 |
| --- | --- |
| `.csv` / `.tsv` | 表（1行目が見出し） |
| 画像 | 埋め込み画像（`@import "a.png" {width=300}` のように属性も指定可） |
| `.md` | 本文をそのまま展開（フロントマターは除き、入れ子の取り込みも可。循環は検出してスキップ） |
| その他 | 拡張子を言語名にしたコードブロック |

コードブロックの中に書いたものは取り込みません。文の途中の `![](x.csv)` は、表のページへのリンクになります。

## mdBook との比較

[tools/compare-mdbook](tools/compare-mdbook/) に、同じバンドルを mdBook でも書き出して、同じページのスクリーンショットを並べるスクリプトがあります（mdBook と Node.js の Playwright が必要）。`okf-export summary` が出す `SUMMARY.md` を使うので、目次の並びも同じになります。

意図して mdBook と違えているところ:

- フォントは同梱せず、OS のフォントを使います（mdBook は Open Sans と Source Code Pro を同梱）。出力を小さく保ち、日本語のフォントを OS に任せるためです。
- 目次は `SUMMARY.md` ではなくディレクトリ構造から作り、章番号は付けません。
- 全ページを1つにまとめた印刷用ページ（mdBook の `print.html`）は作らず、印刷は今のページだけです。
- コードの色分けはブラウザの JavaScript（highlight.js）ではなく、書き出すときに行います（[chroma](https://github.com/alecthomas/chroma)）。
- OKF のための追加：フロントマターの表示、Sources と Cited by、ページの道具、フォルダ・CSV・画像などのページ、1ファイルの出力。

## LLM に HTML を書かせる場合

`okf-export` を実行できない環境では、[prompts/single-html.md](prompts/single-html.md) のプロンプトを LLM に渡すと、`-single` とほぼ同じ形式の1ファイルの HTML を書かせられます。画像は埋め込めないので、きちんと出力したいときは `okf-export -single` を使ってください。

## 制限

- 表として描画するのは 1,000 行までです。それより後はダウンロードしたファイルで見てください。
- 1MB を超えるファイルは、テキストのページと元のファイルの表示・コピーを作りません（ダウンロードはできます）。
- `@import` で取り込んだ `.md` の中の相対リンクは、取り込んだ側のファイルを基準に解決されます。
- オンラインのファイル（`@import "https://..."`）と、Crossnote の数式（KaTeX・MathJax）・Mermaid・PlantUML・Graphviz の描画には対応していません。
- ページごとの出力では、サイドバーと検索を JavaScript で読み込みます（`okf-toc.js`・`okf-search.js`）。JavaScript を切ったブラウザでは、サイドバーと検索は使えません。
- 解決できないリンクや取り込みは、そのまま残して標準エラーに警告を出します。

## インストール

```
go install github.com/Mokkori-Tom/okf-export@latest
```

## ライセンス

MIT。

- 画面の構成とテーマの配色は [mdBook](https://github.com/rust-lang/mdBook)（MPL-2.0）に合わせていますが、mdBook のコード・CSS・フォントは含んでいません。CSS と JavaScript はこのリポジトリで書いたものです。
- アイコンは [Octicons](https://github.com/primer/octicons)（MIT, © GitHub, Inc.）の図形を使っています。
- 依存ライブラリ：[goldmark](https://github.com/yuin/goldmark)（MIT）、[goldmark-highlighting](https://github.com/yuin/goldmark-highlighting)（MIT）、[chroma](https://github.com/alecthomas/chroma)（MIT）、[yaml.v3](https://github.com/go-yaml/yaml)（MIT / Apache-2.0）。
