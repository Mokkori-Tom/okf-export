// Command okf-export renders an Open Knowledge Format (OKF) bundle into a
// GitHub-styled static site: one HTML page per concept, CSV file and
// directory, or all of them in a single HTML file. CSS, images and other local
// resources are embedded, so the output has no external dependencies.
// "okf-export serve" previews a bundle with live reload.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const usage = `usage:
  okf-export -in <bundle> -out <dir>             one HTML file per page
  okf-export -in <bundle> -single -out <file>    everything in one HTML file
  okf-export serve -in <bundle> [-addr host:port] live preview
  okf-export summary -in <bundle>                print the sidebar as an mdBook SUMMARY.md

flags:
`

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		serveMain(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "summary" {
		summaryMain(os.Args[2:])
		return
	}
	fl := flag.NewFlagSet("okf-export", flag.ExitOnError)
	in := fl.String("in", "", "OKF bundle directory (required)")
	out := fl.String("out", "", "output directory, or output file with -single (required)")
	single := fl.Bool("single", false, "write the whole bundle into one HTML file")
	name := fl.String("name", "", "bundle name shown in the header (default: input directory name)")
	unsafe := fl.Bool("unsafe", false, "pass raw HTML in markdown through to the output")
	fl.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		fl.PrintDefaults()
	}
	_ = fl.Parse(os.Args[1:])
	if *in == "" || *out == "" {
		fl.Usage()
		os.Exit(2)
	}

	b, err := loadBundle(*in)
	check(err)
	ex := newExporter(b, bundleName(*in, *name), *unsafe, *single)
	pages, st, err := ex.build()
	check(err)
	if *single {
		check(ex.writeSingle(*out, pages))
	} else {
		check(ex.writeSite(*out, pages))
	}
	fmt.Printf("wrote %d concept pages, %d table pages, %d text pages, %d other file pages, %d directory pages, embedded %d resources to %s\n",
		st.pages, st.tables, st.texts, st.files, st.dirs, st.resources, *out)
	if st.broken > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d unresolved local link(s)\n", st.broken)
	}
}

func serveMain(args []string) {
	fl := flag.NewFlagSet("okf-export serve", flag.ExitOnError)
	in := fl.String("in", "", "OKF bundle directory (required)")
	addr := fl.String("addr", "127.0.0.1:8080", "listen address; use :8080 to allow other devices on the LAN")
	name := fl.String("name", "", "bundle name shown in the header (default: input directory name)")
	unsafe := fl.Bool("unsafe", false, "pass raw HTML in markdown through to the output")
	fl.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		fl.PrintDefaults()
	}
	_ = fl.Parse(args)
	if *in == "" {
		fl.Usage()
		os.Exit(2)
	}
	check(serve(*in, bundleName(*in, *name), *addr, *unsafe))
}

// summaryMain prints the sidebar's tree as an mdBook SUMMARY.md linking to
// the markdown sources, so the bundle can be built with mdBook to compare.
func summaryMain(args []string) {
	fl := flag.NewFlagSet("okf-export summary", flag.ExitOnError)
	in := fl.String("in", "", "OKF bundle directory (required)")
	name := fl.String("name", "", "bundle name (default: input directory name)")
	fl.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		fl.PrintDefaults()
	}
	_ = fl.Parse(args)
	if *in == "" {
		fl.Usage()
		os.Exit(2)
	}
	b, err := loadBundle(*in)
	check(err)
	ex := newExporter(b, bundleName(*in, *name), false, false)
	_, _, err = ex.build()
	check(err)
	// Pages map back to sources: x.html is x.md, dir/index.html dir/index.md
	// (an mdBook chapter needs a file; a folder without one gets a blank page).
	src := map[string]string{}
	for _, d := range b.docs {
		src[b.pageFor(d)] = d
	}
	ex.nav.writeSummary(os.Stdout, func(page string) string {
		if s, ok := src[page]; ok {
			return s
		}
		return "" // mdBook draft chapter
	})
}

func bundleName(in, name string) string {
	if name != "" {
		return name
	}
	abs, _ := filepath.Abs(in)
	return filepath.Base(abs)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// bundle is the file inventory of an OKF bundle. All paths are slash-separated
// and relative to the bundle root ("" is the root directory itself).
type bundle struct {
	root  string
	docs  []string            // markdown files
	files map[string]bool     // every regular file
	texts map[string]bool     // plain text files shown as a code page, see isTextFile
	dirs  map[string][]string // directory -> direct children (files and subdirs)
}

func loadBundle(root string) (*bundle, error) {
	st, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}
	b := &bundle{root: root, files: map[string]bool{}, texts: map[string]bool{}, dirs: map[string][]string{"": nil}}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		parent := path.Dir(rel)
		if parent == "." {
			parent = ""
		}
		if d.IsDir() {
			b.dirs[rel] = b.dirs[rel]
			b.dirs[parent] = append(b.dirs[parent], rel)
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b.files[rel] = true
		b.dirs[parent] = append(b.dirs[parent], rel)
		if strings.EqualFold(path.Ext(rel), ".md") {
			b.docs = append(b.docs, rel)
		} else if isTextFile(rel, p, d) {
			b.texts[rel] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(b.docs)
	return b, nil
}

func (b *bundle) isDir(p string) bool {
	_, ok := b.dirs[p]
	return ok
}

// pageFor maps a bundle path (markdown file or directory) to its output HTML
// path. A directory's index.md is merged into the directory page.
func (b *bundle) pageFor(p string) string {
	if b.isDir(p) {
		return path.Join(p, "index.html")
	}
	if path.Base(p) == "index.md" {
		return path.Join(path.Dir(p), "index.html")
	}
	if b.files[p] && !strings.EqualFold(path.Ext(p), ".md") {
		return p + ".html" // every other file gets a page, like GitHub's file view
	}
	return strings.TrimSuffix(p, path.Ext(p)) + ".html"
}

// href links output page `from` to output page `to`: a relative URL for the
// multi-page site, or a "#/to" route in single-file mode.
func (ex *exporter) href(from, to string) string {
	if ex.single {
		return "#/" + to
	}
	return relURL(from, to)
}

// relURL returns the relative URL from output page `from` to output file `to`.
func relURL(from, to string) string {
	r, err := filepath.Rel(filepath.FromSlash(path.Dir(from)), filepath.FromSlash(to))
	if err != nil {
		return to
	}
	return filepath.ToSlash(r)
}
