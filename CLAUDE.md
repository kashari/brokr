# brokr

Finite state machine implementation with an event bus, written in Go.

## Commands

- Build: `go build ./...`
- Test: `go test ./...`
- Lint/vet: `go vet ./...`
- Format: `gofmt -w .`
- Dev server: `docker-compose up -d db && go run main.go`

## Notes

- Postgres runs via `docker-compose.yml` (service `db`, host port 5436, db `workflow`).
- Router: `github.com/kashari/draupnir`. ORM: GORM with the Postgres driver.
- Schema auto-migration runs on startup (`config.Db.AutoMigrate`).
- The state machine visualizer is served at `GET /workflows/:id/visualizer` (single-file React app, no build step); its data comes from `GET /workflows/:id/visualization-data`, driven by the live instance — never a file upload.
