# CONN-ARR-MCP-RADARR — Implementation tasks

Governing spec: [spec.md](spec.md). Design: [plan.md](plan.md). Rows: [requirements.md](requirements.md).

- [x] **CONN-ARR-MCP-RADARR-R001:** Add the standalone executable and Streamable HTTP handler at `/mcp/radarr`; prove MCP initialization and catalog through `TestMCPContract`.
- [x] **CONN-ARR-MCP-RADARR-R002:** Add trusted config and `X-Api-Key` auth with redirect and prefix handling; prove validation and key containment.
- [x] **CONN-ARR-MCP-RADARR-R003:** Add library read tools and local filtering/paging; prove typed responses and rejected bounds.
- [x] **CONN-ARR-MCP-RADARR-R004:** Add lookup, quality profile, and root folder tools; prove each typed response.
- [x] **CONN-ARR-MCP-RADARR-R005:** Add duplicate-safe movie creation with explicit profile/folder and no implicit search; prove body and short-circuit.
- [x] **CONN-ARR-MCP-RADARR-R006:** Add monitoring control via full-fetch-then-PUT; prove field preservation.
- [x] **CONN-ARR-MCP-RADARR-R007:** Add release search and command tracking; prove payload and tracked state.
- [x] **CONN-ARR-MCP-RADARR-R008:** Add bounded queue and history reads; prove paging and decoding.
- [x] **CONN-ARR-MCP-RADARR-R009:** Bound and sanitize requests; prove cancellation, TLS default, redirect refusal, and secret redaction.
- [x] **CONN-ARR-MCP-RADARR-R010:** Add docs and a non-root Docker image; build the image successfully.
- [x] Run the Go gates, Docker build, and spec-standard/status checks; regenerate `status.json`.
- [x] **CONN-ARR-MCP-RADARR-R011:** Generate and embed the complete Radarr domain action catalog; add `radarr_action` while retaining typed tools.
- [x] **CONN-ARR-MCP-RADARR-R012:** Require explicit confirmation for mutating/destructive catalog actions and prove no request is sent without it.
- [x] **CONN-ARR-MCP-RADARR-R013:** Add AST-based action generation and Python/Go parity tests; reject unsupported method shapes.
