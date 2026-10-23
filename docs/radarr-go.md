# Standalone Radarr MCP (Go)

A separate executable/container that does not replace or require the Python Arr MCP. It exposes one MCP server over Streamable HTTP at `/mcp/radarr` and a health endpoint at `/health`, and it talks to Radarr API v3.

## Configuration

Set `RADARR_URL` to the trusted Radarr base URL and `RADARR_API_KEY` to its API key (Radarr → Settings → General → Security). A base URL with an installation path prefix is supported, for example `https://host/radarr`.

If Radarr restricts hostnames (Settings → General → Security: "Allowed Hostnames"), the MCP must call Radarr through a hostname Radarr accepts. Behind Kubernetes internal Services this typically requires overriding the limit by environment variable, for example `RADARR__SERVER__ALLOWEDHOSTS=radarr.your.domain,radarr.your-namespace.svc.cluster.local`; the typical symptom otherwise is an HTML `Bad Request - Invalid host name` from Kestrel.

`RADARR_LISTEN_ADDR` defaults to `127.0.0.1:8080`. For containers, set it to `0.0.0.0:8080` and expose the port only on a trusted network or behind TLS and access control. The server does not provide inbound authentication. TLS verification for upstream HTTPS remains enabled, and redirects are never followed so the API key cannot be replayed to another origin.

## Tools

Read tools: `radarr_list_movies` (with `monitored`/`has_file` filters and `limit`/`offset`), `radarr_get_movie`, `radarr_lookup_movies`, `radarr_list_quality_profiles`, `radarr_list_root_folders`, `radarr_get_queue`, `radarr_get_history`, and `radarr_get_command`. List results are wrapped in an object — `{"movies": [...]}`, `{"candidates": [...]}`, `{"profiles": [...]}`, `{"folders": [...]}` — so every tool's `outputSchema` is a top-level `object`, as required by Python MCP SDK clients.

Write tools: `radarr_add_movie`, `radarr_set_movie_monitored`, and `radarr_search_movie`.

A typical flow is `radarr_lookup_movies` → `radarr_list_quality_profiles` + `radarr_list_root_folders` → `radarr_add_movie` → `radarr_search_movie` → `radarr_get_command`.

`radarr_add_movie` requires `tmdb_id`, `quality_profile_id`, and `root_folder_path`; there are no silent defaults for those. It never starts a release search and returns `already_exists` without writing when the TMDB ID is already in the library. `radarr_set_movie_monitored` changes only the `monitored` flag of one movie. `radarr_search_movie` asks Radarr to search indexers for one explicit movie and can start a download; it returns a command ID that `radarr_get_command` tracks.

Errors report the operation and HTTP status only, never the API key or the upstream response body. In addition to the guided typed tools, `radarr_action` exposes all 230 supported Radarr Python-client domain actions. Discover names with `{"action":"list_actions"}`; pass the Python method's argument names and values in `params_json`, for example `{"action":"get_movie","params_json":"{\"movieId\":42}"}`. Results use an object envelope (`result`).

All write verbs (POST, PUT, PATCH, DELETE) through `radarr_action` require the tool argument `confirm: true`; without it the tool returns `cancelled` and sends no request. For example, `delete_movie_id` requires `params_json={"id":42}` and `confirm=true`. The composite `lookup_movie` and `add_movie` actions are available through the same router; `add_movie` requires confirmation because Python's default can start a release search. Credential-like fields in routed responses are redacted.

The action catalog intentionally excludes only the HTTP helper and UI/non-domain methods: `request`, `get_api`, login/logout, the `.ics` calendar feed, `/ping`, and generic root/content paths (`get_ping`, `get_content_path`, `get_`, `get_path`). These exclusions are asserted by the Python/Go parity test.

## Run locally

```sh
RADARR_URL=http://127.0.0.1:7878 \
RADARR_API_KEY='...' \
go run ./cmd/radarr-mcp
```

Build with `go build ./cmd/radarr-mcp`. To build the isolated image, run `docker build -f docker/radarr.Dockerfile -t radarr-mcp .` and provide the credentials at runtime through the deployment secret manager. The container runs as a non-root user with no Python runtime, and declares `HEALTHCHECK CMD ["/radarr-mcp", "--healthcheck"]`, which is the same command a Kubernetes `livenessProbe`/`readinessProbe` should use.

## Validation

The default tests are offline and use mocked Radarr responses. Run `go test ./...`, `go vet ./...`, and `go test -race ./...`. No live Radarr instance is required. Live validation is read-only by default; add, monitoring, and search against a real instance require explicit authorization.
