# CONN-ARR-MCP — Requirements

Governing spec: [spec.md](spec.md).

| ID | Requirement | Acceptance |
|---|---|---|
| CONN-ARR-MCP-R001 | **Standalone server.** `cmd/qbittorrent-mcp/main.go` serves an independent official-SDK MCP at `/mcp/qbittorrent`. | `TestMCPReadAndControlContract` initializes the real entrypoint handler and lists its catalog. |
| CONN-ARR-MCP-R002 | **Safe authentication.** `internal/qbittorrent/config.go` and `client.go` load trusted environment configuration and support Bearer API key or cookie login/renewal. | `TestConfigValidation`, `TestAPIKeyAndTorrentReads`, and `TestPasswordLoginRenewsSessionOnce` pass without exposing secrets. |
| CONN-ARR-MCP-R003 | **Read torrents.** `internal/qbittorrent/client.go` and `tools.go` expose validated typed list and properties tools. | `TestAPIKeyAndTorrentReads` and `TestToolValidation` verify data, filters, and rejection cases. |
| CONN-ARR-MCP-R004 | **Read transfer state.** `internal/qbittorrent/client.go` and `tools.go` expose global transfer information, limits, and alternative-speed mode. | `TestAPIKeyAndTorrentReads` and `TestMCPReadAndControlContract` verify decoded responses. |
| CONN-ARR-MCP-R005 | **Control torrent state.** `internal/qbittorrent/tools.go` pauses/resumes explicitly selected hashes without an all-torrents operation. | `TestMutationFormsAndErrorRedaction` and `TestMCPReadAndControlContract` assert form fields and wildcard rejection. |
| CONN-ARR-MCP-R006 | **Control transfer limits.** `internal/qbittorrent/tools.go` sets byte-per-second limits and toggles alternative speed mode. | `TestMCPReadAndControlContract` verifies exact forms and validation. |
| CONN-ARR-MCP-R007 | **Bound network behavior.** `internal/qbittorrent/client.go` applies timeouts, context cancellation, default TLS verification, bounded responses, and sanitized errors. | `TestMutationFormsAndErrorRedaction` verifies errors omit upstream body content. |
| CONN-ARR-MCP-R008 | **Document and package pilot.** `docs/qbittorrent-go.md`, `docker/qbittorrent.Dockerfile`, and `deploy/docker/qbittorrent.compose.yml` document and package the server. | `docker build -f docker/qbittorrent.Dockerfile .` succeeds. |
