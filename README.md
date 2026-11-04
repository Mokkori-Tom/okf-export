# okf-export

Open Knowledge Format（OKF）のバンドルを、GitHub 風の見た目の静的サイトとして書き出すツールです。

- **ページごとの出力**：`.md` 1つにつき `.html` を1ページ出力し、CSV/TSV のページとフォルダごとの一覧ページも作ります。
- **1ファイルの出力（`-single`）**：全ページを1つの HTML に入れ、`#/tables/x.html` のような URL のハッシュで表示するページを切り替えます。
- **ライブプレビュー（`serve`）**：ファイルを保存すると、開いているブラウザが自動で再読み込みします。

どの方式でも CSS、画像、ダウンロード用のファイルを埋め込みます（ページごとの出力では、検索用の `okf-search.js` も一緒に書き出します）。CDN もフォントも使わないので、オフラインでも表示できます。ライト／ダークは OS の設定に合わせて切り替わり、スマホ幅にも対応しています。

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
| フォルダ・`a/index.md` | `a/index.html`（そのフォルダの `index.md` を表示。なければ同じ形の一覧を自動で作る） |
| `a/x.csv`・`a/x.tsv` | `a/x.csv.html`（GitHub と同じく行番号付きの表） |
| その他のテキストファイル（`.txt`・`.yaml`・`.go` など 1MB まで） | `a/x.txt.html`（行番号と色分け付きで表示） |
| `/x.md`・`../x.md`・`x/`・`x.csv` へのリンク | 出力ページへのリンク |
| 画像 `a/x.png` | `a/x.png.html`（GitHub と同じく画像を表示。ファイルサイズ・縦横のピクセル数・ダウンロードボタン付き） |
| PDF `a/x.pdf` | `a/x.pdf.html`（ブラウザの PDF ビューアでページ内に表示。表示できないブラウザ（スマホなど）ではダウンロードボタン） |
| その他のファイル（zip・docx など、1MB を超えるテキスト） | `a/x.zip.html`（「表示できません」とダウンロードボタン） |
| ファイルへのリンク | クリックすると上のページが開く（いきなりダウンロードしない）。ダウンロードはページのボタンから |
| 本文の画像 | data URI で埋め込み。クリックするとその画像のページが開く（外部の画像は元の URL を新しいタブで開く） |
| ルートの `index.md` の `okf_version` | トップページのタイトルの下に `OKF 0.2` のようなラベル |
| フロントマター | タイトル、説明、type、status、tags を見出し部分に、sources をページ末尾の Sources に表示。それ以外の項目（resource など）は「Metadata」に表として表示（無ければ Metadata 欄は出さない） |
| `[^id]`（`sources` の id） | ページ末尾の Sources への番号付きリンク |
| 被リンク | ページ末尾に「Cited by」として表示（`okf backlinks` と同じく、`index.md`・`log.md` からのリンクは数えない） |

## OKF の index.md

OKF では `index.md` はフォルダの一覧で、フロントマターを持ちません。持てるのはバンドルのルートの `index.md` だけで、書けるのは `okf_version` だけです（仕様 §6・§8・§12）。okf-export もこれに合わせています。

- ページのタイトルはフォルダ名（ルートはバンドル名）です。`index.md` の `title`・`description`・`tags` は使いません。仕様にないフロントマターがあれば、警告を出して無視します。
- フォルダのページには、GitHub のようなファイル一覧ではなく、そのフォルダの `index.md` を表示します。見出しで分けた `* [タイトル](リンク) - 説明` の一覧が、そのまま目次になります（`okf index` で作ったものも、手で書いたものも同じ）。フォルダの説明は、親フォルダの `index.md` に `* [名前](sub/) - 説明` と書きます。
- `index.md` が無いフォルダでは、同じ形の一覧を自動で作ります（仕様 §8 で認められている動き）。文書はタイトルとフロントマターの `description`、その他のファイルとサブフォルダは名前を、Concepts・Files・Subdirectories に分けて並べます。

## ページの機能

- **ファイルのヘッダーのボタン**（GitHub のファイル表示と同じ）
  - `Preview` / `Raw`：描画結果と、元のファイル（行番号付き）を切り替えます。
  - コピー（Copy raw file）：元のファイルの全文をクリップボードにコピーします。
  - ダウンロード（Download raw file）：元のファイルを保存します。
  - アウトライン（Outline）：見出しの一覧を開き、クリックでその見出しへ移動します（Markdown のみ）。
- **コードブロックのコピーボタン**：コードブロックにマウスを乗せると右上に出ます（スマホでは常に表示）。
- **検索**：ヘッダーの検索欄（`/` で移動）で、全ページのタイトル・パス・説明・本文を検索します。空白で区切ると AND 検索です。`↑` `↓` で選び、`Enter` で開きます。
- **アラート**：GitHub と同じく、1行目が `[!NOTE]`・`[!TIP]`・`[!IMPORTANT]`・`[!WARNING]`・`[!CAUTION]` の引用を色付きの枠で表示します。

  ```
  > [!TIP]
  > 便利なヒント
  ```

- **属性の指定**（Markdown Preview Enhanced 互換）：画像やリンクの直後に `{...}` を書くと属性になります。見出しの末尾の `{#id}` も使えます。

  ```
  ![](image.png){height=800px}
  ![](image.png){width=300 .border #fig1}
  [資料](doc.pdf){title="説明"}
  @import "image.png" {width="50%"}
  ```

  `width` と `height` は CSS として出力します（数字だけなら px）。`.class`・`#id`・`title` なども使えます。
  画像の縦横比は保ちます。`height` だけを指定した画像が画面の幅に収まらないときは、高さも同じ比率で縮めます。`width` と `height` を両方指定すると、その枠の中に収めます。

- **`<img>` タグ**（GitHub 互換）：GitHub で画像の大きさを変えるときの書き方も、そのまま使えます。`src` のパスは `![](...)` と同じく解決して埋め込み、`alt`・`title`・`width`・`height`・`align` だけを使います（`style` や `onerror` などは捨てます）。`-unsafe` を付けなくても効きます。

  ```html
  <img src="images/a.png" alt="説明" width="300">

  <p align="center">
    <img src="logo.png" width="200">
  </p>
  ```

  `<img>` だけの HTML（`<p>`・`<div>` で囲んだものも可）に限ります。`<a>` で囲んだ画像など、ほかのタグが混ざった HTML は、これまでどおり除きます（`-unsafe` ならそのまま出力）。

## LLM に HTML を書かせる場合

`okf-export` を実行できない環境では、[prompts/single-html.md](prompts/single-html.md) のプロンプトを LLM に渡すと、`-single` とほぼ同じ形式の1ファイルの HTML を書かせられます。画像は埋め込めないので、きちんと出力したいときは `okf-export -single` を使ってください。

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
| 画像 | 埋め込み画像（`@import "a.png" {width=300}` のように属性も指定可） |
| `.md` | 本文をそのまま展開（フロントマターは除き、入れ子の取り込みも可。循環は検出してスキップ） |
| その他 | 拡張子を言語名にしたコードブロック |

`![[name]]` は相対パスで見つからない場合、バンドル全体からファイル名で探します。文の途中の `![](x.csv)` は、表のページへのリンクになります。

## 制限

- 表として描画するのは 1,000 行までです。それより後はダウンロードしたファイルで見てください。
- 1MB を超えるファイルは、テキストのページと `Raw` 表示・コピーを作りません（ダウンロードはできます）。
- 検索の対象は1ページにつき本文の先頭 20,000 バイトまでです。
- `@import` で取り込んだ `.md` の中の相対リンクは、取り込んだ側のファイルを基準に解決されます。
- オンラインのファイル（`@import "https://..."`）と、Crossnote の PDF・PlantUML・Graphviz・Mermaid の描画には対応していません。
- 解決できないリンクや取り込みは、そのまま残して標準エラーに警告を出します。

## インストール

```
go install github.com/Mokkori-Tom/okf-export@latest
```

## ライセンス

MIT。`assets/github-markdown.css` は [github-markdown-css](https://github.com/sindresorhus/github-markdown-css)（MIT, © Sindre Sorhus）を同梱しています。
