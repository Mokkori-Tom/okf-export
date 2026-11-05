package main

import (
	"regexp"
	"strings"
)

// maxSearchText caps the text kept per page in the search index.
const maxSearchText = 100000

var headingTag = regexp.MustCompile(`(?s)<h[1-6]\b[^>]*?\bid="([^"]*)"[^>]*>(.*?)</h[1-6]>`)

// pageSearch splits a page into search entries, one per heading, so a hit
// can link to the heading it is under. where is the page's place in the
// bundle, shown with each result.
func pageSearch(p *pageData, where string) []searchEntry {
	base := searchEntry{Page: p.Page, Title: p.Title, Path: where}
	budget := maxSearchText
	text := func(s string) string {
		if len(s) > budget {
			s = strings.ToValidUTF8(s[:budget], "")
		}
		budget -= len(s)
		return s
	}
	if p.Body == "" || p.searchText != "" {
		base.Text = text(strings.TrimSpace(p.Meta.Description + " " + spaceRe.ReplaceAllString(p.searchText, " ")))
		return []searchEntry{base}
	}
	body := string(p.Body)
	locs := headingTag.FindAllStringSubmatchIndex(body, -1)
	first := len(body)
	if len(locs) > 0 {
		first = locs[0][0]
	}
	top := base
	top.Text = text(strings.TrimSpace(p.Meta.Description + " " + plainText(body[:first])))
	out := []searchEntry{top}
	for i, l := range locs {
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		e := base
		e.Anchor = body[l[2]:l[3]]
		e.Heading = plainText(body[l[4]:l[5]])
		e.Text = text(plainText(body[l[1]:end]))
		out = append(out, e)
	}
	return out
}

var leadingH1 = regexp.MustCompile(`(?s)^\s*<h1\b[^>]*>(.*?)</h1>\n?`)

// leadingTitle splits off body's opening <h1> if it reads title.
func leadingTitle(body, title string) (h1, rest string, ok bool) {
	m := leadingH1.FindStringSubmatchIndex(body)
	if m == nil || plainText(body[m[2]:m[3]]) != strings.TrimSpace(title) {
		return "", body, false
	}
	return strings.TrimSpace(body[m[0]:m[1]]), body[m[1]:], true
}
