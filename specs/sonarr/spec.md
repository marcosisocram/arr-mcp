# CONN-ARR-MCP-SONARR — Standalone Go Sonarr MCP

Owner: arr-mcp. Cross-repo IDs: none.
Public provenance: [Issue #2](https://github.com/marcosisocram/arr-mcp/issues/2), the Sonarr integration deliverable after the qBittorrent pilot and the [Radarr](../radarr/spec.md) integration.

## Purpose and user stories

As an MCP client operator, I want an independently deployable Go MCP for Sonarr so that I can search, add, monitor, and trigger release searches for TV series without depending on the Python Arr server.

## Requirements and acceptance

- FR-001: The standalone executable uses the official `github.com/modelcontextprotocol/go-sdk` and serves one Sonarr catalog over Streamable HTTP at `/mcp/sonarr`, talking to Sonarr API v3.
- FR-002: `SONARR_URL` is required and is trusted configuration, never a tool argument; it may include an installation path prefix. `SONARR_API_KEY` is required and is sent as `X-Api-Key`. `SONARR_LISTEN_ADDR` defaults to `127.0.0.1:8080`.
- FR-003: Tools list the library and return one series by internal ID using typed, trimmed fields, including per-season numbers and monitoring flags. List results are wrapped in an object (`{"series": ...}`, `{"candidates": ...}`, `{"profiles": ...}`, `{"folders": ...}`) so every tool's `outputSchema` declares top-level `type: "object"`, which Python MCP SDK clients require (amended 2026-10-16, issue #8).
- FR-004: Tools look up candidate series by term, list quality profiles, and list root folders.
- FR-005: Tools list episodes of one explicit series with optional season scoping, and return one episode by internal ID. Episode list results are wrapped in an object (`{"episodes": ...}`, amended 2026-10-16, issue #8).
- FR-006: `sonarr_add_series` requires a TVDB ID, a quality profile ID, and a root folder path; it never starts a search and never creates a duplicate for a TVDB ID that is already in the library.
- FR-007: `sonarr_set_series_monitored` changes only the `monitored` flag of one explicit series and preserves every other field of that series. `sonarr_set_episode_monitored` changes only the `monitored` flag of explicit episodes.
- FR-008: `sonarr_search_series`, `sonarr_search_season`, and `sonarr_search_episodes` post `SeriesSearch`, `SeasonSearch`, and `EpisodeSearch` commands for explicit targets, and `sonarr_get_command` reports a command's state.
- FR-009: `sonarr_get_queue` and `sonarr_get_history` return bounded pages of tracked activity.
- FR-010: All outbound requests use bounded timeouts, caller cancellation, bounded response reads, default TLS verification, and never follow redirects; errors identify operation/status without including credentials, request headers, or upstream bodies.
- FR-011: Provide offline mocked tests, operator documentation, and a non-root multi-stage Docker image. Do not change or remove the Python server.
- SC-001: `go test ./...`, `go vet ./...`, and `go test -race ./...` pass and prove initialization, catalog discovery, read/add/monitor/search calls, validation, and sanitized errors without a live Sonarr instance.
- SC-002: The Sonarr executable builds and runs independently and is containerized with no Python package or runtime.

## Why a new spec

`specs/README.md` lists two local capability specs, `CONN-ARR-MCP` ([qBittorrent](../qbittorrent/spec.md)) and `CONN-ARR-MCP-RADARR` ([Radarr](../radarr/spec.md)). The first owns qBittorrent behavior only and the second owns Radarr movies only; neither owns Sonarr series, seasons, or episodes. Issue #2 requires every integration to own its own client, catalog, and tests. Sonarr therefore has no existing owner to extend and gets its own spec. The Radarr integration is reused as a structural pattern only (one package, one executable, bounded HTTP, sanitized errors, offline mocked tests); no Radarr types are reused because the resource models differ.

## Scope and interfaces

The first release covers library reads, TVDB lookup, add, series/episode monitoring, series/season/episode release search, command tracking, and queue/history reads. Proposed tool names: `sonarr_list_series`, `sonarr_get_series`, `sonarr_lookup_series`, `sonarr_list_quality_profiles`, `sonarr_list_root_folders`, `sonarr_list_episodes`, `sonarr_get_episode`, `sonarr_add_series`, `sonarr_set_series_monitored`, `sonarr_set_episode_monitored`, `sonarr_search_series`, `sonarr_search_season`, `sonarr_search_episodes`, `sonarr_get_command`, `sonarr_get_queue`, and `sonarr_get_history`.

Configuration: `SONARR_URL`, `SONARR_API_KEY`, and `SONARR_LISTEN_ADDR` (default `127.0.0.1:8080`). The server is exposed at `/mcp/sonarr`. Validation target: the operator's Sonarr `4.0.20.3014-ls329` (build 2026-10-15) through `/api/v3`; API v3 does not itself identify the application version, so other releases stay unverified until tested.

Explicit non-goals: removing series or files, manual import, editing indexers/download clients/config, backups, per-season monitoring control, generic administrative or destructive commands that are not a series/season/episode release search, alternate transports (stdio), the aggregate host, Kubernetes manifests, Compose examples, inbound authentication, and Python migration/removal. HTTP deployments must be protected by a trusted TLS/access-control proxy; route separation is not a security boundary.

## Traceability

| Requirement | Design section | Test ID | Evidence |
|---|---|---|---|
| FR-001 | [Architecture](plan.md#architecture) | T-001 | `go test ./internal/sonarr -run TestMCPContract` ok. |
| FR-002 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-002 | `TestConfigValidation`, `TestNoRedirectLeaksAPIKey` ok. |
| FR-003 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-003 | `TestAPIKeyAndSeriesReads`, `TestToolValidation` ok. |
| FR-004 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-004 | `TestLookupProfilesAndRootFolders` ok. |
| FR-005 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-005 | `TestEpisodeReads` ok. |
| FR-006 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-006 | `TestAddSeriesIsDuplicateSafe` ok. |
| FR-007 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-007 | `TestMutationBodiesAndErrorRedaction`, `TestEpisodeMonitorBody` ok. |
| FR-008 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-008 | `TestSearchQueueAndHistory` ok. |
| FR-009 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-009 | `TestSearchQueueAndHistory`, `TestToolValidation` ok. |
| FR-010 | [Security and failure behavior](plan.md#security-and-failure-behavior) | T-010 | `TestCancellationAndTLSDefaults`, `TestNoRedirectLeaksAPIKey` ok. |
| FR-011 | [Quality and release gates](plan.md#quality-and-release-gates) | T-011 | `docker build -f docker/sonarr.Dockerfile .` ok; container non-root, healthy, SIGTERM exit 0. |

## Open questions

- Sonarr has no server-side paging for `/api/v3/series` or for `/api/v3/episode`, so `sonarr_list_series` and `sonarr_list_episodes` filter and page locally after fetching. Revisit if a library grows large enough for those reads to matter.
- The complete-tool compatibility floor for Sonarr versions older than the validated `4.0.20.3014-ls329` is unconfirmed; `/api/v3` is stable across Sonarr v3/v4, but only the operator's release is exercised.
- Whether per-season monitoring control deserves its own tool is deferred; season monitoring is readable today through `sonarr_get_series` and writable only as part of a full series update outside this catalog.
- Whether to add stdio transport and the aggregate host is deferred. Streamable HTTP is the required deployment transport.
