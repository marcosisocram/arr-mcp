# CONN-ARR-MCP-BAZARR — Standalone Go Bazarr MCP

Owner: arr-mcp. Cross-repo IDs: none.
Public provenance: [Issue #2](https://github.com/marcosisocram/arr-mcp/issues/2), the Bazarr integration deliverable.

## Purpose and user stories

As an MCP client operator, I want an independently deployable Go MCP for Bazarr so that I can inspect subtitle coverage and history and explicitly request subtitle searches/downloads without depending on the Python Arr server.

## Requirements and acceptance

- FR-001: The standalone executable uses the official `github.com/modelcontextprotocol/go-sdk` and serves one Bazarr catalog over Streamable HTTP at `/mcp/bazarr`, talking to the Bazarr REST API rooted at `/api`.
- FR-002: `BAZARR_URL` is required trusted configuration and may include an installation prefix; `BAZARR_API_KEY` is required and sent as `X-API-KEY`; `BAZARR_LISTEN_ADDR` defaults to `127.0.0.1:8080`.
- FR-003: Read tools expose trimmed system status, available languages, and language profiles; system status must not expose local paths, database version details, or other unneeded host metadata.
- FR-004: Read tools list series and movies with bounded server-side paging and return a requested series/movie by explicit Sonarr/Radarr ID.
- FR-005: Read tools list episodes for an explicit series, and return bounded wanted episodes and wanted movies.
- FR-006: Read tools list providers and bounded series/movie history.
- FR-007: Profile tools set the language profile of one explicit series or movie; only IDs and the explicit profile ID (or `none`) are accepted.
- FR-008: Explicit subtitle-search tools may trigger downloads: series/movie mass search for one explicit item and single-episode subtitle download for one explicit episode/language/flags. Tool descriptions identify this side effect. No implicit search is performed by read or profile tools.
- FR-009: Every MCP tool has a top-level object `outputSchema` (Python MCP SDK compatibility, issue #8); tests assert this over Streamable HTTP.
- FR-010: Outbound requests use bounded timeouts, caller cancellation, bounded response reads, default TLS verification, and never follow redirects; errors report operation/status only without credentials or upstream bodies.
- FR-011: Provide offline mocked tests, operator documentation, and a non-root multi-stage Docker image with numeric UID 65532 and healthcheck. Do not change or remove the Python server.
- SC-001: `go test ./...`, `go vet ./...`, and `go test -race ./...` pass and prove initialization, all 18 tools, object output schemas, representative read/write calls, validation, and sanitized errors without live Bazarr.
- SC-002: The Bazarr executable builds and runs independently and is containerized with no Python runtime.
- FR-012: `bazarr_action` exposes every public Bazarr domain action implemented by the Python API client, except its HTTP helper. `action="list_actions"` discovers the catalog; existing typed tools remain available. Credential-like fields in routed results are redacted.
- FR-013: Mutating or destructive actions invoked through `bazarr_action` require `confirm=true`; without it, the tool returns a cancellation result and makes no upstream request. All requests retain FR-010 security bounds.
- FR-014: A parity test derives the expected catalog from the Python client and fails on additions, removals, or renames not reflected in Go. The generator rejects unsupported method bodies rather than silently omitting actions.
- FR-015: The `bazarr_action` output schema represents its dynamic `result` value with a valid JSON Schema object, not a boolean schema, so strict MCP clients can parse `tools/list`.

## Why a new spec

`specs/README.md` lists the qBittorrent, Radarr, and Sonarr Go integrations. None owns subtitle management or Bazarr API behavior. Issue #2 requires each integration to own its client, catalog, and tests; Bazarr therefore gets a separate spec. Existing integrations are structural references only.

## Scope and interfaces

Target: Bazarr `v1.6.2`, validated through its `/api` endpoints. The 17 guided tools are `bazarr_get_status`, `bazarr_list_languages`, `bazarr_list_language_profiles`, `bazarr_list_series`, `bazarr_get_series`, `bazarr_list_episodes`, `bazarr_list_wanted_episodes`, `bazarr_list_movies`, `bazarr_get_movie`, `bazarr_list_wanted_movies`, `bazarr_list_providers`, `bazarr_get_history`, `bazarr_set_series_profile`, `bazarr_set_movie_profile`, `bazarr_search_series_subtitles`, `bazarr_search_movie_subtitles`, and `bazarr_download_episode_subtitle`. `bazarr_action` additionally routes all 25 Python API-client domain actions; `action="list_actions"` discovers the catalog.

Configuration: `BAZARR_URL`, `BAZARR_API_KEY`, `BAZARR_LISTEN_ADDR` (default `127.0.0.1:8080`); MCP route `/mcp/bazarr`. Bazarr uses `X-API-KEY`, not the Arr `X-Api-Key` header. Its paginated APIs return `data` and `total`; MCP list results retain this envelope. The status tool returns only Bazarr/package and connected Sonarr/Radarr version fields, omitting filesystem paths, DB versions, Python/runtime details, and host data.

The two mass subtitle searches and subtitle downloads remain explicit domain operations. All public Python-client domain actions, including subtitle upload/delete/sync/translation/modification, provider reset, configuration, backup, webhooks, and administration, are in scope; mutations require explicit `confirm=true`. HTTP deployments require a trusted TLS/access-control proxy; there is no inbound authentication.

## Traceability

| Requirement | Design section | Test ID | Evidence |
|---|---|---|---|
| FR-001 | [Architecture](plan.md#architecture) | T-001 | `TestMCPContract` initializes and lists 18 tools. |
| FR-002 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-002 | `TestConfigValidation`, `TestAPIKeyPrefixAndStatusRedaction`, `TestCancellationTLSRedirectAndRedaction`. |
| FR-003 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-003 | `TestAPIKeyPrefixAndStatusRedaction`, `TestReadEndpointsAndPaging`. |
| FR-004 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-004 | `TestReadEndpointsAndPaging`. |
| FR-005 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-005 | `TestReadEndpointsAndPaging`, `TestMCPContract`. |
| FR-006 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-006 | `TestReadEndpointsAndPaging`. |
| FR-007 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-007 | `TestWriteEndpoints`, `TestMCPContract`. |
| FR-008 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-008 | `TestWriteEndpoints`, `TestMCPContract`. |
| FR-009 | [Security and failure behavior](plan.md#security-and-failure-behavior) | T-009 | `TestMCPContract` asserts top-level object schema on every tool. |
| FR-010 | [Security and failure behavior](plan.md#security-and-failure-behavior) | T-010 | `TestCancellationTLSRedirectAndRedaction`. |
| FR-011 | [Quality and release gates](plan.md#quality-and-release-gates) | T-011 | Go gates green; image built as numeric UID 65532; health endpoint and SIGTERM validated. |
| FR-012 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-012 | `internal/bazarr/actions.json` has 25 Python-derived actions; `TestMCPContract` discovers them through `bazarr_action`. |
| FR-013 | [Security and failure behavior](plan.md#security-and-failure-behavior) | T-013 | `internal/actiondispatch/dispatch_test.go`, `TestMCPContract` prove unconfirmed mutations send no request. |
| FR-014 | [Quality and release gates](plan.md#quality-and-release-gates) | T-014 | `tests/test_go_action_parity.py` verifies generated catalog against Python source. |
| FR-015 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-015 | |

## Amendments

- **2026-10-22:** Extend the Go catalog to all 25 domain actions in the Python client behind `bazarr_action`; keep the guided tools. Every POST/PUT/PATCH/DELETE requires `confirm=true`. Exclude the HTTP helper and enforce catalog parity from the Python AST.

## Open questions

- Bazarr versions before `v1.6.2` have not been verified; `/api` is versioned by the Bazarr release rather than a formal API version.
- Bazarr returns language profiles with a nested `items` shape; tests will capture and assert the v1.6.2 response contract without exposing mutable profile internals as tool arguments.
