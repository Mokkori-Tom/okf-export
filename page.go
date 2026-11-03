package main

import (
	"embed"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"gopkg.in/yaml.v3"
)

//go:embed assets/github-markdown.css
var markdownCSS string

//go:embed assets/layout.css
var layoutCSS string

//go:embed assets/*.html
var templateFS embed.FS

var tmpl = template.Must(template.ParseFS(templateFS, "assets/*.html"))

// highlightCSS is the chroma stylesheet for code blocks, following the
// system light/dark preference like github-markdown.css does.
var highlightCSS = func() string {
	var light, dark strings.Builder
	f := chromahtml.New(chromahtml.WithClasses(true))
	_ = f.WriteCSS(&light, styles.Get("github"))
	_ = f.WriteCSS(&dark, styles.Get("github-dark"))
	return light.String() + "@media (prefers-color-scheme: dark) {\n" + dark.String() + "}\n" +
		// Let the markdown CSS own the code block background.
		".markdown-body .chroma { background: transparent; }\n"
}()

var stylesheets = template.CSS(markdownCSS + highlightCSS + layoutCSS)

type crumb struct {
	Name string
	URL  string // empty for the current page
}

type entry struct {
	Dir         bool
	Name        string
	URL         template.URL
	Download    string
	Title       string
	Description string
}

// csvView is the GitHub-style rendered view of a CSV/TSV file.
type csvView struct {
	Rows        int
	Table       template.HTML
	DownloadURL template.URL
}

type pageData struct {
	Page        string // output path, also the route in single-file mode
	BundleName  string
	HomeURL     string
	Title       string
	Crumbs      []crumb
	Meta        docMeta
	FileName    string
	Front       template.HTML
	Body        template.HTML
	Sources     []source
	Entries     []entry
	Backlinks   []entry
	CSV         *csvView
	Stylesheets template.CSS
	LiveReload  bool
}

type stats struct{ pages, dirs, tables, resources, broken int }

// build renders every page of the bundle. Page bodies may still contain
// resource placeholders; see fillResources and inlineResources.
func (ex *exporter) build() ([]*pageData, stats, error) {
	var st stats
	docs := map[string]*doc{}
	for _, p := range ex.b.docs {
		d, err := ex.parse(p)
		if err != nil {
			return nil, st, err
		}
		docs[p] = d
		st.broken += d.broken
	}

	backlinks := map[string][]*doc{}
	for _, p := range ex.b.docs {
		d := docs[p]
		for target := range d.links {
			if target != p {
				backlinks[target] = append(backlinks[target], d)
			}
		}
	}

	var pages []*pageData
	add := func(page string, data *pageData) {
		data.Page = page
		data.BundleName = ex.name
		data.HomeURL = ex.href(page, "index.html")
		pages = append(pages, data)
	}

	// Concept pages. index.md is rendered as part of its directory page.
	for _, p := range ex.b.docs {
		if path.Base(p) == "index.md" {
			continue
		}
		d := docs[p]
		data := ex.docPage(d, backlinks[p])
		data.Crumbs = ex.crumbs(d.page, path.Dir(p), path.Base(p))
		add(d.page, data)
		st.pages++
	}

	// CSV/TSV pages, like GitHub's rendered table view.
	for _, p := range sortedKeys(ex.b.files) {
		if !isTable(p) {
			continue
		}
		raw, err := os.ReadFile(ex.abs(p))
		if err != nil {
			return nil, st, err
		}
		page := ex.b.pageFor(p)
		data := &pageData{Title: path.Base(p), FileName: path.Base(p)}
		data.CSV = &csvView{Rows: countRows(p, raw), Table: template.HTML(csvTable(p, raw, true))}
		if i, err := ex.embed(p); err == nil {
			data.CSV.DownloadURL = template.URL(fmt.Sprintf("okfres-%d-x", i))
		}
		data.Crumbs = ex.crumbs(page, path.Dir(p), path.Base(p))
		add(page, data)
		st.tables++
	}

	// Directory pages: a GitHub-style file list plus index.md (or README.md).
	for _, dir := range sortedKeys(ex.b.dirs) {
		page := ex.b.pageFor(dir)
		var data *pageData
		var readme *doc
		for _, n := range []string{"index.md", "README.md", "readme.md"} {
			if d := docs[path.Join(dir, n)]; d != nil {
				readme = d
				break
			}
		}
		if readme != nil && path.Base(readme.path) == "index.md" {
			data = ex.docPage(readme, backlinks[readme.path])
		} else {
			data = &pageData{}
			if readme != nil {
				data.FileName = path.Base(readme.path)
				data.Body = template.HTML(readme.body) // README.html shares this directory
				data.Sources = readme.meta.Sources
			}
		}
		if dir == "" {
			data.Title = ex.name
		} else if readme == nil || path.Base(readme.path) != "index.md" || !readme.frontTitle {
			data.Title = path.Base(dir)
		}
		data.Crumbs = ex.crumbs(page, path.Dir(dir), path.Base(dir))
		if dir == "" {
			data.Crumbs = []crumb{{Name: ex.name}}
		}
		data.Entries = ex.entries(dir, page, docs)
		add(page, data)
		st.dirs++
	}
	st.resources = len(ex.res)
	return pages, st, nil
}

// renderPage renders one standalone page with its resources inlined.
func (ex *exporter) renderPage(data *pageData, live bool) ([]byte, error) {
	data.Stylesheets = stylesheets
	data.LiveReload = live
	var sb strings.Builder
	if err := tmpl.ExecuteTemplate(&sb, "page.html", data); err != nil {
		return nil, err
	}
	return []byte(ex.fillResources(sb.String())), nil
}

// writeSite writes one HTML file per page under out.
func (ex *exporter) writeSite(out string, pages []*pageData) error {
	for _, data := range pages {
		b, err := ex.renderPage(data, false)
		if err != nil {
			return err
		}
		dst := filepath.Join(out, filepath.FromSlash(data.Page))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

var resAttr = regexp.MustCompile(`(src|href)="okfres-(\d+)-x"`)

// writeSingle writes every page into one HTML file. Pages are switched by a
// small script based on the "#/page" URL hash, and each resource is stored
// once and attached to the elements that use it at load time.
func (ex *exporter) writeSingle(out string, pages []*pageData) error {
	var sb strings.Builder
	data := struct {
		BundleName  string
		Stylesheets template.CSS
		Pages       []*pageData
		Resources   []string
	}{ex.name, stylesheets, pages, ex.res}
	if err := tmpl.ExecuteTemplate(&sb, "single.html", data); err != nil {
		return err
	}
	s := resAttr.ReplaceAllString(sb.String(), `data-okf-$1="$2"`)
	if dir := filepath.Dir(out); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(out, []byte(s), 0o644)
}

func (ex *exporter) docPage(d *doc, refs []*doc) *pageData {
	data := &pageData{
		Title:    d.meta.Title,
		Meta:     d.meta,
		FileName: path.Base(d.path),
		Body:     template.HTML(d.body),
		Sources:  d.meta.Sources,
	}
	if d.front != nil {
		data.Front = template.HTML(renderYAML(d.front))
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].meta.Title < refs[j].meta.Title })
	for _, r := range refs {
		data.Backlinks = append(data.Backlinks, entry{
			Name:        r.path,
			Title:       r.meta.Title,
			URL:         template.URL(ex.href(d.page, r.page)),
			Description: r.meta.Description,
		})
	}
	return data
}

// crumbs builds "bundle / dir / dir / leaf" for a page whose subject lives in
// parent with base name leaf.
func (ex *exporter) crumbs(page, parent, leaf string) []crumb {
	cs := []crumb{{Name: ex.name, URL: ex.href(page, "index.html")}}
	if parent != "." && parent != "" {
		acc := ""
		for _, seg := range strings.Split(parent, "/") {
			acc = path.Join(acc, seg)
			cs = append(cs, crumb{Name: seg, URL: ex.href(page, ex.b.pageFor(acc))})
		}
	}
	return append(cs, crumb{Name: leaf})
}

func (ex *exporter) entries(dir, page string, docs map[string]*doc) []entry {
	var es []entry
	for _, c := range ex.b.dirs[dir] {
		name := path.Base(c)
		if ex.b.isDir(c) {
			e := entry{Dir: true, Name: name, URL: template.URL(ex.href(page, ex.b.pageFor(c)))}
			if d := docs[path.Join(c, "index.md")]; d != nil {
				e.Description = d.meta.Description
			}
			es = append(es, e)
			continue
		}
		if name == "index.md" {
			continue // shown below the list
		}
		e := entry{Name: name}
		switch d := docs[c]; {
		case d != nil:
			e.URL = template.URL(ex.href(page, d.page))
			e.Title = d.meta.Title
			e.Description = d.meta.Description
		case isTable(c):
			e.URL = template.URL(ex.href(page, ex.b.pageFor(c)))
		default:
			if i, err := ex.embed(c); err == nil {
				e.URL = template.URL(fmt.Sprintf("okfres-%d-x", i))
				e.Download = name
			}
		}
		es = append(es, e)
	}
	sort.Slice(es, func(i, j int) bool {
		if es[i].Dir != es[j].Dir {
			return es[i].Dir
		}
		return strings.ToLower(es[i].Name) < strings.ToLower(es[j].Name)
	})
	return es
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// renderYAML renders frontmatter as nested key/value tables, the way GitHub
// shows YAML frontmatter, but vertical so it fits on narrow screens.
func renderYAML(n *yaml.Node) string {
	var b strings.Builder
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		switch n.Kind {
		case yaml.DocumentNode:
			for _, c := range n.Content {
				walk(c)
			}
		case yaml.AliasNode:
			walk(n.Alias)
		case yaml.MappingNode:
			if len(n.Content) == 0 {
				b.WriteString(`<span class="muted">—</span>`)
				return
			}
			b.WriteString(`<table class="fm"><tbody>`)
			for i := 0; i+1 < len(n.Content); i += 2 {
				b.WriteString("<tr><th>")
				b.WriteString(template.HTMLEscapeString(n.Content[i].Value))
				b.WriteString("</th><td>")
				walk(n.Content[i+1])
				b.WriteString("</td></tr>")
			}
			b.WriteString("</tbody></table>")
		case yaml.SequenceNode:
			if len(n.Content) == 0 {
				b.WriteString(`<span class="muted">—</span>`)
				return
			}
			b.WriteString(`<ul class="fm-list">`)
			for _, c := range n.Content {
				b.WriteString("<li>")
				walk(c)
				b.WriteString("</li>")
			}
			b.WriteString("</ul>")
		case yaml.ScalarNode:
			v := n.Value
			switch {
			case v == "":
				b.WriteString(`<span class="muted">—</span>`)
			case strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "http://"):
				fmt.Fprintf(&b, `<a href="%s">%s</a>`, template.HTMLEscapeString(v), template.HTMLEscapeString(v))
			default:
				b.WriteString(template.HTMLEscapeString(v))
			}
		}
	}
	walk(n)
	return b.String()
}
