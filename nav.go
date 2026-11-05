package main

import (
	"fmt"
	"html"
	"io"
	"path"
	"sort"
	"strings"
)

// navItem is one entry of the sidebar: a directory (its index page) or a
// markdown document. The tree follows the bundle's directories: in each one
// its documents by file name, then its subdirectories by name. Directories
// without any markdown below them are left out.
type navItem struct {
	Title    string
	Page     string // output path
	Children []*navItem
}

func (ex *exporter) navTree(docs map[string]*doc) *navItem {
	var walk func(dir string) *navItem
	walk = func(dir string) *navItem {
		it := &navItem{Title: path.Base(dir), Page: ex.b.pageFor(dir)}
		if dir == "" {
			it.Title = ex.name
		}
		var files, subdirs []string
		for _, c := range ex.b.dirs[dir] {
			switch {
			case ex.b.isDir(c):
				subdirs = append(subdirs, c)
			case docs[c] != nil && path.Base(c) != "index.md":
				files = append(files, c)
			}
		}
		byName := func(s []string) {
			sort.Slice(s, func(i, j int) bool { return strings.ToLower(s[i]) < strings.ToLower(s[j]) })
		}
		byName(files)
		byName(subdirs)
		for _, f := range files {
			it.Children = append(it.Children, &navItem{Title: docs[f].meta.Title, Page: docs[f].page})
		}
		for _, s := range subdirs {
			if sub := walk(s); sub != nil {
				it.Children = append(it.Children, sub)
			}
		}
		if dir != "" && len(it.Children) == 0 && docs[path.Join(dir, "index.md")] == nil {
			return nil
		}
		return it
	}
	return walk("")
}

// top is the sidebar's first level: the bundle's front page, then what is
// directly in the bundle, like an mdBook's introduction and chapters.
func (root *navItem) top() []*navItem {
	return append([]*navItem{{Title: root.Title, Page: root.Page}}, root.Children...)
}

// order lists the pages in reading order, for previous/next links.
func (root *navItem) order() []*navItem {
	var out []*navItem
	var walk func(its []*navItem)
	walk = func(its []*navItem) {
		for _, it := range its {
			out = append(out, it)
			walk(it.Children)
		}
	}
	walk(root.top())
	return out
}

// tocHTML renders the sidebar. Links carry the target in data-path; the page
// script turns it into a URL for the current page and marks where it is.
func (root *navItem) tocHTML() string {
	var b strings.Builder
	var walk func(its []*navItem, class string)
	walk = func(its []*navItem, class string) {
		fmt.Fprintf(&b, `<ol class="%s">`, class)
		for _, it := range its {
			// Open, as in mdBook; the arrows fold a section away.
			b.WriteString(`<li class="chapter-item expanded"><span class="chapter-link">`)
			fmt.Fprintf(&b, `<a data-path="%s">%s</a>`, html.EscapeString(it.Page), html.EscapeString(it.Title))
			if len(it.Children) > 0 {
				b.WriteString(`<button type="button" class="toggle" aria-label="Toggle section"><span>❱</span></button>`)
			}
			b.WriteString(`</span>`)
			if len(it.Children) > 0 {
				walk(it.Children, "section")
			}
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ol>`)
	}
	walk(root.top(), "chapter")
	return b.String()
}

// writeSummary writes the tree as an mdBook SUMMARY.md, with links to the
// markdown sources, so the same bundle can be built with mdBook.
func (root *navItem) writeSummary(w io.Writer, src func(page string) string) {
	fmt.Fprintf(w, "# Summary\n\n[%s](%s)\n\n", escapeMD(root.Title), src(root.Page))
	var walk func(its []*navItem, depth int)
	walk = func(its []*navItem, depth int) {
		for _, it := range its {
			fmt.Fprintf(w, "%s- [%s](%s)\n", strings.Repeat("  ", depth), escapeMD(it.Title), src(it.Page))
			walk(it.Children, depth+1)
		}
	}
	walk(root.Children, 0)
}
