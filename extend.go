package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// inlineAttrs applies Markdown Preview Enhanced style attribute lists written
// right after an image or link, e.g. ![](a.png){height=800px} or
// [x](y){.btn #top title="..."}.
type inlineAttrs struct{}

func (inlineAttrs) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	src := reader.Source()
	var targets []ast.Node
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch n.(type) {
			case *ast.Image, *ast.Link:
				targets = append(targets, n)
			}
		}
		return ast.WalkContinue, nil
	})
	for _, n := range targets {
		applyInlineAttrs(n, src)
	}
}

func applyInlineAttrs(n ast.Node, src []byte) {
	// The "{...}" may be split over several adjacent text nodes.
	var texts []*ast.Text
	var sb strings.Builder
	end := -1
	for c := n.NextSibling(); c != nil; c = c.NextSibling() {
		t, ok := c.(*ast.Text)
		if !ok || (len(texts) > 0 && t.Segment.Start != texts[len(texts)-1].Segment.Stop) {
			break
		}
		v := string(t.Segment.Value(src))
		if len(texts) == 0 && !strings.HasPrefix(v, "{") {
			return
		}
		texts = append(texts, t)
		if i := strings.IndexByte(v, '}'); i >= 0 {
			sb.WriteString(v[:i])
			end = i + 1
			break
		}
		sb.WriteString(v)
		if t.SoftLineBreak() || t.HardLineBreak() {
			break
		}
	}
	if end < 0 {
		return
	}
	attrs, ok := parseAttrList(strings.TrimPrefix(sb.String(), "{"))
	if !ok {
		return
	}
	for _, a := range attrs {
		n.SetAttributeString(a[0], []byte(a[1]))
	}
	// Drop the consumed "{...}" from the text that follows.
	for i, t := range texts {
		if i == len(texts)-1 {
			t.Segment = t.Segment.WithStart(t.Segment.Start + end)
		} else {
			t.Segment = t.Segment.WithStart(t.Segment.Stop)
		}
	}
}

var (
	attrName  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)
	cssLength = regexp.MustCompile(`^(?:\d+(?:\.\d+)?(?:px|em|rem|%|vh|vw|ch|ex|cm|mm|in|pt)?|auto)$`)
	plainNum  = regexp.MustCompile(`^\d+(?:\.\d+)?$`)
)

// parseAttrList parses the inside of "{...}": key=value, key="quoted value",
// .class and #id, separated by spaces. width and height become CSS so units
// such as 800px or 50% work; bare numbers mean pixels.
func parseAttrList(s string) ([][2]string, bool) {
	var toks []string
	for s = strings.TrimSpace(s); s != ""; s = strings.TrimSpace(s) {
		i := 0
		for i < len(s) && s[i] != ' ' && s[i] != '\t' {
			if s[i] == '"' || s[i] == '\'' {
				q := s[i]
				j := strings.IndexByte(s[i+1:], q)
				if j < 0 {
					return nil, false
				}
				i += j + 2
				continue
			}
			i++
		}
		toks = append(toks, s[:i])
		s = s[i:]
	}
	if len(toks) == 0 {
		return nil, false
	}
	var classes, styles []string
	var out [][2]string
	for _, t := range toks {
		switch {
		case strings.HasPrefix(t, "."):
			if !attrName.MatchString(t[1:]) {
				return nil, false
			}
			classes = append(classes, t[1:])
		case strings.HasPrefix(t, "#"):
			if !attrName.MatchString(t[1:]) {
				return nil, false
			}
			out = append(out, [2]string{"id", t[1:]})
		default:
			k, v, found := strings.Cut(t, "=")
			if !found || !attrName.MatchString(k) {
				return nil, false
			}
			if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
				v = v[1 : len(v)-1]
			}
			switch k = strings.ToLower(k); k {
			case "width", "height":
				if !cssLength.MatchString(v) {
					continue
				}
				if plainNum.MatchString(v) {
					v += "px"
				}
				styles = append(styles, k+":"+v)
			case "class":
				classes = append(classes, strings.Fields(v)...)
			case "style":
				styles = append(styles, strings.TrimRight(v, "; "))
			default:
				// The renderer drops attributes that aren't safe for the element.
				out = append(out, [2]string{k, v})
			}
		}
	}
	if len(classes) > 0 {
		out = append(out, [2]string{"class", strings.Join(classes, " ")})
	}
	if len(styles) > 0 {
		out = append(out, [2]string{"style", strings.Join(styles, ";")})
	}
	return out, true
}

// Alerts and callouts: a blockquote whose first line is [!TYPE], optionally
// followed by a title, as on GitHub and in Markdown Preview Enhanced /
// Obsidian:
//
//	> [!TIP] Custom title
//	> text
//
// GitHub's NOTE, TIP, IMPORTANT, WARNING and CAUTION and Obsidian's callout
// types are known; others look like a note. "[!TIP]-" folds the alert
// (closed) and "[!TIP]+" folds it open.
var alertMarker = regexp.MustCompile(`^\s*\[!([A-Za-z]+)\]([+-]?)[ \t]*(.*?)\s*$`)

// alertKinds maps a type to the style it uses.
var alertKinds = map[string]string{
	"note": "note", "tip": "tip", "hint": "tip", "important": "important",
	"warning": "warning", "attention": "warning", "caution": "caution",
	"info": "info", "todo": "info", "abstract": "abstract", "summary": "abstract", "tldr": "abstract",
	"success": "success", "check": "success", "done": "success",
	"question": "question", "help": "question", "faq": "question",
	"failure": "failure", "fail": "failure", "missing": "failure",
	"danger": "danger", "error": "danger", "bug": "bug",
	"example": "example", "quote": "quote", "cite": "quote",
}

var kindAlert = ast.NewNodeKind("Alert")

type alertNode struct {
	ast.BaseBlock
	style string // key of alertIcons, also the CSS class
	fold  string // "", "-" (closed) or "+" (open)
}

// alertTitle is an alert's first child: the title's inline nodes.
var kindAlertTitle = ast.NewNodeKind("AlertTitle")

type alertTitle struct {
	ast.BaseBlock
	alert *alertNode
}

func (n *alertTitle) Kind() ast.NodeKind         { return kindAlertTitle }
func (n *alertTitle) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

func (n *alertNode) Kind() ast.NodeKind { return kindAlert }

func (n *alertNode) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"style": n.style}, nil)
}

type alertTransformer struct{}

func (alertTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	src := reader.Source()
	var quotes []*ast.Blockquote
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if q, ok := n.(*ast.Blockquote); ok && entering {
			quotes = append(quotes, q)
		}
		return ast.WalkContinue, nil
	})
	for _, q := range quotes {
		p, ok := q.FirstChild().(*ast.Paragraph)
		if !ok || p.Lines().Len() == 0 {
			continue
		}
		first := p.Lines().At(0)
		line := first.Value(src)
		m := alertMarker.FindSubmatchIndex(line)
		if m == nil {
			continue
		}
		typ := strings.ToLower(string(line[m[2]:m[3]]))
		a := &alertNode{style: alertKinds[typ], fold: string(line[m[4]:m[5]])}
		if a.style == "" {
			a.style = "note"
		}
		// The first line's inline nodes: the marker goes, the rest is the
		// title (so **bold** or `code` in it work).
		title := &alertTitle{alert: a}
		titleAt := first.Start + m[6]
		for c := p.FirstChild(); c != nil && inlineStart(c) < first.Stop; {
			next := c.NextSibling()
			p.RemoveChild(p, c)
			if t, ok := c.(*ast.Text); ok {
				if t.Segment.Stop <= titleAt {
					c = next
					continue
				}
				if t.Segment.Start < titleAt {
					t.Segment = t.Segment.WithStart(titleAt)
				}
				t.SetSoftLineBreak(false)
			} else if inlineStart(c) < titleAt {
				c = next
				continue
			}
			title.AppendChild(title, c)
			c = next
		}
		if !title.HasChildren() {
			title.AppendChild(title, ast.NewString([]byte(strings.ToUpper(typ[:1])+typ[1:])))
		}
		if !p.HasChildren() {
			q.RemoveChild(q, p)
		}
		a.AppendChild(a, title)
		for c := q.FirstChild(); c != nil; {
			next := c.NextSibling()
			a.AppendChild(a, c)
			c = next
		}
		q.Parent().ReplaceChild(q.Parent(), q, a)
	}
}

// inlineStart is where inline node n starts in the source: its own segment
// for text, else that of the first text inside it.
func inlineStart(n ast.Node) int {
	start := -1
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := c.(*ast.Text); ok && entering {
			start = t.Segment.Start
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	if start < 0 {
		return int(^uint(0) >> 1)
	}
	return start
}

var alertIcons = map[string]string{
	"success":   `M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0Z`,
	"question":  `M0 8a8 8 0 1 1 16 0A8 8 0 0 1 0 8Zm8-6.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13ZM6.92 6.085h.001a.749.749 0 1 1-1.342-.67c.169-.339.436-.701.849-.977C6.845 4.16 7.369 4 8 4a2.756 2.756 0 0 1 1.637.525c.503.377.863.965.863 1.725 0 .448-.115.83-.329 1.15-.205.307-.47.513-.692.662-.109.072-.22.138-.313.195l-.006.004a6.24 6.24 0 0 0-.26.16.952.952 0 0 0-.276.245.75.75 0 0 1-1.248-.832c.184-.264.42-.489.692-.661.103-.067.207-.132.313-.195l.007-.004c.1-.061.182-.11.258-.161a.969.969 0 0 0 .277-.245C8.96 6.514 9 6.427 9 6.25a.612.612 0 0 0-.262-.525A1.27 1.27 0 0 0 8 5.5c-.369 0-.595.09-.74.187a1.01 1.01 0 0 0-.34.398ZM9 11a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z`,
	"quote":     `M1.75 2.5h10.5a.75.75 0 0 1 0 1.5H1.75a.75.75 0 0 1 0-1.5Zm4 5h8.5a.75.75 0 0 1 0 1.5h-8.5a.75.75 0 0 1 0-1.5Zm0 5h8.5a.75.75 0 0 1 0 1.5h-8.5a.75.75 0 0 1 0-1.5ZM2.5 7.75v6a.75.75 0 0 1-1.5 0v-6a.75.75 0 0 1 1.5 0Z`,
	"note":      `M0 8a8 8 0 1 1 16 0A8 8 0 0 1 0 8Zm8-6.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13ZM6.5 7.75A.75.75 0 0 1 7.25 7h1a.75.75 0 0 1 .75.75v2.75h.25a.75.75 0 0 1 0 1.5h-2a.75.75 0 0 1 0-1.5h.25v-2h-.25a.75.75 0 0 1-.75-.75ZM8 6a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z`,
	"tip":       `M8 1.5c-2.363 0-4 1.69-4 3.75 0 .984.424 1.625.984 2.304l.214.253c.223.264.47.556.673.848.284.411.537.896.621 1.49a.75.75 0 0 1-1.484.211c-.04-.282-.163-.547-.37-.847a8.456 8.456 0 0 0-.542-.68c-.084-.1-.173-.205-.268-.32C3.201 7.75 2.5 6.766 2.5 5.25 2.5 2.31 4.863 0 8 0s5.5 2.31 5.5 5.25c0 1.516-.701 2.5-1.328 3.259-.095.115-.184.22-.268.319-.207.245-.383.453-.541.681-.208.3-.33.565-.37.847a.751.751 0 0 1-1.485-.212c.084-.593.337-1.078.621-1.489.203-.292.45-.584.673-.848.075-.088.147-.173.213-.253.561-.679.985-1.32.985-2.304 0-2.06-1.637-3.75-4-3.75ZM5.75 12h4.5a.75.75 0 0 1 0 1.5h-4.5a.75.75 0 0 1 0-1.5ZM6 15.25a.75.75 0 0 1 .75-.75h2.5a.75.75 0 0 1 0 1.5h-2.5a.75.75 0 0 1-.75-.75Z`,
	"important": `M0 1.75C0 .784.784 0 1.75 0h12.5C15.216 0 16 .784 16 1.75v9.5A1.75 1.75 0 0 1 14.25 13H8.06l-2.573 2.573A1.458 1.458 0 0 1 3 14.543V13H1.75A1.75 1.75 0 0 1 0 11.25Zm1.75-.25a.25.25 0 0 0-.25.25v9.5c0 .138.112.25.25.25h2a.75.75 0 0 1 .75.75v2.19l2.72-2.72a.749.749 0 0 1 .53-.22h6.5a.25.25 0 0 0 .25-.25v-9.5a.25.25 0 0 0-.25-.25Zm7 2.25v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 9a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z`,
	"warning":   `M6.457 1.047c.659-1.234 2.427-1.234 3.086 0l6.082 11.378A1.75 1.75 0 0 1 14.082 15H1.918a1.75 1.75 0 0 1-1.543-2.575Zm1.763.707a.25.25 0 0 0-.44 0L1.698 13.132a.25.25 0 0 0 .22.368h12.164a.25.25 0 0 0 .22-.368Zm.53 3.996v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 11a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z`,
	"caution":   `M4.47.22A.749.749 0 0 1 5 0h6c.199 0 .389.079.53.22l4.25 4.25c.141.14.22.331.22.53v6a.749.749 0 0 1-.22.53l-4.25 4.25A.749.749 0 0 1 11 16H5a.749.749 0 0 1-.53-.22L.22 11.53A.749.749 0 0 1 0 11V5c0-.199.079-.389.22-.53Zm.84 1.28L1.5 5.31v5.38l3.81 3.81h5.38l3.81-3.81V5.31L10.69 1.5ZM8 4a.75.75 0 0 1 .75.75v3.5a.75.75 0 0 1-1.5 0v-3.5A.75.75 0 0 1 8 4Zm0 8a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z`,
}

type alertRenderer struct{}

// alertIcon gives the icon for each style; the extra styles borrow one.
func alertIcon(style string) string {
	switch style {
	case "info", "abstract":
		style = "note"
	case "failure", "danger", "bug":
		style = "caution"
	case "example":
		style = "tip"
	}
	return alertIcons[style]
}

func (alertRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindAlert, func(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		a := n.(*alertNode)
		tag, open := "div", ""
		if a.fold != "" {
			tag = "details"
			if a.fold == "+" {
				open = ` open=""` // also valid XML, for EPUB
			}
		}
		if entering {
			fmt.Fprintf(w, `<%s class="markdown-alert markdown-alert-%s"%s>`, tag, a.style, open)
		} else {
			fmt.Fprintf(w, "</%s>\n", tag)
		}
		return ast.WalkContinue, nil
	})
	reg.Register(kindAlertTitle, func(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		a := n.(*alertTitle).alert
		tag := "p"
		if a.fold != "" {
			tag = "summary"
		}
		if entering {
			fmt.Fprintf(w, `<%s class="markdown-alert-title"><svg class="octicon" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="%s"></path></svg>`,
				tag, alertIcon(a.style))
		} else {
			fmt.Fprintf(w, "</%s>\n", tag)
		}
		return ast.WalkContinue, nil
	})
}

// headingIDs makes heading ids the way mdBook does, keeping letters in any
// script: "## トラブル シューティング" gets #トラブル-シューティング rather
// than goldmark's #heading. Repeats get -1, -2, ...
type headingIDs struct{ used map[string]bool }

func newHeadingIDs() *headingIDs { return &headingIDs{used: map[string]bool{}} }

func (h *headingIDs) Generate(value []byte, kind ast.NodeKind) []byte {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(string(value))) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	id := b.String()
	if id == "" {
		id = "heading"
	}
	base := id
	for i := 1; h.used[id]; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}
	h.used[id] = true
	return []byte(id)
}

func (h *headingIDs) Put(value []byte) { h.used[string(value)] = true }
