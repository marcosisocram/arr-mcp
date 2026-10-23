# CONN-ARR-MCP-BAZARR — Test specification

Governing spec: [spec.md](spec.md).

| Test ID | Requirement | Level | Setup and input | Expected observation | Evidence |
|---|---|---|---|---|---|
| T-001 | FR-001 | Contract | SDK client connects to mocked handler over Streamable HTTP. | Initialization succeeds and exactly 17 Bazarr tools are listed. | |
| T-002 | FR-002 | Unit/HTTP | Valid/invalid config; mock requests through prefixed URL and redirect. | URL validation, `X-API-KEY`, prefix preservation; redirects refused without forwarding key. | |
| T-003 | FR-003 | HTTP/tool | Mock status, languages, and profiles. | Typed outputs returned; status excludes sensitive host/database fields. | |
| T-004 | FR-004 | HTTP/tool | Mock series and movie collection endpoints. | Server paging and explicit ID filtering are encoded; Bazarr `data`/`total` envelope preserved. | |
| T-005 | FR-005 | HTTP/tool | Mock episodes and wanted endpoints. | Episode series ID required; bounds and wanted envelopes decode correctly. | |
| T-006 | FR-006 | HTTP/tool | Mock providers and both history endpoints. | Provider history flag and bounded paging passed through; data decoded. | |
| T-007 | FR-007 | HTTP/tool | Set one series/movie profile. | Correct POST fields; `none` allowed; invalid/non-positive IDs rejected before network. | |
| T-008 | FR-008 | HTTP/tool | Invoke specific series/movie search and episode subtitle download. | Correct PATCH route/query; target IDs, language, flags passed; invalid input rejected; descriptions disclose download side effects. | |
| T-009 | FR-009 | Contract | List tools through SDK transport. | Every tool has `outputSchema.type == "object"`. | |
| T-010 | FR-010 | Unit/HTTP | Cancellation, oversized response, redirect and upstream error containing secret. | Bounds honored and error excludes secret/body. | |
| T-011 | FR-011 | Package | Run Go gates and build/run container. | Non-root numeric UID, health endpoint healthy, SIGTERM exits cleanly. | |
| T-015 | FR-015 | MCP contract | Read the `bazarr_action` output schema from `tools/list`. | `properties.result` is a JSON object/schema, not boolean `true`; ordinary and dynamic results remain valid. | `TestMCPContract`, `go test ./...`, `go test -race ./...`. |

## Negative and boundary cases

Test missing/malformed URL, URL userinfo/query/fragment, missing API key, negative and excessive paging, invalid page size, zero/negative series/movie/episode IDs, empty or overlong language codes, invalid boolean flags, redirect, cancellation, oversized response, and secret-bearing upstream error. No live Bazarr instance is required.

## Quality and release proof

Run `gofmt -l cmd internal`, `go test ./...`, `go vet ./...`, `go test -race ./...`, `docker build -f docker/bazarr.Dockerfile .`, and spec checks. Record test/build/probe outcomes. Do not perform live mutating validation without operator authorization.

Recorded on 2026-10-17 (Go 1.26.2):

- `gofmt -l cmd internal` — no output.
- `go vet ./...` — clean.
- `go test ./...` — all packages pass, including Bazarr's 17-tool Streamable HTTP contract.
- `go test -race ./...` — all packages pass.
- `docker build -f docker/bazarr.Dockerfile -t marcosisocram/bazarr-mcp:latest .` — built successfully.
- Container smoke: read-only filesystem, all capabilities dropped, `User=65532:65532`, `/health` returned `{"status":"ok"}`, `--healthcheck` succeeded, SIGTERM exit code 0.
- `pipelines-hook` unavailable in this environment; spec-standard/status commands could not be run.
- No live Bazarr instance was exercised; Bazarr v1.6.2 API behavior was modeled from upstream source and verified with offline HTTP mocks.

## Amendments

- **2026-10-22 (Hermes v0.22.0 compatibility):** `bazarr_action` now publishes an object-form permissive schema for its dynamic `result`; T-015 checks the wire schema.
