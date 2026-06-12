package terminal

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/creack/pty"
)

func startPTY(ctx context.Context, options startOptions) (process, error) {
	shell := defaultShell()
	cmd := exec.CommandContext(ctx, shell)
	cmd.Dir = options.cwd
	cmd.Env = terminalEnv(options.cwd)
	cols, rows := normalizeSize(options.cols, options.rows)
	file, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		return nil, err
	}
	return &osProcess{cmd: cmd, file: file}, nil
}

func defaultShell() string {
	if runtime.GOOS == "windows" {
		if shell := os.Getenv("COMSPEC"); shell != "" {
			return shell
		}
		return "cmd.exe"
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

func terminalEnv(cwd string) []string {
	return append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"PWD="+cwd,
	)
}

type osProcess struct {
	cmd      *exec.Cmd
	file     *os.File
	waitOnce sync.Once
	waitErr  error
}

func (p *osProcess) Read(buf []byte) (int, error) {
	return p.file.Read(buf)
}

func (p *osProcess) Write(buf []byte) (int, error) {
	return p.file.Write(buf)
}

func (p *osProcess) PID() int {
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

func (p *osProcess) Resize(cols, rows int) error {
	cols, rows = normalizeSize(cols, rows)
	return pty.Setsize(p.file, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

func (p *osProcess) Close() error {
	return p.file.Close()
}

func (p *osProcess) Wait() error {
	p.waitOnce.Do(func() {
		p.waitErr = p.cmd.Wait()
	})
	return p.waitErr
}

func (p *osProcess) ExitCode() *int {
	if p.cmd.ProcessState == nil {
		return nil
	}
	code := p.cmd.ProcessState.ExitCode()
	return &code
}
