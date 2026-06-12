package desktop

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/flowerrealm/multivibing/internal/projects"
	"github.com/flowerrealm/multivibing/internal/terminal"
)

type Bridge struct {
	projects  *projects.Store
	terminals *terminal.Manager
	mu        sync.Mutex
	ctx       context.Context
}

func NewBridge(projects *projects.Store, terminals *terminal.Manager) *Bridge {
	return &Bridge{projects: projects, terminals: terminals}
}

func (b *Bridge) Startup(ctx context.Context) {
	b.mu.Lock()
	b.ctx = ctx
	b.mu.Unlock()
	b.terminals.SetEventHandler(func(e terminal.Event) {
		ctx := b.context()
		if ctx == nil {
			return
		}
		payload := map[string]any{"terminalId": e.TerminalID}
		if e.Data != "" {
			payload["data"] = e.Data
		}
		if e.ExitCode != nil {
			payload["exitCode"] = *e.ExitCode
		}
		if e.Error != "" {
			payload["error"] = e.Error
		}
		runtime.EventsEmit(ctx, e.Name, payload)
	})
}

func (b *Bridge) Shutdown(context.Context) {
	b.mu.Lock()
	b.ctx = nil
	b.mu.Unlock()
	b.terminals.SetEventHandler(nil)
	b.terminals.Shutdown()
}

func (b *Bridge) ListProjects() ([]projects.Project, error)      { return b.projects.List() }
func (b *Bridge) ForgetProject(id string) error                   { return b.projects.Forget(id) }
func (b *Bridge) ListTerminals(projectID string) []terminal.Session { return b.terminals.List(projectID) }
func (b *Bridge) WriteTerminal(id, data string) error             { return b.terminals.Write(id, data) }
func (b *Bridge) CloseTerminal(id string) error                   { return b.terminals.Close(id) }

func (b *Bridge) ResizeTerminal(id string, cols, rows int) error {
	return b.terminals.Resize(id, cols, rows)
}

func (b *Bridge) OpenProjectDialog() (*projects.Project, error) {
	ctx := b.context()
	if ctx == nil {
		return nil, nil
	}
	path, err := runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{Title: "Open Project"})
	if err != nil || path == "" {
		return nil, err
	}
	p, err := b.projects.Open(path)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (b *Bridge) StartTerminal(projectID string, cols, rows int) (terminal.Session, error) {
	p, err := b.projects.Get(projectID)
	if err != nil {
		return terminal.Session{}, err
	}
	return b.terminals.Start(p.ID, p.Path, cols, rows)
}

func (b *Bridge) context() context.Context {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ctx
}
