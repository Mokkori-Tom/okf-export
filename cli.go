package main

// cli.go: the commands, named after mdBook's (build, serve, --version), and
// the older "okf-export -in <bundle> -out <dir>" form, which still works.

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

const usage = `okf-export - write an OKF bundle as an mdBook-style documentation site

usage:
  okf-export build [dir] [-d <dest>] [--open]      build the site (default: dir ".", dest <dir>/book)
  okf-export build [dir] --single [-d <file>]      everything in one HTML file
  okf-export build [dir] --pdf --epub              also <name>.pdf (needs Chrome/Chromium) and <name>.epub
  okf-export serve [dir] [-n host] [-p port] [--open]   live preview (default http://localhost:3000/)
  okf-export summary [dir]                          print the sidebar as an mdBook SUMMARY.md
  okf-export --version

  older form: okf-export -in <bundle> -out <dir> [-single]

flags:
`

func main() {
	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "build":
		buildMain(args)
	case "serve":
		serveMain(args)
	case "summary":
		summaryMain(args)
	case "version":
		fmt.Println(version())
	case "":
		buildMain(args) // okf-export -in x -out y, or okf-export --version
	default:
		fmt.Fprintf(os.Stderr, "okf-export: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
}

// buildVersion is set by the release build (-ldflags "-X main.buildVersion=v0.1.0").
var buildVersion string

// version is the release's tag, else the module version go install recorded
// (with the commit), or "(devel)" for a local build.
func version() string {
	if buildVersion != "" {
		return "okf-export " + buildVersion
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		v := bi.Main.Version
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				v += " " + s.Value[:7]
			}
		}
		return "okf-export " + v
	}
	return "okf-export (unknown)"
}

// common holds the flags every command takes.
type common struct {
	dir, name    string
	unsafe, vers bool
	lang         string
	cfg          config // the bundle's book.toml
}

func newFlags(name string, c *common) *flag.FlagSet {
	fl := flag.NewFlagSet(name, flag.ExitOnError)
	fl.StringVar(&c.dir, "in", "", "OKF bundle directory (same as the dir argument)")
	fl.StringVar(&c.name, "name", "", "book title in the menu bar (default: book.toml's title, else the bundle directory's name)")
	fl.BoolVar(&c.unsafe, "unsafe", false, "pass raw HTML in markdown through to the output")
	fl.BoolVar(&c.vers, "version", false, "print the version")
	fl.BoolVar(&c.vers, "V", false, "print the version")
	fl.StringVar(&c.lang, "language", "", "language of the book, for <html lang> and EPUB (default: book.toml's, else ja)")
	fl.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		fl.PrintDefaults()
	}
	return fl
}

// parse reads flags before and after the one positional argument, the
// bundle directory, as in "okf-export build docs --open".
func parse(fl *flag.FlagSet, c *common, args []string) {
	_ = fl.Parse(args)
	for fl.NArg() > 0 {
		if c.dir != "" {
			fmt.Fprintf(os.Stderr, "okf-export: unexpected argument %q\n", fl.Arg(0))
			os.Exit(2)
		}
		c.dir = fl.Arg(0)
		_ = fl.Parse(fl.Args()[1:])
	}
	if c.vers {
		fmt.Println(version())
		os.Exit(0)
	}
	if c.dir == "" {
		c.dir = "."
	}
	var err error
	c.cfg, err = readConfig(c.dir)
	check(err)
	if c.name == "" {
		c.name = c.cfg.Title
	}
	if c.lang == "" {
		c.lang = c.cfg.Language
	}
	if c.lang == "" {
		c.lang = "ja"
	}
}

func buildMain(args []string) {
	var c common
	fl := newFlags("okf-export build", &c)
	var dest string
	fl.StringVar(&dest, "dest-dir", "", "output directory, or file with --single (default: <dir>/book, <dir>/book.html)")
	fl.StringVar(&dest, "d", "", "short for --dest-dir")
	fl.StringVar(&dest, "out", "", "same as --dest-dir")
	single := fl.Bool("single", false, "write the whole book into one HTML file")
	open := fl.Bool("open", false, "open the result in the browser")
	fl.BoolVar(open, "o", false, "short for --open")
	pdf := fl.Bool("pdf", false, "also write <name>.pdf from print.html with a headless Chrome/Chromium ($OKF_CHROME)")
	epub := fl.Bool("epub", false, "also write <name>.epub")
	parse(fl, &c, args)
	lang := &c.lang
	if dest == "" && c.cfg.BuildDir != "" {
		dest = filepath.Join(c.dir, c.cfg.BuildDir)
		if *single {
			dest += ".html"
		}
	}
	if dest == "" {
		dest = filepath.Join(c.dir, "book")
		if *single {
			dest += ".html"
		}
	}

	b, err := loadBundle(c.dir)
	check(err)
	name := bundleName(c.dir, c.name)
	ex := newExporter(b, name, c.unsafe, *single)
	ex.lang = *lang
	pages, st, err := ex.build()
	check(err)
	target := dest
	if *single {
		check(ex.writeSingle(dest, pages))
	} else {
		check(ex.writeSite(dest, pages))
		target = filepath.Join(dest, "index.html")
		page, err := printPage(b, name, c.unsafe, *lang)
		check(err)
		check(os.WriteFile(filepath.Join(dest, printFile), page, 0o644))
	}
	fmt.Printf("wrote %d concept pages, %d table pages, %d text pages, %d other file pages, %d directory pages, embedded %d resources to %s\n",
		st.pages, st.tables, st.texts, st.files, st.dirs, st.resources, dest)
	if st.broken > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d unresolved local link(s)\n", st.broken)
	}

	// PDF and EPUB go next to the site (or the single file).
	side := dest
	if *single {
		side = filepath.Dir(dest)
	}
	if *pdf {
		out := filepath.Join(side, fileName(name)+".pdf")
		check(writePDF(b, name, c.unsafe, *lang, out))
		fmt.Println("wrote", out)
	}
	if *epub {
		out := filepath.Join(side, fileName(name)+".epub")
		check(writeEPUB(b, name, *lang, out))
		fmt.Println("wrote", out)
	}
	if *open {
		openBrowser(target)
	}
}

func serveMain(args []string) {
	var c common
	fl := newFlags("okf-export serve", &c)
	host := fl.String("hostname", "localhost", "host to listen on; 0.0.0.0 lets other devices on the LAN in")
	fl.StringVar(host, "n", "localhost", "short for --hostname")
	port := fl.String("port", "3000", "port to listen on")
	fl.StringVar(port, "p", "3000", "short for --port")
	addr := fl.String("addr", "", "host:port, instead of --hostname and --port")
	open := fl.Bool("open", false, "open the preview in the browser")
	fl.BoolVar(open, "o", false, "short for --open")
	parse(fl, &c, args)
	if *addr == "" {
		*addr = *host + ":" + *port
	}
	check(serve(c.dir, bundleName(c.dir, c.name), *addr, c.unsafe, c.lang, *open))
}

// summaryMain prints the sidebar's tree as an mdBook SUMMARY.md linking to
// the markdown sources, so the bundle can be built with mdBook to compare.
func summaryMain(args []string) {
	var c common
	fl := newFlags("okf-export summary", &c)
	parse(fl, &c, args)
	b, err := loadBundle(c.dir)
	check(err)
	ex := newExporter(b, bundleName(c.dir, c.name), false, false)
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

// fileName makes a book title usable as a file name.
func fileName(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < ' ' {
			return '_'
		}
		return r
	}, strings.TrimSpace(s))
	if s == "" || s == "." {
		return "book"
	}
	return s
}

// openBrowser opens a file or URL in the system's browser, as mdbook --open
// does. Failing to is only a warning.
func openBrowser(target string) {
	if !strings.Contains(target, "://") {
		if abs, err := filepath.Abs(target); err == nil {
			target = abs
		}
	}
	var cmd *exec.Cmd
	switch {
	case runtime.GOOS == "darwin":
		cmd = exec.Command("open", target)
	case runtime.GOOS == "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case os.Getenv("TERMUX_VERSION") != "" || hasCmd("termux-open-url"):
		if strings.Contains(target, "://") {
			cmd = exec.Command("termux-open-url", target)
		} else {
			cmd = exec.Command("termux-open", target)
		}
	case os.Getenv("WSL_DISTRO_NAME") != "" && hasCmd("wslview"):
		cmd = exec.Command("wslview", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not open a browser (%v); open %s yourself\n", err, target)
		return
	}
	go func() { _ = cmd.Wait() }()
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
