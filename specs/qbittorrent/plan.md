# CONN-ARR-MCP — Design and implementation plan

Governing spec: [spec.md](spec.md).

Component: qBittorrent MCP
Code: ../../cmd/qbittorrent-mcp/main.go ../../internal/qbittorrent/http.go ../../internal/qbittorrent/client.go ../../internal/qbittorrent/tools.go ../../internal/qbittorrent/client_test.go ../../internal/qbittorrent/mcp_test.go

## Existing system and reuse

The repository currently contains a Python Arr MCP and no Go module or qBittorrent implementation. The Python implementation remains untouched. Use the official Go MCP SDK, Go standard library HTTP client, and qBittorrent WebAPI v2. The reference implementation at `arr-mcps/qbittorrent-mcp` documents API-key Bearer authentication and qBittorrent 5.0 endpoint names; do not port its runtime or framework.

## Architecture

`cmd/qbittorrent-mcp` loads validated environment configuration, constructs the qBittorrent API client, registers typed tools on one official SDK server, and mounts Streamable HTTP at `/mcp/qbittorrent`. `internal/qbittorrent` owns API types, auth/session handling, endpoint methods, and tests. The MCP layer calls that client; no reverse dependency or shared Arr runtime is introduced. The HTTP server has graceful shutdown and a bounded read/header/idle timeout.

## Interfaces and data model

Use SDK typed tool input/output structs. List filters use an allowlist of qBittorrent values and bounded pagination; hashes accept valid 40- or 64-character hexadecimal identifiers and are submitted explicitly as a pipe-separated list. File and peer results have hard limits and truncation indicators. Tracker URL credentials/query secrets are redacted, and server-state fields are allowlisted. Torrent removal requires `confirm=true`, validates 1–100 hashes, and defaults to preserving downloaded files. A separate typed torrent-properties response and transfer-info response represent useful fields without exposing arbitrary HTTP responses. Limit values are `int64` bytes/second, constrained to >= 0. qBittorrent 5.0 uses `/torrents/stop` and `/torrents/start`; limit reads/writes use `/transfer/downloadLimit`, `/transfer/uploadLimit`, `/transfer/setDownloadLimit`, `/transfer/setUploadLimit`; alt-speed mode uses `/transfer/speedLimitsMode` and `/transfer/toggleSpeedLimitsMode`.

API-key requests carry `Authorization: Bearer <key>`. Password login posts credentials to `/auth/login` with the configured base URL as Referer; the HTTP cookie jar retains SID. On 401/403, password auth logs in once and retries once; API-key requests are never retried as login. No automatic retries for mutations except this single auth refresh before replay, and mutation replay only occurs when the server rejected the unauthenticated request. URL must be http(s), have no userinfo, and is not overrideable through tool inputs. TLS verification is on by default.

## Live integration path

Entrypoint: `cmd/qbittorrent-mcp/main.go` → config → `internal/qbittorrent.Client` → registered SDK tools → SDK Streamable HTTP handler mounted at `/mcp/qbittorrent` → qBittorrent `/api/v2`. Deployment can run the standalone binary/container; no Python entrypoint is involved.

## Security and failure behavior

Use context-bound requests, explicit timeout, bounded response reads, and sanitized error text (status and API operation only). Never log credentials, cookies, request body, or upstream error body. Do not accept arbitrary upstream URLs, arbitrary API paths, wildcard `all`, or unbounded hash lists from tool input. The MCP SDK protects localhost requests; standard-library cross-origin protection wraps the HTTP mux. Require ingress TLS/access control for network deployment; container runs non-root with read-only root filesystem where practical.

## Quality and release gates

- `gofmt -w` on Go sources and `go test ./...` (including SDK client/server contract tests).
- `go vet ./...` and `go test -race ./...`.
- `docker build -f docker/qbittorrent.Dockerfile .`.
- `pipelines-hook spec-standard --root .` and `pipelines-hook spec-status --root .`.

## Risks, alternatives, and decisions

Bearer API key is only supported by qBittorrent >=5.2.0; username/password remains supported for older versions. Hash validation should accommodate valid v1/v2 torrent identifiers according to WebAPI documentation, rather than assuming only 40 hex digits if v2 hashes differ. The tools deliberately omit a wildcard pause/resume to prevent accidental global state changes. No aggregate server is implemented in this pilot.

## Amendments

- **2026-10-16 (issue #8):** `qbittorrent_list_torrents` output changes from a bare array to `{"torrents": [...]}`. The go-sdk infers `{"type": ["null", "array"]}` for top-level slices, which Python MCP SDK clients (pydantic validation, e.g. hermes-agent) reject before any client-side tool filtering, dropping the whole session. Every tool output is now a top-level object so `outputSchema.type` is `"object"`. Input schemas are unchanged.
- **2026-10-22:** Extend the catalog with bounded file/peer reads, tracker redaction, category/tag/version/server-state reads, and confirmation-gated torrent removal.
