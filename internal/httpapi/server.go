package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/flowerrealm/multivibing/internal/projects"
	"github.com/flowerrealm/multivibing/internal/terminal"
)

type Server struct {
	version   string
	started   time.Time
	projects  *projects.Store
	terminals *terminal.Manager
	staticDir string
	upgrader  websocket.Upgrader
	eventMu   sync.Mutex
	eventConn *websocket.Conn
}

type pathRequest struct {
	Path string `json:"path"`
}

type terminalRequest struct {
	ProjectID string `json:"projectId"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

type inputRequest struct {
	Data string `json:"data"`
}

type sizeRequest struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

func NewServer(version string, projectStore *projects.Store, terminals *terminal.Manager, staticDir string) *Server {
	if terminals == nil {
		terminals = terminal.NewManager(nil)
	}
	return &Server{
		version:   version,
		started:   time.Now().UTC(),
		projects:  projectStore,
		terminals: terminals,
		staticDir: staticDir,
		upgrader:  websocket.Upgrader{CheckOrigin: sameLocalOrigin},
	}
}

func NewTerminalServer(version string, projectStore *projects.Store, staticDir string) *Server {
	s := &Server{version: version, started: time.Now().UTC(), projects: projectStore, staticDir: staticDir}
	s.terminals = terminal.NewManager(s.publishTerminalEvent)
	s.upgrader = websocket.Upgrader{CheckOrigin: sameLocalOrigin}
	return s
}

func sameLocalOrigin(r *http.Request) bool {
	host := r.Host
	origin := r.Header.Get("Origin")
	return origin == "" ||
		origin == "http://"+host ||
		origin == "https://"+host ||
		origin == "http://127.0.0.1:5173" ||
		origin == "http://localhost:5173"
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.withCORS(s.handleHealth))
	mux.HandleFunc("/api/projects", s.withCORS(s.handleProjects))
	mux.HandleFunc("/api/projects/", s.withCORS(s.handleProject))
	mux.HandleFunc("/api/terminals", s.withCORS(s.handleTerminals))
	mux.HandleFunc("/api/terminals/", s.withCORS(s.handleTerminal))
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/", s.handleStatic)
	return mux
}

func (s *Server) Shutdown() {
	s.terminals.Shutdown()
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
	writeJSON(w, struct {
		OK        bool   `json:"ok"`
		Name      string `json:"name"`
		Version   string `json:"version"`
		Mode      string `json:"mode"`
		StartedAt string `json:"startedAt"`
	}{
		OK:        true,
		Name:      "MultiVibing",
		Version:   s.version,
		Mode:      "web",
		StartedAt: s.started.Format(time.RFC3339),
	})
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		projectList, err := s.projects.List()
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, projectList)
	case http.MethodPost:
		var req pathRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, err, http.StatusBadRequest)
			return
		}
		project, err := s.projects.Open(req.Path)
		if err != nil {
			writeError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, project)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := strings.CutPrefix(r.URL.Path, "/api/projects/")
	if !ok || id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	if err := s.projects.Forget(id); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTerminals(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.terminals.List(r.URL.Query().Get("projectId")))
	case http.MethodPost:
		var req terminalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, err, http.StatusBadRequest)
			return
		}
		project, err := s.projects.Get(req.ProjectID)
		if err != nil {
			writeError(w, err, http.StatusNotFound)
			return
		}
		session, err := s.terminals.Start(project.ID, project.Path, req.Cols, req.Rows)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, session)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTerminal(w http.ResponseWriter, r *http.Request) {
	rest, ok := strings.CutPrefix(r.URL.Path, "/api/terminals/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id, action := parts[0], parts[1]
	switch action {
	case "input":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req inputRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, err, http.StatusBadRequest)
			return
		}
		if err := s.terminals.Write(id, req.Data); err != nil {
			writeError(w, err, http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case "resize":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req sizeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, err, http.StatusBadRequest)
			return
		}
		if err := s.terminals.Resize(id, req.Cols, req.Rows); err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case "close":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := s.terminals.Close(id); err != nil {
			writeError(w, err, http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.eventMu.Lock()
	if s.eventConn != nil {
		_ = s.eventConn.Close()
	}
	s.eventConn = conn
	s.eventMu.Unlock()
	defer func() {
		s.eventMu.Lock()
		if s.eventConn == conn {
			s.eventConn = nil
		}
		s.eventMu.Unlock()
		_ = conn.Close()
	}()
	for {
		if _, _, err := conn.NextReader(); err != nil {
			return
		}
	}
}

func (s *Server) publishTerminalEvent(event terminal.Event) {
	s.eventMu.Lock()
	conn := s.eventConn
	if conn == nil {
		s.eventMu.Unlock()
		return
	}
	if err := conn.WriteJSON(terminalEventPayload(event)); err != nil {
		_ = conn.Close()
		if s.eventConn == conn {
			s.eventConn = nil
		}
	}
	s.eventMu.Unlock()
}

func terminalEventPayload(event terminal.Event) map[string]any {
	payload := map[string]any{"terminalId": event.TerminalID}
	if event.Data != "" {
		payload["data"] = event.Data
	}
	if event.ExitCode != nil {
		payload["exitCode"] = *event.ExitCode
	}
	if event.Error != "" {
		payload["error"] = event.Error
	}
	payload["type"] = event.Name
	return payload
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
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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

func writeError(w http.ResponseWriter, err error, status int) {
	http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), status)
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
