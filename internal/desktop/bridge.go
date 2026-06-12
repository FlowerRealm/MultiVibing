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

	mu  sync.Mutex
	ctx context.Context
}

func NewBridge(projects *projects.Store, terminals *terminal.Manager) *Bridge {
	return &Bridge{projects: projects, terminals: terminals}
}

func (b *Bridge) Startup(ctx context.Context) {
	b.mu.Lock()
	b.ctx = ctx
	b.mu.Unlock()

	b.terminals.SetEventHandler(func(event terminal.Event) {
		current := b.context()
		if current == nil {
			return
		}
		payload := map[string]any{"terminalId": event.TerminalID}
		if event.Data != "" {
			payload["data"] = event.Data
		}
		if event.ExitCode != nil {
			payload["exitCode"] = *event.ExitCode
		}
		if event.Error != "" {
			payload["error"] = event.Error
		}
		runtime.EventsEmit(current, event.Name, payload)
	})
}

func (b *Bridge) Shutdown(context.Context) {
	b.mu.Lock()
	b.ctx = nil
	b.mu.Unlock()
	b.terminals.SetEventHandler(nil)
	b.terminals.Shutdown()
}

func (b *Bridge) ListProjects() ([]projects.Project, error) {
	return b.projects.List()
}

func (b *Bridge) OpenProjectDialog() (*projects.Project, error) {
	ctx := b.context()
	if ctx == nil {
		return nil, nil
	}
	path, err := runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "Open Project",
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	project, err := b.projects.Open(path)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (b *Bridge) ForgetProject(id string) error {
	return b.projects.Forget(id)
}

func (b *Bridge) ListTerminals(projectID string) []terminal.Session {
	return b.terminals.List(projectID)
}

func (b *Bridge) StartTerminal(projectID string, cols int, rows int) (terminal.Session, error) {
	project, err := b.projects.Get(projectID)
	if err != nil {
		return terminal.Session{}, err
	}
	return b.terminals.Start(project.ID, project.Path, cols, rows)
}

func (b *Bridge) WriteTerminal(id string, data string) error {
	return b.terminals.Write(id, data)
}

func (b *Bridge) ResizeTerminal(id string, cols int, rows int) error {
	return b.terminals.Resize(id, cols, rows)
}

func (b *Bridge) CloseTerminal(id string) error {
	return b.terminals.Close(id)
}

func (b *Bridge) context() context.Context {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ctx
}
