package main

// firefox.go: printing print.html to PDF with a headless Firefox when there
// is no Chrome or Chromium. Firefox is driven over WebDriver BiDi, whose
// browsingContext.print returns the PDF. The WebSocket client is the least
// needed for that local connection.

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// firefox finds Firefox: $OKF_FIREFOX, then the usual commands and places.
func firefox() (string, error) {
	if f := os.Getenv("OKF_FIREFOX"); f != "" {
		return f, nil
	}
	for _, c := range []string{"firefox", "firefox-esr"} {
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	var places []string
	switch runtime.GOOS {
	case "darwin":
		places = []string{"/Applications/Firefox.app/Contents/MacOS/firefox"}
	case "windows":
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
			if d := os.Getenv(env); d != "" {
				places = append(places, filepath.Join(d, `Mozilla Firefox\firefox.exe`))
			}
		}
	}
	for _, p := range places {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no Firefox found")
}

var bidiListening = regexp.MustCompile(`WebDriver BiDi listening on (ws://\S+)`)

// firefoxPDF prints the page at fileURL to a PDF and returns it.
func firefoxPDF(browser, fileURL string) ([]byte, error) {
	profile, err := os.MkdirTemp("", "okf-export-firefox")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(profile)
	cmd := exec.Command(browser, "--headless", "--no-remote", "--profile", profile, "--remote-debugging-port=0", "about:blank")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Firefox says where it listens on stderr.
	found := make(chan string, 1)
	var log strings.Builder
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if m := bidiListening.FindStringSubmatch(sc.Text()); m != nil {
				found <- m[1]
			} else if log.Len() < 4000 {
				log.WriteString(sc.Text() + "\n")
			}
		}
	}()
	var endpoint string
	select {
	case endpoint = <-found:
	case <-time.After(60 * time.Second):
		return nil, fmt.Errorf("Firefox did not start its remote interface\n%s", strings.TrimSpace(log.String()))
	}

	ws, err := dialWS(strings.TrimSuffix(endpoint, "/") + "/session")
	if err != nil {
		return nil, err
	}
	defer ws.Close()
	b := &bidi{ws: ws}
	if _, err := b.call("session.new", map[string]any{"capabilities": map[string]any{}}); err != nil {
		return nil, err
	}
	tree, err := b.call("browsingContext.getTree", map[string]any{})
	if err != nil {
		return nil, err
	}
	var t struct {
		Contexts []struct {
			Context string `json:"context"`
		} `json:"contexts"`
	}
	if err := json.Unmarshal(tree, &t); err != nil || len(t.Contexts) == 0 {
		return nil, fmt.Errorf("Firefox has no window to print from")
	}
	ctx := t.Contexts[0].Context
	if _, err := b.call("browsingContext.navigate", map[string]any{"context": ctx, "url": fileURL, "wait": "complete"}); err != nil {
		return nil, err
	}
	res, err := b.call("browsingContext.print", map[string]any{
		"context":    ctx,
		"background": true,
		"page":       map[string]any{"width": 21.0, "height": 29.7}, // A4, in cm
		"margin":     map[string]any{"top": 1.5, "bottom": 1.5, "left": 1.5, "right": 1.5},
	})
	if err != nil {
		return nil, err
	}
	var p struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(res, &p); err != nil {
		return nil, err
	}
	_, _ = b.call("session.end", map[string]any{})
	return base64.StdEncoding.DecodeString(p.Data)
}

// bidi sends WebDriver BiDi commands and waits for their results, skipping
// the events in between.
type bidi struct {
	ws *wsConn
	id int
}

func (b *bidi) call(method string, params any) (json.RawMessage, error) {
	b.id++
	msg, _ := json.Marshal(map[string]any{"id": b.id, "method": method, "params": params})
	if err := b.ws.writeText(msg); err != nil {
		return nil, err
	}
	for {
		data, err := b.ws.readMessage(5 * time.Minute)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", method, err)
		}
		var r struct {
			ID      int             `json:"id"`
			Type    string          `json:"type"`
			Result  json.RawMessage `json:"result"`
			Error   string          `json:"error"`
			Message string          `json:"message"`
		}
		if err := json.Unmarshal(data, &r); err != nil || r.ID != b.id {
			continue // an event
		}
		if r.Type == "error" {
			return nil, fmt.Errorf("%s: %s: %s", method, r.Error, r.Message)
		}
		return r.Result, nil
	}
}

// wsConn is a minimal WebSocket client (RFC 6455): text messages only,
// client frames masked, server frames possibly fragmented.
type wsConn struct {
	c net.Conn
	r *bufio.Reader
}

func dialWS(url string) (*wsConn, error) {
	rest, ok := strings.CutPrefix(url, "ws://")
	if !ok {
		return nil, fmt.Errorf("unsupported WebSocket URL %q", url)
	}
	host, path, _ := strings.Cut(rest, "/")
	c, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		return nil, err
	}
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	fmt.Fprintf(c, "GET /%s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		path, host, base64.StdEncoding.EncodeToString(key))
	r := bufio.NewReader(c)
	resp, err := http.ReadResponse(r, nil)
	if err != nil {
		c.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		c.Close()
		return nil, fmt.Errorf("WebSocket handshake: %s", resp.Status)
	}
	return &wsConn{c: c, r: r}, nil
}

func (w *wsConn) Close() error { return w.c.Close() }

func (w *wsConn) writeFrame(op byte, payload []byte) error {
	hdr := []byte{0x80 | op}
	switch n := len(payload); {
	case n < 126:
		hdr = append(hdr, 0x80|byte(n))
	case n < 1<<16:
		hdr = append(hdr, 0x80|126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, 0x80|127)
		hdr = binary.BigEndian.AppendUint64(hdr, uint64(n))
	}
	mask := make([]byte, 4)
	_, _ = rand.Read(mask)
	hdr = append(hdr, mask...)
	masked := make([]byte, len(payload))
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	_, err := w.c.Write(append(hdr, masked...))
	return err
}

func (w *wsConn) writeText(p []byte) error { return w.writeFrame(1, p) }

func (w *wsConn) readMessage(timeout time.Duration) ([]byte, error) {
	_ = w.c.SetReadDeadline(time.Now().Add(timeout))
	var msg []byte
	for {
		var h [2]byte
		if _, err := io.ReadFull(w.r, h[:]); err != nil {
			return nil, err
		}
		fin, op := h[0]&0x80 != 0, h[0]&0x0f
		n := uint64(h[1] & 0x7f)
		switch n {
		case 126:
			var b [2]byte
			if _, err := io.ReadFull(w.r, b[:]); err != nil {
				return nil, err
			}
			n = uint64(binary.BigEndian.Uint16(b[:]))
		case 127:
			var b [8]byte
			if _, err := io.ReadFull(w.r, b[:]); err != nil {
				return nil, err
			}
			n = binary.BigEndian.Uint64(b[:])
		}
		var mask [4]byte
		if h[1]&0x80 != 0 {
			if _, err := io.ReadFull(w.r, mask[:]); err != nil {
				return nil, err
			}
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(w.r, payload); err != nil {
			return nil, err
		}
		if h[1]&0x80 != 0 {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
		switch op {
		case 8:
			return nil, fmt.Errorf("connection closed by Firefox")
		case 9:
			_ = w.writeFrame(10, payload) // ping → pong
			continue
		case 10:
			continue
		}
		msg = append(msg, payload...)
		if fin {
			return msg, nil
		}
	}
}
