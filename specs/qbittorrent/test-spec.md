# CONN-ARR-MCP — Test specification

Governing spec: [spec.md](spec.md).

| Test ID | Requirement | Level | Setup and input | Expected observation | Evidence |
|---|---|---|---|---|---|
| T-001 | FR-001 | Contract | Start server with mocked upstream; connect SDK client over Streamable HTTP. | Initialize succeeds; only this integration's tools are listed. | |
| T-002 | FR-002 | Unit | Validate missing URL/auth, API key precedence, and incomplete password pair. | Invalid config rejected; API key wins when both methods supplied. | |
| T-003 | FR-002 | HTTP unit | Mock auth login, cookie response, and expired session. | Bearer header is correct; password login retains cookie; one re-login occurs on auth expiry; secrets absent from errors. | |
| T-004 | FR-003 | HTTP/tool contract | Mock list and properties endpoints; invoke typed tools. | Filters and hash encoded correctly; typed data returned; invalid filters/hashes rejected. | |
| T-005 | FR-004 | HTTP/tool contract | Mock transfer info, limit, and speed mode endpoints. | Rates/limits/mode decoded and returned with units documented. | |
| T-006 | FR-005 | HTTP/tool contract | Invoke pause/resume with one or multiple valid hashes, empty set and `all`. | Correct stop/start POST form; invalid/empty/wildcard selections rejected. | |
| T-007 | FR-006 | HTTP/tool contract | Set limits at zero and positive values, then negative values; toggle alt speed. | Correct API form values; negative limits rejected; toggle endpoint called. | |
| T-008 | FR-007 | Unit | Mock timeout, cancellation, TLS client, and upstream 401/500 with secret-bearing body. | Cancellation honored; TLS verification not disabled; safe operation/status error contains no secret or body. | |
| T-009 | FR-008 | Build/package | Run Go gates and build Docker image. | Go checks pass; container builds and runs non-root without Python. | |
| T-010 | FR-009, FR-011 | HTTP/tool contract | Mock files, trackers, peers, categories, tags, version, and sync endpoints including oversized lists and sensitive tracker/server-state data. | Typed wrappers decode correctly; limits/truncation apply; tracker secrets are redacted; only allowlisted state fields are returned. | |
| T-011 | FR-010 | HTTP/tool contract | Invoke deletion with missing/false/true confirmation, invalid hashes, and `delete_files` omitted/true. | Unconfirmed calls are cancelled without upstream requests; confirmed calls send only validated hashes and the correct `deleteFiles` value. | |
| T-012 | FR-012 | Contract | List and invoke the expanded MCP catalog. | Exactly 17 tools are discoverable and every output schema is a top-level object. | |

## Negative and boundary cases

Cover malformed upstream URL, invalid auth combinations, unsafe hash/filter values, empty/oversized hash arrays, wildcard `all`, negative limits, list truncation, oversized responses, tracker userinfo/query secrets, non-allowlisted server state, timeout/cancellation, login denial, repeated unauthorized response, and upstream errors containing credentials. No live instance is required for the default suite.

## Quality and release proof

Run `go test ./...`, `go vet ./...`, `go test -race ./...`, `docker build -f docker/qbittorrent.Dockerfile .`, and the spec-standard/status checks. Record command outcomes before marking delivery complete. Live qBittorrent validation is optional and read-only; the implementation's mocked tests cover controls without mutating a real service.

## Amendments

- **2026-10-16 (issue #8):** `TestMCPContract` now asserts every tool's `outputSchema.type` is `"object"` over the wire, and the list tool returns `{"torrents": [...]}`. All Go gates re-run green; images rebuilt and pushed to Docker Hub as `latest`.
- **2026-10-22:** Added T-010–T-012 for the expanded read catalog, secret redaction, and confirmed deletion. `go test ./...`, `go vet ./...`, `go test -race ./...`, gofmt, and the qBittorrent Docker build pass. `pipelines-hook` is unavailable in this environment; no live instance or registry push was exercised.
