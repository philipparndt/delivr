package editor

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/philipparndt/delivr/internal/render"
)

//go:embed page.html
var pageHTML string

// Options configure the editor server.
type Options struct {
	// Port to listen on. 0 picks a free one.
	Port int
	// ReadOnly refuses every write, for when the editor is being used to
	// measure rather than to change.
	ReadOnly bool
	// Open launches a browser at the URL.
	Open bool
}

// Server serves the editor.
//
// It binds to the loopback interface only, and issues a token at start-up that
// the page is served with and every API call must carry. Not because a
// screenshot layout is a secret, but because the apply endpoint writes to the
// user's source files, and "any process on this machine can POST to it" is not
// a property to hand out by accident.
type Server struct {
	project  *Project
	renderer *render.Renderer
	cache    *previewCache
	readOnly bool
	token    string

	http *http.Server
	URL  string
}

// New loads the config and prepares a server. Call Serve to run it.
func New(configPath string, opts Options) (*Server, error) {
	project, err := LoadProject(configPath)
	if err != nil {
		return nil, err
	}

	token, err := sessionToken(project.ConfigPath)
	if err != nil {
		return nil, err
	}

	s := &Server{
		project:  project,
		renderer: render.NewRenderer(project.Config(), "", false),
		cache:    newPreviewCache(),
		readOnly: opts.ReadOnly,
		token:    token,
	}
	return s, nil
}

// sessionToken returns the token for a project, stable across restarts.
//
// A fresh token per run means every restart silently orphans any open tab: the
// page keeps working until the next request, then fails with a bare network
// error that says nothing about why. Restarting the editor is a normal thing to
// do — after changing a config by hand, after a crash, after a rebuild — so the
// URL it prints should keep working.
//
// The token still does its job. It lives in the user's cache directory, mode
// 0600, so it is no more readable than the config files the editor writes; it
// is not in the project, so it cannot be committed by accident; and it is
// per-config, so two projects do not share one.
func sessionToken(configPath string) (string, error) {
	fresh := func() (string, error) {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("failed to generate a session token: %w", err)
		}
		return hex.EncodeToString(buf), nil
	}

	dir, err := os.UserCacheDir()
	if err != nil {
		return fresh()
	}
	dir = filepath.Join(dir, "delivr")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fresh()
	}

	sum := sha256.Sum256([]byte(configPath))
	path := filepath.Join(dir, "edit-"+hex.EncodeToString(sum[:8])+".token")

	if buf, err := os.ReadFile(path); err == nil {
		if tok := strings.TrimSpace(string(buf)); len(tok) >= 32 {
			return tok, nil
		}
	}

	tok, err := fresh()
	if err != nil {
		return "", err
	}
	// A failure to persist is not a failure to run; the token just will not
	// survive the next restart.
	_ = os.WriteFile(path, []byte(tok), 0600)
	return tok, nil
}

// Close releases the renderer's resources.
func (s *Server) Close() { s.renderer.Close() }

// Serve listens on localhost and blocks until the context is cancelled.
func (s *Server) Serve(ctx context.Context, opts Options) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", opts.Port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handlePage)
	mux.HandleFunc("/api/state", s.guard(s.handleState))
	mux.HandleFunc("/api/preview", s.guard(s.handlePreview))
	mux.HandleFunc("/api/layer", s.guard(s.handleLayer))
	mux.HandleFunc("/api/solve", s.guard(s.handleSolve))
	mux.HandleFunc("/api/targets", s.guard(s.handleTargets))
	mux.HandleFunc("/api/apply", s.guard(s.handleApply))
	mux.HandleFunc("/api/reorder", s.guard(s.handleReorder))
	mux.HandleFunc("/api/reload", s.guard(s.handleReload))

	s.URL = fmt.Sprintf("http://%s/?t=%s", listener.Addr().String(), s.token)
	s.http = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	fmt.Printf("delivr edit — %s\n", s.URL)
	if s.readOnly {
		fmt.Println("read-only: values can be copied, not written")
	}
	fmt.Println("Press Ctrl-C to stop.")

	if opts.Open {
		openBrowser(s.URL)
	}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.http.Shutdown(shutdown)
	}()

	if err := s.http.Serve(listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// guard rejects API calls that do not carry this run's token.
func (s *Server) guard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// A bad config or a missing asset should come back as a message in the
		// page, not as a dropped connection with a stack trace in the terminal.
		defer func() {
			if rec := recover(); rec != nil {
				fmt.Fprintf(os.Stderr, "editor: recovered: %v\n", rec)
				writeError(w, fmt.Errorf("internal error: %v", rec))
			}
		}()

		token := r.Header.Get("X-Delivr-Token")
		if token == "" {
			token = r.URL.Query().Get("t")
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(pageHTML))
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.State())
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	var req PreviewRequest
	if !readJSON(w, r, &req) {
		return
	}
	resp, err := s.Preview(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, resp)
}

// handleLayer serves a rendered layer by content address. The key is a hash of
// everything that produced the pixels, so the bytes behind a URL can never
// change and the browser is told to keep them forever — which is what makes a
// drag cost no network at all.
func (s *Server) handleLayer(w http.ResponseWriter, r *http.Request) {
	buf, ok := s.cache.Layer(r.URL.Query().Get("k"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(buf)
}

func (s *Server) handleSolve(w http.ResponseWriter, r *http.Request) {
	var req SolveRequest
	if !readJSON(w, r, &req) {
		return
	}
	resp, err := s.Solve(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, resp)
}

func (s *Server) handleTargets(w http.ResponseWriter, r *http.Request) {
	var req TargetsRequest
	if !readJSON(w, r, &req) {
		return
	}
	targets, err := s.Targets(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]any{"targets": targets, "readOnly": s.readOnly})
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	var req ApplyRequest
	if !readJSON(w, r, &req) {
		return
	}
	result, err := s.Apply(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleReorder(w http.ResponseWriter, r *http.Request) {
	var req ReorderRequest
	if !readJSON(w, r, &req) {
		return
	}
	result, err := s.Reorder(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if err := s.project.Reload(); err != nil {
		writeError(w, err)
		return
	}
	s.cache.Reset()
	s.renderer.Close()
	s.renderer = render.NewRenderer(s.project.Config(), "", false)
	writeJSON(w, map[string]any{"ok": true})
}

func readJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return false
	}
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		writeError(w, fmt.Errorf("bad request: %w", err))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "editor: failed to write response: %v\n", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "could not open a browser: %v\n", err)
	}
}

// TokenFromURL is a small helper for tests and callers that need the token.
func (s *Server) Token() string { return s.token }

// Addr is the host:port the server bound to, once serving.
func (s *Server) Addr() string {
	if s.URL == "" {
		return ""
	}
	return strings.TrimPrefix(strings.Split(s.URL, "/?")[0], "http://")
}
