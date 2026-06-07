package app

import (
	"context"
	"time"

	"github.com/flowerrealm/multivibing/internal/codex"
	"github.com/flowerrealm/multivibing/internal/events"
)

type Service struct {
	version string
	codex   codex.Gateway
	bus     *events.Bus
	started time.Time
}

type Health struct {
	OK        bool   `json:"ok"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Mode      string `json:"mode"`
	StartedAt string `json:"startedAt"`
}

func NewService(version string, gateway codex.Gateway, bus *events.Bus) *Service {
	if bus == nil {
		bus = events.NewBus()
	}
	return &Service{
		version: version,
		codex:   gateway,
		bus:     bus,
		started: time.Now().UTC(),
	}
}

func (s *Service) Health(mode string) Health {
	return Health{
		OK:        true,
		Name:      "MultiVibing",
		Version:   s.version,
		Mode:      mode,
		StartedAt: s.started.Format(time.RFC3339),
	}
}

func (s *Service) CodexStatus(ctx context.Context) codex.Status {
	return s.codex.Status(ctx)
}

func (s *Service) Subscribe(buffer int) (<-chan events.Event, func()) {
	return s.bus.Subscribe(buffer)
}
