# CONN-ARR-MCP-RADARR — Requirements

Governing spec: [spec.md](spec.md).

| ID | Requirement | Acceptance |
|---|---|---|
| CONN-ARR-MCP-RADARR-R001 | **Standalone server.** `cmd/radarr-mcp/main.go` serves an independent official-SDK MCP at `/mcp/radarr` over Streamable HTTP against Radarr API v3. | `TestMCPContract` initializes the real entrypoint handler and lists its catalog. |
| CONN-ARR-MCP-RADARR-R002 | **Trusted configuration and authentication.** `internal/radarr/config.go` and `client.go` load `RADARR_URL` (trusted, never a tool argument), require `RADARR_API_KEY`, default `RADARR_LISTEN_ADDR`, send the key as `X-Api-Key`, and never follow redirects. | `TestConfigValidation`, `TestAPIKeyAndMovieReads`, and `TestNoRedirectLeaksAPIKey` pass without exposing secrets. |
| CONN-ARR-MCP-RADARR-R003 | **Read the library.** `internal/radarr/client.go` and `tools.go` expose typed `radarr_list_movies` and `radarr_get_movie` reads. | `TestAPIKeyAndMovieReads` and `TestToolValidation` verify decoded data, filters, and rejection cases. |
| CONN-ARR-MCP-RADARR-R004 | **Discover prerequisites.** `internal/radarr/tools.go` exposes `radarr_lookup_movies`, `radarr_list_quality_profiles`, and `radarr_list_root_folders`. | `TestLookupProfilesAndRootFolders` verifies each typed response. |
| CONN-ARR-MCP-RADARR-R005 | **Add a movie safely.** `internal/radarr/tools.go` adds one movie from an explicit TMDB ID, quality profile, and root folder, without a release search and without duplicating an existing TMDB ID. | `TestAddMovieIsDuplicateSafe` asserts the POST body and the duplicate short-circuit. |
| CONN-ARR-MCP-RADARR-R006 | **Control monitoring.** `internal/radarr/tools.go` changes only the `monitored` flag of one explicit movie. | `TestMutationBodiesAndErrorRedaction` asserts the preserved fields and the `moveFiles=false` update. |
| CONN-ARR-MCP-RADARR-R007 | **Search and track.** `internal/radarr/tools.go` issues a `MoviesSearch` command for one explicit movie and reads a command by ID. | `TestSearchQueueAndHistory` asserts the command payload and the tracked state. |
| CONN-ARR-MCP-RADARR-R008 | **Track activity.** `internal/radarr/tools.go` returns bounded queue and history pages. | `TestSearchQueueAndHistory` and `TestToolValidation` verify paging bounds and decoding. |
| CONN-ARR-MCP-RADARR-R009 | **Bound network behavior.** `internal/radarr/client.go` applies timeouts, context cancellation, default TLS verification, bounded responses, and sanitized errors. | `TestCancellationAndTLSDefaults` and `TestMutationBodiesAndErrorRedaction` verify the bounds and redaction. |
| CONN-ARR-MCP-RADARR-R010 | **Document and package.** `docs/radarr-go.md` and `docker/radarr.Dockerfile` document and package the server. | `docker build -f docker/radarr.Dockerfile .` succeeds. |
| CONN-ARR-MCP-RADARR-R011 | **Complete domain action catalog.** `radarr_action` exposes all supported Python-client domain actions and `list_actions`; existing typed tools remain. | Generated catalog matches the AST-derived Python action set and tool contract test discovers it. |
| CONN-ARR-MCP-RADARR-R012 | **Confirm mutations.** All actions classified mutating/destructive require `confirm=true`; otherwise no HTTP request is sent. | Dispatcher tests assert cancellation and zero requests; confirmed calls exercise mocked HTTP. |
| CONN-ARR-MCP-RADARR-R013 | **Keep language parity.** A generator and parity test detect Python method additions/removals and reject unsupported method shapes. | `pytest tests/test_go_action_parity.py -q` passes and fails on an intentional fixture drift. |
| CONN-ARR-MCP-RADARR-R014 | **Hermes-compatible action schema.** `radarr_action` advertises `result` as a JSON Schema object (permissive schema) rather than boolean `true`, retaining support for dynamic JSON results. | MCP contract asserts `outputSchema.properties.result` is a JSON object/schema and not a boolean. |
