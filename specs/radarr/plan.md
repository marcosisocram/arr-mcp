# CONN-ARR-MCP-RADARR — Design and implementation plan

Governing spec: [spec.md](spec.md).

Component: Radarr MCP
Code: ../../cmd/radarr-mcp/main.go ../../internal/radarr/http.go ../../internal/radarr/client.go ../../internal/radarr/tools.go ../../internal/radarr/config.go ../../internal/radarr/client_test.go ../../internal/radarr/mcp_test.go

## Existing system and reuse

The repository holds the Python Arr MCP and one landed Go pilot, `internal/qbittorrent`, which already proves the official SDK, Streamable HTTP mounting, bounded HTTP, sanitized errors, and offline mocked tests. Radarr has no Go implementation yet. The Python implementation remains untouched. Reuse the pilot's shape — one package per integration, one executable per integration, no shared runtime — instead of inventing a framework. Do not reuse the pilot's `internal/qbittorrent` types; Radarr's API, auth, and resource model differ.

## Architecture

`cmd/radarr-mcp` loads validated environment configuration, constructs the Radarr API client, registers typed tools on one official SDK server, and mounts Streamable HTTP at `/mcp/radarr`. `internal/radarr` owns API types, endpoint methods, tool schemas, and tests. The MCP layer calls that client; no reverse dependency and no shared Arr runtime is introduced. The HTTP server has graceful shutdown and bounded read/header/idle timeouts. The bootstrap (`main.go`, `http.go`) intentionally mirrors the qBittorrent pilot rather than extracting a shared transport package, because issue #2 defers shared infrastructure and requires each integration to compile and run alone.

## Interfaces and data model

Use SDK typed tool input/output structs; responses are trimmed to useful fields rather than raw upstream JSON. Configuration is `RADARR_URL`, `RADARR_API_KEY`, `RADARR_LISTEN_ADDR`; the base URL is trimmed of a trailing slash and may carry an installation prefix, so endpoints become `<RADARR_URL>/api/v3/<path>`.

Endpoint mapping (`/api/v3`, Radarr `6.4.4.10685-ls321`):

| Tool | Request | Notes |
|---|---|---|
| `radarr_list_movies` | `GET /movie` | No server-side paging; `monitored`/`has_file` filters and `limit`/`offset` are applied locally. |
| `radarr_get_movie` | `GET /movie/{id}` | |
| `radarr_lookup_movies` | `GET /movie/lookup?term=` | Bounded to 50 results locally. |
| `radarr_list_quality_profiles` | `GET /qualityprofile` | |
| `radarr_list_root_folders` | `GET /rootfolder` | |
| `radarr_add_movie` | `GET /movie?tmdbId=` then `GET /movie/lookup/tmdb?tmdbId=` then `POST /movie` | Returns `already_exists` without a POST when the TMDB ID is present. |
| `radarr_set_movie_monitored` | `GET /movie/{id}` then `PUT /movie/{id}?moveFiles=false` | Full-fetch-then-PUT preserves every other field. |
| `radarr_search_movie` | `POST /command` `{"name":"MoviesSearch","movieIds":[id]}` | |
| `radarr_get_command` | `GET /command/{id}` | |
| `radarr_get_queue` | `GET /queue?page=&pageSize=&includeMovie=` | Server-side paging, 1-100 per page. |
| `radarr_get_history` | `GET /history?page=&pageSize=` | Server-side paging, 1-100 per page. |

`radarr_add_movie` requires `tmdb_id`, `quality_profile_id`, and `root_folder_path`, and validates `monitored` (default true), `minimum_availability` against `tba|announced|inCinemas|released|deleted` (default `released`), and `monitor` against `movieOnly|movieAndCollection|none` (default `movieOnly`). It always sends `addOptions.searchForMovie=false`; a release search is a separate, explicit `radarr_search_movie` call. Deprecated Radarr fields (`sizeleft`, `timeleft`) are not exposed.

## Live integration path

Entrypoint: `cmd/radarr-mcp/main.go` → `internal/radarr.LoadConfig` → `internal/radarr.NewClient` → registered SDK tools → SDK Streamable HTTP handler mounted at `/mcp/radarr` → Radarr `/api/v3`. Deployment runs the standalone binary/container; no Python entrypoint is involved. Read-only live validation is the default; add, monitoring, and search against a real instance require explicit operator authorization.

## Security and failure behavior

Use context-bound requests, an explicit 15-second timeout, bounded response reads (8 MiB), and sanitized error text (status and API operation only). Never log the API key, request body, or upstream error body. The HTTP client refuses to follow redirects so the `X-Api-Key` header cannot be replayed to another origin. Reject arbitrary upstream URLs: `RADARR_URL` must be http(s), with no userinfo, query, or fragment, and is never overridable through a tool argument. Reject non-positive movie/TMDB/profile IDs and unknown enum values before any upstream call. The MCP SDK protects localhost requests; standard-library cross-origin protection wraps the HTTP mux. Require ingress TLS/access control for network deployment; the container runs non-root with a read-only root filesystem where practical.

## Quality and release gates

- `gofmt -l .` on Go sources and `go test ./...` (including the SDK client/server contract test).
- `go vet ./...` and `go test -race ./...`.
- `docker build -f docker/radarr.Dockerfile .`.
- `pipelines-hook spec-standard --root .` and `pipelines-hook spec-status --root .`.

## Risks, alternatives, and decisions

- The bootstrap duplicates the qBittorrent pilot's `main.go`/`http.go` shape instead of extracting shared transport helpers. Accepted: issue #2 defers shared infrastructure and keeps integrations independently buildable; extraction is a follow-up when the aggregate host is specified.
- `radarr_list_movies` reads the whole library because API v3 offers no movie paging. Accepted for this milestone with local filtering and a documented limit.
- Radarr error bodies can carry the operator's input, so failures report status only. Accepted trade-off: duplicate detection happens through the pre-add `tmdbId` query rather than by parsing a Radarr rejection.
- Radarr's `GET /movie/{id}` + `PUT /movie/{id}` round-trip for monitoring costs one extra request but guarantees no field is dropped by a partial update.
- No live mutation happens in the default suite; real add/monitor/search calls stay opt-in.
