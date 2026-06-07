package desktop

import (
	"context"
	"time"

	"github.com/flowerrealm/multivibing/internal/app"
	"github.com/flowerrealm/multivibing/internal/codex"
)

type Bridge struct {
	service *app.Service
}

func NewBridge(service *app.Service) *Bridge {
	return &Bridge{service: service}
}

func (b *Bridge) Health() app.Health {
	return b.service.Health("desktop")
}

func (b *Bridge) CodexStatus() codex.Status {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return b.service.CodexStatus(ctx)
}
