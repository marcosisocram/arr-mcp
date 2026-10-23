# CONN-ARR-MCP-SONARR — Implementation tasks

Governing spec: [spec.md](spec.md). Design: [plan.md](plan.md). Rows: [requirements.md](requirements.md).

- [x] **CONN-ARR-MCP-SONARR-R001:** Add the standalone executable and Streamable HTTP handler at `/mcp/sonarr`; prove MCP initialization and catalog through `TestMCPContract`.
- [x] **CONN-ARR-MCP-SONARR-R002:** Add trusted config and `X-Api-Key` auth with redirect and prefix handling; prove validation and key containment.
- [x] **CONN-ARR-MCP-SONARR-R003:** Add series read tools with season data and local filtering/paging; prove typed responses and rejected bounds.
- [x] **CONN-ARR-MCP-SONARR-R004:** Add lookup, quality profile, and root folder tools; prove each typed response.
- [x] **CONN-ARR-MCP-SONARR-R005:** Add episode list/get tools scoped to an explicit series; prove scoping, season filtering, and decoding.
- [x] **CONN-ARR-MCP-SONARR-R006:** Add duplicate-safe series creation with explicit profile/folder and no implicit search; prove body and short-circuit.
- [x] **CONN-ARR-MCP-SONARR-R007:** Add series monitoring control via full-fetch-then-PUT and episode monitoring via the patch endpoint; prove field preservation and payload.
- [x] **CONN-ARR-MCP-SONARR-R008:** Add series/season/episode release search and command tracking; prove payloads and tracked state.
- [x] **CONN-ARR-MCP-SONARR-R009:** Add bounded queue and history reads; prove paging and decoding.
- [x] **CONN-ARR-MCP-SONARR-R010:** Bound and sanitize requests; prove cancellation, TLS default, redirect refusal, and secret redaction.
- [x] **CONN-ARR-MCP-SONARR-R011:** Add docs and a non-root Docker image; build the image successfully.
- [x] Run the Go gates, Docker build, and container behavior checks; regenerate `status.json`.
- [x] **CONN-ARR-MCP-SONARR-R012:** Generate and embed the complete Sonarr domain action catalog; add `sonarr_action` while retaining typed tools.
- [x] **CONN-ARR-MCP-SONARR-R013:** Require explicit confirmation for mutating/destructive catalog actions and prove no request is sent without it.
- [x] **CONN-ARR-MCP-SONARR-R014:** Add AST-based action generation and Python/Go parity tests; reject unsupported method shapes.
- [x] **CONN-ARR-MCP-SONARR-R015:** Publish a non-boolean permissive JSON Schema for `sonarr_action.result` and assert it in the MCP contract.
