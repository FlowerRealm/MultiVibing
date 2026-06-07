package events

import (
	"sync"
	"time"
)

type Event struct {
	Type      string         `json:"type"`
	Message   string         `json:"message"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

type Bus struct {
	mu          sync.RWMutex
	nextID      int
	subscribers map[int]chan Event
}

func NewBus() *Bus {
	return &Bus{subscribers: make(map[int]chan Event)}
}

func (b *Bus) Publish(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, subscriber := range b.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (b *Bus) Subscribe(buffer int) (<-chan Event, func()) {
	if buffer <= 0 {
		buffer = 16
	}

	b.mu.Lock()
	id := b.nextID
	b.nextID++
	ch := make(chan Event, buffer)
	b.subscribers[id] = ch
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if current, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(current)
		}
		b.mu.Unlock()
	}

	return ch, cancel
}
