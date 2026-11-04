package main

import (
	"path"
	"regexp"
	"strings"
)

// OKF index files (§8): a body of headings and entries
//
//   - [Title](relative-url) - short description
//   - [Subdirectory](subdir/) - short description of the subdirectory
//
// "okf index" writes the same as a table, | [Title](file) | Type | Description |,
// with "- [sub/](sub/index.md)" for subdirectories and a "Bundle root." or
// "Directory: `x/`" line under the heading.
var (
	indexItem    = regexp.MustCompile(`^\s*[*+-]\s+\[[^\]]*\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)\s*(?:[-–—:]\s*(.*?))?\s*$`)
	indexRow     = regexp.MustCompile(`^\s*\|\s*\[[^\]]*\]\(\s*<?([^)\s>]+)>?\s*\)\s*\|(?:.*\|)?\s*([^|]*?)\s*\|\s*$`)
	indexTableLn = regexp.MustCompile(`^\s*\|.*\|\s*$`)
	indexHeading = regexp.MustCompile(`^\s{0,3}#{1,6}(?:\s|$)`)
	indexOKFLine = regexp.MustCompile("^(?:Bundle root\\.|Directory: `[^`]*`)\\s*$")
)

// indexInfo is what an index.md says about its directory.
type indexInfo struct {
	desc    map[string]string // bundle path (file or directory) -> description
	listing bool              // the body is only the listing, nothing else to read
}

// readIndex parses index.md d. Entries pointing at a directory, as "sub/" or
// "sub/index.md", are keyed by the directory.
func (ex *exporter) readIndex(d *doc) indexInfo {
	info := indexInfo{desc: map[string]string{}}
	_, body := splitFrontmatter(d.raw)
	entries, other := 0, false
	for _, line := range strings.Split(string(body), "\n") {
		var m []string
		switch {
		case strings.TrimSpace(line) == "", indexHeading.MatchString(line), indexOKFLine.MatchString(strings.TrimSpace(line)):
			continue
		case indexItem.MatchString(line):
			m = indexItem.FindStringSubmatch(line)
		case indexRow.MatchString(line):
			m = indexRow.FindStringSubmatch(line)
		case indexTableLn.MatchString(line):
			continue // table header or separator
		default:
			other = true
			continue
		}
		entries++
		p, _, ok := ex.resolveLocal(d.path, m[1])
		if !ok {
			continue
		}
		if path.Base(p) == "index.md" {
			p = path.Dir(p)
			if p == "." {
				p = ""
			}
		}
		if desc := strings.TrimSpace(m[2]); desc != "" && info.desc[p] == "" {
			info.desc[p] = desc
		}
	}
	info.listing = entries > 0 && !other
	return info
}
