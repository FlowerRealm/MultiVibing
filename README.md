# MultiVibing

MultiVibing is a Go-first client shell for Codex CLI. The initial project only
contains architecture scaffolding: a browser-mode server, a Wails desktop shell,
and a Codex app-server gateway.

## Development

```bash
npm install
npm run dev
```

`npm run dev` starts the Vite frontend and the Go browser server, then opens the
local browser.

Desktop mode uses Wails v2:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
npm run dev:desktop
npm run build:desktop
```

## Verification

```bash
go test ./...
npm run build
```
