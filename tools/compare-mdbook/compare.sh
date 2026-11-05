#!/bin/sh
# Build one OKF bundle with both okf-export and mdBook and put screenshots of
# the same pages side by side, to check how far the look and behaviour drift.
#
#   tools/compare-mdbook/compare.sh <bundle> [out-dir]
#
# Needs: mdbook (https://rust-lang.github.io/mdBook/), node with the
# playwright package and a Chromium for it, and okf-export (built from this
# repository if not on PATH). Opens nothing; read <out-dir>/report.html.
set -eu

bundle=${1:?usage: compare.sh <bundle> [out-dir]}
out=${2:-compare-out}
here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)

command -v mdbook >/dev/null || { echo "mdbook not found: install it first (cargo install mdbook, or a release binary)" >&2; exit 1; }
command -v node >/dev/null || { echo "node not found" >&2; exit 1; }
okf=$(command -v okf-export || true)
if [ -z "$okf" ]; then
	(cd "$repo" && go build -o "$repo/okf-export" .)
	okf=$repo/okf-export
fi

rm -rf "$out"
mkdir -p "$out/book/src"
name=$(basename "$(cd "$bundle" && pwd)")

# mdBook source: the bundle without frontmatter (mdBook would print it), and
# the sidebar okf-export builds, as SUMMARY.md.
(cd "$bundle" && find . -path '*/.*' -prune -o -type f -print) | while read -r f; do
	mkdir -p "$out/book/src/$(dirname "$f")"
	case $f in
	*.md) awk 'NR==1 && $0=="---" {fm=1; next} fm && $0=="---" {fm=0; next} !fm' "$bundle/$f" >"$out/book/src/$f" ;;
	*) cp "$bundle/$f" "$out/book/src/$f" ;;
	esac
done
"$okf" summary -in "$bundle" -name "$name" >"$out/book/src/SUMMARY.md"
printf '[book]\ntitle = "%s"\nsrc = "src"\n\n[build]\nbuild-dir = "../mdbook"\ncreate-missing = false\n' "$name" >"$out/book/book.toml"
mdbook build "$out/book"

"$okf" -in "$bundle" -name "$name" -out "$out/okf"

node "$here/shots.js" "$out"
echo "report: $out/report.html"
