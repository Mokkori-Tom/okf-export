package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// maxTableRows caps how many CSV rows are rendered as HTML; the full file is
// still offered for download.
const maxTableRows = 1000

// maxImportDepth bounds nested markdown @imports.
const maxImportDepth = 8

func isTable(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".csv", ".tsv":
		return true
	}
	return false
}

func isImage(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".apng", ".svg", ".bmp", ".webp", ".avif", ".ico":
		return true
	}
	return false
}

// File imports in the style of Crossnote / Markdown Preview Enhanced. Each
// must stand alone on its line:
//
//	@import "file"          <!-- @import "file" -->
//	![](file.csv)           ![[ file ]]
var (
	importLine   = regexp.MustCompile(`^\s{0,3}@import\s+"([^"]+)"(?:\s*\{[^}]*\})?\s*$`)
	importHTML   = regexp.MustCompile(`^\s{0,3}<!--\s*@import\s+"([^"]+)"(?:\s*\{[^}]*\})?\s*-->\s*$`)
	imageCSVLine = regexp.MustCompile(`(?i)^\s{0,3}!\[[^\]]*\]\(\s*<?([^)\s>]+\.(?:csv|tsv))>?(?:\s+"[^"]*")?\s*\)\s*$`)
	wikiEmbed    = regexp.MustCompile(`^\s{0,3}!\[\[\s*([^\]|]+?)\s*(?:\|[^\]]*)?\]\](?:\{[^}]*\})?\s*$`)
	fenceOpen    = regexp.MustCompile("^\\s{0,3}(`{3,}|~{3,})")
	blockMarker  = regexp.MustCompile(`<p>OKFBLOCK-(\d+)-X</p>`)
)

// preprocess expands file imports in the markdown of doc d. src is the
// markdown of file `from` (d.path, or a markdown file being imported into
// it); seen guards against import cycles.
func (ex *exporter) preprocess(d *doc, from string, src []byte, seen map[string]bool) []byte {
	var out bytes.Buffer
	fence := ""
	for _, line := range strings.SplitAfter(string(src), "\n") {
		trimmed := strings.TrimRight(line, "\n")
		if fence != "" {
			if strings.HasPrefix(strings.TrimSpace(trimmed), fence) {
				fence = ""
			}
			out.WriteString(line)
			continue
		}
		if m := fenceOpen.FindStringSubmatch(trimmed); m != nil {
			fence = m[1]
			out.WriteString(line)
			continue
		}
		target, wiki := "", false
		if m := importLine.FindStringSubmatch(trimmed); m != nil {
			target = m[1]
		} else if m := importHTML.FindStringSubmatch(trimmed); m != nil {
			target = m[1]
		} else if m := imageCSVLine.FindStringSubmatch(trimmed); m != nil {
			target = m[1]
		} else if m := wikiEmbed.FindStringSubmatch(trimmed); m != nil {
			target, wiki = m[1], true
		}
		if target == "" {
			out.WriteString(line)
			continue
		}
		p := ex.resolveImport(from, target, wiki)
		if p == "" {
			if strings.Contains(target, "://") {
				fmt.Fprintf(os.Stderr, "warning: %s: online import %q is not supported\n", from, target)
			} else {
				d.broken++
				fmt.Fprintf(os.Stderr, "warning: %s: unresolved import %q\n", from, target)
			}
			out.WriteString(line)
			continue
		}
		out.WriteString(ex.expand(d, p, seen))
		out.WriteString("\n")
	}
	return out.Bytes()
}

// resolveImport finds the bundle file an import names. Wiki embeds may also
// name a file by its base name (with or without ".md") anywhere in the bundle.
func (ex *exporter) resolveImport(from, target string, wiki bool) string {
	if p, _, ok := ex.resolveLocal(from, target); ok && ex.b.files[p] {
		return p
	}
	if !wiki {
		return ""
	}
	for _, cand := range []string{target, target + ".md"} {
		if p, _, ok := ex.resolveLocal("", "/"+cand); ok && ex.b.files[p] {
			return p
		}
	}
	for _, f := range sortedKeys(ex.b.files) {
		if b := path.Base(f); b == target || b == target+".md" {
			return f
		}
	}
	return ""
}

// expand returns the markdown that replaces an import of bundle file p.
func (ex *exporter) expand(d *doc, p string, seen map[string]bool) string {
	switch {
	case isImage(p):
		// The link rewriter embeds it like any other image.
		return "![" + escapeMD(path.Base(p)) + "](</" + p + ">)\n"
	case isTable(p):
		data, err := os.ReadFile(ex.abs(p))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", d.path, err)
			return ""
		}
		d.blocks = append(d.blocks, csvTable(p, data, false))
		// A paragraph of its own, swapped for the table after rendering.
		return fmt.Sprintf("\nOKFBLOCK-%d-X\n", len(d.blocks)-1)
	case strings.EqualFold(path.Ext(p), ".md"):
		if seen[p] || len(seen) > maxImportDepth {
			fmt.Fprintf(os.Stderr, "warning: %s: skipping recursive import of %s\n", d.path, p)
			return ""
		}
		data, err := os.ReadFile(ex.abs(p))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", d.path, err)
			return ""
		}
		_, body := splitFrontmatter(data)
		seen[p] = true
		defer delete(seen, p)
		return "\n" + string(ex.preprocess(d, p, body, seen)) + "\n"
	default:
		// Anything else becomes a code block, as in Crossnote.
		data, err := os.ReadFile(ex.abs(p))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", d.path, err)
			return ""
		}
		fence := "```"
		for strings.Contains(string(data), fence) {
			fence += "`"
		}
		lang := strings.TrimPrefix(strings.ToLower(path.Ext(p)), ".")
		return "\n" + fence + lang + "\n" + strings.TrimRight(string(data), "\n") + "\n" + fence + "\n"
	}
}

func (d *doc) fillBlocks(s string) string {
	if len(d.blocks) == 0 {
		return s
	}
	return blockMarker.ReplaceAllStringFunc(s, func(m string) string {
		i, _ := strconv.Atoi(blockMarker.FindStringSubmatch(m)[1])
		return d.blocks[i]
	})
}

func (ex *exporter) abs(p string) string {
	return filepath.Join(ex.b.root, filepath.FromSlash(p))
}

// csvTable renders CSV/TSV data as an HTML table whose first row is the
// header. lineNumbers adds a GitHub-style row number column.
func csvTable(p string, data []byte, lineNumbers bool) string {
	rows, err := readTable(p, data)
	var b strings.Builder
	if err != nil {
		fmt.Fprintf(&b, `<p class="csv-error">Could not parse %s: %s</p>`, html.EscapeString(path.Base(p)), html.EscapeString(err.Error()))
	}
	if len(rows) == 0 {
		return b.String()
	}
	b.WriteString(`<table class="csv-table">`)
	b.WriteString("<thead><tr>")
	if lineNumbers {
		b.WriteString(`<th class="ln"></th>`)
	}
	for _, c := range rows[0] {
		b.WriteString("<th>" + html.EscapeString(c) + "</th>")
	}
	b.WriteString("</tr></thead><tbody>")
	body := rows[1:]
	shown := body
	if len(shown) > maxTableRows {
		shown = shown[:maxTableRows]
	}
	for i, r := range shown {
		b.WriteString("<tr>")
		if lineNumbers {
			fmt.Fprintf(&b, `<td class="ln">%d</td>`, i+1)
		}
		for _, c := range r {
			b.WriteString("<td>" + html.EscapeString(c) + "</td>")
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")
	if len(shown) < len(body) {
		fmt.Fprintf(&b, `<p class="csv-note">Showing the first %d of %d rows.</p>`, len(shown), len(body))
	}
	return b.String()
}

func readTable(p string, data []byte) ([][]string, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	r := csv.NewReader(bytes.NewReader(data))
	if strings.EqualFold(path.Ext(p), ".tsv") {
		r.Comma = '\t'
	}
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	var rows [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return rows, err
		}
		rows = append(rows, rec)
	}
}

// countRows returns the number of data rows (excluding the header).
func countRows(p string, data []byte) int {
	rows, _ := readTable(p, data)
	if len(rows) == 0 {
		return 0
	}
	return len(rows) - 1
}

func escapeMD(s string) string {
	return strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`).Replace(s)
}
