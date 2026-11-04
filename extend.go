package main

import (
	"fmt"
	"regexp"
	"strings"

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

// GitHub alerts: a blockquote whose first line is [!NOTE], [!TIP],
// [!IMPORTANT], [!WARNING] or [!CAUTION].
var alertMarker = regexp.MustCompile(`(?i)^\s*\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*$`)

var kindAlert = ast.NewNodeKind("Alert")

type alertNode struct {
	ast.BaseBlock
	kind string // lower case: note, tip, ...
}

func (n *alertNode) Kind() ast.NodeKind { return kindAlert }

func (n *alertNode) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"kind": n.kind}, nil)
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
		m := alertMarker.FindSubmatch(first.Value(src))
		if m == nil {
			continue
		}
		// Remove the marker line's inline nodes.
		for c := p.FirstChild(); c != nil; {
			t, ok := c.(*ast.Text)
			if !ok || t.Segment.Start >= first.Stop {
				break
			}
			next := c.NextSibling()
			p.RemoveChild(p, c)
			c = next
		}
		if !p.HasChildren() {
			q.RemoveChild(q, p)
		}
		a := &alertNode{kind: strings.ToLower(string(m[1]))}
		for c := q.FirstChild(); c != nil; {
			next := c.NextSibling()
			a.AppendChild(a, c)
			c = next
		}
		q.Parent().ReplaceChild(q.Parent(), q, a)
	}
}

var alertIcons = map[string]string{
	"note":      `M0 8a8 8 0 1 1 16 0A8 8 0 0 1 0 8Zm8-6.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13ZM6.5 7.75A.75.75 0 0 1 7.25 7h1a.75.75 0 0 1 .75.75v2.75h.25a.75.75 0 0 1 0 1.5h-2a.75.75 0 0 1 0-1.5h.25v-2h-.25a.75.75 0 0 1-.75-.75ZM8 6a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z`,
	"tip":       `M8 1.5c-2.363 0-4 1.69-4 3.75 0 .984.424 1.625.984 2.304l.214.253c.223.264.47.556.673.848.284.411.537.896.621 1.49a.75.75 0 0 1-1.484.211c-.04-.282-.163-.547-.37-.847a8.456 8.456 0 0 0-.542-.68c-.084-.1-.173-.205-.268-.32C3.201 7.75 2.5 6.766 2.5 5.25 2.5 2.31 4.863 0 8 0s5.5 2.31 5.5 5.25c0 1.516-.701 2.5-1.328 3.259-.095.115-.184.22-.268.319-.207.245-.383.453-.541.681-.208.3-.33.565-.37.847a.751.751 0 0 1-1.485-.212c.084-.593.337-1.078.621-1.489.203-.292.45-.584.673-.848.075-.088.147-.173.213-.253.561-.679.985-1.32.985-2.304 0-2.06-1.637-3.75-4-3.75ZM5.75 12h4.5a.75.75 0 0 1 0 1.5h-4.5a.75.75 0 0 1 0-1.5ZM6 15.25a.75.75 0 0 1 .75-.75h2.5a.75.75 0 0 1 0 1.5h-2.5a.75.75 0 0 1-.75-.75Z`,
	"important": `M0 1.75C0 .784.784 0 1.75 0h12.5C15.216 0 16 .784 16 1.75v9.5A1.75 1.75 0 0 1 14.25 13H8.06l-2.573 2.573A1.458 1.458 0 0 1 3 14.543V13H1.75A1.75 1.75 0 0 1 0 11.25Zm1.75-.25a.25.25 0 0 0-.25.25v9.5c0 .138.112.25.25.25h2a.75.75 0 0 1 .75.75v2.19l2.72-2.72a.749.749 0 0 1 .53-.22h6.5a.25.25 0 0 0 .25-.25v-9.5a.25.25 0 0 0-.25-.25Zm7 2.25v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 9a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z`,
	"warning":   `M6.457 1.047c.659-1.234 2.427-1.234 3.086 0l6.082 11.378A1.75 1.75 0 0 1 14.082 15H1.918a1.75 1.75 0 0 1-1.543-2.575Zm1.763.707a.25.25 0 0 0-.44 0L1.698 13.132a.25.25 0 0 0 .22.368h12.164a.25.25 0 0 0 .22-.368Zm.53 3.996v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 11a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z`,
	"caution":   `M4.47.22A.749.749 0 0 1 5 0h6c.199 0 .389.079.53.22l4.25 4.25c.141.14.22.331.22.53v6a.749.749 0 0 1-.22.53l-4.25 4.25A.749.749 0 0 1 11 16H5a.749.749 0 0 1-.53-.22L.22 11.53A.749.749 0 0 1 0 11V5c0-.199.079-.389.22-.53Zm.84 1.28L1.5 5.31v5.38l3.81 3.81h5.38l3.81-3.81V5.31L10.69 1.5ZM8 4a.75.75 0 0 1 .75.75v3.5a.75.75 0 0 1-1.5 0v-3.5A.75.75 0 0 1 8 4Zm0 8a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z`,
}

type alertRenderer struct{}

func (alertRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindAlert, func(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		a := n.(*alertNode)
		if !entering {
			_, _ = w.WriteString("</div>\n")
			return ast.WalkContinue, nil
		}
		fmt.Fprintf(w, `<div class="markdown-alert markdown-alert-%s"><p class="markdown-alert-title">`+
			`<svg class="octicon" viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"><path d="%s"></path></svg>%s</p>`+"\n",
			a.kind, alertIcons[a.kind], strings.ToUpper(a.kind[:1])+a.kind[1:])
		return ast.WalkContinue, nil
	})
}
