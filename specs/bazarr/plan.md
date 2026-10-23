# CONN-ARR-MCP-BAZARR — Design and implementation plan

Governing spec: [spec.md](spec.md).

Component: Bazarr MCP
Code: ../../cmd/bazarr-mcp/main.go ../../internal/bazarr/http.go ../../internal/bazarr/client.go ../../internal/bazarr/tools.go ../../internal/bazarr/config.go

## Existing system and reuse

The repository has independent Go MCPs for qBittorrent, Radarr, and Sonarr. Reuse their package/executable/test shape, Streamable HTTP handler pattern, HTTP safeguards, lifecycle, and numeric Docker UID; do not introduce shared framework code. Bazarr v1.6.2 is Python/Flask-RESTX with API mounted at `/api`, `X-API-KEY` authentication, query/form arguments, and response envelopes `data`/`total`.

## Architecture

`cmd/bazarr-mcp` validates environment configuration, constructs one Bazarr client, registers 17 typed tools, and serves one catalog at `/mcp/bazarr`. `internal/bazarr` owns request types, response types, tools, and offline tests. All upstream URLs come only from trusted configuration. The executable supports `/health`, bounded HTTP server timeouts, and graceful SIGTERM.

## Interfaces and data model

Base endpoint is `<BAZARR_URL>/api`; preserve URL installation prefixes. API key uses `X-API-KEY`. `GET /system/status`, `/system/languages`, `/system/languages/profiles`, `/series`, `/episodes`, `/episodes/wanted`, `/movies`, `/movies/wanted`, `/providers`, `/episodes/history`, `/movies/history`. Series/movie library and wanted/history endpoints use `start` and `length`; ID filters are `seriesid[]`, `episodeid[]`, `radarrid[]` (or singular `episodeid`/`radarrid` on history). Profile writes POST to `/series` with repeated `seriesid`/`profileid` pairs and `/movies` with `radarrid`/`profileid`. Search-missing is PATCH `/series?seriesid=<id>&action=search-missing` or `/movies?radarrid=<id>&action=search-missing`. Episode subtitle download is PATCH `/episodes/subtitles?seriesid=&episodeid=&language=&forced=&hi=`.

All list outputs are objects, normally Bazarr's own `{data,total}` envelope. Status output is intentionally allowlisted to version/package and Sonarr/Radarr versions; never expose host paths, database details, Python runtime, or host metadata. Search tools never target all items; each requires one positive ID. Paging is capped to 100 records, with explicit defaults.

## Live integration path

`cmd/bazarr-mcp/main.go` → config/client → MCP tools → Streamable HTTP `/mcp/bazarr` → Bazarr `/api`. Reads and actions use the same configured API key. Default tests are offline; live write calls require explicit operator authorization.

## Security and failure behavior

Use 15-second request timeout, request context cancellation, 8 MiB response cap, TLS verification defaults, refuse redirects, and return errors containing only operation and status. Reject invalid URLs, missing API key, non-positive IDs, unsupported paging and malformed language codes before requests. No tool accepts an arbitrary URL or filesystem path. No inbound authentication is included; deployments require trusted TLS/access control.

## Quality and release gates

- `gofmt -l cmd internal`
- `go test ./...`, `go vet ./...`, `go test -race ./...`
- `docker build -f docker/bazarr.Dockerfile .`
- `pipelines-hook spec-standard --root .` and `pipelines-hook spec-status --root .` when available.

## Risks, alternatives, and decisions

- The API is not formally versioned; target v1.6.2 and pin fixture expectations to its route/argument contract.
- Bazarr’s history endpoint computes derived fields and may be slower than library reads; page size is capped.
- Mass subtitle searches can initiate downloads. Explicit one-item scope and tool descriptions make this visible; global `search-wanted` actions are intentionally excluded.
- The `/api/system/status` response includes sensitive local metadata; only a deliberate allowlist is returned.

## Amendments

- **2026-10-22 (Hermes v0.22.0):** The dynamic `bazarr_action.result` schema is supplied explicitly as a JSON Schema `anyOf` over JSON value types, avoiding boolean schemas that strict MCP clients cannot parse.
