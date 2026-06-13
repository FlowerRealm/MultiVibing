package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flowerrealm/multivibing/internal/projects"
	"github.com/flowerrealm/multivibing/internal/terminal"
)

func TestHealthEndpoint(t *testing.T) {
	server := httptest.NewServer(NewServer("test-version", testStore(t), terminal.NewManager(nil), "missing-dist").Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var health struct {
		OK      bool   `json:"ok"`
		Mode    string `json:"mode"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if !health.OK || health.Mode != "web" || health.Version != "test-version" {
		t.Fatalf("unexpected health: %#v", health)
	}
}

func TestProjectsEndpointStartsEmpty(t *testing.T) {
	server := httptest.NewServer(NewServer("test-version", testStore(t), terminal.NewManager(nil), "missing-dist").Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/projects")
	if err != nil {
		t.Fatalf("GET /api/projects failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var projectList []projects.Project
	if err := json.NewDecoder(resp.Body).Decode(&projectList); err != nil {
		t.Fatalf("decode projects: %v", err)
	}
	if len(projectList) != 0 {
		t.Fatalf("projects = %#v, want empty", projectList)
	}
}

func TestTerminalSnapshotEndpoint(t *testing.T) {
	terminals := &fakeTerminalService{
		snapshots: map[string]terminal.Snapshot{
			"term-1": {
				Session: terminal.Session{
					ID:        "term-1",
					ProjectID: "proj-1",
					Cwd:       "/tmp/project",
					PID:       123,
					Status:    terminal.StatusRunning,
				},
				History:   "hello\n",
				LastSeq:   3,
				Truncated: true,
			},
		},
	}
	server := httptest.NewServer(NewServer("test-version", testStore(t), terminals, "missing-dist").Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/terminals/term-1/snapshot")
	if err != nil {
		t.Fatalf("GET snapshot failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var snapshot terminal.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshot.Session.ID != "term-1" || snapshot.History != "hello\n" || snapshot.LastSeq != 3 || !snapshot.Truncated {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}

func TestTerminalSnapshotEndpointNotFound(t *testing.T) {
	server := httptest.NewServer(NewServer("test-version", testStore(t), &fakeTerminalService{}, "missing-dist").Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/terminals/missing/snapshot")
	if err != nil {
		t.Fatalf("GET missing snapshot failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func testStore(t *testing.T) *projects.Store {
	t.Helper()
	return projects.NewStore(t.TempDir() + "/projects.json")
}

type fakeTerminalService struct {
	snapshots map[string]terminal.Snapshot
}

func (f *fakeTerminalService) List(projectID string) []terminal.Session {
	return nil
}

func (f *fakeTerminalService) Start(projectID, cwd string, cols, rows int) (terminal.Session, error) {
	return terminal.Session{}, fmt.Errorf("not implemented")
}

func (f *fakeTerminalService) Write(id, data string) error {
	return nil
}

func (f *fakeTerminalService) Resize(id string, cols, rows int) error {
	return nil
}

func (f *fakeTerminalService) Close(id string) error {
	return nil
}

func (f *fakeTerminalService) Snapshot(id string) (terminal.Snapshot, error) {
	if f.snapshots != nil {
		if snapshot, ok := f.snapshots[id]; ok {
			return snapshot, nil
		}
	}
	return terminal.Snapshot{}, fmt.Errorf("terminal %q not found", id)
}

func (f *fakeTerminalService) Shutdown() {}
