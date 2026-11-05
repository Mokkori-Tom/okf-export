package main

import (
	"bytes"
	"html"
	"io"
	"io/fs"
	"net/http"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// maxTextSize is the largest file shown as a text page or raw view; bigger
// files are only offered for download.
const maxTextSize = 1 << 20

// isTextFile reports whether bundle file rel (at file path p) gets a text
// page: anything that isn't markdown, a table or an image and looks like
// UTF-8 text.
func isTextFile(rel, p string, d fs.DirEntry) bool {
	if isTable(rel) || isImage(rel) {
		return false
	}
	info, err := d.Info()
	if err != nil || info.Size() > maxTextSize {
		return false
	}
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 8192)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	if bytes.IndexByte(head, 0) >= 0 {
		return false
	}
	return strings.HasPrefix(http.DetectContentType(head), "text/")
}

var rawFormatter = chromahtml.New(
	chromahtml.WithClasses(true),
	chromahtml.WithLineNumbers(true),
	chromahtml.LineNumbersInTable(true),
)

// codeView renders a file's source with line numbers and syntax highlighting
// chosen by its name, like GitHub's code view.
func codeView(name string, data []byte) string {
	src := strings.ReplaceAll(string(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))), "\r\n", "\n")
	if !utf8.ValidString(src) {
		src = strings.ToValidUTF8(src, "�")
	}
	lexer := lexers.Match(name)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)
	var b strings.Builder
	it, err := lexer.Tokenise(nil, src)
	if err == nil {
		err = rawFormatter.Format(&b, styles.Get("github"), it)
	}
	if err != nil {
		return "<pre><code>" + html.EscapeString(src) + "</code></pre>"
	}
	return b.String()
}

var (
	tagRe   = regexp.MustCompile(`(?s)<script.*?</script>|<style.*?</style>|<[^>]*>`)
	spaceRe = regexp.MustCompile(`\s+`)
)

// plainText strips tags from rendered HTML for the search index.
func plainText(s string) string {
	s = html.UnescapeString(tagRe.ReplaceAllString(s, " "))
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}
