package server

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/Owloops/tfjournal/run"
	"github.com/Owloops/tfjournal/storage"
)

const DefaultLimit = 20

var Version = "dev"

//go:embed dist/*
var distFS embed.FS

type Server struct {
	store storage.Store
	mux   *http.ServeMux
	hasS3 bool
}

func New(store storage.Store) *Server {
	_, hasS3 := store.(*storage.HybridStore)
	s := &Server{
		store: store,
		mux:   http.NewServeMux(),
		hasS3: hasS3,
	}

	s.mux.HandleFunc("GET /api/runs", s.handleListRuns)
	s.mux.HandleFunc("GET /api/runs/local", s.handleListRunsLocal)
	s.mux.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	s.mux.HandleFunc("GET /api/runs/{id}/output", s.handleGetOutput)
	s.mux.HandleFunc("GET /api/version", s.handleGetVersion)
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("POST /api/sync", s.handleSync)

	distSubFS, _ := fs.Sub(distFS, "dist")
	s.mux.Handle("GET /", http.FileServer(http.FS(distSubFS)))

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) WithBasicAuth(username, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="tfjournal"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		s.ServeHTTP(w, r)
	})
}

func (s *Server) parseListOptions(r *http.Request) storage.ListOptions {
	opts := storage.ListOptions{Limit: DefaultLimit}

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
		if d, err := run.ParseDuration(since); err == nil {
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

	return opts
}

func (s *Server) handleListRunsLocal(w http.ResponseWriter, r *http.Request) {
	opts := s.parseListOptions(r)
	runs, err := s.store.ListRunsLocal(opts)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, runs)
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	opts := s.parseListOptions(r)
	runs, err := s.store.ListRuns(opts)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, runs)
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.jsonError(w, "missing run id", http.StatusBadRequest)
		return
	}

	run, err := s.store.GetRun(id)
	if err != nil {
		if errors.Is(err, storage.ErrRunNotFound) {
			s.jsonError(w, "run not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, storage.ErrInvalidRunID) {
			s.jsonError(w, "invalid run id", http.StatusBadRequest)
			return
		}
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, run)
}

func (s *Server) handleGetOutput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.jsonError(w, "missing run id", http.StatusBadRequest)
		return
	}

	output, err := s.store.GetOutput(id)
	if err != nil {
		if errors.Is(err, storage.ErrOutputNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if errors.Is(err, storage.ErrInvalidRunID) {
			s.jsonError(w, "invalid run id", http.StatusBadRequest)
			return
		}
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(output)
}

func (s *Server) handleGetVersion(w http.ResponseWriter, _ *http.Request) {
	s.jsonResponse(w, map[string]string{"version": Version})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	s.jsonResponse(w, map[string]bool{"s3_enabled": s.hasS3})
}

func (s *Server) handleSync(w http.ResponseWriter, _ *http.Request) {
	result, err := s.store.Sync()
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, result)
}

func (s *Server) jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
