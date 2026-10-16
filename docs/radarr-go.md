# Standalone Radarr MCP (Go)

A separate executable/container that does not replace or require the Python Arr MCP. It exposes one MCP server over Streamable HTTP at `/mcp/radarr` and a health endpoint at `/health`, and it talks to Radarr API v3.

## Configuration

Set `RADARR_URL` to the trusted Radarr base URL and `RADARR_API_KEY` to its API key (Radarr → Settings → General → Security). A base URL with an installation path prefix is supported, for example `https://host/radarr`.

`RADARR_LISTEN_ADDR` defaults to `127.0.0.1:8080`. For containers, set it to `0.0.0.0:8080` and expose the port only on a trusted network or behind TLS and access control. The server does not provide inbound authentication. TLS verification for upstream HTTPS remains enabled, and redirects are never followed so the API key cannot be replayed to another origin.

## Tools

Read tools: `radarr_list_movies` (with `monitored`/`has_file` filters and `limit`/`offset`), `radarr_get_movie`, `radarr_lookup_movies`, `radarr_list_quality_profiles`, `radarr_list_root_folders`, `radarr_get_queue`, `radarr_get_history`, and `radarr_get_command`.

Write tools: `radarr_add_movie`, `radarr_set_movie_monitored`, and `radarr_search_movie`.

A typical flow is `radarr_lookup_movies` → `radarr_list_quality_profiles` + `radarr_list_root_folders` → `radarr_add_movie` → `radarr_search_movie` → `radarr_get_command`.

`radarr_add_movie` requires `tmdb_id`, `quality_profile_id`, and `root_folder_path`; there are no silent defaults for those. It never starts a release search and returns `already_exists` without writing when the TMDB ID is already in the library. `radarr_set_movie_monitored` changes only the `monitored` flag of one movie. `radarr_search_movie` asks Radarr to search indexers for one explicit movie and can start a download; it returns a command ID that `radarr_get_command` tracks.

Errors report the operation and HTTP status only, never the API key or the upstream response body. Removing movies or files, manual import, config/indexer/download-client editing, backups, and generic administrative commands are not part of this integration.

## Run locally

```sh
RADARR_URL=http://127.0.0.1:7878 \
RADARR_API_KEY='...' \
go run ./cmd/radarr-mcp
```

Build with `go build ./cmd/radarr-mcp`. To build the isolated image, run `docker build -f docker/radarr.Dockerfile -t radarr-mcp .` and provide the credentials at runtime through the deployment secret manager. The container runs as a non-root user with no Python runtime, and declares `HEALTHCHECK CMD ["/radarr-mcp", "--healthcheck"]`, which is the same command a Kubernetes `livenessProbe`/`readinessProbe` should use.

## Validation

The default tests are offline and use mocked Radarr responses. Run `go test ./...`, `go vet ./...`, and `go test -race ./...`. No live Radarr instance is required. Live validation is read-only by default; add, monitoring, and search against a real instance require explicit authorization.
