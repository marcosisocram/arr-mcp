# CONN-ARR-MCP — Standalone Go qBittorrent MCP

Owner: arr-mcp. Cross-repo IDs: none.
Public provenance: [Issue #2](https://github.com/marcosisocram/arr-mcp/issues/2), pilot for [Issue #1](https://github.com/marcosisocram/arr-mcp/issues/1).

## Purpose and user stories

As an MCP client operator, I want an independently deployable Go MCP for qBittorrent so that I can inspect torrent/transfer state and perform bounded operational controls without depending on the Python Arr server.

## Requirements and acceptance

- FR-001: The standalone executable uses the official `github.com/modelcontextprotocol/go-sdk` and serves one qBittorrent catalog over Streamable HTTP.
- FR-002: `QBITTORRENT_URL` is required and is trusted configuration, never a tool argument. `QBITTORRENT_API_KEY` is preferred; otherwise both `QBITTORRENT_USERNAME` and `QBITTORRENT_PASSWORD` are required. API key auth uses `Authorization: Bearer`; password auth uses `/auth/login`, cookie reuse, and at most one re-login/retry after 401/403.
- FR-003: Tools provide torrent listing and individual properties, with typed inputs/outputs and validation. Listed torrents are returned wrapped in an object (`{"torrents": [...]}`) so every tool's `outputSchema` declares top-level `type: "object"`, which Python MCP SDK clients require (amended 2026-10-16, issue #8). Read responses do not expose configured credentials or session cookies.
- FR-004: Tools provide global transfer information, upload/download limit reads, and alternative-speed mode read.
- FR-005: Tools pause/resume only explicitly supplied torrent hashes using qBittorrent WebAPI v2 `/torrents/stop` and `/torrents/start`; an empty hash list and the `all` sentinel are rejected.
- FR-006: Tools set upload/download limits in bytes per second (zero means unlimited) and toggle alternative-speed mode. Negative limits are rejected.
- FR-007: All outbound requests use bounded timeouts and caller cancellation; TLS verification remains enabled by default. Errors identify operation/status without including credentials, auth response bodies, or request headers.
- FR-008: Provide offline mocked tests, operator documentation, and a non-root multi-stage Docker image. Do not change or remove the Python server.
- FR-009: Provide bounded typed reads for torrent files, trackers, peers, categories, tags, app/WebAPI versions, and an allowlisted server-state snapshot. Validate torrent hashes; cap file and peer lists and report truncation.
- FR-010: Provide explicit torrent removal through `/torrents/delete`; require `confirm=true`, accept 1–100 valid hashes only, reject wildcard/empty/invalid lists, and default `delete_files` to false. Without confirmation, return cancellation without an upstream request.
- FR-011: Tracker URLs returned to callers redact URL userinfo and values of sensitive query parameters. Server-state responses expose only an explicit field allowlist.
- FR-012: Update operator docs and mocked client/MCP contract tests for the expanded catalog; every tool output remains a top-level object.
- SC-001: `go test ./...` passes and proves initialization, tool discovery, representative read/control calls, auth modes, validation, and sanitized errors without a live qBittorrent instance.
- SC-002: The qBittorrent executable can be built and run independently and is containerized with no Python package/runtime.

## Why a new spec

Read `specs/README.md` and the local templates. The index states that no local capability spec exists; the existing `.specify/specs/code-enhancement-*` documents are codebase audit work, not qBittorrent or Go MCP behavior. Issue #2 explicitly requires a pilot integration spec, so this capability has no existing owner to extend.

## Scope and interfaces

The release covers the existing nine tools plus `qbittorrent_list_torrent_files`, `qbittorrent_list_torrent_trackers`, `qbittorrent_list_torrent_peers`, `qbittorrent_list_categories`, `qbittorrent_list_tags`, `qbittorrent_app_version`, `qbittorrent_server_state`, and `qbittorrent_delete_torrents`.

Configuration: `QBITTORRENT_URL`, optional `QBITTORRENT_API_KEY`, or `QBITTORRENT_USERNAME` plus `QBITTORRENT_PASSWORD`; `QBITTORRENT_LISTEN_ADDR` defaults to `127.0.0.1:8080`. The server is exposed at `/mcp/qbittorrent`. qBittorrent API key support requires qBittorrent 5.2.0 / WebAPI 2.14.1; username/password is the compatibility path. Minimum server/API version for the complete tool set must be confirmed in integration validation.

Explicit non-goals: adding torrents, deleting files from disk by default, RSS, search, logs, full/incremental sync, per-torrent limits, recheck/reannounce, Arr server integration, aggregate host, Kubernetes manifests, OAuth/input authentication, and Python migration/removal. HTTP deployments must be protected by a trusted TLS/auth proxy; route separation is not a security boundary.

## Traceability

| Requirement | Design section | Test ID | Evidence |
|---|---|---|---|
| FR-001 | [Architecture](plan.md#architecture) | T-001 | |
| FR-002 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-002, T-003 | |
| FR-003 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-004 | |
| FR-004 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-005 | |
| FR-005 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-006 | |
| FR-006 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-007 | |
| FR-007 | [Security and failure behavior](plan.md#security-and-failure-behavior) | T-008 | |
| FR-008 | [Quality and release gates](plan.md#quality-and-release-gates) | T-009 | |
| FR-009 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-010 | |
| FR-010 | [Security and failure behavior](plan.md#security-and-failure-behavior) | T-011 | |
| FR-011 | [Security and failure behavior](plan.md#security-and-failure-behavior) | T-010, T-012 | |
| FR-012 | [Quality and release gates](plan.md#quality-and-release-gates) | T-009–T-012 | |

## Open questions

- Confirm complete-tool compatibility floor against the current qBittorrent WebAPI documentation before release; API-key authentication itself requires 5.2.0.
- Whether to include stdio transport in this first pilot is deferred; Streamable HTTP is the required deployment transport.
