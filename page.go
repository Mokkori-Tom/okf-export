package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
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

//go:embed assets/app.js
var appJS string

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
	Rows  int
	Table template.HTML
}

// fileView backs the buttons on a file's header: show the raw source, copy
// it, download it, and (for markdown) an outline of the headings.
type fileView struct {
	Name        string
	DownloadURL template.URL
	Code        template.HTML // source with line numbers; empty if too large
	Lines       int
	Rendered    bool // has a rendered view to switch with the source
	Outline     bool
}

// mediaView is the page of a file that is neither markdown, a table nor
// text: an image or PDF shown in the page, or anything else with only a
// download button, like GitHub's file view.
type mediaView struct {
	Kind string // "image", "pdf", "large" (text over maxTextSize) or "binary"
	Src  template.URL
	Size string
}

// searchEntry is one page in the client-side search index.
type searchEntry struct {
	Page        string `json:"u"`
	Title       string `json:"t"`
	Path        string `json:"p"`
	Description string `json:"d,omitempty"`
	Text        string `json:"x,omitempty"`
}

type pageData struct {
	Page        string // output path, also the route in single-file mode
	BundleName  string
	HomeURL     string
	Root        string // relative URL of the output root, "" or "../.."
	Script      template.JS
	Title       string
	Crumbs      []crumb
	Meta        docMeta
	FileName    string
	Front       template.HTML
	Body        template.HTML
	Sources     []source
	Backlinks   []entry
	OKFVersion  string // okf_version declared by the bundle root's index.md
	CSV         *csvView
	File        *fileView
	Media       *mediaView
	Stylesheets template.CSS
	LiveReload  bool

	search searchEntry
}

type stats struct{ pages, dirs, tables, texts, files, resources, broken int }

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

	// Only concepts cite: links from the reserved index.md and log.md are
	// navigation, as in "okf backlinks".
	backlinks := map[string][]*doc{}
	for _, p := range ex.b.docs {
		if b := strings.ToLower(path.Base(p)); b == "index.md" || b == "log.md" {
			continue
		}
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
		data.Root = strings.Repeat("../", strings.Count(page, "/"))
		data.Script = template.JS(appJS)
		data.search.Page = page
		data.search.Title = data.Title
		if data.Meta.Description != "" {
			data.search.Description = data.Meta.Description
		}
		var names []string
		for _, c := range data.Crumbs[1:] {
			names = append(names, c.Name)
		}
		data.search.Path = strings.Join(names, "/")
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

	// Other text files, shown as source with line numbers.
	for _, p := range sortedKeys(ex.b.texts) {
		raw, err := os.ReadFile(ex.abs(p))
		if err != nil {
			return nil, st, err
		}
		page := ex.b.pageFor(p)
		data := &pageData{Title: path.Base(p), FileName: path.Base(p)}
		data.File = ex.fileView(p, raw, false)
		data.search.Text = plainText(html.EscapeString(string(raw)))
		data.Crumbs = ex.crumbs(page, path.Dir(p), path.Base(p))
		add(page, data)
		st.texts++
	}

	// Every other file: images and PDFs are shown, the rest can be downloaded.
	for _, p := range sortedKeys(ex.b.files) {
		if isTable(p) || ex.b.texts[p] || strings.EqualFold(path.Ext(p), ".md") {
			continue
		}
		page := ex.b.pageFor(p)
		data := &pageData{Title: path.Base(p), FileName: path.Base(p)}
		data.File = &fileView{Name: path.Base(p)}
		data.Media = &mediaView{Kind: mediaKind(ex.abs(p))}
		if info, err := os.Stat(ex.abs(p)); err == nil {
			data.Media.Size = humanSize(info.Size())
		}
		if i, err := ex.embed(p); err == nil {
			data.File.DownloadURL = template.URL(fmt.Sprintf("okfres-%d-x", i))
			data.Media.Src = data.File.DownloadURL
		}
		data.Crumbs = ex.crumbs(page, path.Dir(p), path.Base(p))
		add(page, data)
		st.files++
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
		data.File = ex.fileView(p, raw, true)
		data.search.Text = plainText(string(data.CSV.Table))
		data.Crumbs = ex.crumbs(page, path.Dir(p), path.Base(p))
		add(page, data)
		st.tables++
	}

	// Directory pages show the directory's index.md, the listing OKF uses to
	// navigate a bundle (§8): headings with "[Title](url) - description"
	// entries. Where there is none, one is made from the directory's contents,
	// which §8 allows. index.md has no frontmatter, except that the bundle
	// root's may declare okf_version (§12).
	for _, dir := range sortedKeys(ex.b.dirs) {
		page := ex.b.pageFor(dir)
		data := &pageData{}
		if idx := docs[path.Join(dir, "index.md")]; idx != nil {
			data.FileName = "index.md"
			data.Body = template.HTML(idx.body)
			data.Sources = idx.meta.Sources
			data.File = ex.fileView(idx.path, idx.raw, true)
			data.File.Outline = true
			data.Backlinks = ex.backlinks(idx, backlinks[idx.path])
			if dir == "" {
				data.OKFVersion = frontValue(idx.front, "okf_version")
			}
		} else {
			data.FileName = "Index"
			data.Body = template.HTML(synthIndex(ex.entries(dir, page, docs)))
		}
		data.search.Text = plainText(string(data.Body))
		data.Title = path.Base(dir)
		data.Crumbs = ex.crumbs(page, path.Dir(dir), path.Base(dir))
		if dir == "" {
			data.Title = ex.name
			data.Crumbs = []crumb{{Name: ex.name}}
		}
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
	js, err := searchJS(pages)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, searchFile), js, 0o644)
}

// searchFile is the search index next to the top index.html. Pages load it
// with a <script> tag, which also works for files opened from disk.
const searchFile = "okf-search.js"

func searchIndex(pages []*pageData) []searchEntry {
	idx := make([]searchEntry, len(pages))
	for i, p := range pages {
		idx[i] = p.search
	}
	return idx
}

func searchJS(pages []*pageData) ([]byte, error) {
	b, err := json.Marshal(searchIndex(pages))
	if err != nil {
		return nil, err
	}
	return append(append([]byte("window.OKF_SEARCH="), b...), ";\n"...), nil
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
		Script      template.JS
		Pages       []*pageData
		Resources   []string
		Search      []searchEntry
	}{ex.name, stylesheets, template.JS(appJS), pages, ex.res, searchIndex(pages)}
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
	data.File = ex.fileView(d.path, d.raw, true)
	data.File.Outline = true
	data.search.Text = plainText(d.body)
	data.Backlinks = ex.backlinks(d, refs)
	return data
}

// backlinks lists the docs in refs, which link to d, for its "Cited by" box.
func (ex *exporter) backlinks(d *doc, refs []*doc) []entry {
	sort.Slice(refs, func(i, j int) bool { return refs[i].meta.Title < refs[j].meta.Title })
	var es []entry
	for _, r := range refs {
		es = append(es, entry{
			Name:        r.path,
			Title:       r.meta.Title,
			URL:         template.URL(ex.href(d.page, r.page)),
			Description: r.meta.Description,
		})
	}
	return es
}

// frontValue returns the scalar value of key in a frontmatter mapping.
func frontValue(n *yaml.Node, key string) string {
	if n == nil {
		return ""
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key && n.Content[i+1].Kind == yaml.ScalarNode {
			return n.Content[i+1].Value
		}
	}
	return ""
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

// entries lists dir's contents for a generated index, folders first.
func (ex *exporter) entries(dir, page string, docs map[string]*doc) []entry {
	var es []entry
	for _, c := range ex.b.dirs[dir] {
		name := path.Base(c)
		if ex.b.isDir(c) {
			es = append(es, entry{Dir: true, Name: name, URL: template.URL(ex.href(page, ex.b.pageFor(c)))})
			continue
		}
		if name == "index.md" {
			continue
		}
		e := entry{Name: name}
		switch d := docs[c]; {
		case d != nil:
			e.URL = template.URL(ex.href(page, d.page))
			e.Title = d.meta.Title
			e.Description = d.meta.Description
		default:
			e.URL = template.URL(ex.href(page, ex.b.pageFor(c)))
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

func (ex *exporter) fileView(p string, raw []byte, rendered bool) *fileView {
	f := &fileView{Name: path.Base(p), Rendered: rendered}
	if i, err := ex.embed(p); err == nil {
		f.DownloadURL = template.URL(fmt.Sprintf("okfres-%d-x", i))
	}
	if len(raw) <= maxTextSize {
		f.Code = template.HTML(codeView(p, raw))
		f.Lines = bytes.Count(raw, []byte("\n"))
		if len(raw) > 0 && raw[len(raw)-1] != '\n' {
			f.Lines++
		}
	}
	return f
}

func mediaKind(file string) string {
	switch strings.ToLower(path.Ext(file)) {
	case ".pdf":
		return "pdf"
	}
	if isImage(file) {
		return "image"
	}
	if f, err := os.Open(file); err == nil {
		defer f.Close()
		head := make([]byte, 8192)
		n, _ := io.ReadFull(f, head)
		if !bytes.Contains(head[:n], []byte{0}) && strings.HasPrefix(http.DetectContentType(head[:n]), "text/") {
			return "large" // isTextFile only rejects such files for their size
		}
	}
	return "binary"
}

func humanSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d Bytes", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(n)/(1024*1024*1024))
}

// synthIndex writes an index in the shape of OKF §8 for a directory without
// an index.md: concepts by title with their descriptions, then other files,
// then subdirectories.
func synthIndex(es []entry) string {
	var concepts, files, dirs strings.Builder
	item := func(b *strings.Builder, e entry, label string) {
		fmt.Fprintf(b, `<li><a href="%s">%s</a>`, html.EscapeString(string(e.URL)), html.EscapeString(label))
		if e.Description != "" {
			b.WriteString(" - " + html.EscapeString(e.Description))
		}
		b.WriteString("</li>\n")
	}
	for _, e := range es {
		switch {
		case e.Dir:
			item(&dirs, e, e.Name+"/")
		case e.Title != "":
			item(&concepts, e, e.Title)
		default:
			item(&files, e, e.Name)
		}
	}
	var b strings.Builder
	for _, s := range []struct{ head, items string }{{"Concepts", concepts.String()}, {"Files", files.String()}, {"Subdirectories", dirs.String()}} {
		if s.items != "" {
			fmt.Fprintf(&b, "<h2>%s</h2>\n<ul>\n%s</ul>\n", s.head, s.items)
		}
	}
	if b.Len() == 0 {
		return `<p class="muted">This directory is empty.</p>`
	}
	return b.String()
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
