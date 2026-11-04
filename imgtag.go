package main

import (
	"html"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// imgTags turns raw HTML made only of <img> tags into image nodes, so the
// GitHub way of sizing an image, <img src="a.png" width="300">, works here
// too: the path is resolved and embedded like ![](a.png), and only the
// attributes GitHub keeps (alt, title, width, height, align) are used.
// A block may wrap the images in <p> or <div>, as in
// <p align="center"><img src="logo.png" width="200"></p>.
// Any other HTML is left to the renderer (dropped unless -unsafe).
type imgTags struct{}

var (
	htmlTag  = regexp.MustCompile(`<\s*(/?)\s*([A-Za-z][A-Za-z0-9]*)((?:[^>"']|"[^"]*"|'[^']*')*?)\s*/?\s*>`)
	htmlAttr = regexp.MustCompile(`([A-Za-z_:][-A-Za-z0-9_:.]*)(?:\s*=\s*("[^"]*"|'[^']*'|[^\s"'=<>` + "`" + `]+))?`)
)

func (imgTags) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	src := reader.Source()
	var raws []ast.Node
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch n.(type) {
			case *ast.RawHTML, *ast.HTMLBlock:
				raws = append(raws, n)
			}
		}
		return ast.WalkContinue, nil
	})
	for _, n := range raws {
		switch n := n.(type) {
		case *ast.RawHTML:
			var b strings.Builder
			for i := 0; i < n.Segments.Len(); i++ {
				seg := n.Segments.At(i)
				b.Write(seg.Value(src))
			}
			imgs, _, ok := parseImgHTML(b.String(), false)
			if ok && len(imgs) == 1 {
				n.Parent().ReplaceChild(n.Parent(), n, imgs[0])
			}
		case *ast.HTMLBlock:
			var b strings.Builder
			for i := 0; i < n.Lines().Len(); i++ {
				line := n.Lines().At(i)
				b.Write(line.Value(src))
			}
			if n.HasClosure() {
				b.Write(n.ClosureLine.Value(src))
			}
			imgs, align, ok := parseImgHTML(b.String(), true)
			if !ok || len(imgs) == 0 {
				continue
			}
			p := ast.NewParagraph()
			if align != "" {
				p.SetAttributeString("style", []byte("text-align:"+align))
			}
			for i, img := range imgs {
				if i > 0 {
					p.AppendChild(p, ast.NewString([]byte(" ")))
				}
				p.AppendChild(p, img)
			}
			n.Parent().ReplaceChild(n.Parent(), n, p)
		}
	}
}

// parseImgHTML reads s as <img> tags with only white space between them and,
// for a block, <p>/<div>/<br> around them. It returns the images and the
// wrapper's align, or ok=false if s holds anything else.
func parseImgHTML(s string, block bool) (imgs []ast.Node, align string, ok bool) {
	last := 0
	for _, m := range htmlTag.FindAllStringSubmatchIndex(s, -1) {
		if strings.TrimSpace(s[last:m[0]]) != "" {
			return nil, "", false
		}
		last = m[1]
		closing := m[2] != m[3]
		name := strings.ToLower(s[m[4]:m[5]])
		attrs := tagAttrs(s[m[6]:m[7]])
		switch {
		case name == "img" && !closing:
			img := imgNode(attrs)
			if img == nil {
				return nil, "", false
			}
			imgs = append(imgs, img)
		case block && (name == "p" || name == "div"):
			if a := strings.ToLower(attrs["align"]); !closing && (a == "center" || a == "left" || a == "right") {
				align = a
			}
		case block && name == "br":
		default:
			return nil, "", false
		}
	}
	if strings.TrimSpace(s[last:]) != "" {
		return nil, "", false
	}
	return imgs, align, true
}

func tagAttrs(s string) map[string]string {
	attrs := map[string]string{}
	for _, m := range htmlAttr.FindAllStringSubmatch(s, -1) {
		v := m[2]
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') {
			v = v[1 : len(v)-1]
		}
		attrs[strings.ToLower(m[1])] = html.UnescapeString(v)
	}
	return attrs
}

// imgNode builds ![alt](src "title") with width/height as CSS, the same as
// {width=..} {height=..}, so sizes like 300 or 50% keep the aspect ratio.
func imgNode(attrs map[string]string) ast.Node {
	src := strings.TrimSpace(attrs["src"])
	if src == "" {
		return nil
	}
	link := ast.NewLink()
	link.Destination = []byte(src)
	if t := attrs["title"]; t != "" {
		link.Title = []byte(t)
	}
	img := ast.NewImage(link)
	if alt := attrs["alt"]; alt != "" {
		img.AppendChild(img, ast.NewString([]byte(alt)))
	}
	var styles []string
	for _, k := range []string{"width", "height"} {
		v := strings.TrimSpace(attrs[k])
		if !cssLength.MatchString(v) {
			continue
		}
		if plainNum.MatchString(v) {
			v += "px"
		}
		styles = append(styles, k+":"+v)
	}
	if len(styles) > 0 {
		img.SetAttributeString("style", []byte(strings.Join(styles, ";")))
	}
	if a := strings.ToLower(attrs["align"]); a == "left" || a == "right" {
		img.SetAttributeString("align", []byte(a))
	}
	return img
}
