package codex

import "testing"

func TestAppServerCommandSpecUsesStdioWithoutShell(t *testing.T) {
	spec := AppServerCommandSpec(LaunchConfig{})

	if spec.Binary != "codex" {
		t.Fatalf("Binary = %q, want codex", spec.Binary)
	}
	want := []string{"app-server", "--listen", "stdio://"}
	if len(spec.Args) != len(want) {
		t.Fatalf("Args = %#v, want %#v", spec.Args, want)
	}
	for i := range want {
		if spec.Args[i] != want[i] {
			t.Fatalf("Args[%d] = %q, want %q", i, spec.Args[i], want[i])
		}
	}
}

func TestAppServerCommandSpecPreservesExplicitConfig(t *testing.T) {
	spec := AppServerCommandSpec(LaunchConfig{
		Binary: "/usr/local/bin/codex",
		Listen: "ws://127.0.0.1:4500",
		Cwd:    "/tmp/project",
		Env:    []string{"CODEX_HOME=/tmp/codex-home"},
	})

	if spec.Binary != "/usr/local/bin/codex" {
		t.Fatalf("Binary = %q", spec.Binary)
	}
	if spec.Args[2] != "ws://127.0.0.1:4500" {
		t.Fatalf("listen arg = %q", spec.Args[2])
	}
	if spec.Cwd != "/tmp/project" {
		t.Fatalf("Cwd = %q", spec.Cwd)
	}
	if len(spec.Env) != 1 || spec.Env[0] != "CODEX_HOME=/tmp/codex-home" {
		t.Fatalf("Env = %#v", spec.Env)
	}
}
