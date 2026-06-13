package terminal

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestSnapshotHistoryIsBounded(t *testing.T) {
	m := NewManager(nil)
	m.historyLimit = 5
	e := &entry{session: Session{ID: "term-1"}}
	m.entries["term-1"] = e
	m.order = append(m.order, "term-1")

	if event, ok := m.recordOutput("term-1", e, []byte("abc")); !ok || event.Seq != 1 {
		t.Fatalf("first record = (%#v, %t), want seq 1 and ok", event, ok)
	}
	if event, ok := m.recordOutput("term-1", e, []byte("defg")); !ok || event.Seq != 2 {
		t.Fatalf("second record = (%#v, %t), want seq 2 and ok", event, ok)
	}

	snapshot, err := m.Snapshot("term-1")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.History != "cdefg" {
		t.Fatalf("snapshot history = %q, want %q", snapshot.History, "cdefg")
	}
	if !snapshot.Truncated {
		t.Fatal("snapshot Truncated = false, want true")
	}
	if snapshot.LastSeq != 2 {
		t.Fatalf("snapshot LastSeq = %d, want 2", snapshot.LastSeq)
	}
}

func TestExitWaitsForOutputDrain(t *testing.T) {
	events := make(chan Event, 2)
	m := NewManager(func(event Event) {
		events <- event
	})
	ctx, cancel := context.WithCancel(context.Background())
	e := &entry{
		session:    Session{ID: "term-1", Status: StatusRunning},
		cmd:        exec.CommandContext(ctx, "true"),
		cancel:     cancel,
		outputDone: make(chan struct{}),
	}
	m.entries["term-1"] = e
	m.order = append(m.order, "term-1")

	if err := e.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go m.waitExit("term-1", e)

	select {
	case event := <-events:
		t.Fatalf("got event before output drain: %#v", event)
	case <-time.After(100 * time.Millisecond):
	}

	if event, ok := m.recordOutput("term-1", e, []byte("tail")); !ok || event.Seq != 1 {
		t.Fatalf("record tail = (%#v, %t), want seq 1 and ok", event, ok)
	}
	close(e.outputDone)

	select {
	case event := <-events:
		if event.Name != EventExit {
			t.Fatalf("event name = %q, want %q", event.Name, EventExit)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for exit event")
	}

	snapshot, err := m.Snapshot("term-1")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.History != "tail" {
		t.Fatalf("snapshot history = %q, want %q", snapshot.History, "tail")
	}
	if snapshot.Session.Status != StatusExited {
		t.Fatalf("snapshot status = %q, want %q", snapshot.Session.Status, StatusExited)
	}
}
