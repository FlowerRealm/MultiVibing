package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/flowerrealm/multivibing/internal/app"
	"github.com/flowerrealm/multivibing/internal/codex"
	"github.com/flowerrealm/multivibing/internal/desktop"
	"github.com/flowerrealm/multivibing/internal/events"
)

const desktopVersion = "0.1.0"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	bus := events.NewBus()
	gateway := codex.NewAppServerGateway(codex.DefaultLaunchConfig(), bus)
	service := app.NewService(desktopVersion, gateway, bus)
	bridge := desktop.NewBridge(service)

	err := wails.Run(&options.App{
		Title:  "MultiVibing",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Bind: []interface{}{bridge},
	})
	if err != nil {
		log.Fatal(err)
	}
}
