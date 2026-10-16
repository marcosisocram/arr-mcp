# CONN-ARR-MCP-SONARR — Test specification

Governing spec: [spec.md](spec.md).

| Test ID | Requirement | Level | Setup and input | Expected observation | Evidence |
|---|---|---|---|---|---|
| T-001 | FR-001 | Contract | Start the handler with a mocked Sonarr upstream; connect an SDK client over Streamable HTTP. | Initialize succeeds and exactly this integration's 16 tools are listed. | |
| T-002 | FR-002 | Unit/HTTP unit | Load invalid and valid configuration; send reads through a base URL with a path prefix. | Invalid URL/auth rejected; `X-Api-Key` sent; prefix preserved; a redirect is not followed and the key is not forwarded. | |
| T-003 | FR-003 | HTTP/tool contract | Mock `GET /series` and `GET /series/{id}`; invoke both tools and the filter. | Typed trimmed fields plus per-season data decoded; monitored filter and local paging applied; invalid offset/limit rejected. | |
| T-004 | FR-004 | HTTP/tool contract | Mock lookup, quality profile, and root folder endpoints. | Lookup term encoded, results bounded, and each typed response returned. | |
| T-005 | FR-005 | HTTP/tool contract | Mock `GET /episode` with series and season scoping plus `GET /episode/{id}`. | Episodes scoped to the explicit series, season filter applied locally, and each typed response returned; missing series ID rejected. | |
| T-006 | FR-006 | HTTP/tool contract | Add an absent TVDB ID, then add it again. | First call POSTs lookup metadata plus explicit profile/folder and no search flags; second call returns `already_exists` with no POST; unknown `monitor`/`series_type` and missing identifiers rejected. | |
| T-007 | FR-007 | HTTP/tool contract | Mock `GET /series/{id}` and `PUT /series/{id}`; flip series monitoring. Mock `PUT /episode/monitor`; flip episode monitoring. | Series update uses `moveFiles=false`; every other fetched field is preserved; only `monitored` changes. Episode patch sends exactly the explicit IDs and the flag. | |
| T-008 | FR-008 | HTTP/tool contract | Mock `POST /command` and `GET /command/{id}`. | `SeriesSearch` with exactly the explicit series ID; `SeasonSearch` with series ID and season number; `EpisodeSearch` with exactly the explicit episode IDs; tracked state decoded; non-positive IDs rejected. | |
| T-009 | FR-009 | HTTP/tool contract | Mock `GET /queue` and `GET /history`. | Server-side paging passed through; records decoded; page/pageSize bounds enforced. | |
| T-010 | FR-010 | Unit | Mock timeout, cancellation, redirect, and an upstream 500 whose body contains a secret. | Cancellation honored; TLS defaults retained; redirect not followed; error contains operation/status only. | |
| T-011 | FR-011 | Build/package | Run the Go gates and build the Docker image. | Go checks pass; the container builds and runs non-root without Python. | |

## Negative and boundary cases

Cover malformed or userinfo-bearing upstream URLs, a missing API key, oversized and negative paging, unknown `monitor` and `series_type` values, non-positive series/TVDB/quality-profile/episode/command IDs, negative season numbers, an empty or oversized episode ID list, an empty root folder path, duplicate add, a lookup with no matching TVDB entry, redirected requests, canceled requests, oversized responses, and upstream errors that contain credentials. No live Sonarr instance is required for the default suite.

## Quality and release proof

Run `gofmt -l .`, `go test ./...`, `go vet ./...`, `go test -race ./...`, `docker build -f docker/sonarr.Dockerfile .`, and the spec-standard/status checks. Record command outcomes before marking delivery complete. Live Sonarr validation is read-only by default; add, monitoring, and search against a real instance require explicit operator authorization.

Recorded on 2026-10-16 (container host, Go 1.26.2):

- `gofmt -l cmd internal` — no output.
- `go vet ./...` — clean.
- `go test ./...` — `internal/sonarr` ok; `internal/radarr` ok; `internal/qbittorrent` ok.
- `go test -race ./...` — ok.
- `docker build -f docker/sonarr.Dockerfile .` — ok; the built image runs `User=nonroot:nonroot`, reports `Health` via `/sonarr-mcp --healthcheck`, serves `/health` as `{"status":"ok"}`, and exits 0 on `SIGTERM` with a read-only root filesystem and all capabilities dropped.
- `pipelines-hook` is not installed in this environment, so the spec-standard/status checks could not be run; the spec follows the `_template/` file set and the `CONN-ARR-MCP-SONARR` ID form.
- No live Sonarr instance was exercised in this revision; the operator's `4.0.20.3014-ls329` instance remains to be validated read-only.
