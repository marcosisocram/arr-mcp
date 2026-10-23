# Standalone qBittorrent MCP (Go pilot)

The Go pilot is a separate executable/container and does not replace or require the Python Arr MCP. It exposes one MCP server over Streamable HTTP at `/mcp/qbittorrent` and a health endpoint at `/health`.

## Configuration

Set `QBITTORRENT_URL` to the trusted qBittorrent WebUI base URL. Authenticate with either:

- `QBITTORRENT_API_KEY` (preferred; qBittorrent 5.2.0 / WebAPI 2.14.1 or newer), or
- both `QBITTORRENT_USERNAME` and `QBITTORRENT_PASSWORD` (WebUI login; cookie sessions are renewed after an authorization expiry).

`QBITTORRENT_LISTEN_ADDR` defaults to `127.0.0.1:8080`. For containers, set it to `0.0.0.0:8080` and expose the port only on a trusted network or behind TLS and access control. The server does not provide inbound authentication. TLS verification for upstream HTTPS remains enabled.

## Tools

Read tools: `qbittorrent_list_torrents`, `qbittorrent_torrent_properties`, `qbittorrent_transfer_info`, `qbittorrent_get_transfer_limits`, `qbittorrent_get_speed_limits_mode`, `qbittorrent_list_torrent_files`, `qbittorrent_list_torrent_trackers`, `qbittorrent_list_torrent_peers`, `qbittorrent_list_categories`, `qbittorrent_list_tags`, `qbittorrent_app_version`, and `qbittorrent_server_state`. File/peer results are capped (default 500/200; maximum 5000/1000) and report `truncated`. Tracker URLs redact userinfo and sensitive query values; server state is allowlisted. Every tool output is a top-level object, which Python MCP SDK clients require when validating `outputSchema`.

Control tools: `qbittorrent_pause_torrents`, `qbittorrent_resume_torrents`, `qbittorrent_set_transfer_limits`, `qbittorrent_toggle_speed_limits`, and `qbittorrent_delete_torrents`. Pause/resume and removal require explicit torrent hashes; empty selections and the `all` wildcard are rejected. Removal requires `confirm=true`, accepts 1–100 hashes, and defaults `delete_files` to false. Setting `delete_files=true` permanently removes downloaded data. Transfer limits are bytes per second; zero means unlimited.

Note on upstream authentication: qBittorrent does not accept the `X-Api-Key` header used by the other *arr apps. The client authenticates with `Authorization: Bearer <api key>` (validated against qBittorrent 5.2.x) or the WebUI login flow.

Adding torrents, deleting files by default, RSS, search, logs, full/incremental sync, per-torrent limits, recheck, and reannounce are not part of this pilot.

## Run locally

```sh
QBITTORRENT_URL=http://127.0.0.1:8080 \
QBITTORRENT_API_KEY='qbt_...' \
go run ./cmd/qbittorrent-mcp
```

Build with `go build ./cmd/qbittorrent-mcp`. To build the isolated image, run `docker build -f docker/qbittorrent.Dockerfile -t qbittorrent-mcp .`. The Compose example is `deploy/docker/qbittorrent.compose.yml`; provide credentials via an untracked `.env` file or the deployment secret manager.

## Validation

The default tests are offline and use mocked qBittorrent responses. Run `go test ./...`, `go vet ./...`, and `go test -race ./...`. No live qBittorrent instance is required.
