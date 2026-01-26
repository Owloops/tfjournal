package serve

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Owloops/tfjournal/run"
	"github.com/Owloops/tfjournal/storage"
)

//go:embed dist/*
var distFS embed.FS

const (
	_defaultPort  = 8080
	_defaultBind  = "127.0.0.1"
	_defaultLimit = 20
)

var (
	_port     int
	_bindAddr string
)

var Cmd = &cobra.Command{
	Use:   "serve",
	Short: "Start web UI server",
	Long:  "Launch HTTP server to browse terraform runs in a web interface",
	RunE:  runServe,
}

func init() {
	Cmd.Flags().IntVarP(&_port, "port", "p", _defaultPort, "Port to listen on")
	Cmd.Flags().StringVarP(&_bindAddr, "bind", "b", _defaultBind, "Address to bind to")
}

func runServe(cmd *cobra.Command, args []string) error {
	store, err := storage.NewFromEnv()
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer func() { _ = store.Close() }()

	h := newHandler(store)
	addr := fmt.Sprintf("%s:%d", _bindAddr, _port)

	username := os.Getenv("TFJOURNAL_USERNAME")
	password := os.Getenv("TFJOURNAL_PASSWORD")

	var finalHandler http.Handler = h
	if username != "" && password != "" {
		finalHandler = basicAuthMiddleware(h, username, password)
		fmt.Fprintf(os.Stderr, "tfjournal web ui: http://%s (basic auth enabled)\n", addr)
	} else {
		fmt.Fprintf(os.Stderr, "tfjournal web ui: http://%s\n", addr)
	}

	return http.ListenAndServe(addr, finalHandler)
}

func basicAuthMiddleware(next http.Handler, username, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="tfjournal"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type handler struct {
	store storage.Store
	mux   *http.ServeMux
}

func newHandler(store storage.Store) *handler {
	h := &handler{
		store: store,
		mux:   http.NewServeMux(),
	}

	h.mux.HandleFunc("GET /api/runs", h.handleListRuns)
	h.mux.HandleFunc("GET /api/runs/{id}", h.handleGetRun)
	h.mux.HandleFunc("GET /api/runs/{id}/output", h.handleGetOutput)

	distSubFS, _ := fs.Sub(distFS, "dist")
	h.mux.Handle("GET /", http.FileServer(http.FS(distSubFS)))

	return h
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *handler) handleListRuns(w http.ResponseWriter, r *http.Request) {
	opts := storage.ListOptions{Limit: _defaultLimit}

	if status := r.URL.Query().Get("status"); status != "" {
		opts.Status = run.Status(status)
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		_, _ = fmt.Sscanf(limitStr, "%d", &opts.Limit)
	}

	if workspace := r.URL.Query().Get("workspace"); workspace != "" {
		opts.Workspace = workspace
	}

	if user := r.URL.Query().Get("user"); user != "" {
		opts.User = user
	}

	if since := r.URL.Query().Get("since"); since != "" {
		if d, err := parseDuration(since); err == nil {
			opts.Since = time.Now().Add(-d)
		}
	}

	if program := r.URL.Query().Get("program"); program != "" {
		opts.Program = program
	}

	if branch := r.URL.Query().Get("branch"); branch != "" {
		opts.Branch = branch
	}

	if r.URL.Query().Get("has-changes") == "true" {
		opts.HasChanges = true
	}

	runs, err := h.store.ListRuns(opts)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, runs)
}

func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var d int
		if _, err := fmt.Sscanf(days, "%d", &d); err != nil {
			return 0, err
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func (h *handler) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.jsonError(w, "missing run id", http.StatusBadRequest)
		return
	}

	run, err := h.store.GetRun(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.jsonError(w, "run not found", http.StatusNotFound)
			return
		}
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, run)
}

func (h *handler) handleGetOutput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.jsonError(w, "missing run id", http.StatusBadRequest)
		return
	}

	output, err := h.store.GetOutput(id)
	if err != nil {
		if strings.Contains(err.Error(), "no such file") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(output)
}

func (h *handler) jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func (h *handler) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
