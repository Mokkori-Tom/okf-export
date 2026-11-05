// Command okf-export renders an Open Knowledge Format (OKF) bundle into a
// static documentation site laid out like mdBook: one HTML page per concept,
// CSV file and directory, or all of them in a single HTML file, and also
// print.html, PDF and EPUB. CSS, images and other local resources are
// embedded, so the output has no external dependencies. The commands follow
// mdBook's: build, serve (live preview), --version.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

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
		if d.IsDir() && isOutput(p) {
			return filepath.SkipDir // our own output, e.g. book/ from "okf-export build"
		}
		if rel == configFile {
			return nil // book.toml configures the book; it isn't a page
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

// isOutput reports whether dir holds okf-export's own output, which
// "okf-export build" writes into the bundle (book/) as mdbook does.
func isOutput(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, tocFile))
	return err == nil
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
