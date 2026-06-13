package config

import (
	"errors"
	"flag"
	"fmt"
)

const (
	DefaultHost = "127.0.0.1"
	DefaultPort = 34117
)

type ServerConfig struct {
	Host      string
	Port      int
	StaticDir string
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host:      DefaultHost,
		Port:      DefaultPort,
		StaticDir: "frontend/dist",
	}
}

func (c ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func ParseServerArgs(args []string) (ServerConfig, error) {
	cfg := DefaultServerConfig()

	fs := flag.NewFlagSet("multivibing", flag.ContinueOnError)
	fs.StringVar(&cfg.Host, "host", cfg.Host, "host for the web HTTP server")
	fs.IntVar(&cfg.Port, "port", cfg.Port, "port for the web HTTP server")
	fs.StringVar(&cfg.StaticDir, "static-dir", cfg.StaticDir, "directory containing built frontend assets")

	if err := fs.Parse(args); err != nil {
		return ServerConfig{}, err
	}
	if cfg.Host == "" {
		return ServerConfig{}, errors.New("host cannot be empty")
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		return ServerConfig{}, fmt.Errorf("port out of range: %d", cfg.Port)
	}
	return cfg, nil
}
