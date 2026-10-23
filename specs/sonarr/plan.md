# CONN-ARR-MCP-SONARR — Design and implementation plan

Governing spec: [spec.md](spec.md).

Component: Sonarr MCP
Code: ../../cmd/sonarr-mcp/main.go ../../internal/sonarr/http.go ../../internal/sonarr/client.go ../../internal/sonarr/tools.go ../../internal/sonarr/config.go ../../internal/sonarr/client_test.go ../../internal/sonarr/mcp_test.go

## Existing system and reuse

The repository holds the Python Arr MCP and two landed Go integrations, `internal/qbittorrent` and `internal/radarr`, which already prove the official SDK, Streamable HTTP mounting, bounded HTTP, sanitized errors, and offline mocked tests. Sonarr has no Go implementation yet. The Python implementation remains untouched. Reuse the Radarr shape — one package per integration, one executable per integration, no shared runtime — instead of inventing a framework. Do not reuse the Radarr `internal/radarr` types; Sonarr's API, auth header aside, and resource model (series/seasons/episodes, TVDB identity, three search command shapes) differ.

## Architecture

`cmd/sonarr-mcp` loads validated environment configuration, constructs the Sonarr API client, registers typed tools on one official SDK server, and mounts Streamable HTTP at `/mcp/sonarr`. `internal/sonarr` owns API types, endpoint methods, tool schemas, and tests. The MCP layer calls that client; no reverse dependency and no shared Arr runtime is introduced. The HTTP server has graceful shutdown and bounded read/header/idle timeouts. The bootstrap (`main.go`, `http.go`) intentionally mirrors the Radarr integration rather than extracting a shared transport package, because issue #2 defers shared infrastructure and requires each integration to compile and run alone.

## Interfaces and data model

Use SDK typed tool input/output structs; responses are trimmed to useful fields rather than raw upstream JSON. Configuration is `SONARR_URL`, `SONARR_API_KEY`, `SONARR_LISTEN_ADDR`; the base URL is trimmed of a trailing slash and may carry an installation prefix, so endpoints become `<SONARR_URL>/api/v3/<path>`.

Endpoint mapping (`/api/v3`, Sonarr `4.0.20.3014-ls329`):

| Tool | Request | Notes |
|---|---|---|
| `sonarr_list_series` | `GET /series` | No server-side paging; `monitored` filter and `limit`/`offset` are applied locally. |
| `sonarr_get_series` | `GET /series/{id}` | Includes per-season numbers and monitoring flags. |
| `sonarr_lookup_series` | `GET /series/lookup?term=` | Bounded to 50 results locally. |
| `sonarr_list_quality_profiles` | `GET /qualityprofile` | |
| `sonarr_list_root_folders` | `GET /rootfolder` | |
| `sonarr_list_episodes` | `GET /episode?seriesId=` (+ `seasonNumber=`) | No server-side paging; season/`monitored`/`has_file` filters and `limit`/`offset` applied locally. |
| `sonarr_get_episode` | `GET /episode/{id}` | |
| `sonarr_add_series` | `GET /series?tvdbId=` then `GET /series/lookup?term=tvdb:{id}` then `POST /series` | Returns `already_exists` without a POST when the TVDB ID is present. |
| `sonarr_set_series_monitored` | `GET /series/{id}` then `PUT /series/{id}?moveFiles=false` | Full-fetch-then-PUT preserves every other field. |
| `sonarr_set_episode_monitored` | `PUT /episode/monitor` `{"episodeIds":[...],"monitored":...}` | No fetch needed; the endpoint patches flags only. |
| `sonarr_search_series` | `POST /command` `{"name":"SeriesSearch","seriesId":id}` | |
| `sonarr_search_season` | `POST /command` `{"name":"SeasonSearch","seriesId":id,"seasonNumber":n}` | |
| `sonarr_search_episodes` | `POST /command` `{"name":"EpisodeSearch","episodeIds":[...]}` | |
| `sonarr_get_command` | `GET /command/{id}` | |
| `sonarr_get_queue` | `GET /queue?page=&pageSize=` | Server-side paging, 1-100 per page. |
| `sonarr_get_history` | `GET /history?page=&pageSize=` | Server-side paging, 1-100 per page. |

`sonarr_add_series` requires `tvdb_id`, `quality_profile_id`, and `root_folder_path`, and validates `monitored` (default true), `season_folder` (default true), `series_type` against `standard|daily|anime` (default `standard`), and `monitor` against `unknown|all|future|missing|existing|pilot|firstSeason|lastSeason|latestSeason|none` (default `all`). It always sends `addOptions.searchForMissingEpisodes=false` and `addOptions.searchForCutoffUnmetEpisodes=false`; a release search is a separate, explicit `sonarr_search_*` call. Sonarr has no TMDB-lookup-by-ID endpoint, so the candidate metadata comes from `GET /series/lookup?term=tvdb:{id}` and a missing title is rejected before any POST.

## Live integration path

Entrypoint: `cmd/sonarr-mcp/main.go` → `internal/sonarr.LoadConfig` → `internal/sonarr.NewClient` → registered SDK tools → SDK Streamable HTTP handler mounted at `/mcp/sonarr` → Sonarr `/api/v3`. Deployment runs the standalone binary/container; no Python entrypoint is involved. Read-only live validation is the default; add, monitoring, and search against a real instance require explicit operator authorization.

## Security and failure behavior

Use context-bound requests, an explicit 15-second timeout, bounded response reads (8 MiB), and sanitized error text (status and API operation only). Never log the API key, request body, or upstream error body. The HTTP client refuses to follow redirects so the `X-Api-Key` header cannot be replayed to another origin. Reject arbitrary upstream URLs: `SONARR_URL` must be http(s), with no userinfo, query, or fragment, and is never overridable through a tool argument. Reject non-positive series/TVDB/profile/episode IDs, empty episode ID lists (capped at 100), unknown enum values, and negative season numbers before any upstream call. The MCP SDK protects localhost requests; standard-library cross-origin protection wraps the HTTP mux. Require ingress TLS/access control for network deployment; the container runs non-root with a read-only root filesystem where practical.

## Quality and release gates

- `gofmt -l .` on Go sources and `go test ./...` (including the SDK client/server contract test).
- `go vet ./...` and `go test -race ./...`.
- `docker build -f docker/sonarr.Dockerfile .`.
- `pipelines-hook spec-standard --root .` and `pipelines-hook spec-status --root .`.

## Risks, alternatives, and decisions

- The bootstrap duplicates the Radarr `main.go`/`http.go` shape instead of extracting shared transport helpers. Accepted: issue #2 defers shared infrastructure and keeps integrations independently buildable; extraction is a follow-up when the aggregate host is specified.
- `sonarr_list_series` and `sonarr_list_episodes` read whole collections because API v3 offers no paging for them. Accepted for this milestone with local filtering and a documented limit.
- Sonarr error bodies can carry the operator's input, so failures report status only. Accepted trade-off: duplicate detection happens through the pre-add `tvdbId` query rather than by parsing a Sonarr rejection.
- Sonarr's `series/lookup?term=tvdb:{id}` returns an array rather than a single resource; the add path takes the first entry whose `tvdbId` matches and rejects the add when none matches. Accepted: exact-ID lookup endpoints do not exist in API v3.
- The `GET /series/{id}` + `PUT /series/{id}` round-trip for series monitoring costs one extra request but guarantees no field is dropped by a partial update. Episode monitoring uses the dedicated `PUT /episode/monitor` patch endpoint and needs no fetch.
- No live mutation happens in the default suite; real add/monitor/search calls stay opt-in.

## Amendments

- **2026-10-16 (issue #8):** `sonarr_list_series`, `sonarr_lookup_series`, `sonarr_list_quality_profiles`, `sonarr_list_root_folders`, and `sonarr_list_episodes` outputs change from bare arrays to `{"series": [...]}`, `{"candidates": [...]}`, `{"profiles": [...]}`, `{"folders": [...]}`, and `{"episodes": [...]}`. The go-sdk infers `{"type": ["null", "array"]}` for top-level slices, which Python MCP SDK clients (pydantic validation, e.g. hermes-agent) reject before any client-side tool filtering, dropping the whole session. Every tool output is now a top-level object so `outputSchema.type` is `"object"`. Input schemas are unchanged.
- **2026-10-22 (Hermes v0.22.0):** The dynamic `sonarr_action.result` schema is supplied explicitly as a JSON Schema `anyOf` over JSON value types, avoiding boolean schemas that strict MCP clients cannot parse.
