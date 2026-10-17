# Standalone Bazarr MCP (Go)

An independently deployable MCP server for Bazarr, written in Go. It exposes Streamable HTTP at `/mcp/bazarr` and `/health`, and targets the Bazarr `v1.6.2` REST API under `/api`.

## Run in Docker

```sh
docker run -d --name bazarr-mcp \
  --read-only --cap-drop ALL --security-opt no-new-privileges:true \
  -e BAZARR_URL=http://your-bazarr:6767 \
  -e BAZARR_API_KEY='your-bazarr-api-key' \
  -e BAZARR_LISTEN_ADDR=0.0.0.0:8080 \
  -p 127.0.0.1:8093:8080 \
  marcosisocram/bazarr-mcp:latest
```

- MCP endpoint: `http://127.0.0.1:8093/mcp/bazarr`
- Health: `http://127.0.0.1:8093/health`

The image uses a distroless base, runs as numeric UID/GID `65532:65532`, and includes a healthcheck. The server has no inbound authentication; expose it only to a trusted network or behind a TLS/access-control proxy.

## Configuration

| Variable | Required | Default | Description |
|---|---|---|---|
| `BAZARR_URL` | yes | — | Trusted Bazarr base URL; install path prefixes are supported, e.g. `https://host/bazarr` |
| `BAZARR_API_KEY` | yes | — | Bazarr API key (Settings → General → Security) |
| `BAZARR_LISTEN_ADDR` | no | `127.0.0.1:8080` | Listen address; use `0.0.0.0:8080` in containers |

Bazarr authentication uses `X-API-KEY`; it does not use the Radarr/Sonarr `X-Api-Key` spelling/contract. The client verifies TLS by default and refuses redirects so the key is never replayed to another origin.

## Tools

Read tools: `bazarr_get_status`, `bazarr_list_languages`, `bazarr_list_language_profiles`, `bazarr_list_series`, `bazarr_get_series`, `bazarr_list_episodes`, `bazarr_list_wanted_episodes`, `bazarr_list_movies`, `bazarr_get_movie`, `bazarr_list_wanted_movies`, `bazarr_list_providers`, and `bazarr_get_history`.

Write tools: `bazarr_set_series_profile` and `bazarr_set_movie_profile` set or clear only the language profile; they do not start searches. `bazarr_search_series_subtitles` and `bazarr_search_movie_subtitles` search missing subtitles for one explicit item and may cause downloads. `bazarr_download_episode_subtitle` requests one language/forced/HI subtitle for one explicit episode and may write a subtitle file beside the media.

List results use object outputs. Bazarr's paginated endpoints return `data` and `total`; the MCP preserves both. The status tool only returns Bazarr/package and connected Sonarr/Radarr versions. Local paths, database versions, Python/runtime details, and host metadata are omitted.

## Scope and safety

The MCP does not expose subtitle uploads/deletion, sync/translation/modification, provider reset, shutdown/restart, configuration editing, backups, or webhooks. Tool inputs never accept an upstream URL or filesystem path. Reads and profile updates do not trigger subtitle searches. Mass search and per-episode subtitle download are distinct explicit tools with visible side-effect descriptions.

## Run locally

```sh
BAZARR_URL=http://127.0.0.1:6767 \
BAZARR_API_KEY='...' \
go run ./cmd/bazarr-mcp
```

Build the standalone binary with `go build ./cmd/bazarr-mcp` or the image with `docker build -f docker/bazarr.Dockerfile -t bazarr-mcp .`. Tests are offline; run `go test ./...`, `go vet ./...`, and `go test -race ./...`.
