package config

import "testing"

func TestParseServerArgsDefaultsToWebMode(t *testing.T) {
	cfg, err := ParseServerArgs(nil)
	if err != nil {
		t.Fatalf("ParseServerArgs returned error: %v", err)
	}

	if cfg.Host != DefaultHost {
		t.Fatalf("host = %q, want %q", cfg.Host, DefaultHost)
	}
	if cfg.Port != DefaultPort {
		t.Fatalf("port = %d, want %d", cfg.Port, DefaultPort)
	}
	if cfg.StaticDir != "frontend/dist" {
		t.Fatalf("StaticDir = %q", cfg.StaticDir)
	}
}

func TestParseServerArgsCustomPortAndStaticDir(t *testing.T) {
	cfg, err := ParseServerArgs([]string{
		"--port", "4000",
		"--static-dir", "custom-dist",
	})
	if err != nil {
		t.Fatalf("ParseServerArgs returned error: %v", err)
	}

	if cfg.Addr() != "127.0.0.1:4000" {
		t.Fatalf("Addr = %q", cfg.Addr())
	}
	if cfg.StaticDir != "custom-dist" {
		t.Fatalf("StaticDir = %q", cfg.StaticDir)
	}
}
