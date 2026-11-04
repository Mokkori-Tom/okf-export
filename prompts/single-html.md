# LLM に 1ファイルの HTML を書かせるときのプロンプト

`okf-export` を使えない環境（コマンドを実行できないチャットなど）で、LLM に OKF バンドルを `okf-export -single` と同じ形式の HTML に変換させるためのプロンプトです。

## 使い方

1. 下の「プロンプト」の枠の中をそのままコピーして LLM に渡します。
2. 続けて、バンドルの各ファイルを次の形で貼り付けます。パスはバンドルのルートからの相対パスです。

   ````
   === path: datasets/ga4.md ===
   （ファイルの中身）
   === path: tables/events.csv ===
   （ファイルの中身）
   ````

3. 返ってきた HTML を `.html` として保存し、ブラウザで開きます。

画像などのバイナリは貼れないので、LLM には「画像はリンクだけ残す」ように指示しています。画像まで埋め込みたいときや、ファイルが多いとき（目安として合計 30KB を超えるとき）は `okf-export -single` を使ってください。LLM は長い出力の途中で止まったり、CSS や JavaScript を省略したりしやすいためです。

## プロンプト

````text
あなたは Open Knowledge Format（OKF）バンドルを、1つの自己完結した HTML ファイルに変換する変換器です。
この後に貼り付ける各ファイル（"=== path: <パス> ===" で区切ってあります）から、次の仕様どおりの HTML を1つだけ出力してください。説明文は書かず、<!DOCTYPE html> から </html> までを1つのコードブロックで出力します。CSS・JavaScript・内容のどれも「…省略…」のように省いてはいけません。

# 全体
- 外部ファイル・CDN・Web フォントは一切読み込まない。CSS は <style>、JavaScript は <script> に直接書く。
- <html lang="ja">、<meta charset="utf-8">、<meta name="viewport" content="width=device-width, initial-scale=1">、<meta name="color-scheme" content="light dark">。
- 見た目は GitHub のリポジトリのファイル表示に合わせる。文字は -apple-system, "Segoe UI", "Noto Sans", Helvetica, Arial, sans-serif、14px、行間 1.5。
- 色は CSS 変数で定義し、@media (prefers-color-scheme: dark) でダーク用に切り替える。
  ライト: 文字 #1f2328 / 薄い文字 #59636e / リンク #0969da / 背景 #ffffff / 薄い背景 #f6f8fa / 枠線 #d1d9e0
  ダーク: 文字 #f0f6fc / 薄い文字 #9198a1 / リンク #4493f8 / 背景 #0d1117 / 薄い背景 #151b23 / 枠線 #3d444d
- 幅 767px 以下では余白を 16px に詰め、スマホでも横スクロールせずに読めるようにする。
- [hidden] { display: none !important; } を入れる。

# ページの作り方
バンドル内の各ファイル・各フォルダを「ページ」とし、それぞれ <main class="container okf-page" data-page="<出力パス>" data-title="<タイトル> · <バンドル名>" hidden> に入れて、全部を1つの HTML に並べる。出力パスは次のとおり。
- a/b.md → a/b.html
- a/index.md → a/index.html（フォルダ a のページに統合する）
- フォルダ a → a/index.html、ルートは index.html
- a/x.csv・a/x.tsv → a/x.csv.html・a/x.tsv.html
- その他のテキストファイル a/x.txt・a/x.yaml など → a/x.txt.html・a/x.yaml.html
バンドル名は、ユーザーが指定した名前とする。指定がなければ "bundle" とする。

## ページ共通の部品
1. ヘッダー（全ページで1つ、<main> の外）: 左に本のアイコン（16x16 の SVG）とバンドル名（#/index.html へのリンク）、右に検索欄。バッジなどほかの表示は付けない。
2. パンくず: 「バンドル名 / フォルダ / … / ファイル名」。最後以外は各ページへのリンク。
3. タイトル行: <h1 class="page-title"> にタイトル。フロントマターに description があればその下に薄い文字で表示。type・status はふちどりのラベル、tags は青い丸いラベルで並べる。
4. フッター（全ページで1つ）:「Exported from an Open Knowledge Format bundle with okf-export」。

## Markdown のページ
- 先頭の --- で囲まれた YAML をフロントマターとして読む。タイトルは title、なければ本文の最初の「# 見出し」、なければファイル名（拡張子なし）。
- 枠（.box: 1px の枠線、角丸 6px）の中に表示する。枠の上端のヘッダー行（薄い背景）に、左からファイル名、右端にボタンを置く:
  [Preview | Raw] の切り替え、コピー（Copy raw file）、ダウンロード（Download raw file）、アウトライン（Outline）。ボタンは小さい枠付き、アイコンは 16x16 の SVG。
  - Raw: 描画結果を隠し、元の Markdown を行番号付きの等幅表示（行番号は薄い色・選択不可）で見せる。Preview で戻す。
  - Copy raw file: 元のファイルの全文（フロントマター込み）をクリップボードにコピーし、1.5秒だけアイコンをチェックマークにする。navigator.clipboard が使えないときは textarea と document.execCommand("copy") で代用する。
  - Download raw file: 元のファイルを data:text/markdown;charset=utf-8;base64,… の href と download="<ファイル名>" で保存させる。
  - Outline: ヘッダー行の下に、本文の h1〜h6 の一覧を開閉する。レベルごとに字下げし、各項目は見出しの id へのリンク。
- 元の Markdown は <pre class="raw-source" hidden> などに HTML エスケープして入れておき、Raw 表示・コピー・ダウンロードはそれを使う。
- 本文の先頭に <details><summary>Metadata</summary> を置き、フロントマターの全項目を「キー | 値」の縦の表で表示する。入れ子は表の入れ子、配列は箇条書き、空の値は「—」、http(s) の値はリンク。
- 本文は GitHub Flavored Markdown として HTML にする（表、取り消し線、タスクリスト、自動リンク、脚注）。生の HTML タグは出力しない（テキストとしてエスケープする）。
- 見出しには id を付ける（英数字は小文字にして空白を - に。日本語だけの見出しは heading、heading-1 … と連番）。行末の {#id} があればそれを id にする。
- コードブロックは <pre><code> で出し、各コードブロックの右上にコピーボタンを付ける（ふだんは非表示、マウスを乗せると表示。タッチ端末では常に表示）。押すとコードだけをコピーする。
- GitHub のアラート: 1行目が [!NOTE]・[!TIP]・[!IMPORTANT]・[!WARNING]・[!CAUTION] の引用は
  <div class="markdown-alert markdown-alert-tip"><p class="markdown-alert-title">（アイコン）Tip</p>…本文…</div>
  の形にする。左に 4px の線を引き、線と見出しの色は NOTE 青 #0969da、TIP 緑 #1a7f37、IMPORTANT 紫 #8250df、WARNING 黄土 #9a6700、CAUTION 赤 #d1242f（ダークでは #4493f8, #3fb950, #ab7df8, #d29922, #f85149）。
- Markdown Preview Enhanced と同じ属性指定: 画像やリンクの直後の {…} は属性として扱い、表示しない。
  ![](a.png){height=800px} → style="height:800px"、{width=300} → style="width:300px"（数字だけなら px）、{.class} → class、{#id} → id、{title="…"} → title。
- リンク: バンドル内の .md へのリンクは "#/<出力パス>"、フォルダへのリンクは "#/<フォルダ>/index.html"、CSV・テキストファイルへのリンクはそのページへ。"#見出し" はそのまま。外部 URL はそのまま。
- 画像: 中身を受け取れないので、src は元の相対パスのまま残す（alt は保つ）。
- [^id] がフロントマターの sources の id と一致したら <sup><a href="#source-id">[番号]</a></sup> にし、本文の最後に「Sources」の見出しと番号付きの一覧（title をリンク文字、resource を URL、末尾に id をコード表示）を付ける。
- 単独行の @import "x.csv" や ![](x.csv) は、その CSV を表にしてその場に入れる。@import "x.md" はその本文をその場に展開する。それ以外の @import "x.ext" は拡張子を言語名にしたコードブロックにする。
- 他のページからリンクされているページには、末尾に「Cited by」の枠を作り、リンク元のタイトル（リンク）・パス・description を並べる。index.md と log.md からのリンクは数えない。

## CSV・TSV のページ
- 枠のヘッダー行に表のアイコン、ファイル名、「N rows」（見出し行を除いた行数）、右端に [Preview | Raw]・コピー・ダウンロードのボタン。
- 1行目を見出しにした表で、左端に行番号（1から、薄い色）を付ける。1,000 行を超える分は表示せず「Showing the first 1000 of N rows.」と書く。

## その他のテキストファイルのページ
- 枠のヘッダー行にファイルのアイコン、ファイル名、「N lines」、右端にコピーとダウンロードのボタン（Preview/Raw とアウトラインはなし）。
- 中身を行番号付きの等幅表示で見せる。

## フォルダのページ
- GitHub のファイル一覧と同じ枠: 先にフォルダ（青いフォルダのアイコン）、次にファイル（ファイルのアイコン）を、それぞれ名前の大文字小文字を区別しない順で並べる。index.md は一覧に出さない。
- 各行はアイコンと名前（ページへのリンク）。名前の右に説明を薄い文字で表示する（文書はフロントマターの description、なければ下の index.md の行の説明）。見出しやタイトルは出さない。説明がなければ何も出さない。
- OKF の index.md はフォルダの一覧で、フロントマターを持たない（ルートの index.md だけが okf_version を持てる）。index.md の title・description・tags は使わない。
- index.md の本文は「見出し」と「* [名前](リンク) - 説明」の行の一覧（表で「| [名前](リンク) | … | 説明 |」と書かれていることもある）。各行の説明は、リンク先のファイルやフォルダ（"sub/" や "sub/index.md" はフォルダ sub）の行の名前の右に出す。文書にフロントマターの description があればそちらを優先する。フォルダの説明はこの方法でしか書けない。
- 一覧の下に index.md を Markdown のページと同じ枠（ボタン付き、Metadata 欄なし）で表示する。ただし、見出しと一覧の行（と「Bundle root.」「Directory: `…`」の行）しかない index.md は、ファイル一覧と同じ内容なので表示しない。表示する index.md がなければ README.md を表示する。
- タイトルはフォルダ名（ルートはバンドル名）。
- ルートの index.md に okf_version があれば、トップページのタイトルの下に「OKF 0.2」のようなふちどりのラベルを出す。

# 検索
- ヘッダーの右に検索欄（placeholder "Search"、右端に「/」のキー表示）。どこにもフォーカスがないとき「/」で検索欄に移る。
- 全ページの {出力パス, タイトル, 元のパス, description, 本文のテキスト} を JSON にして <script type="application/json" id="okf-search"> に入れておく。
- 入力のたびに、空白で区切った語すべてを（大文字小文字を区別せず）含むページを探し、タイトルに含むもの、パスに含むもの、description に含むもの、の順に上へ並べて、最大 30 件を検索欄の下のドロップダウンに出す。
- 各結果はタイトル（太字）、パス（小さい薄い文字）、本文の該当箇所の前後（約 140 文字）を表示し、一致した語を <mark> で強調する。結果がなければ「No results」。
- ↑↓ で選択、Enter で開く、Esc で閉じる。結果の外をクリックしても閉じる。

# ページの切り替え（JavaScript）
- URL のハッシュでページを切り替える。"#/a/b.html" はそのページだけを表示して他を hidden にし、document.title を data-title にする。"#/a/b.html#id" はそのページを出してから id の要素へスクロール、"#id" だけなら今のページの中でスクロールする。該当しなければ "#/index.html" を表示する。
- hashchange で切り替え、読み込み時にも1回実行する。ページを切り替えたら先頭までスクロールする。

貼り付けるファイルは以上の仕様で使うこと。ファイルにない内容を足したり、内容を要約したりしてはいけません。
````
