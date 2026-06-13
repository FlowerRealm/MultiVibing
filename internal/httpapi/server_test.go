package httpapi

import (
	"encoding/json"
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

func testStore(t *testing.T) *projects.Store {
	t.Helper()
	return projects.NewStore(t.TempDir() + "/projects.json")
}
