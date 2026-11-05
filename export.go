package main

// export.go: the whole book on one page (print.html, like mdBook's), PDF
// printed from it by a headless Chrome/Chromium, and EPUB 3.

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// printFile is the whole book on one page, next to the site's index.html.
const printFile = "print.html"

// bookOrder returns the pages in reading order (the sidebar's), then the
// pages of other files that aren't in the sidebar.
func (ex *exporter) bookOrder(pages []*pageData) (inNav, rest []*pageData) {
	byPage := map[string]*pageData{}
	for _, p := range pages {
		byPage[p.Page] = p
	}
	seen := map[string]bool{}
	for _, it := range ex.nav.order() {
		if p := byPage[it.Page]; p != nil && !seen[it.Page] {
			inNav = append(inNav, p)
			seen[it.Page] = true
		}
	}
	for _, p := range pages {
		if !seen[p.Page] {
			rest = append(rest, p)
		}
	}
	return inNav, rest
}

var crossPageAnchor = regexp.MustCompile(`href="#/[^"#]*#([^"]+)"`)

// printPage renders print.html: every chapter in reading order, each in a
// section whose id is its "/page" route, so the single-file links
// ("#/a/b.html") lead to it. Links to a heading on another page go to the
// heading of that name.
func printPage(b *bundle, name string, unsafe bool, lang string) ([]byte, error) {
	ex := newExporter(b, name, unsafe, true)
	ex.lang = lang
	pages, _, err := ex.build()
	if err != nil {
		return nil, err
	}
	chapters, _ := ex.bookOrder(pages)
	var sb strings.Builder
	data := struct {
		BundleName, Lang string
		Stylesheets      any
		Pages            []*pageData
	}{name, lang, stylesheets, chapters}
	if err := tmpl.ExecuteTemplate(&sb, "print.html", data); err != nil {
		return nil, err
	}
	s := crossPageAnchor.ReplaceAllString(sb.String(), `href="#$1"`)
	return []byte(ex.fillResources(s)), nil
}

// chrome finds a Chrome or Chromium to print the PDF with: $OKF_CHROME, then
// the usual commands and install locations.
func chrome() (string, error) {
	if c := os.Getenv("OKF_CHROME"); c != "" {
		return c, nil
	}
	for _, c := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "chrome", "microsoft-edge", "msedge", "brave-browser"} {
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	var places []string
	switch runtime.GOOS {
	case "darwin":
		places = []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Chromium.app/Contents/MacOS/Chromium", "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"}
	case "windows":
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
			if d := os.Getenv(env); d != "" {
				places = append(places, filepath.Join(d, `Google\Chrome\Application\chrome.exe`), filepath.Join(d, `Microsoft\Edge\Application\msedge.exe`))
			}
		}
	}
	for _, p := range places {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no Chrome or Chromium found for the PDF: install one, set OKF_CHROME to its path, or print print.html from a browser")
}

// writePDF prints print.html to out with a headless Chrome/Chromium.
func writePDF(b *bundle, name string, unsafe bool, lang, out string) error {
	browser, err := chrome()
	if err != nil {
		return err
	}
	page, err := printPage(b, name, unsafe, lang)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "okf-export-pdf")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	src := filepath.Join(tmp, printFile)
	if err := os.WriteFile(src, page, 0o644); err != nil {
		return err
	}
	abs, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	args := []string{"--headless=new", "--disable-gpu", "--no-pdf-header-footer",
		"--user-data-dir=" + filepath.Join(tmp, "profile"),
		"--virtual-time-budget=10000", "--print-to-pdf=" + abs, (&urlFile{src}).String()}
	if runtime.GOOS == "linux" && os.Geteuid() == 0 {
		args = append([]string{"--no-sandbox"}, args...) // Chrome refuses to run as root otherwise
	}
	cmd := exec.Command(browser, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %v\n%s", browser, err, strings.TrimSpace(stderr.String()))
	}
	if st, err := os.Stat(abs); err != nil || st.Size() == 0 {
		return fmt.Errorf("%s wrote no PDF\n%s", browser, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// urlFile is a file:// URL for a local path.
type urlFile struct{ path string }

func (u *urlFile) String() string {
	p := filepath.ToSlash(u.path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // C:/x on Windows
	}
	return "file://" + strings.ReplaceAll(p, " ", "%20")
}

// ---- EPUB ----

// epubTypes are the image types EPUB readers must support; other embedded
// files are left out of the book.
var epubTypes = map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/gif": "gif", "image/svg+xml": "svg", "image/webp": "webp"}

var (
	xmlEntity = regexp.MustCompile(`&([A-Za-z][A-Za-z0-9]{1,31});`)
	localHTML = regexp.MustCompile(`(href|src)="([^":?#]*)\.html((?:#[^"]*)?)"`)
)

// writeEPUB writes the book as EPUB 3: one XHTML document per page, in the
// sidebar's order, with the sidebar as the table of contents.
func writeEPUB(b *bundle, name, lang, out string) error {
	ex := newExporter(b, name, false, false)
	ex.lang = lang
	ex.setMarkdown(false, true)
	pages, _, err := ex.build()
	if err != nil {
		return err
	}
	chapters, rest := ex.bookOrder(pages)

	// Embedded images become files in the book.
	resPath := map[int]string{}
	resData := map[int][]byte{}
	for i, uri := range ex.res {
		typ, data, ok := decodeDataURI(uri)
		if ext := epubTypes[typ]; ok && ext != "" {
			resPath[i] = fmt.Sprintf("res/%d.%s", i, ext)
			resData[i] = data
		}
	}

	type item struct{ id, href, typ, props string }
	var manifest []item
	docs := map[string][]byte{}
	render := func(p *pageData) error {
		var sb strings.Builder
		sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
		if err := tmpl.ExecuteTemplate(&sb, "epub", p); err != nil {
			return err
		}
		s := resPlaceholder.ReplaceAllStringFunc(sb.String(), func(m string) string {
			var i int
			fmt.Sscanf(m, "okfres-%d-x", &i)
			if rp, ok := resPath[i]; ok {
				return relURL(p.Page, rp)
			}
			return "#"
		})
		s = localHTML.ReplaceAllString(s, `$1="$2.xhtml$3"`)
		s = xmlEntity.ReplaceAllStringFunc(s, func(m string) string {
			switch m {
			case "&amp;", "&lt;", "&gt;", "&quot;", "&apos;":
				return m
			}
			return html.EscapeString(html.UnescapeString(m))
		})
		if err := wellFormed(s); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: not well-formed XHTML for EPUB: %v\n", p.Page, err)
		}
		href := strings.TrimSuffix(p.Page, ".html") + ".xhtml"
		docs[href] = []byte(s)
		var props []string
		if strings.Contains(s, "<svg") {
			props = append(props, "svg")
		}
		if strings.Contains(s, `src="http://`) || strings.Contains(s, `src="https://`) {
			props = append(props, "remote-resources")
		}
		manifest = append(manifest, item{fmt.Sprintf("p%d", len(manifest)+1), href, "application/xhtml+xml", strings.Join(props, " ")})
		return nil
	}
	for _, p := range append(chapters, rest...) {
		if err := render(p); err != nil {
			return err
		}
	}

	// nav.xhtml: the sidebar as the table of contents.
	var nav strings.Builder
	fmt.Fprintf(&nav, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="%[1]s" xml:lang="%[1]s">
<head><meta charset="UTF-8" /><title>%[2]s</title><link rel="stylesheet" type="text/css" href="style.css" /></head>
<body class="content"><nav epub:type="toc" id="toc"><h1>%[2]s</h1>`, html.EscapeString(lang), html.EscapeString(name))
	var walk func(its []*navItem)
	walk = func(its []*navItem) {
		nav.WriteString("<ol>")
		for _, it := range its {
			fmt.Fprintf(&nav, `<li><a href="%s">%s</a>`, html.EscapeString(strings.TrimSuffix(it.Page, ".html")+".xhtml"), html.EscapeString(it.Title))
			if len(it.Children) > 0 {
				walk(it.Children)
			}
			nav.WriteString("</li>")
		}
		nav.WriteString("</ol>")
	}
	walk(ex.nav.top())
	nav.WriteString("</nav></body></html>\n")

	// content.opf
	// A name-based UUID, so rebuilding the same book keeps its identifier.
	sum := sha1.Sum([]byte("okf-export:" + name))
	sum[6] = sum[6]&0x0f | 0x50
	sum[8] = sum[8]&0x3f | 0x80
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
	var opf strings.Builder
	fmt.Fprintf(&opf, `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid" xml:lang="%s">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:identifier id="bookid">urn:uuid:%s</dc:identifier>
<dc:title>%s</dc:title>
<dc:language>%s</dc:language>
<meta property="dcterms:modified">%s</meta>
</metadata>
<manifest>
<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
<item id="css" href="style.css" media-type="text/css"/>
`, html.EscapeString(lang), uuid, html.EscapeString(name), html.EscapeString(lang), time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	for _, it := range manifest {
		props := ""
		if it.props != "" {
			props = ` properties="` + it.props + `"`
		}
		fmt.Fprintf(&opf, `<item id="%s" href="%s" media-type="%s"%s/>`+"\n", it.id, html.EscapeString(it.href), it.typ, props)
	}
	for i := range ex.res {
		if rp, ok := resPath[i]; ok {
			typ, _, _ := decodeDataURI(ex.res[i])
			fmt.Fprintf(&opf, `<item id="r%d" href="%s" media-type="%s"/>`+"\n", i, rp, typ)
		}
	}
	opf.WriteString("</manifest>\n<spine>\n")
	for i, it := range manifest {
		linear := ""
		if i >= len(chapters) {
			linear = ` linear="no"`
		}
		fmt.Fprintf(&opf, `<itemref idref="%s"%s/>`+"\n", it.id, linear)
	}
	opf.WriteString("</spine>\n</package>\n")

	// The zip: mimetype first and uncompressed, as EPUB requires.
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	z := zip.NewWriter(f)
	add := func(name string, data []byte, store bool) error {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()}
		if store {
			h.Method = zip.Store
		}
		w, err := z.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	files := []struct {
		name  string
		data  []byte
		store bool
	}{
		{"mimetype", []byte("application/epub+zip"), true},
		{"META-INF/container.xml", []byte(`<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
`), false},
		{"OEBPS/content.opf", []byte(opf.String()), false},
		{"OEBPS/nav.xhtml", []byte(nav.String()), false},
		{"OEBPS/style.css", []byte(epubCSS()), false},
	}
	for _, it := range manifest {
		files = append(files, struct {
			name  string
			data  []byte
			store bool
		}{path.Join("OEBPS", it.href), docs[it.href], false})
	}
	for i, rp := range resPath {
		files = append(files, struct {
			name  string
			data  []byte
			store bool
		}{path.Join("OEBPS", rp), resData[i], false})
	}
	for _, fl := range files {
		if err := add(fl.name, fl.data, fl.store); err != nil {
			f.Close()
			return err
		}
	}
	if err := z.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func decodeDataURI(uri string) (typ string, data []byte, ok bool) {
	rest, found := strings.CutPrefix(uri, "data:")
	if !found {
		return "", nil, false
	}
	meta, b64, found := strings.Cut(rest, ",")
	if !found || !strings.HasSuffix(meta, ";base64") {
		return "", nil, false
	}
	typ, _, _ = strings.Cut(strings.TrimSuffix(meta, ";base64"), ";")
	data, err := base64.StdEncoding.DecodeString(b64)
	return typ, data, err == nil
}

// wellFormed checks that s parses as XML.
func wellFormed(s string) error {
	d := xml.NewDecoder(strings.NewReader(s))
	d.Strict = true
	for {
		if _, err := d.Token(); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
	}
}

// epubCSS is the site's stylesheet for a light page: the Light theme's
// colours on the root, since readers may drop the class on <html>, and the
// code colours without the theme prefix.
func epubCSS() string {
	css := strings.Replace(bookCSS, "html.light {", ":root, html.light {", 1)
	var code strings.Builder
	for _, line := range strings.Split(highlightCSS, "\n") {
		if strings.Contains(line, "html.light .chroma") {
			code.WriteString(strings.Replace(line, "html.light .chroma", ".chroma", 1) + "\n")
		}
	}
	return css + code.String() + `
/* EPUB: no screen chrome, the reader sets the margins. */
body { margin: 0; overflow: visible; }
.content { padding: 0; overflow: visible; }
.content main { max-width: none; }
.content pre { white-space: pre-wrap; }
`
}
