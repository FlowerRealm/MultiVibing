package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flowerrealm/multivibing/internal/config"
	"github.com/flowerrealm/multivibing/internal/httpapi"
	"github.com/flowerrealm/multivibing/internal/projects"
	"github.com/flowerrealm/multivibing/internal/system"
	"github.com/flowerrealm/multivibing/internal/terminal"
)

const version = "0.1.0"

func main() {
	cfg, err := config.ParseServerArgs(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	projectStorePath, err := projects.DefaultStorePath()
	if err != nil {
		log.Fatal(err)
	}
	projectStore := projects.NewStore(projectStorePath)
	terminals := terminal.NewManager()
	api := httpapi.NewServer(version, projectStore, terminals, cfg.StaticDir)
	server := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	listener, err := net.Listen("tcp", cfg.Addr())
	if err != nil {
		log.Fatal(err)
	}

	url := cfg.PublicURL()
	log.Printf("MultiVibing web server listening at http://%s", listener.Addr().String())
	if cfg.OpenBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			if err := system.OpenBrowser(url); err != nil {
				log.Printf("open browser failed: %v", err)
			}
		}()
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(listener)
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-stopCh:
		log.Printf("received %s, shutting down", sig)
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "shutdown failed: %v\n", err)
	}
	terminals.Shutdown()
}
