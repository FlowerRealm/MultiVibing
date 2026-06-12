package terminal

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

func TestManagerLifecycle(t *testing.T) {
	starter := &fakeStarter{}
	manager := newManagerWithStarter(starter.start)
	events := make(chan Event, 8)
	manager.SetEventHandler(func(event Event) {
		events <- event
	})

	session, err := manager.Start("project-1", "/tmp/project-one", 90, 25)
	if err != nil {
		t.Fatalf("start terminal: %v", err)
	}
	if session.ProjectID != "project-1" || session.Cwd != "/tmp/project-one" || session.Status != StatusRunning {
		t.Fatalf("session = %#v", session)
	}
	if starter.options[0].cwd != "/tmp/project-one" || starter.options[0].cols != 90 || starter.options[0].rows != 25 {
		t.Fatalf("start options = %#v", starter.options[0])
	}

	if err := manager.Write(session.ID, "echo ready\n"); err != nil {
		t.Fatalf("write terminal: %v", err)
	}
	if got := starter.processes[0].inputString(); got != "echo ready\n" {
		t.Fatalf("terminal input = %q", got)
	}
	if err := manager.Resize(session.ID, 120, 40); err != nil {
		t.Fatalf("resize terminal: %v", err)
	}
	if cols, rows := starter.processes[0].lastSize(); cols != 120 || rows != 40 {
		t.Fatalf("resize = %dx%d, want 120x40", cols, rows)
	}

	starter.processes[0].emit("ready\n")
	output := waitForTerminalEvent(t, events, EventData)
	if output.TerminalID != session.ID || output.Data != "ready\n" {
		t.Fatalf("output event = %#v", output)
	}

	starter.processes[0].finish(7)
	exited := waitForTerminalEvent(t, events, EventExit)
	if exited.TerminalID != session.ID || exited.ExitCode == nil || *exited.ExitCode != 7 {
		t.Fatalf("exit event = %#v", exited)
	}
	current, err := manager.Get(session.ID)
	if err != nil {
		t.Fatalf("get exited terminal: %v", err)
	}
	if current.Status != StatusExited || current.ExitCode == nil || *current.ExitCode != 7 {
		t.Fatalf("exited session = %#v", current)
	}
}

func TestManagerCloseRemovesSession(t *testing.T) {
	starter := &fakeStarter{}
	manager := newManagerWithStarter(starter.start)

	session, err := manager.Start("project-1", "/tmp/project-one", 100, 30)
	if err != nil {
		t.Fatalf("start terminal: %v", err)
	}
	if err := manager.Close(session.ID); err != nil {
		t.Fatalf("close terminal: %v", err)
	}
	if _, err := manager.Get(session.ID); err == nil {
		t.Fatal("Get closed terminal returned nil error")
	}
	if sessions := manager.List("project-1"); len(sessions) != 0 {
		t.Fatalf("sessions after close = %#v, want empty", sessions)
	}
}

type fakeStarter struct {
	mu        sync.Mutex
	options   []startOptions
	processes []*fakeProcess
}

func (s *fakeStarter) start(_ context.Context, options startOptions) (process, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	process := newFakeProcess(1000 + len(s.processes))
	s.options = append(s.options, options)
	s.processes = append(s.processes, process)
	return process, nil
}

type fakeProcess struct {
	pid int

	outR *io.PipeReader
	outW *io.PipeWriter

	mu       sync.Mutex
	input    bytes.Buffer
	cols     int
	rows     int
	exitCode *int

	done      chan struct{}
	closeOnce sync.Once
}

func newFakeProcess(pid int) *fakeProcess {
	outR, outW := io.Pipe()
	return &fakeProcess{
		pid:  pid,
		outR: outR,
		outW: outW,
		done: make(chan struct{}),
	}
}

func (p *fakeProcess) Read(buf []byte) (int, error) {
	return p.outR.Read(buf)
}

func (p *fakeProcess) Write(buf []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.input.Write(buf)
}

func (p *fakeProcess) PID() int {
	return p.pid
}

func (p *fakeProcess) Resize(cols, rows int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cols = cols
	p.rows = rows
	return nil
}

func (p *fakeProcess) Close() error {
	p.closeOnce.Do(func() {
		_ = p.outW.Close()
		close(p.done)
	})
	return nil
}

func (p *fakeProcess) Wait() error {
	<-p.done
	return nil
}

func (p *fakeProcess) ExitCode() *int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.exitCode == nil {
		return nil
	}
	code := *p.exitCode
	return &code
}

func (p *fakeProcess) emit(data string) {
	_, _ = io.WriteString(p.outW, data)
}

func (p *fakeProcess) finish(code int) {
	p.mu.Lock()
	p.exitCode = &code
	p.mu.Unlock()
	_ = p.Close()
}

func (p *fakeProcess) inputString() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.input.String()
}

func (p *fakeProcess) lastSize() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cols, p.rows
}

func waitForTerminalEvent(t *testing.T, events <-chan Event, eventName string) Event {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		select {
		case event := <-events:
			if event.Name == eventName {
				return event
			}
		case <-timeout:
			t.Fatalf("timed out waiting for %s", eventName)
		}
	}
}
