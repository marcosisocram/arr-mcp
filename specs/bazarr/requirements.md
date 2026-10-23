# CONN-ARR-MCP-BAZARR — Requirements

Governing spec: [spec.md](spec.md).

| ID | Requirement | Acceptance |
|---|---|---|
| CONN-ARR-MCP-BAZARR-R001 | **Standalone server.** `cmd/bazarr-mcp/main.go` serves an independent official-SDK MCP at `/mcp/bazarr`. | `TestMCPContract` initializes the handler and lists exactly 18 tools. |
| CONN-ARR-MCP-BAZARR-R002 | **Trusted config and authentication.** `internal/bazarr/config.go` and `client.go` load trusted `BAZARR_URL`, require `BAZARR_API_KEY`, default listen address, send `X-API-KEY`, and refuse redirects. | Config, key-header, prefix, and redirect tests pass. |
| CONN-ARR-MCP-BAZARR-R003 | **System and language reads.** `internal/bazarr/tools.go` returns allowlisted system status, languages, and profiles. | Tests verify response mapping and status redaction. |
| CONN-ARR-MCP-BAZARR-R004 | **Series/movie library reads.** Tools expose bounded server-side pages and explicit ID reads. | Tests verify Bazarr `start`/`length`, `total`, ID query parameters, and decoding. |
| CONN-ARR-MCP-BAZARR-R005 | **Episode/wanted reads.** Episode listing requires a series ID; wanted episode/movie pages are bounded. | Tests verify required IDs, paging, and response envelopes. |
| CONN-ARR-MCP-BAZARR-R006 | **Provider/history reads.** Providers and bounded history pages are exposed. | Tests verify query parameters and typed data. |
| CONN-ARR-MCP-BAZARR-R007 | **Explicit profile writes.** Only one series/movie language profile is changed per call. | Tests assert POST form/query payloads and reject invalid IDs/profile values before requests. |
| CONN-ARR-MCP-BAZARR-R008 | **Explicit subtitle actions.** Two mass-search tools target one explicit series/movie; episode subtitle download targets one explicit episode and language. | Tests assert PATCH routes/arguments and validation; descriptions disclose possible download side effects. |
| CONN-ARR-MCP-BAZARR-R009 | **Object output schemas.** Every tool output schema is top-level object for Python SDK compatibility. | `TestMCPContract` asserts all 17 `outputSchema.type` values are `object`. |
| CONN-ARR-MCP-BAZARR-R010 | **Bounded, sanitized HTTP.** Requests have timeout, cancellation, bounded reads, TLS defaults, no redirects, and sanitized errors. | Network behavior and secret-redaction tests pass. |
| CONN-ARR-MCP-BAZARR-R011 | **Document and package.** Docs and non-root image with numeric UID and healthcheck are provided. | Docker build and container health/SIGTERM checks pass. |
| CONN-ARR-MCP-BAZARR-R012 | **Complete domain action catalog.** `bazarr_action` exposes all supported Python-client domain actions and `list_actions`; existing typed tools remain. | Generated catalog matches the AST-derived Python action set and tool contract test discovers it. |
| CONN-ARR-MCP-BAZARR-R013 | **Confirm mutations.** All actions classified mutating/destructive require `confirm=true`; otherwise no HTTP request is sent. | Dispatcher tests assert cancellation and zero requests; confirmed calls exercise mocked HTTP. |
| CONN-ARR-MCP-BAZARR-R014 | **Keep language parity.** A generator and parity test detect Python method additions/removals and reject unsupported method shapes. | `pytest tests/test_go_action_parity.py -q` passes and fails on an intentional fixture drift. |
