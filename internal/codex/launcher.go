package codex

import "strings"

const DefaultBinary = "codex"

type LaunchConfig struct {
	Binary string
	Listen string
	Cwd    string
	Env    []string
}

type CommandSpec struct {
	Binary string
	Args   []string
	Cwd    string
	Env    []string
}

func DefaultLaunchConfig() LaunchConfig {
	return LaunchConfig{
		Binary: DefaultBinary,
		Listen: "stdio://",
	}
}

func AppServerCommandSpec(cfg LaunchConfig) CommandSpec {
	if strings.TrimSpace(cfg.Binary) == "" {
		cfg.Binary = DefaultBinary
	}
	if strings.TrimSpace(cfg.Listen) == "" {
		cfg.Listen = "stdio://"
	}

	return CommandSpec{
		Binary: cfg.Binary,
		Args:   []string{"app-server", "--listen", cfg.Listen},
		Cwd:    cfg.Cwd,
		Env:    append([]string(nil), cfg.Env...),
	}
}
