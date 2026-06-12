# MultiVibing

MultiVibing is a small Wails desktop app for opening local projects and running
project-scoped terminals.

## Development

```bash
npm install
npm run dev
```

`npm run dev` starts Wails in development mode. A standalone browser server is
not part of the app.

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
npm run build:desktop
```

## Verification

```bash
go test ./...
npm run build
```
