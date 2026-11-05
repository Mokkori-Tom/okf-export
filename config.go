package main

// config.go: book.toml in the bundle's root, as in mdBook, for what would
// otherwise be typed on every run:
//
//	[book]
//	title = "社内マニュアル"     # instead of the directory name (--name)
//	language = "ja"             # --language
//
//	[build]
//	build-dir = "../site"        # where "okf-export build" writes (-d), from the bundle
//
// Flags on the command line win over the file.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const configFile = "book.toml"

type config struct {
	Title, Language, BuildDir string
}

// readConfig reads dir/book.toml if there is one. Only the keys above are
// read, from a TOML subset: [section] lines and key = "string" lines.
func readConfig(dir string) (config, error) {
	var c config
	f, err := os.Open(filepath.Join(dir, configFile))
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	defer f.Close()
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = strings.TrimSpace(strings.Trim(line, "[]"))
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		val, err := tomlString(v)
		if err != nil {
			continue // a value this reader doesn't need (number, array, ...)
		}
		switch section + "." + k {
		case "book.title":
			c.Title = val
		case "book.language":
			c.Language = val
		case "build.build-dir":
			c.BuildDir = val
		}
	}
	if err := sc.Err(); err != nil {
		return c, fmt.Errorf("%s: %w", configFile, err)
	}
	return c, nil
}

// tomlString reads a basic ("...") or literal ('...') TOML string, with a
// trailing # comment allowed.
func tomlString(v string) (string, error) {
	if strings.HasPrefix(v, "'") {
		if end := strings.Index(v[1:], "'"); end >= 0 {
			return v[1 : 1+end], nil
		}
		return "", fmt.Errorf("unterminated string")
	}
	if !strings.HasPrefix(v, `"`) {
		return "", fmt.Errorf("not a string")
	}
	for i := 1; i < len(v); i++ {
		switch v[i] {
		case '\\':
			i++
		case '"':
			return strconv.Unquote(v[:i+1])
		}
	}
	return "", fmt.Errorf("unterminated string")
}
