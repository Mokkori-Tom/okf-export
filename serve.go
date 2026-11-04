package main

import (
	"fmt"
	"hash/fnv"
	"io/fs"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// server renders the bundle into memory and re-renders it whenever a file
// changes, telling open browsers to reload over Server-Sent Events.
type server struct {
	root, name string
	unsafe     bool

	mu    sync.RWMutex
	pages map[string][]byte
	subs  map[chan struct{}]bool
}

func serve(root, name, addr string, unsafe bool) error {
	s := &server{root: root, name: name, unsafe: unsafe, subs: map[chan struct{}]bool{}}
	if err := s.rebuild(); err != nil {
		return err
	}
	go s.watch()
	mux := http.NewServeMux()
	mux.HandleFunc("/__okf/events", s.events)
	mux.HandleFunc("/", s.page)
	log.Printf("serving %s on http://%s/ (Ctrl-C to stop)", root, displayAddr(addr))
	return http.ListenAndServe(addr, mux)
}

func (s *server) rebuild() error {
	b, err := loadBundle(s.root)
	if err != nil {
		return err
	}
	ex := newExporter(b, s.name, s.unsafe, false)
	pages, st, err := ex.build()
	if err != nil {
		return err
	}
	out := make(map[string][]byte, len(pages))
	for _, p := range pages {
		html, err := ex.renderPage(p, true)
		if err != nil {
			return err
		}
		out[p.Page] = html
	}
	js, err := searchJS(pages)
	if err != nil {
		return err
	}
	out[searchFile] = js
	s.mu.Lock()
	s.pages = out
	s.mu.Unlock()
	log.Printf("rendered %d pages (%d unresolved links)", len(pages), st.broken)
	return nil
}

// watch polls the bundle for changes. Polling keeps the tool dependency-free
// and is cheap for bundles of a few thousand files.
func (s *server) watch() {
	last := fingerprint(s.root)
	for range time.Tick(500 * time.Millisecond) {
		fp := fingerprint(s.root)
		if fp == last {
			continue
		}
		last = fp
		if err := s.rebuild(); err != nil {
			log.Printf("rebuild failed: %v", err)
			continue
		}
		s.mu.Lock()
		for c := range s.subs {
			select {
			case c <- struct{}{}:
			default:
			}
		}
		s.mu.Unlock()
	}
}

func fingerprint(root string) uint64 {
	h := fnv.New64a()
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if p != root && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fmt.Fprintf(h, "%s\x00%d\x00%d\x00", p, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	return h.Sum64()
}

func (s *server) events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	c := make(chan struct{}, 1)
	s.mu.Lock()
	s.subs[c] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, c)
		s.mu.Unlock()
	}()
	fmt.Fprint(w, ": connected\n\n")
	fl.Flush()
	keepAlive := time.NewTicker(30 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-c:
			fmt.Fprint(w, "data: reload\n\n")
			fl.Flush()
		case <-keepAlive.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		}
	}
}

func (s *server) page(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if p == "" || strings.HasSuffix(r.URL.Path, "/") {
		p = path.Join(p, "index.html")
	}
	s.mu.RLock()
	body, ok := s.pages[p]
	_, isDir := s.pages[path.Join(p, "index.html")]
	s.mu.RUnlock()
	if !ok {
		if isDir {
			http.Redirect(w, r, "/"+p+"/", http.StatusFound)
			return
		}
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(p, ".js") {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body)
}

func displayAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
}
