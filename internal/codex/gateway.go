package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/flowerrealm/multivibing/internal/events"
)

type Status struct {
	Available bool   `json:"available"`
	Running   bool   `json:"running"`
	Version   string `json:"version,omitempty"`
	PID       int    `json:"pid,omitempty"`
	Error     string `json:"error,omitempty"`
}

type Gateway interface {
	Start(context.Context) error
	Stop() error
	Status(context.Context) Status
	Subscribe(buffer int) (<-chan events.Event, func())
}

type AppServerGateway struct {
	cfg LaunchConfig
	bus *events.Bus

	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	cancel context.CancelFunc
	nextID int
}

func NewAppServerGateway(cfg LaunchConfig, bus *events.Bus) *AppServerGateway {
	if bus == nil {
		bus = events.NewBus()
	}
	return &AppServerGateway{cfg: cfg, bus: bus}
}

func (g *AppServerGateway) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cmd != nil && g.cmd.Process != nil {
		return nil
	}

	spec := AppServerCommandSpec(g.cfg)
	runCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(runCtx, spec.Binary, spec.Args...)
	cmd.Dir = spec.Cwd
	cmd.Env = append(cmd.Environ(), spec.Env...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}

	g.cmd = cmd
	g.stdin = stdin
	g.cancel = cancel
	g.publish("codex.started", "codex app-server started", map[string]any{"pid": cmd.Process.Pid})

	go g.readLines(stdout, "codex.message")
	go g.readLines(stderr, "codex.stderr")
	go g.wait(cmd, cancel)

	if err := g.initializeLocked(); err != nil {
		_ = g.stopLocked()
		return err
	}

	return nil
}

func (g *AppServerGateway) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.stopLocked()
}

func (g *AppServerGateway) Status(ctx context.Context) Status {
	status := Status{}
	version, err := detectVersion(ctx, AppServerCommandSpec(g.cfg).Binary)
	if err != nil {
		status.Error = err.Error()
	} else {
		status.Available = true
		status.Version = version
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cmd != nil && g.cmd.Process != nil {
		status.Running = true
		status.PID = g.cmd.Process.Pid
	}
	return status
}

func (g *AppServerGateway) Subscribe(buffer int) (<-chan events.Event, func()) {
	return g.bus.Subscribe(buffer)
}

func (g *AppServerGateway) initializeLocked() error {
	if g.stdin == nil {
		return errors.New("codex app-server stdin is not available")
	}
	if err := g.sendLocked("initialize", map[string]any{
		"clientInfo": map[string]string{
			"name":    "multivibing",
			"title":   "MultiVibing",
			"version": "0.1.0",
		},
	}); err != nil {
		return err
	}
	return g.sendNotificationLocked("initialized", map[string]any{})
}

func (g *AppServerGateway) sendLocked(method string, params map[string]any) error {
	g.nextID++
	payload := map[string]any{
		"id":     g.nextID,
		"method": method,
		"params": params,
	}
	return writeJSONLine(g.stdin, payload)
}

func (g *AppServerGateway) sendNotificationLocked(method string, params map[string]any) error {
	payload := map[string]any{
		"method": method,
		"params": params,
	}
	return writeJSONLine(g.stdin, payload)
}

func (g *AppServerGateway) stopLocked() error {
	if g.cancel != nil {
		g.cancel()
	}
	if g.stdin != nil {
		_ = g.stdin.Close()
	}
	if g.cmd != nil && g.cmd.Process != nil {
		_ = g.cmd.Process.Kill()
	}
	g.cmd = nil
	g.stdin = nil
	g.cancel = nil
	g.publish("codex.stopped", "codex app-server stopped", nil)
	return nil
}

func (g *AppServerGateway) wait(cmd *exec.Cmd, cancel context.CancelFunc) {
	err := cmd.Wait()
	cancel()
	data := map[string]any{}
	if err != nil {
		data["error"] = err.Error()
	}

	g.mu.Lock()
	if g.cmd == cmd {
		g.cmd = nil
		g.stdin = nil
		g.cancel = nil
	}
	g.mu.Unlock()

	g.publish("codex.exited", "codex app-server exited", data)
}

func (g *AppServerGateway) readLines(reader io.Reader, eventType string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		data := map[string]any{"raw": line}
		var parsed map[string]any
		if json.Unmarshal([]byte(line), &parsed) == nil {
			data["json"] = parsed
		}
		g.publish(eventType, "codex app-server output", data)
	}
	if err := scanner.Err(); err != nil {
		g.publish("codex.read_error", "failed to read codex app-server output", map[string]any{"error": err.Error()})
	}
}

func (g *AppServerGateway) publish(eventType, message string, data map[string]any) {
	g.bus.Publish(events.Event{Type: eventType, Message: message, Data: data})
}

func writeJSONLine(writer io.Writer, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	_, err = writer.Write(encoded)
	return err
}

func detectVersion(ctx context.Context, binary string) (string, error) {
	if binary == "" {
		binary = DefaultBinary
	}
	versionCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	output, err := exec.CommandContext(versionCtx, binary, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s --version failed: %w", binary, err)
	}
	return string(bytesTrimSpace(output)), nil
}

func bytesTrimSpace(input []byte) []byte {
	for len(input) > 0 && (input[0] == ' ' || input[0] == '\n' || input[0] == '\r' || input[0] == '\t') {
		input = input[1:]
	}
	for len(input) > 0 {
		last := input[len(input)-1]
		if last != ' ' && last != '\n' && last != '\r' && last != '\t' {
			break
		}
		input = input[:len(input)-1]
	}
	return input
}
