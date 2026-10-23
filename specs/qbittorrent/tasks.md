# CONN-ARR-MCP — Implementation tasks

Governing spec: [spec.md](spec.md). Design: [plan.md](plan.md). Rows: [requirements.md](requirements.md).

- [x] **CONN-ARR-MCP-R001:** Add the standalone Go module, executable, and Streamable HTTP handler; prove MCP initialization/catalog through `TestMCPReadAndControlContract`.
- [x] **CONN-ARR-MCP-R002:** Add config/auth in `config.go` and `client.go`; prove auth modes and renewal in client tests.
- [x] **CONN-ARR-MCP-R003:** Add torrent read tools in `client.go` and `tools.go`; prove typed responses and validation.
- [x] **CONN-ARR-MCP-R004:** Add transfer state reads in `client.go` and `tools.go`; prove tool response behavior.
- [x] **CONN-ARR-MCP-R005:** Add hash-scoped pause/resume; prove explicit hashes and reject wildcards.
- [x] **CONN-ARR-MCP-R006:** Add transfer limit setting and alternative-speed toggle; prove form fields and validation.
- [x] **CONN-ARR-MCP-R007:** Bound and sanitize HTTP requests; prove cancellation, timeout configuration, and secret redaction.
- [x] **CONN-ARR-MCP-R008:** Add docs, Compose, and non-root Docker image; build image successfully.
- [x] **CONN-ARR-MCP-R009:** Add seven bounded typed read tools and mocked endpoint/response tests.
- [x] **CONN-ARR-MCP-R010:** Add explicit confirmed torrent deletion with safe defaults, validation, and zero-request cancellation test.
- [x] **CONN-ARR-MCP-R011:** Redact tracker credentials and allowlist server-state fields with regression tests.
- [x] **CONN-ARR-MCP-R012:** Update docs and MCP catalog contract; rerun Go, Docker, and spec gates and record evidence.
- [x] Run Go tests, vet, race, Docker build, spec-standard, and regenerate `status.json` with `pipelines-hook spec-status --write`.
