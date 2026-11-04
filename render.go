package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"gopkg.in/yaml.v3"
)

// doc is one parsed markdown file.
type doc struct {
	path       string     // bundle path, e.g. "tables/events_.md"
	page       string     // output path, e.g. "tables/events_.html"
	front      *yaml.Node // mapping node, nil when there is no frontmatter
	meta       docMeta
	frontTitle bool            // title came from frontmatter rather than a fallback
	raw        []byte          // source as read from disk
	body       string          // rendered HTML
	blocks     []string        // HTML for @import'ed tables, see preprocess
	links      map[string]bool // bundle paths of linked docs (for backlinks)
	broken     int
}

// docMeta holds the frontmatter keys the page layout treats specially.
type docMeta struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Type        string   `yaml:"type"`
	Status      string   `yaml:"status"`
	Resource    string   `yaml:"resource"`
	Tags        []string `yaml:"tags"`
	Sources     []source `yaml:"sources"`
}

type source struct {
	ID       string `yaml:"id"`
	Title    string `yaml:"title"`
	Resource string `yaml:"resource"`
}

// splitFrontmatter separates a leading YAML block delimited by "---" lines.
func splitFrontmatter(src []byte) (front, body []byte) {
	s := bytes.TrimPrefix(src, []byte("\xef\xbb\xbf"))
	s = bytes.ReplaceAll(s, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(s, []byte("---\n")) {
		return nil, s
	}
	rest := s[4:]
	if bytes.HasPrefix(rest, []byte("---\n")) {
		return nil, rest[4:]
	}
	end := bytes.Index(rest, []byte("\n---\n"))
	if end < 0 {
		if bytes.HasSuffix(rest, []byte("\n---")) {
			return rest[:len(rest)-4], nil
		}
		return nil, s
	}
	return rest[:end], rest[end+5:]
}

var ctxDocKey = parser.NewContextKey()

type exporter struct {
	b       *bundle
	name    string
	single  bool // links are "#/page" routes inside one HTML file
	md      goldmark.Markdown
	res     []string       // data URIs, indexed by placeholder number
	resIdx  map[string]int // bundle path -> index in res
	resErrs map[string]bool
}

func newExporter(b *bundle, name string, unsafe, single bool) *exporter {
	ex := &exporter{b: b, name: name, single: single, resIdx: map[string]int{}, resErrs: map[string]bool{}}
	var ropts []goldmark.Option
	if unsafe {
		ropts = append(ropts, goldmark.WithRendererOptions(gmhtml.WithUnsafe()))
	}
	ex.md = goldmark.New(append(ropts,
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			highlighting.NewHighlighting(
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
				highlighting.WithGuessLanguage(false),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithAttribute(),
			parser.WithASTTransformers( // lower priority runs first
				util.Prioritized(imgTags{}, 90), // before linkRewriter resolves the src
				// before linkRewriter wraps images in links, so {...} after an image stays on it
				util.Prioritized(inlineAttrs{}, 95),
				util.Prioritized(&linkRewriter{ex: ex}, 100),
				util.Prioritized(alertTransformer{}, 120),
			),
		),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(alertRenderer{}, 500))),
	)...)
	return ex
}

func (ex *exporter) parse(p string) (*doc, error) {
	src, err := os.ReadFile(filepath.Join(ex.b.root, filepath.FromSlash(p)))
	if err != nil {
		return nil, err
	}
	d := &doc{path: p, page: ex.b.pageFor(p), raw: src, links: map[string]bool{}}
	front, body := splitFrontmatter(src)
	if front != nil {
		var n yaml.Node
		if err := yaml.Unmarshal(front, &n); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: bad frontmatter: %v\n", p, err)
		} else if len(n.Content) == 1 && n.Content[0].Kind == yaml.MappingNode {
			d.front = n.Content[0]
			_ = d.front.Decode(&d.meta) // best effort; odd shapes just leave fields empty
		}
	}
	ctx := parser.NewContext()
	ctx.Set(ctxDocKey, d)
	var buf bytes.Buffer
	src = ex.preprocess(d, p, body, map[string]bool{p: true})
	if err := ex.md.Convert(src, &buf, parser.WithContext(ctx)); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	d.body = linkSourceRefs(d.fillBlocks(buf.String()), d.meta.Sources)
	d.frontTitle = d.meta.Title != ""
	if d.meta.Title == "" {
		d.meta.Title = firstHeading(body)
	}
	if d.meta.Title == "" {
		d.meta.Title = strings.TrimSuffix(path.Base(p), path.Ext(p))
	}
	return d, nil
}

// linkRewriter points links and images at the exported pages and replaces
// local resources with placeholders that are later swapped for data URIs.
type linkRewriter struct{ ex *exporter }

func (t *linkRewriter) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	d := pc.Get(ctxDocKey).(*doc)
	var tableImages []*ast.Image
	type linkImage struct {
		img  *ast.Image
		href string
	}
	var linkImages []linkImage
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Link:
			if dest, file, ok := t.ex.rewrite(d, string(n.Destination), false); ok {
				n.Destination = []byte(dest)
				if file != "" {
					// Let the browser save the embedded file under its original name.
					n.SetAttributeString("download", []byte(file))
				}
			}
		case *ast.Image:
			if p, _, ok := t.ex.resolveLocal(d.path, string(n.Destination)); ok && isTable(p) && t.ex.b.files[p] {
				// A table can't be an inline image; standalone ones were
				// already expanded by preprocess, so link to the table page.
				tableImages = append(tableImages, n)
				return ast.WalkSkipChildren, nil
			}
			orig := string(n.Destination)
			if _, inLink := n.Parent().(*ast.Link); !inLink {
				// GitHub opens an image when it is clicked; here that is its page.
				if p, _, ok := t.ex.resolveLocal(d.path, orig); ok && t.ex.b.files[p] {
					linkImages = append(linkImages, linkImage{n, t.ex.href(d.page, t.ex.b.pageFor(p))})
				} else if strings.HasPrefix(orig, "https://") || strings.HasPrefix(orig, "http://") {
					linkImages = append(linkImages, linkImage{n, orig})
				}
			}
			if dest, _, ok := t.ex.rewrite(d, orig, true); ok {
				n.Destination = []byte(dest)
			}
		}
		return ast.WalkContinue, nil
	})
	for _, li := range linkImages {
		link := ast.NewLink()
		link.Destination = []byte(li.href)
		if strings.HasPrefix(li.href, "http") {
			link.SetAttributeString("target", []byte("_blank"))
			link.SetAttributeString("rel", []byte("noopener noreferrer"))
		}
		parent := li.img.Parent()
		parent.ReplaceChild(parent, li.img, link)
		link.AppendChild(link, li.img)
	}
	for _, img := range tableImages {
		dest, _, _ := t.ex.rewrite(d, string(img.Destination), false)
		link := ast.NewLink()
		link.Destination = []byte(dest)
		link.Title = img.Title
		for c := img.FirstChild(); c != nil; {
			next := c.NextSibling()
			link.AppendChild(link, c)
			c = next
		}
		if !link.HasChildren() {
			link.AppendChild(link, ast.NewString([]byte(path.Base(string(img.Destination)))))
		}
		img.Parent().ReplaceChild(img.Parent(), img, link)
	}
}

// resolveLocal turns a link destination into a bundle path plus the trailing
// "#fragment"/"?query" part. ok is false for external or in-page links.
func (ex *exporter) resolveLocal(from, dest string) (p, suffix string, ok bool) {
	if dest == "" || strings.HasPrefix(dest, "#") || strings.HasPrefix(dest, "//") {
		return "", "", false
	}
	if u, err := url.Parse(dest); err != nil || u.Scheme != "" {
		return "", "", false
	}
	if i := strings.IndexAny(dest, "?#"); i >= 0 {
		dest, suffix = dest[:i], dest[i:]
	}
	if un, err := url.PathUnescape(dest); err == nil {
		dest = un
	}
	if strings.HasPrefix(dest, "/") {
		p = path.Clean(dest[1:])
	} else {
		p = path.Join(path.Dir(from), dest)
	}
	if p == "." {
		p = ""
	}
	if strings.HasPrefix(p, "../") || p == ".." {
		return "", "", false // escapes the bundle; leave untouched
	}
	return p, suffix, true
}

// rewrite returns the new destination and, for embedded non-image files, the
// file name to offer for download.
func (ex *exporter) rewrite(d *doc, dest string, image bool) (string, string, bool) {
	p, suffix, ok := ex.resolveLocal(d.path, dest)
	if !ok {
		return "", "", false
	}
	switch {
	case ex.b.isDir(p) && !image:
		if idx := path.Join(p, "index.md"); ex.b.files[idx] {
			d.links[idx] = true
		}
		return ex.href(d.page, ex.b.pageFor(p)) + suffix, "", true
	case ex.b.files[p] && strings.EqualFold(path.Ext(p), ".md") && !image:
		d.links[p] = true
		return ex.href(d.page, ex.b.pageFor(p)) + suffix, "", true
	case ex.b.files[p] && !image:
		// Like GitHub, a link to a file opens its page; downloading is a
		// button there.
		return ex.href(d.page, ex.b.pageFor(p)) + suffix, "", true
	case ex.b.files[p]:
		i, err := ex.embed(p)
		if err != nil {
			if !ex.resErrs[p] {
				fmt.Fprintf(os.Stderr, "warning: %s: cannot embed %s: %v\n", d.path, p, err)
				ex.resErrs[p] = true
			}
			return "", "", false
		}
		file := ""
		if !image {
			file = path.Base(p)
		}
		return fmt.Sprintf("okfres-%d-x", i), file, true
	default:
		d.broken++
		fmt.Fprintf(os.Stderr, "warning: %s: unresolved link %q\n", d.path, dest)
		return "", "", false
	}
}

func (ex *exporter) embed(p string) (int, error) {
	if i, ok := ex.resIdx[p]; ok {
		return i, nil
	}
	data, err := os.ReadFile(filepath.Join(ex.b.root, filepath.FromSlash(p)))
	if err != nil {
		return 0, err
	}
	typ := mime.TypeByExtension(strings.ToLower(path.Ext(p)))
	if typ == "" {
		typ = http.DetectContentType(data)
	}
	if i := strings.Index(typ, ";"); i >= 0 && !strings.HasPrefix(typ, "text/") {
		typ = typ[:i]
	}
	typ = strings.ReplaceAll(typ, " ", "") // "text/csv; charset=utf-8"
	uri := "data:" + typ + ";base64," + base64.StdEncoding.EncodeToString(data)
	ex.res = append(ex.res, uri)
	ex.resIdx[p] = len(ex.res) - 1
	return len(ex.res) - 1, nil
}

var resPlaceholder = regexp.MustCompile(`okfres-(\d+)-x`)

// fillResources swaps resource placeholders for their data URIs.
func (ex *exporter) fillResources(s string) string {
	return resPlaceholder.ReplaceAllStringFunc(s, func(m string) string {
		i, _ := strconv.Atoi(resPlaceholder.FindStringSubmatch(m)[1])
		return ex.res[i]
	})
}

var (
	sourceRef = regexp.MustCompile(`\[\^([A-Za-z0-9_.:-]+)\]`)
	codeSpans = regexp.MustCompile(`(?s)<pre[\s>].*?</pre>|<code[\s>].*?</code>`)
)

// linkSourceRefs turns OKF citations like "[^ga4-docs]" that name a
// frontmatter source into links to the page's Sources section. Text inside
// code blocks and spans is left alone.
func linkSourceRefs(body string, sources []source) string {
	num := map[string]int{}
	for i, s := range sources {
		if s.ID != "" {
			num[s.ID] = i + 1
		}
	}
	if len(num) == 0 {
		return body
	}
	repl := func(seg string) string {
		return sourceRef.ReplaceAllStringFunc(seg, func(m string) string {
			id := sourceRef.FindStringSubmatch(m)[1]
			n, ok := num[html.UnescapeString(id)]
			if !ok {
				return m
			}
			return fmt.Sprintf(`<sup class="src-ref"><a href="#source-%s">[%d]</a></sup>`, html.EscapeString(id), n)
		})
	}
	var out strings.Builder
	last := 0
	for _, loc := range codeSpans.FindAllStringIndex(body, -1) {
		out.WriteString(repl(body[last:loc[0]]))
		out.WriteString(body[loc[0]:loc[1]])
		last = loc[1]
	}
	out.WriteString(repl(body[last:]))
	return out.String()
}

var atxH1 = regexp.MustCompile(`(?m)^#\s+(.+?)\s*#*\s*$`)

func firstHeading(body []byte) string {
	if m := atxH1.FindSubmatch(body); m != nil {
		return string(m[1])
	}
	return ""
}
