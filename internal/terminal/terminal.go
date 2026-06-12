package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/creack/pty"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusExited  Status = "exited"
	EventData            = "terminal:data"
	EventExit            = "terminal:exit"
	EventError           = "terminal:error"
)

type Session struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"projectId"`
	Cwd       string  `json:"cwd"`
	PID       int     `json:"pid"`
	Status    Status  `json:"status"`
	ExitCode  *int    `json:"exitCode,omitempty"`
}

type Event struct {
	Name       string
	TerminalID string
	Data       string
	ExitCode   *int
	Error      string
}

type entry struct {
	session Session
	cmd     *exec.Cmd
	ptm     *os.File
	cancel  context.CancelFunc
	writeMu sync.Mutex
}

type Manager struct {
	mu      sync.RWMutex
	nextID  int
	entries map[string]*entry
	order   []string
	emit    func(Event)
}

func NewManager() *Manager {
	return &Manager{entries: make(map[string]*entry)}
}

func (m *Manager) SetEventHandler(emit func(Event)) {
	m.mu.Lock()
	m.emit = emit
	m.mu.Unlock()
}

func (m *Manager) List(projectID string) []Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Session, 0, len(m.entries))
	for _, id := range m.order {
		if e, ok := m.entries[id]; ok && (projectID == "" || e.session.ProjectID == projectID) {
			out = append(out, e.session)
		}
	}
	return out
}

func (m *Manager) Get(id string) (Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if e, ok := m.entries[id]; ok {
		return e.session, nil
	}
	return Session{}, fmt.Errorf("terminal %q not found", id)
}

func (m *Manager) Start(projectID, cwd string, cols, rows int) (Session, error) {
	if projectID == "" {
		return Session{}, errors.New("project id cannot be empty")
	}
	if cwd == "" {
		return Session{}, errors.New("terminal cwd cannot be empty")
	}
	cols, rows = clampSize(cols, rows)

	ctx, cancel := context.WithCancel(context.Background())
	shell := defaultShell()
	cmd := exec.CommandContext(ctx, shell)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "PWD="+cwd)
	ptm, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		cancel()
		return Session{}, err
	}
	m.mu.Lock()
	m.nextID++
	id := fmt.Sprintf("term-%d", m.nextID)
	e := &entry{
		session: Session{ID: id, ProjectID: projectID, Cwd: cwd, PID: cmd.Process.Pid, Status: StatusRunning},
		cmd:     cmd,
		ptm:     ptm,
		cancel:  cancel,
	}
	m.entries[id] = e
	m.order = append(m.order, id)
	m.mu.Unlock()
	go m.readOutput(id, e)
	go m.waitExit(id, e)
	return e.session, nil
}

func (m *Manager) Write(id, data string) error {
	if data == "" {
		return nil
	}
	m.mu.RLock()
	e, ok := m.entries[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("terminal %q not found", id)
	}
	if e.session.Status != StatusRunning {
		return fmt.Errorf("terminal %q is not running", id)
	}
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	_, err := io.WriteString(e.ptm, data)
	return err
}

func (m *Manager) Resize(id string, cols, rows int) error {
	m.mu.RLock()
	e, ok := m.entries[id]
	m.mu.RUnlock()
	if !ok || e.session.Status != StatusRunning {
		return nil
	}
	cols, rows = clampSize(cols, rows)
	return pty.Setsize(e.ptm, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (m *Manager) Close(id string) error {
	m.mu.Lock()
	e, ok := m.entries[id]
	if ok {
		delete(m.entries, id)
		m.removeFromOrder(id)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("terminal %q not found", id)
	}
	e.cancel()
	_ = e.ptm.Close()
	return nil
}

func (m *Manager) Shutdown() {
	for _, s := range m.List("") {
		_ = m.Close(s.ID)
	}
}

func (m *Manager) readOutput(id string, e *entry) {
	buf := make([]byte, 8192)
	for {
		n, err := e.ptm.Read(buf)
		if n > 0 && m.owns(id, e) {
			m.publish(Event{Name: EventData, TerminalID: id, Data: string(buf[:n])})
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) && m.owns(id, e) {
				m.publish(Event{Name: EventError, TerminalID: id, Error: err.Error()})
			}
			return
		}
	}
}

func (m *Manager) waitExit(id string, e *entry) {
	_ = e.cmd.Wait()
	e.cancel()
	m.mu.Lock()
	cur, ok := m.entries[id]
	if !ok || cur != e {
		m.mu.Unlock()
		return
	}
	cur.session.Status = StatusExited
	if e.cmd.ProcessState != nil {
		code := e.cmd.ProcessState.ExitCode()
		cur.session.ExitCode = &code
	}
	code := cur.session.ExitCode
	m.mu.Unlock()
	m.publish(Event{Name: EventExit, TerminalID: id, ExitCode: code})
}

func (m *Manager) publish(event Event) {
	m.mu.RLock()
	emit := m.emit
	m.mu.RUnlock()
	if emit != nil {
		emit(event)
	}
}

func (m *Manager) owns(id string, e *entry) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.entries[id] == e
}

func (m *Manager) removeFromOrder(id string) {
	for i, cur := range m.order {
		if cur == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			return
		}
	}
}

func defaultShell() string {
	if runtime.GOOS == "windows" {
		if s := os.Getenv("COMSPEC"); s != "" {
			return s
		}
		return "cmd.exe"
	}
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}

func clampSize(cols, rows int) (int, int) {
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}
	return min(cols, 500), min(rows, 200)
}
