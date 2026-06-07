package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/flowerrealm/multivibing/internal/app"
)

type Server struct {
	app       *app.Service
	staticDir string
	upgrader  websocket.Upgrader
}

func NewServer(service *app.Service, staticDir string) *Server {
	return &Server{
		app:       service,
		staticDir: staticDir,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				host := r.Host
				origin := r.Header.Get("Origin")
				return origin == "" ||
					origin == "http://"+host ||
					origin == "https://"+host ||
					origin == "http://127.0.0.1:5173" ||
					origin == "http://localhost:5173"
			},
		},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.withCORS(s.handleHealth))
	mux.HandleFunc("/api/codex/status", s.withCORS(s.handleCodexStatus))
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/", s.handleStatic)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.app.Health("browser"))
}

func (s *Server) handleCodexStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	writeJSON(w, s.app.CodexStatus(ctx))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	events, cancel := s.app.Subscribe(32)
	defer cancel()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := conn.WriteJSON(event); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := filepath.Clean(r.URL.Path)
	if path == "." || path == string(filepath.Separator) {
		path = "index.html"
	} else {
		path = path[1:]
	}

	target := filepath.Join(s.staticDir, path)
	if !isInside(s.staticDir, target) {
		http.NotFound(w, r)
		return
	}
	if _, err := os.Stat(target); err == nil {
		http.ServeFile(w, r, target)
		return
	}

	index := filepath.Join(s.staticDir, "index.html")
	if _, err := os.Stat(index); err == nil {
		http.ServeFile(w, r, index)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("MultiVibing frontend has not been built yet. Run npm run build."))
}

func (s *Server) withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://127.0.0.1:5173" || origin == "http://localhost:5173" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func isInside(base, target string) bool {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absBase, absTarget)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
