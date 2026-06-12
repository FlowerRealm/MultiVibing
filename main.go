package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/flowerrealm/multivibing/internal/desktop"
	"github.com/flowerrealm/multivibing/internal/projects"
	"github.com/flowerrealm/multivibing/internal/terminal"
)

const desktopVersion = "0.1.0"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	projectStorePath, err := projects.DefaultStorePath()
	if err != nil {
		log.Fatal(err)
	}
	projectStore := projects.NewStore(projectStorePath)
	terminals := terminal.NewManager()
	bridge := desktop.NewBridge(projectStore, terminals)

	err = wails.Run(&options.App{
		Title:  "MultiVibing",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: bridge.Startup,
		OnShutdown: func(ctx context.Context) {
			bridge.Shutdown(ctx)
		},
		Bind: []interface{}{bridge},
	})
	if err != nil {
		log.Fatal(err)
	}
}
