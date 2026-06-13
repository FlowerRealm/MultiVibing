package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flowerrealm/multivibing/internal/app"
	"github.com/flowerrealm/multivibing/internal/codex"
	"github.com/flowerrealm/multivibing/internal/events"
)

type fakeGateway struct{}

func (fakeGateway) Start(context.Context) error { return nil }
func (fakeGateway) Stop() error                 { return nil }
func (fakeGateway) Subscribe(int) (<-chan events.Event, func()) {
	ch := make(chan events.Event)
	close(ch)
	return ch, func() {}
}
func (fakeGateway) Status(context.Context) codex.Status {
	return codex.Status{Available: true, Version: "codex-cli test"}
}

func TestHealthEndpoint(t *testing.T) {
	service := app.NewService("test-version", fakeGateway{}, events.NewBus())
	server := httptest.NewServer(NewServer(service, "missing-dist").Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var health app.Health
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if !health.OK || health.Mode != "web" || health.Version != "test-version" {
		t.Fatalf("unexpected health: %#v", health)
	}
}

func TestCodexStatusEndpoint(t *testing.T) {
	service := app.NewService("test-version", fakeGateway{}, events.NewBus())
	server := httptest.NewServer(NewServer(service, "missing-dist").Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/codex/status")
	if err != nil {
		t.Fatalf("GET /api/codex/status failed: %v", err)
	}
	defer resp.Body.Close()

	var status codex.Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if !status.Available || status.Version != "codex-cli test" {
		t.Fatalf("unexpected status: %#v", status)
	}
}
