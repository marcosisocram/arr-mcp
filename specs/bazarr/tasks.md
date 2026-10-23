# CONN-ARR-MCP-BAZARR — Implementation tasks

Governing spec: [spec.md](spec.md). Design: [plan.md](plan.md). Rows: [requirements.md](requirements.md).

- [x] **CONN-ARR-MCP-BAZARR-R001:** Add standalone executable and Streamable HTTP `/mcp/bazarr`; verify 17-tool catalog.
- [x] **CONN-ARR-MCP-BAZARR-R002:** Add trusted config and `X-API-KEY` client with prefix and redirect safety.
- [x] **CONN-ARR-MCP-BAZARR-R003:** Add allowlisted system status, languages, and language profiles.
- [x] **CONN-ARR-MCP-BAZARR-R004:** Add bounded series/movie reads and explicit-ID reads.
- [x] **CONN-ARR-MCP-BAZARR-R005:** Add episode and wanted-item reads.
- [x] **CONN-ARR-MCP-BAZARR-R006:** Add provider and history reads.
- [x] **CONN-ARR-MCP-BAZARR-R007:** Add explicit series/movie profile setters.
- [x] **CONN-ARR-MCP-BAZARR-R008:** Add explicit per-item subtitle search/download actions.
- [x] **CONN-ARR-MCP-BAZARR-R009:** Assert object output schema for every tool.
- [x] **CONN-ARR-MCP-BAZARR-R010:** Add bounded, cancelable, redirect-safe and sanitized HTTP.
- [x] **CONN-ARR-MCP-BAZARR-R011:** Add docs, numeric-UID Docker image and healthcheck.
- [x] Run Go gates and Docker build/container checks; record outcomes.
- [x] **CONN-ARR-MCP-BAZARR-R012:** Generate and embed the complete Bazarr domain action catalog; add `bazarr_action` while retaining typed tools.
- [x] **CONN-ARR-MCP-BAZARR-R013:** Require explicit confirmation for mutating/destructive catalog actions and prove no request is sent without it.
- [x] **CONN-ARR-MCP-BAZARR-R014:** Add AST-based action generation and Python/Go parity tests; reject unsupported method shapes.
- [x] **CONN-ARR-MCP-BAZARR-R015:** Publish a non-boolean permissive JSON Schema for `bazarr_action.result` and assert it in the MCP contract.
