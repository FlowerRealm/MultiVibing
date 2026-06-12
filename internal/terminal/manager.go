package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

type Manager struct {
	mu       sync.RWMutex
	nextID   int
	sessions map[string]*managedSession
	order    []string
	start    startFunc
	emit     func(Event)
}

type managedSession struct {
	session Session
	process process
	cancel  context.CancelFunc
	writeMu sync.Mutex
}

func NewManager() *Manager {
	return newManagerWithStarter(startPTY)
}

func newManagerWithStarter(start startFunc) *Manager {
	return &Manager{
		sessions: make(map[string]*managedSession),
		start:    start,
	}
}

func (m *Manager) SetEventHandler(emit func(Event)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emit = emit
}

func (m *Manager) List(projectID string) []Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]Session, 0, len(m.sessions))
	for _, id := range m.order {
		managed, ok := m.sessions[id]
		if !ok {
			continue
		}
		if projectID == "" || managed.session.ProjectID == projectID {
			sessions = append(sessions, managed.session)
		}
	}
	return sessions
}

func (m *Manager) Get(id string) (Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	managed, ok := m.sessions[id]
	if !ok {
		return Session{}, fmt.Errorf("terminal %q not found", id)
	}
	return managed.session, nil
}

func (m *Manager) Start(projectID, cwd string, cols, rows int) (Session, error) {
	if projectID == "" {
		return Session{}, errors.New("project id cannot be empty")
	}
	if cwd == "" {
		return Session{}, errors.New("terminal cwd cannot be empty")
	}
	cols, rows = normalizeSize(cols, rows)

	ctx, cancel := context.WithCancel(context.Background())
	process, err := m.start(ctx, startOptions{cwd: cwd, cols: cols, rows: rows})
	if err != nil {
		cancel()
		return Session{}, err
	}

	m.mu.Lock()
	m.nextID++
	id := fmt.Sprintf("term-%d", m.nextID)
	session := Session{
		ID:        id,
		ProjectID: projectID,
		Cwd:       cwd,
		PID:       process.PID(),
		Status:    StatusRunning,
	}
	m.sessions[id] = &managedSession{session: session, process: process, cancel: cancel}
	m.order = append(m.order, id)
	m.mu.Unlock()

	go m.readOutput(id, process)
	go m.wait(id, process, cancel)
	return session, nil
}

func (m *Manager) Write(id, data string) error {
	if data == "" {
		return nil
	}
	managed, session, err := m.getManaged(id)
	if err != nil {
		return err
	}
	if session.Status != StatusRunning {
		return fmt.Errorf("terminal %q is not running", id)
	}

	managed.writeMu.Lock()
	defer managed.writeMu.Unlock()
	_, err = io.WriteString(managed.process, data)
	return err
}

func (m *Manager) Resize(id string, cols, rows int) error {
	managed, session, err := m.getManaged(id)
	if err != nil {
		return err
	}
	if session.Status != StatusRunning {
		return nil
	}
	cols, rows = normalizeSize(cols, rows)
	if err := managed.process.Resize(cols, rows); err != nil {
		return err
	}
	return nil
}

func (m *Manager) Close(id string) error {
	m.mu.Lock()
	managed, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
		m.removeFromOrderLocked(id)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("terminal %q not found", id)
	}

	managed.cancel()
	_ = managed.process.Close()
	return nil
}

func (m *Manager) Shutdown() {
	for _, session := range m.List("") {
		_ = m.Close(session.ID)
	}
}

func (m *Manager) getManaged(id string) (*managedSession, Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	managed, ok := m.sessions[id]
	if !ok {
		return nil, Session{}, fmt.Errorf("terminal %q not found", id)
	}
	return managed, managed.session, nil
}

func (m *Manager) readOutput(id string, process process) {
	buf := make([]byte, 8192)
	for {
		n, err := process.Read(buf)
		if n > 0 {
			if m.hasProcess(id, process) {
				m.publish(Event{Name: EventData, TerminalID: id, Data: string(buf[:n])})
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
				if m.hasProcess(id, process) {
					m.publish(Event{Name: EventError, TerminalID: id, Error: err.Error()})
				}
			}
			return
		}
	}
}

func (m *Manager) wait(id string, process process, cancel context.CancelFunc) {
	_ = process.Wait()
	cancel()

	m.mu.Lock()
	managed, ok := m.sessions[id]
	if !ok || managed.process != process {
		m.mu.Unlock()
		return
	}
	managed.session.Status = StatusExited
	managed.session.ExitCode = process.ExitCode()
	exitCode := managed.session.ExitCode
	m.mu.Unlock()

	m.publish(Event{Name: EventExit, TerminalID: id, ExitCode: exitCode})
}

func (m *Manager) publish(event Event) {
	m.mu.RLock()
	emit := m.emit
	m.mu.RUnlock()
	if emit != nil {
		emit(event)
	}
}

func (m *Manager) hasProcess(id string, process process) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	managed, ok := m.sessions[id]
	if !ok || managed.process != process {
		return false
	}
	return true
}

func (m *Manager) removeFromOrderLocked(id string) {
	for index, current := range m.order {
		if current == id {
			m.order = append(m.order[:index], m.order[index+1:]...)
			return
		}
	}
}
