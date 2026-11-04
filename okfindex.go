package main

import (
	"bytes"
	"html/template"
	"path"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/parser"
)

// OKF index files (§8): a body of headings and entries
//
//   - [Title](relative-url) - short description
//   - [Subdirectory](subdir/) - short description of the subdirectory
//
// "okf index" writes the same as a table, | [Title](file) | Type | Description |,
// with "- [sub/](sub/index.md)" for subdirectories and a "Bundle root." or
// "Directory: `x/`" line under the heading.
var (
	indexItem    = regexp.MustCompile(`^\s*[*+-]\s+\[[^\]]*\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)\s*(?:[-–—:]\s*(.*?))?\s*$`)
	indexRow     = regexp.MustCompile(`^\s*\|\s*\[[^\]]*\]\(\s*<?([^)\s>]+)>?\s*\)\s*\|(?:.*\|)?\s*([^|]*?)\s*\|\s*$`)
	indexTableLn = regexp.MustCompile(`^\s*\|.*\|\s*$`)
	indexHeading = regexp.MustCompile(`^\s{0,3}#{1,6}(?:\s|$)`)
	indexOKFLine = regexp.MustCompile("^(?:Bundle root\\.|Directory: `[^`]*`)\\s*$")
)

// indexInfo is what an index.md says about its directory.
type indexInfo struct {
	desc    map[string]template.HTML // bundle path (file or directory) -> description, rendered
	listing bool                     // the body is only the listing, nothing else to read
}

// readIndex parses index.md d. Entries pointing at a directory, as "sub/" or
// "sub/index.md", are keyed by the directory.
func (ex *exporter) readIndex(d *doc) indexInfo {
	info := indexInfo{desc: map[string]template.HTML{}}
	_, body := splitFrontmatter(d.raw)
	entries, other := 0, false
	for _, line := range strings.Split(string(body), "\n") {
		var m []string
		switch {
		case strings.TrimSpace(line) == "", indexHeading.MatchString(line), indexOKFLine.MatchString(strings.TrimSpace(line)):
			continue
		case indexItem.MatchString(line):
			m = indexItem.FindStringSubmatch(line)
		case indexRow.MatchString(line):
			m = indexRow.FindStringSubmatch(line)
		case indexTableLn.MatchString(line):
			continue // table header or separator
		default:
			other = true
			continue
		}
		entries++
		p, _, ok := ex.resolveLocal(d.path, m[1])
		if !ok {
			continue
		}
		if path.Base(p) == "index.md" {
			p = path.Dir(p)
			if p == "." {
				p = ""
			}
		}
		if desc := strings.TrimSpace(m[2]); desc != "" && info.desc[p] == "" {
			info.desc[p] = ex.inlineMarkdown(d, desc)
		}
	}
	info.listing = entries > 0 && !other
	return info
}

// inlineMarkdown renders one line of markdown from doc d, such as an index
// entry's description, without the surrounding <p>. Links and images are
// resolved like in d's body; warnings about them were already given when d
// itself was rendered.
func (ex *exporter) inlineMarkdown(d *doc, s string) template.HTML {
	tmp := &doc{path: d.path, page: d.page, links: map[string]bool{}, quiet: true}
	ctx := parser.NewContext()
	ctx.Set(ctxDocKey, tmp)
	var buf bytes.Buffer
	if err := ex.md.Convert([]byte(s), &buf, parser.WithContext(ctx)); err != nil {
		return template.HTML(template.HTMLEscapeString(s))
	}
	out := strings.TrimSpace(buf.String())
	if strings.HasPrefix(out, "<p>") && strings.HasSuffix(out, "</p>") && strings.Count(out, "<p>") == 1 {
		out = out[len("<p>") : len(out)-len("</p>")]
	}
	return template.HTML(out)
}
