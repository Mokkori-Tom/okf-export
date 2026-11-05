package main

// mdext.go: markdown syntax from Crossnote / Markdown Preview Enhanced that
// goldmark lacks: [[wiki links]], ==highlight== and [TOC].

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// wikiResolve finds the bundle file a wiki link names, the way OKF resolves
// them (Obsidian's order): relative to the linking file's directory, then
// from the bundle root, then by file name anywhere, preferring the same
// directory and then the shortest path. "/a" means the root only. The name
// may leave out ".md". It returns "" if nothing matches.
func (ex *exporter) wikiResolve(from, target string) string {
	t := strings.TrimSpace(strings.ReplaceAll(target, `\`, "/"))
	rooted := strings.HasPrefix(t, "/")
	t = strings.TrimPrefix(t, "/")
	if t == "" {
		return ""
	}
	has := func(p string) string {
		for _, c := range []string{p, p + ".md"} {
			if ex.b.files[c] {
				return c
			}
		}
		return ""
	}
	dir := path.Dir(from)
	if dir == "." {
		dir = ""
	}
	if !rooted && dir != "" {
		if p := has(path.Join(dir, t)); p != "" {
			return p
		}
	}
	if p := has(path.Clean(t)); p != "" || rooted {
		return p
	}
	name := strings.ToLower(path.Base(t))
	var matches []string
	for f := range ex.b.files {
		b := strings.ToLower(path.Base(f))
		if b == name || b == name+".md" {
			matches = append(matches, f)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		si, sj := path.Dir(matches[i]) == path.Dir(from), path.Dir(matches[j]) == path.Dir(from)
		if si != sj {
			return si
		}
		if len(matches[i]) != len(matches[j]) {
			return len(matches[i]) < len(matches[j])
		}
		return matches[i] < matches[j]
	})
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// headingID is the id a heading with this text gets (see headingIDs), for
// [[page#Heading]] links.
func headingID(s string) string {
	return string(newHeadingIDs().Generate([]byte(s), ast.KindHeading))
}

// wikiLinks parses [[target]], [[target|text]], [[target#heading]] and the
// embed ![[image.png]] inside a line. Links become ordinary links to the
// resolved file, so the link rewriter and backlinks treat them like
// [text](path); an unresolved one is reported as a broken link.
type wikiLinks struct{ ex *exporter }

func (w wikiLinks) Trigger() []byte { return []byte{'[', '!'} }

func (w wikiLinks) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	embed := bytes.HasPrefix(line, []byte("![["))
	start := 2
	if embed {
		start = 3
	} else if !bytes.HasPrefix(line, []byte("[[")) {
		return nil
	}
	end := bytes.Index(line[start:], []byte("]]"))
	if end <= 0 {
		return nil
	}
	inner := string(line[start : start+end])
	if strings.ContainsAny(inner, "[\n") {
		return nil
	}
	block.Advance(start + end + 2)

	target, label, hasLabel := strings.Cut(inner, "|")
	target, heading, _ := strings.Cut(target, "#")
	if !hasLabel {
		label = strings.TrimSpace(inner)
	}
	d := pc.Get(ctxDocKey).(*doc)
	dest := ""
	switch {
	case strings.TrimSpace(target) == "": // [[#heading]] on this page
	default:
		p := w.ex.wikiResolve(d.path, target)
		if p == "" {
			// Like a broken [link](x), but leave the text without a link.
			d.broken++
			fmt.Fprintf(os.Stderr, "warning: %s: unresolved wiki link [[%s]]\n", d.path, inner)
			return ast.NewString([]byte(label))
		}
		dest = (&url.URL{Path: "/" + p}).EscapedPath()
	}
	if heading = strings.TrimSpace(heading); heading != "" {
		dest += "#" + url.PathEscape(headingID(heading))
	}
	link := ast.NewLink()
	link.Destination = []byte(dest)
	if embed {
		img := ast.NewImage(link)
		img.AppendChild(img, ast.NewString([]byte(label)))
		return img
	}
	link.AppendChild(link, ast.NewString([]byte(label)))
	link.SetAttributeString("class", []byte("wikilink"))
	return link
}

// ==highlight== becomes <mark>.
var kindMark = ast.NewNodeKind("Mark")

type markNode struct{ ast.BaseInline }

func (n *markNode) Kind() ast.NodeKind         { return kindMark }
func (n *markNode) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }
func (markParser) Trigger() []byte             { return []byte{'='} }
func (markRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindMark, func(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			_, _ = w.WriteString("<mark>")
		} else {
			_, _ = w.WriteString("</mark>")
		}
		return ast.WalkContinue, nil
	})
}

type markParser struct{}
type markRenderer struct{}

func (markParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, seg := block.PeekLine()
	if !bytes.HasPrefix(line, []byte("==")) || len(line) < 5 || line[2] == ' ' || line[2] == '=' {
		return nil
	}
	end := bytes.Index(line[2:], []byte("=="))
	if end <= 0 || line[2+end-1] == ' ' {
		return nil
	}
	n := &markNode{}
	n.AppendChild(n, ast.NewTextSegment(text.NewSegment(seg.Start+2, seg.Start+2+end)))
	block.Advance(end + 4)
	return n
}

// [TOC] alone in a paragraph becomes a list of the page's headings.
var kindTOC = ast.NewNodeKind("TOC")

type tocNode struct {
	ast.BaseBlock
	items []tocItem
}

type tocItem struct {
	level    int
	id, text string
}

func (n *tocNode) Kind() ast.NodeKind         { return kindTOC }
func (n *tocNode) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

type tocTransformer struct{}

func (tocTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	src := reader.Source()
	var marks []ast.Node
	var items []tocItem
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Paragraph:
			if n.Lines().Len() == 1 {
				if seg := n.Lines().At(0); strings.EqualFold(strings.TrimSpace(string(seg.Value(src))), "[TOC]") {
					marks = append(marks, n)
				}
			}
			return ast.WalkSkipChildren, nil
		case *ast.Heading:
			id, _ := n.AttributeString("id")
			idb, _ := id.([]byte)
			items = append(items, tocItem{n.Level, string(idb), nodeText(n, src)})
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	for _, m := range marks {
		m.Parent().ReplaceChild(m.Parent(), m, &tocNode{items: items})
	}
}

// nodeText is the plain text inside n.
func nodeText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
		case *ast.Text:
			b.Write(c.Segment.Value(src))
			if c.SoftLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(c.Value)
		case *ast.CodeSpan:
			for t := c.FirstChild(); t != nil; t = t.NextSibling() {
				if tx, ok := t.(*ast.Text); ok {
					b.Write(tx.Segment.Value(src))
				}
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

type tocRenderer struct{}

func (tocRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindTOC, func(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		items := n.(*tocNode).items
		if len(items) == 0 {
			return ast.WalkContinue, nil
		}
		top := items[0].level
		for _, it := range items {
			top = min(top, it.level)
		}
		_, _ = w.WriteString(`<nav class="toc"><ul>`)
		depth := top
		for i, it := range items {
			switch {
			case i == 0:
				for ; depth < it.level; depth++ {
					_, _ = w.WriteString("<li><ul>")
				}
			case it.level > depth: // a sub-heading: a list inside the open item
				for ; depth < it.level; depth++ {
					_, _ = w.WriteString("<ul>")
				}
			default:
				for ; depth > it.level; depth-- {
					_, _ = w.WriteString("</li></ul>")
				}
				_, _ = w.WriteString("</li>")
			}
			fmt.Fprintf(w, `<li><a href="#%s">%s</a>`, html.EscapeString(it.id), html.EscapeString(it.text))
		}
		for ; depth > top; depth-- {
			_, _ = w.WriteString("</li></ul>")
		}
		_, _ = w.WriteString("</li></ul></nav>\n")
		return ast.WalkContinue, nil
	})
}
