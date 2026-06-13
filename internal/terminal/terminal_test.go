package terminal_test

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/flowerrealm/multivibing/internal/terminal"
)

func skipIfWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("PTY tests require Unix")
	}
}

func waitDone(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for terminal event")
	}
}

func TestTerminal_StartAndReceiveOutput(t *testing.T) {
	skipIfWindows(t)
	var got []string
	done := make(chan struct{})
	m := terminal.NewManager(func(e terminal.Event) {
		if e.Name == terminal.EventData {
			got = append(got, e.Data)
		}
		if e.Name == terminal.EventExit {
			close(done)
		}
	})
	t.Cleanup(m.Shutdown)

	cwd, _ := os.Getwd()
	sess, err := m.Start("proj-1", cwd, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != terminal.StatusRunning {
		t.Fatalf("expected running, got %s", sess.Status)
	}
	if err := m.Write(sess.ID, "echo hello_marker\nexit\n"); err != nil {
		t.Fatal(err)
	}
	waitDone(t, done)
	if !strings.Contains(strings.Join(got, ""), "hello_marker") {
		t.Errorf("output missing hello_marker, got: %q", strings.Join(got, ""))
	}
}

func TestTerminal_ExitCode(t *testing.T) {
	skipIfWindows(t)
	var exitCode *int
	done := make(chan struct{})
	m := terminal.NewManager(func(e terminal.Event) {
		if e.Name == terminal.EventExit {
			exitCode = e.ExitCode
			close(done)
		}
	})
	t.Cleanup(m.Shutdown)

	cwd, _ := os.Getwd()
	sess, err := m.Start("proj-1", cwd, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	_ = m.Write(sess.ID, "exit 42\n")
	waitDone(t, done)
	if exitCode == nil || *exitCode != 42 {
		t.Errorf("expected exit code 42, got %v", exitCode)
	}
}

func TestTerminal_SnapshotKeepsHistoryAfterExit(t *testing.T) {
	skipIfWindows(t)
	done := make(chan struct{})
	m := terminal.NewManager(func(e terminal.Event) {
		if e.Name == terminal.EventExit {
			close(done)
		}
	})
	t.Cleanup(m.Shutdown)

	cwd, _ := os.Getwd()
	sess, err := m.Start("proj-1", cwd, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Write(sess.ID, "echo snapshot_marker\nexit 7\n"); err != nil {
		t.Fatal(err)
	}
	waitDone(t, done)

	snapshot, err := m.Snapshot(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(snapshot.History, "snapshot_marker") {
		t.Fatalf("snapshot history missing marker: %q", snapshot.History)
	}
	if snapshot.Session.Status != terminal.StatusExited {
		t.Fatalf("snapshot status = %s, want %s", snapshot.Session.Status, terminal.StatusExited)
	}
	if snapshot.Session.ExitCode == nil || *snapshot.Session.ExitCode != 7 {
		t.Fatalf("snapshot exit code = %v, want 7", snapshot.Session.ExitCode)
	}
	if snapshot.LastSeq == 0 {
		t.Fatal("snapshot LastSeq = 0, want non-zero")
	}
}

func TestTerminal_CloseRemovesSession(t *testing.T) {
	skipIfWindows(t)
	m := terminal.NewManager(nil)
	t.Cleanup(m.Shutdown)

	cwd, _ := os.Getwd()
	sess, err := m.Start("proj-1", cwd, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Close(sess.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Get(sess.ID); err == nil {
		t.Error("expected error after Close, got nil")
	}
	if err := m.Close(sess.ID); err == nil {
		t.Error("expected error closing already-closed terminal")
	}
}
