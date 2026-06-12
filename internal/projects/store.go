package projects

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Project struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	LastOpenedAt time.Time `json:"lastOpenedAt"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store { return &Store{path: path} }

func DefaultStorePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "MultiVibing", "projects.json"), nil
}

func (s *Store) List() ([]Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Store) Get(id string) (Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, err := s.load()
	if err != nil {
		return Project{}, err
	}
	for _, p := range ps {
		if p.ID == id {
			return p, nil
		}
	}
	return Project{}, fmt.Errorf("project %q not found", id)
}

func (s *Store) Open(path string) (Project, error) {
	norm, err := normalizeDir(path)
	if err != nil {
		return Project{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, err := s.load()
	if err != nil {
		return Project{}, err
	}
	p := Project{
		ID:           projectID(norm),
		Name:         filepath.Base(norm),
		Path:         norm,
		LastOpenedAt: time.Now().UTC(),
	}
	updated := false
	for i, cur := range ps {
		if cur.ID == p.ID || cur.Path == p.Path {
			ps[i] = p
			updated = true
			break
		}
	}
	if !updated {
		ps = append(ps, p)
	}
	return p, s.save(ps)
}

func (s *Store) Forget(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps, err := s.load()
	if err != nil {
		return err
	}
	next := ps[:0]
	for _, p := range ps {
		if p.ID != id {
			next = append(next, p)
		}
	}
	return s.save(next)
}

func (s *Store) load() ([]Project, error) {
	if s.path == "" {
		return nil, errors.New("project store path is empty")
	}
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []Project{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var data struct {
		Projects []Project `json:"projects"`
	}
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return nil, err
	}
	ps := data.Projects
	sort.SliceStable(ps, func(i, j int) bool {
		if ps[i].LastOpenedAt.Equal(ps[j].LastOpenedAt) {
			return ps[i].Name < ps[j].Name
		}
		return ps[i].LastOpenedAt.After(ps[j].LastOpenedAt)
	})
	return ps, nil
}

func (s *Store) save(ps []Project) error {
	if s.path == "" {
		return errors.New("project store path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".projects-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	writeErr := enc.Encode(map[string]any{"projects": ps})
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(name)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	return os.Rename(name, s.path)
}

func normalizeDir(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", errors.New("project path cannot be empty")
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", err
	}
	stat, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !stat.IsDir() {
		return "", fmt.Errorf("project path is not a directory: %s", abs)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return filepath.Clean(abs), nil
}

func projectID(path string) string {
	sum := sha256.Sum256([]byte(filepath.ToSlash(path)))
	return hex.EncodeToString(sum[:12])
}
