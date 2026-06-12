package terminal

import (
	"context"
	"io"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusExited  Status = "exited"
)

type Session struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Cwd       string `json:"cwd"`
	PID       int    `json:"pid"`
	Status    Status `json:"status"`
	ExitCode  *int   `json:"exitCode,omitempty"`
}

const (
	EventData  = "terminal:data"
	EventExit  = "terminal:exit"
	EventError = "terminal:error"
)

type Event struct {
	Name       string
	TerminalID string
	Data       string
	ExitCode   *int
	Error      string
}

type startOptions struct {
	cwd  string
	cols int
	rows int
}

type startFunc func(context.Context, startOptions) (process, error)

type process interface {
	io.Reader
	io.Writer
	PID() int
	Resize(cols, rows int) error
	Close() error
	Wait() error
	ExitCode() *int
}

func normalizeSize(cols, rows int) (int, int) {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	if cols > 500 {
		cols = 500
	}
	if rows > 200 {
		rows = 200
	}
	return cols, rows
}
