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
	now  func() time.Time
	mu   sync.Mutex
}

type fileData struct {
	Projects []Project `json:"projects"`
}

func DefaultStorePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "MultiVibing", "projects.json"), nil
}

func NewStore(path string) *Store {
	return &Store{
		path: path,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Store) SetNowForTest(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = now
}

func (s *Store) List() ([]Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	return sortedProjects(data.Projects), nil
}

func (s *Store) Get(id string) (Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.loadLocked()
	if err != nil {
		return Project{}, err
	}
	for _, project := range data.Projects {
		if project.ID == id {
			return project, nil
		}
	}
	return Project{}, fmt.Errorf("project %q not found", id)
}

func (s *Store) Open(path string) (Project, error) {
	normalized, err := normalizeExistingDir(path)
	if err != nil {
		return Project{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.loadLocked()
	if err != nil {
		return Project{}, err
	}

	now := s.now()
	project := Project{
		ID:           projectID(normalized),
		Name:         projectName(normalized),
		Path:         normalized,
		LastOpenedAt: now,
	}

	replaced := false
	for i, existing := range data.Projects {
		if existing.ID == project.ID || existing.Path == project.Path {
			data.Projects[i] = project
			replaced = true
			break
		}
	}
	if !replaced {
		data.Projects = append(data.Projects, project)
	}

	if err := s.saveLocked(data); err != nil {
		return Project{}, err
	}
	return project, nil
}

func (s *Store) Forget(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.loadLocked()
	if err != nil {
		return err
	}

	next := data.Projects[:0]
	for _, project := range data.Projects {
		if project.ID != id {
			next = append(next, project)
		}
	}
	data.Projects = next
	return s.saveLocked(data)
}

func (s *Store) loadLocked() (fileData, error) {
	if s.path == "" {
		return fileData{}, errors.New("project store path is empty")
	}

	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return fileData{}, nil
	}
	if err != nil {
		return fileData{}, err
	}
	defer file.Close()

	var data fileData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return fileData{}, err
	}
	return fileData{Projects: sortedProjects(data.Projects)}, nil
}

func (s *Store) saveLocked(data fileData) error {
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
	tmpName := tmp.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			_ = os.Remove(tmpName)
		}
	}()

	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(fileData{Projects: sortedProjects(data.Projects)}); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return err
	}
	removeTmp = false
	return nil
}

func normalizeExistingDir(input string) (string, error) {
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
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		abs = resolved
	}
	return filepath.Clean(abs), nil
}

func projectID(path string) string {
	sum := sha256.Sum256([]byte(filepath.ToSlash(path)))
	return hex.EncodeToString(sum[:12])
}

func projectName(path string) string {
	name := filepath.Base(path)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return path
	}
	return name
}

func sortedProjects(projects []Project) []Project {
	if projects == nil {
		return []Project{}
	}
	next := append([]Project(nil), projects...)
	sort.SliceStable(next, func(i, j int) bool {
		if next[i].LastOpenedAt.Equal(next[j].LastOpenedAt) {
			return next[i].Name < next[j].Name
		}
		return next[i].LastOpenedAt.After(next[j].LastOpenedAt)
	})
	return next
}
