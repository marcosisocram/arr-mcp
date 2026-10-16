# CONN-ARR-MCP-RADARR — Standalone Go Radarr MCP

Owner: arr-mcp. Cross-repo IDs: none.
Public provenance: [Issue #2](https://github.com/marcosisocram/arr-mcp/issues/2), the Radarr integration deliverable after the [qBittorrent pilot](../qbittorrent/spec.md).

## Purpose and user stories

As an MCP client operator, I want an independently deployable Go MCP for Radarr so that I can search, add, monitor, and trigger a release search for movies without depending on the Python Arr server.

## Requirements and acceptance

- FR-001: The standalone executable uses the official `github.com/modelcontextprotocol/go-sdk` and serves one Radarr catalog over Streamable HTTP at `/mcp/radarr`, talking to Radarr API v3.
- FR-002: `RADARR_URL` is required and is trusted configuration, never a tool argument; it may include an installation path prefix. `RADARR_API_KEY` is required and is sent as `X-Api-Key`. `RADARR_LISTEN_ADDR` defaults to `127.0.0.1:8080`.
- FR-003: Tools list the library and return one movie by internal ID using typed, trimmed fields.
- FR-004: Tools look up candidate movies by term, list quality profiles, and list root folders.
- FR-005: `radarr_add_movie` requires a TMDB ID, a quality profile ID, and a root folder path; it never starts a search and never creates a duplicate for a TMDB ID that is already in the library.
- FR-006: `radarr_set_movie_monitored` changes only the `monitored` flag of one explicit movie and preserves every other field of that movie.
- FR-007: `radarr_search_movie` posts a `MoviesSearch` command for one explicit movie ID, and `radarr_get_command` reports that command's state.
- FR-008: `radarr_get_queue` and `radarr_get_history` return bounded pages of tracked activity.
- FR-009: All outbound requests use bounded timeouts, caller cancellation, bounded response reads, default TLS verification, and never follow redirects; errors identify operation/status without including credentials, request headers, or upstream bodies.
- FR-010: Provide offline mocked tests, operator documentation, and a non-root multi-stage Docker image. Do not change or remove the Python server.
- SC-001: `go test ./...`, `go vet ./...`, and `go test -race ./...` pass and prove initialization, catalog discovery, read/add/monitor/search calls, validation, and sanitized errors without a live Radarr instance.
- SC-002: The Radarr executable builds and runs independently and is containerized with no Python package or runtime.

## Why a new spec

`specs/README.md` lists one local capability spec, `CONN-ARR-MCP` ([qBittorrent](../qbittorrent/spec.md)). It owns qBittorrent behavior only, and its non-goals explicitly exclude the remaining Arr integrations and the aggregate host. Issue #2 requires every integration to own its own client, catalog, and tests. Radarr therefore has no existing owner to extend and gets its own spec.

## Scope and interfaces

The first release covers library reads, TMDB lookup, add, monitoring, release search, command tracking, and queue/history reads. Proposed tool names: `radarr_list_movies`, `radarr_get_movie`, `radarr_lookup_movies`, `radarr_list_quality_profiles`, `radarr_list_root_folders`, `radarr_add_movie`, `radarr_set_movie_monitored`, `radarr_search_movie`, `radarr_get_command`, `radarr_get_queue`, and `radarr_get_history`.

Configuration: `RADARR_URL`, `RADARR_API_KEY`, and `RADARR_LISTEN_ADDR` (default `127.0.0.1:8080`). The server is exposed at `/mcp/radarr`. Validation target: the operator's Radarr `6.4.4.10685-ls321` through `/api/v3`; API v3 does not itself identify the application version, so other releases stay unverified until tested.

Explicit non-goals: removing movies or files, manual import, editing indexers/download clients/config, backups, generic administrative or destructive commands that are not a movie release search, alternate transports (stdio), the aggregate host, Kubernetes manifests, Compose examples, inbound authentication, and Python migration/removal. HTTP deployments must be protected by a trusted TLS/access-control proxy; route separation is not a security boundary.

## Traceability

| Requirement | Design section | Test ID | Evidence |
|---|---|---|---|
| FR-001 | [Architecture](plan.md#architecture) | T-001 | `go test ./internal/radarr -run TestMCPContract` ok at `3ad3598`. |
| FR-002 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-002 | `TestConfigValidation`, `TestNoRedirectLeaksAPIKey` ok at `3ad3598`. |
| FR-003 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-003 | `TestAPIKeyAndMovieReads`, `TestToolValidation` ok at `3ad3598`. |
| FR-004 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-004 | `TestLookupProfilesAndRootFolders` ok at `3ad3598`. |
| FR-005 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-005 | `TestAddMovieIsDuplicateSafe` ok at `3ad3598`. |
| FR-006 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-006 | `TestMutationBodiesAndErrorRedaction` ok at `3ad3598`. |
| FR-007 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-007 | `TestSearchQueueAndHistory` ok at `3ad3598`. |
| FR-008 | [Interfaces and data model](plan.md#interfaces-and-data-model) | T-008 | `TestSearchQueueAndHistory`, `TestToolValidation` ok at `3ad3598`. |
| FR-009 | [Security and failure behavior](plan.md#security-and-failure-behavior) | T-009 | `TestCancellationAndTLSDefaults`, `TestNoRedirectLeaksAPIKey` ok at `3ad3598`. |
| FR-010 | [Quality and release gates](plan.md#quality-and-release-gates) | T-010 | `docker build -f docker/radarr.Dockerfile .` ok at `3ad3598`; container non-root, healthy, SIGTERM exit 0. |

## Open questions

- Radarr has no server-side paging for `/api/v3/movie`, so `radarr_list_movies` filters and pages locally after fetching the whole library. Revisit if a library grows large enough for that read to matter.
- The complete-tool compatibility floor for Radarr versions older than the validated `6.4.4.10685-ls321` is unconfirmed; `/api/v3` is stable across Radarr v4/v5/v6, but only the operator's release has been exercised.
- Whether to add stdio transport and the aggregate host is deferred. Streamable HTTP is the required deployment transport.
