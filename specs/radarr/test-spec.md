# CONN-ARR-MCP-RADARR — Test specification

Governing spec: [spec.md](spec.md).

| Test ID | Requirement | Level | Setup and input | Expected observation | Evidence |
|---|---|---|---|---|---|
| T-001 | FR-001 | Contract | Start the handler with a mocked Radarr upstream; connect an SDK client over Streamable HTTP. | Initialize succeeds and exactly this integration's 11 tools are listed. | |
| T-002 | FR-002 | Unit/HTTP unit | Load invalid and valid configuration; send reads through a base URL with a path prefix. | Invalid URL/auth rejected; `X-Api-Key` sent; prefix preserved; a redirect is not followed and the key is not forwarded. | |
| T-003 | FR-003 | HTTP/tool contract | Mock `GET /movie` and `GET /movie/{id}`; invoke both tools and both filters. | Typed trimmed fields decoded; monitored/has-file filters and local paging applied; invalid offset/limit rejected. | |
| T-004 | FR-004 | HTTP/tool contract | Mock lookup, quality profile, and root folder endpoints. | Lookup term encoded, results bounded, and each typed response returned. | |
| T-005 | FR-005 | HTTP/tool contract | Add an absent TMDB ID, then add it again. | First call POSTs metadata plus explicit profile/folder and `searchForMovie=false`; second call returns `already_exists` with no POST; unknown `monitor`/`minimum_availability` and missing identifiers rejected. | |
| T-006 | FR-006 | HTTP/tool contract | Mock `GET /movie/{id}` and `PUT /movie/{id}`; flip monitoring. | `moveFiles=false`; every other fetched field is preserved; only `monitored` changes. | |
| T-007 | FR-007 | HTTP/tool contract | Mock `POST /command` and `GET /command/{id}`. | `MoviesSearch` with exactly the explicit movie ID; tracked state decoded; non-positive ID rejected. | |
| T-008 | FR-008 | HTTP/tool contract | Mock `GET /queue` and `GET /history`. | Server-side paging passed through; records decoded; page/pageSize bounds enforced. | |
| T-009 | FR-009 | Unit | Mock timeout, cancellation, redirect, and an upstream 500 whose body contains a secret. | Cancellation honored; TLS defaults retained; redirect not followed; error contains operation/status only. | |
| T-010 | FR-010 | Build/package | Run the Go gates and build the Docker image. | Go checks pass; the container builds and runs non-root without Python. | `docker build` ok at `3ad3598`; inspected `User=nonroot:nonroot`, `Health=healthy`, `SIGTERM` exit 0. |

## Negative and boundary cases

Cover malformed or userinfo-bearing upstream URLs, a missing API key, oversized and negative paging, unknown `minimum_availability` and `monitor` values, non-positive movie/TMDB/quality-profile IDs, an empty root folder path, duplicate add, redirected requests, canceled requests, oversized responses, and upstream errors that contain credentials. No live Radarr instance is required for the default suite.

## Quality and release proof

Run `gofmt -l .`, `go test ./...`, `go vet ./...`, `go test -race ./...`, `docker build -f docker/radarr.Dockerfile .`, and the spec-standard/status checks. Record command outcomes before marking delivery complete. Live Radarr validation is read-only by default; add, monitoring, and search against a real instance require explicit operator authorization.

Recorded at revision `3ad3598` on 2026-10-16 (container host, Go 1.26.2):

- `gofmt -l cmd internal` — no output.
- `go vet ./...` — clean.
- `go test ./...` — `internal/radarr` ok; `internal/qbittorrent` ok.
- `go test -race ./...` — ok.
- `docker build -f docker/radarr.Dockerfile .` — ok; the built image runs `User=nonroot:nonroot`, reports `Health=healthy`, and exits 0 on `SIGTERM` with a read-only root filesystem and all capabilities dropped.
- `pipelines-hook spec-standard --root .` — clean (2 specs); `pipelines-hook spec-status --root .` — up to date.
- No live Radarr instance was exercised in this revision; `RADARR_URL` for the operator's `6.4.4.10685-ls321` instance remains to be validated read-only.

## Amendments

- **2026-10-16 (issue #8):** `TestMCPContract` now asserts every tool's `outputSchema.type` is `"object"` over the wire, and the four list/lookup tools return `{"movies"}`, `{"candidates"}`, `{"profiles"}`, and `{"folders"}` wrappers. All Go gates re-run green; images rebuilt and pushed to Docker Hub as `latest`.
