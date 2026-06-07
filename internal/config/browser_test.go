package config

import "testing"

func TestParseBrowserArgsDefaultsToOpenBrowserMode(t *testing.T) {
	cfg, err := ParseBrowserArgs(nil)
	if err != nil {
		t.Fatalf("ParseBrowserArgs returned error: %v", err)
	}

	if cfg.Host != DefaultHost {
		t.Fatalf("host = %q, want %q", cfg.Host, DefaultHost)
	}
	if cfg.Port != DefaultPort {
		t.Fatalf("port = %d, want %d", cfg.Port, DefaultPort)
	}
	if !cfg.OpenBrowser {
		t.Fatal("OpenBrowser = false, want true")
	}
	if cfg.PublicURL() != "http://127.0.0.1:34117" {
		t.Fatalf("PublicURL = %q", cfg.PublicURL())
	}
}

func TestParseBrowserArgsNoOpenAndDevFrontend(t *testing.T) {
	cfg, err := ParseBrowserArgs([]string{
		"--no-open",
		"--frontend-dev-url", "http://127.0.0.1:5173",
		"--port", "4000",
	})
	if err != nil {
		t.Fatalf("ParseBrowserArgs returned error: %v", err)
	}

	if cfg.OpenBrowser {
		t.Fatal("OpenBrowser = true, want false")
	}
	if cfg.PublicURL() != "http://127.0.0.1:5173" {
		t.Fatalf("PublicURL = %q", cfg.PublicURL())
	}
	if cfg.Addr() != "127.0.0.1:4000" {
		t.Fatalf("Addr = %q", cfg.Addr())
	}
}
