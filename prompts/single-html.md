# LLM に 1ファイルの HTML を書かせるときのプロンプト

`okf-export` を使えない環境（コマンドを実行できないチャットなど）で、LLM に OKF バンドルを `okf-export -single` と同じ形式（mdBook のようなドキュメントサイト）の HTML に変換させるためのプロンプトです。

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
あなたは Open Knowledge Format（OKF）バンドルを、mdBook のような1つの自己完結した HTML ファイルに変換する変換器です。
この後に貼り付ける各ファイル（"=== path: <パス> ===" で区切ってあります）から、次の仕様どおりの HTML を1つだけ出力してください。説明文は書かず、<!DOCTYPE html> から </html> までを1つのコードブロックで出力します。CSS・JavaScript・内容のどれも「…省略…」のように省いてはいけません。

# 全体
- 外部ファイル・CDN・Web フォントは一切読み込まない。CSS は <style>、JavaScript は <script> に直接書く。
- <html lang="ja">、<meta charset="utf-8">、<meta name="viewport" content="width=device-width, initial-scale=1">。
- 見た目は mdBook（Rust の公式ドキュメントなどの形式）に合わせる。文字は OS の sans-serif（"Hiragino Sans", "Noto Sans CJK JP" を含める）、16px、行間 1.45。本文の最大幅 750px。
- [hidden] { display: none !important; } を入れる。

# 画面の構成（全ページ共通、ページの外に1つずつ）
- 左のサイドバー（幅 300px、position: fixed、上下いっぱい、背景 --sidebar-bg、文字 0.875em）: 目次。下の「目次」を参照。
- 上のメニューバー（高さ 50px、position: sticky; top: 0、背景 --bg、下の境界線はスクロールしたときだけ --table-border-color）:
  左に3つのアイコンボタン（目次の開閉 ☰、テーマ 🖌、検索 🔍。16px の SVG、色 --icons、ホバーで --icons-hover）、中央にバンドル名（#/index.html へのリンク、20px、font-weight 200）、右にアウトラインのボタンと印刷ボタン（window.print()）。
- アウトライン: メニューバーの右のボタン（または「o」キー）で、今のページの見出し（h1〜h6）の一覧をメニューバーのすぐ下にポップアップで開く。メニューバーは画面の上に残るので、ページのどこにいても開ける。今読んでいる見出し（画面の上端より上にある最後の見出し）に色を付け、項目を押すとその見出しへ移動して閉じる。見出しの無いページではボタンを隠す。
- 本文の左右に前後のページへの矢印（‹ ›）。幅 90px・上下いっぱいの position: fixed のリンクで、左はサイドバーのすぐ右、右は画面の右端。文字 2.5em、色 --icons、ホバーで背景 --theme-hover。幅 1080px 以下（サイドバーを開いているときは 1380px 以下）では、代わりに本文の下に 90×50px の角丸ボタンとして並べる。
- サイドバーを閉じると、サイドバーは左へ隠れ、本文は左に寄る（0.3 秒のアニメーション）。幅 620px 以下では、サイドバーは本文の上に重なって開き、リンクを押すか本文を押すと閉じる。開閉状態は localStorage に保存する（初期値: 幅 1080px 未満なら閉じる）。

# テーマ（mdBook と同じ5つ）
テーマボタンで Light / Rust / Coal / Navy / Ayu の一覧を出し、選ぶと <html> の class を変えて localStorage に保存する。初期値は OS がダークなら Navy、そうでなければ Light。各テーマの CSS 変数:
- light: --bg #fff, --fg #000, --sidebar-bg #fafafa, --sidebar-fg #000, --sidebar-active #1f1fff, --icons #747474, --icons-hover #000, --links #20609f, --inline-code-color #301900, --theme-popup-bg #fafafa, --theme-popup-border #ccc, --theme-hover #e6e6e6, --quote-bg hsl(197,37%,96%), --quote-border hsl(197,37%,91%), --table-border-color hsl(0,0%,95%), --table-header-bg hsl(0,0%,80%), --table-alternate-bg hsl(0,0%,97%), --searchbar-bg #fafafa, --searchresults-li-bg #e4f2fe, --search-mark-bg #a2cff5, --code-bg #f6f7f6
- rust: --bg hsl(60,9%,87%), --fg #262625, --sidebar-bg #3b2e2a, --sidebar-fg #c8c9db, --sidebar-active #e69f67, --icons #737480, --icons-hover #262625, --links #2b79a2, --inline-code-color #6e6b5e, --theme-popup-bg #e1e1db, --theme-popup-border #b38f6b, --theme-hover #99908a, --quote-bg hsl(60,5%,75%), --quote-border hsl(60,5%,70%), --table-border-color hsl(60,9%,82%), --table-header-bg #b3a497, --table-alternate-bg hsl(60,9%,84%), --searchbar-bg #fafafa, --searchresults-li-bg #dec2a2, --search-mark-bg #e69f67, --code-bg #f6f7f6
- coal: --bg hsl(200,7%,8%), --fg #98a3ad, --sidebar-bg #292c2f, --sidebar-fg #a1adb8, --sidebar-active #3473ad, --icons #43484d, --icons-hover #b3c0cc, --links #2b79a2, --inline-code-color #c5c8c6, --theme-popup-bg #141617, --theme-popup-border #43484d, --theme-hover #1f2124, --quote-bg hsl(234,21%,18%), --quote-border hsl(234,21%,23%), --table-border-color hsl(200,7%,13%), --table-header-bg hsl(200,7%,28%), --table-alternate-bg hsl(200,7%,11%), --searchbar-bg #b7b7b7, --searchresults-li-bg #2b2b2f, --search-mark-bg #355c7d, --code-bg #1d1f21
- navy: --bg hsl(226,23%,11%), --fg #bcbdd0, --sidebar-bg #282d3f, --sidebar-fg #c8c9db, --sidebar-active #2b79a2, --icons #737480, --icons-hover #b7b9cc, --links #2b79a2, --inline-code-color #c5c8c6, --theme-popup-bg #161923, --theme-popup-border #737480, --theme-hover #282e40, --quote-bg hsl(226,15%,17%), --quote-border hsl(226,15%,22%), --table-border-color hsl(226,23%,16%), --table-header-bg hsl(226,23%,31%), --table-alternate-bg hsl(226,23%,14%), --searchbar-bg #aeaec6, --searchresults-li-bg #242430, --search-mark-bg #a2cff5（mark の文字は黒）, --code-bg #1d1f21
- ayu: --bg hsl(210,25%,8%), --fg #c5c5c5, --sidebar-bg #14191f, --sidebar-fg #c8c9db, --sidebar-active #ffb454, --icons #737480, --icons-hover #b7b9cc, --links #0096cf, --inline-code-color #ffb454, --theme-popup-bg #14191f, --theme-popup-border #5c6773, --theme-hover #191f26, --quote-bg hsl(226,15%,17%), --quote-border hsl(226,15%,22%), --table-border-color hsl(210,25%,13%), --table-header-bg hsl(210,25%,28%), --table-alternate-bg hsl(210,25%,11%), --searchbar-bg #424242（文字 #fff）, --searchresults-li-bg #252932, --search-mark-bg #e3b171（mark の文字は黒）, --code-bg #191f26
コードブロックの背景は --code-bg、インラインコードは文字 --inline-code-color・背景 --quote-bg。テーマのちらつきを防ぐため、<head> の中でテーマとサイドバーの class を先に付ける。

# 目次（サイドバー）
- ディレクトリ構造から作る。1つ目はトップページ（バンドル名、#/index.html）。続けてルートの文書（ファイル名順）とルートのフォルダ（名前順）を並べ、フォルダの下に、そのフォルダの文書（ファイル名順）とサブフォルダ（名前順）を字下げ 20px で入れ子にする。index.md 自体は項目にせず、フォルダの項目がそのフォルダのページ（#/<フォルダ>/index.html）を指す。Markdown を1つも含まないフォルダは出さない。
- 項目の名前は文書のタイトル、フォルダはフォルダ名。行間 1.5em、項目の間 0.6em。今のページの項目は --sidebar-active の色にする。
- 子を持つ項目の右に ❱ のボタンを付け、押すと子を折りたたむ（開いているときは 90 度回して表示）。初めはすべて開いておく。
- 「前後のページ」はこの目次の上から下への順番。

# ページの作り方
バンドル内の各ファイル・各フォルダを「ページ」とし、それぞれ <div class="okf-page" data-page="<出力パス>" data-title="<タイトル> - <バンドル名>" hidden> に入れて、メニューバーの下の本文の場所に全部並べる。各ページの中は <main>（本文、最大幅 750px、中央寄せ）と、そのページの前後の矢印。出力パスは次のとおり。
- a/b.md → a/b.html
- a/index.md → a/index.html（フォルダ a のページとして表示する）
- フォルダ a → a/index.html、ルートは index.html
- a/x.csv・a/x.tsv → a/x.csv.html・a/x.tsv.html
- その他のテキストファイル a/x.txt・a/x.yaml など → a/x.txt.html・a/x.yaml.html
バンドル名は、ユーザーが指定した名前とする。指定がなければ "bundle" とする。

## ページの道具（本文の右上、小さなアイコンボタンを右寄せで並べる）
- 元のファイルの表示（<> のアイコン）: 描画結果を隠し、元のファイルを行番号付きの等幅表示（背景 --code-bg、行番号は薄い色・選択不可）で見せる。もう一度押すと戻す。Markdown と CSV のページだけ。
- コピー: 元のファイルの全文（フロントマター込み）をクリップボードにコピーし、1.5秒だけアイコンをチェックマークにする。navigator.clipboard が使えないときは textarea と document.execCommand("copy") で代用する。
- ダウンロード: 元のファイルを data:<MIME>;charset=utf-8;base64,… の href と download="<ファイル名>" で保存させる。
- このページの保存: 今のページの本文だけを、<style> と今のテーマの class を付けた1つの HTML にして、Blob の URL と download 属性で保存させる（ファイル名はページの出力パスの最後の部分）。サイドバー・メニューバー・スクリプト・ページの道具は含めない。
- 元のファイルは <pre class="raw-source" hidden> などに HTML エスケープして入れておき、表示・コピー・ダウンロードはそれを使う。

## Markdown のページ
- 先頭の --- で囲まれた YAML をフロントマターとして読む。タイトルは title、なければ本文の最初の「# 見出し」、なければファイル名（拡張子なし）。
- 本文の前に: タイトルの <h1>（本文が同じ文字の「# 見出し」で始まるときは出さない）、description（薄い文字、1.1em）、type・status・tags の小さな角丸ラベル（tags は破線の枠）、折りたたんだ <details><summary>Metadata</summary>。
- Metadata には、フロントマターのうち、ほかの場所に出していない項目（title・description・type・status・tags・sources 以外。resource など）を「キー | 値」の縦の表で表示する。残る項目が無ければ Metadata 欄は作らない。入れ子は表の入れ子、配列は箇条書き、空の値は「—」、http(s) の値はリンク。
- 本文は GitHub Flavored Markdown として HTML にする（表、取り消し線、タスクリスト、自動リンク、脚注）。生の HTML タグは出力しない（テキストとしてエスケープする）。
- 見出しには mdBook と同じ規則で id を付ける: 小文字にし、文字（日本語などもそのまま）・数字・_・- を残して、空白は -、それ以外の記号は消す。同じ id は -1, -2 … を付ける。行末の {#id} があればそれを id にする。見出しの文字は、その id へのリンクにする。
- 表: ヘッダー行の背景 --table-header-bg、偶数行の背景 --table-alternate-bg、枠線 --table-border-color、セルの余白 3px 20px。引用: 背景 --quote-bg、上下に --quote-border の線。
- コードブロックは <pre><code> で出し、各コードブロックの右上にコピーボタンを付ける（ふだんは非表示、マウスを乗せると表示。タッチ端末では常に表示）。押すとコードだけをコピーする。
- アラート（コールアウト）: 1行目が [!種類] の引用は、左に 4px の色の線と、アイコン付きの見出しを持つ枠にする。[!種類] の後ろに文字があればそれを見出しにし（例「> [!TIP] hogehoge」の見出しは hogehoge。中の太字なども有効）、なければ種類の名前（Tip など）を見出しにする。[!TIP]- は <details>（閉じた状態）、[!TIP]+ は <details open> にして見出しを <summary> にする。
  種類と色: NOTE・INFO・TODO・ABSTRACT 青 #0969da、TIP・HINT 緑 #1a7f37、SUCCESS・CHECK・DONE 緑 #1a7f37、IMPORTANT・EXAMPLE 紫 #8250df、WARNING・ATTENTION 黄土 #9a6700、QUESTION・HELP・FAQ 橙 #bc4c00、CAUTION・DANGER・ERROR・FAILURE・BUG 赤 #d1242f、QUOTE・CITE 灰色（Coal・Navy・Ayu では青 #4493f8、緑 #3fb950、紫 #ab7df8、黄土 #d29922、橙 #f0883e、赤 #f85149）。知らない種類は NOTE と同じ。
- Crossnote（Markdown Preview Enhanced）の記法:
  - [[名前]]・[[名前|表示名]]・[[名前#見出し]]・[[#見出し]] は Wiki リンク。名前は .md を省いたファイル名かパスで、同じフォルダ、ルートからのパス、バンドル全体のファイル名の順に探す。見つからなければリンクにせず文字だけ残す。
  - ==語== は <mark>。[TOC] だけの段落は、そのページの見出しの入れ子の一覧。
  - 画像やリンクの直後の {…} は属性として扱い、表示しない。![](a.png){height=800px} → style="height:800px"、{width=300} → style="width:300px"（数字だけなら px）、{.class} → class、{#id} → id、{title="…"} → title。
- リンク: バンドル内の .md へのリンクは "#/<出力パス>"、フォルダへのリンクは "#/<フォルダ>/index.html"、CSV・テキストファイルへのリンクはそのページへ。"#見出し" はそのまま。外部 URL はそのまま。
- 画像: 中身を受け取れないので、src は元の相対パスのまま残す（alt は保つ）。
- [^id] がフロントマターの sources の id と一致したら <sup><a href="#source-id">[番号]</a></sup> にし、本文の最後に「Sources」の見出しと番号付きの一覧（title をリンク文字、resource を URL、末尾に id をコード表示）を付ける。
- 単独行の @import "x.csv" や ![](x.csv) は、その CSV を表にしてその場に入れる。@import "x.md" や ![題名](./x.md) はその本文（フロントマターを除く）をその場に展開する。それ以外の @import "x.ext" は拡張子を言語名にしたコードブロックにする。文の途中の ![…](x.md)・![…](x.csv) はそのページへのリンクにする。
- 他のページからリンクされているページには、末尾に区切り線と「Cited by」の見出しを付け、リンク元のタイトル（リンク）・パス・description を並べる。index.md と log.md からのリンクは数えない。

## CSV・TSV のページ
- タイトルはファイル名。その下に「N rows」（見出し行を除いた行数、薄い文字）。
- 1行目を見出しにした表で、左端に行番号（1から、薄い色）を付ける。1,000 行を超える分は表示せず「Showing the first 1000 of N rows.」と書く。

## その他のテキストファイルのページ
- タイトルはファイル名。その下に「N lines」。中身を行番号付きの等幅表示で見せる。道具はコピーとダウンロードだけ。

## フォルダのページ
- タイトルはフォルダ名（ルートはバンドル名）。その下に、そのフォルダの index.md を Markdown のページと同じように（Metadata 欄なしで）表示する。OKF の index.md は「見出し」と「* [タイトル](リンク) - 説明」の行でできた目次なので、それがそのままフォルダの目次になる。
- OKF の index.md はフロントマターを持たない（ルートの index.md だけが okf_version を持てる）。index.md の title・description・tags は使わない。
- index.md が無いフォルダでは、同じ形の目次を作る。見出し「Concepts」の下に Markdown の文書を「タイトル（リンク） - フロントマターの description」で、「Files」の下にその他のファイルを名前で、「Subdirectories」の下にサブフォルダを「名前/」で並べる（空の見出しは出さない）。
- ルートの index.md に okf_version があれば、トップページのタイトルの下に「OKF 0.2」のようなラベルを出す。

# 検索（mdBook と同じ形、日本語にも対応）
- 検索ボタン、または「s」「/」キーで、メニューバーの下に検索欄（幅いっぱい、角丸 3px、背景 --searchbar-bg、placeholder "Search this book …"）を開く。Esc で閉じる。
- 全ページを見出しごとに区切った {出力パス, 見出しの id, ページのタイトル, 見出し, 元のパス, 本文のテキスト} を JSON にして <script type="application/json" id="okf-search"> に入れておく。
- 入力のたびに、空白で区切った語すべてを含む区切りを探す。比べるときは両方を normalize("NFKC") と toLowerCase() にそろえ、部分一致で探す（日本語を単語に区切らない）。見出し・タイトルに含むもの、パスに含むもの、本文に多く含むものの順に上に並べる。
- 結果は検索欄の下に「N search results for '語':」（なければ「No search results for '語'.」）と、最大 30 件の一覧で出す。各結果は「ページのタイトル » 見出し」（太字、その見出しへのリンク）、元のパス（薄い小さな文字）、本文の該当箇所の前後（約 180 文字、0.8em）で、一致した語を背景 --search-mark-bg の <mark> で強調する。
- ↓↑ で結果を選び（背景 --searchresults-li-bg）、Enter で開く。結果から開いたページでは、本文の中の検索語にも同じ色を付ける（Esc で消す）。

# キーボード
- ← → で前後のページへ（入力欄にいるときは除く）。s・/ で検索、Esc で検索やテーマの一覧を閉じる。

# ページの切り替え（JavaScript）
- URL のハッシュでページを切り替える。"#/a/b.html" はそのページだけを表示して他を hidden にし、document.title を data-title にして、目次の今の項目の色を変える。"#/a/b.html#id" はそのページを出してから id の要素へスクロール、"#id" だけなら今のページの中でスクロールする。該当しなければ "#/index.html" を表示する。
- hashchange で切り替え、読み込み時にも1回実行する。ページを切り替えたら先頭までスクロールする。

貼り付けるファイルは以上の仕様で使うこと。ファイルにない内容を足したり、内容を要約したりしてはいけません。
````
