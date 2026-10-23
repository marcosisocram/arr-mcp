# Standalone Sonarr MCP (Go)

A separate executable/container that does not replace or require the Python Arr MCP. It exposes one MCP server over Streamable HTTP at `/mcp/sonarr` and a health endpoint at `/health`, and it talks to Sonarr API v3.

## Configuration

Set `SONARR_URL` to the trusted Sonarr base URL and `SONARR_API_KEY` to its API key (Sonarr → Settings → General → Security). A base URL with an installation path prefix is supported, for example `https://host/sonarr`.

If Sonarr restricts hostnames (Settings → General → Security: "Allowed Hostnames"), the MCP must call Sonarr through a hostname Sonarr accepts. Behind Kubernetes internal Services this typically requires overriding the limit by environment variable, for example `SONARR__SERVER__ALLOWEDHOSTS=sonarr.your.domain,sonarr.your-namespace.svc.cluster.local`; the typical symptom otherwise is an HTML `Bad Request - Invalid host name` from Kestrel.

`SONARR_LISTEN_ADDR` defaults to `127.0.0.1:8080`. For containers, set it to `0.0.0.0:8080` and expose the port only on a trusted network or behind TLS and access control. The server does not provide inbound authentication. TLS verification for upstream HTTPS remains enabled, and redirects are never followed so the API key cannot be replayed to another origin.

## Tools

Read tools: `sonarr_list_series` (with a `monitored` filter and `limit`/`offset`), `sonarr_get_series` (with per-season monitoring flags), `sonarr_lookup_series`, `sonarr_list_quality_profiles`, `sonarr_list_root_folders`, `sonarr_list_episodes` (scoped to one series, with optional season/`monitored`/`has_file` filters), `sonarr_get_episode`, `sonarr_get_queue`, `sonarr_get_history`, and `sonarr_get_command`. List results are wrapped in an object — `{"series": [...]}`, `{"candidates": [...]}`, `{"profiles": [...]}`, `{"folders": [...]}`, `{"episodes": [...]}` — so every tool's `outputSchema` is a top-level `object`, as required by Python MCP SDK clients.

Write tools: `sonarr_add_series`, `sonarr_set_series_monitored`, `sonarr_set_episode_monitored`, `sonarr_search_series`, `sonarr_search_season`, and `sonarr_search_episodes`.

A typical flow is `sonarr_lookup_series` → `sonarr_list_quality_profiles` + `sonarr_list_root_folders` → `sonarr_add_series` → `sonarr_search_series` (or `sonarr_search_season` / `sonarr_search_episodes`) → `sonarr_get_command`.

`sonarr_add_series` requires `tvdb_id`, `quality_profile_id`, and `root_folder_path`; there are no silent defaults for those. It never starts a release search and returns `already_exists` without writing when the TVDB ID is already in the library. `sonarr_set_series_monitored` changes only the `monitored` flag of one series; `sonarr_set_episode_monitored` changes only the `monitored` flag of explicit episodes. The `sonarr_search_*` tools ask Sonarr to search indexers for one explicit target and can start a download; each returns a command ID that `sonarr_get_command` tracks.

Errors report the operation and HTTP status only, never the API key or the upstream response body. In addition to the guided typed tools, `sonarr_action` exposes all 227 supported Sonarr Python-client domain actions. Discover names with `{"action":"list_actions"}`; pass the Python method's argument names and values in `params_json`, for example `{"action":"get_series","params_json":"{\"seriesId\":42}"}`. Results use an object envelope (`result`).

All write verbs (POST, PUT, PATCH, DELETE) through `sonarr_action` require the tool argument `confirm: true`; without it the tool returns `cancelled` and sends no request. For example, `delete_series_id` requires `params_json={"id":42}` and `confirm=true`. The composite `lookup_series` and `add_series` actions are available through the same router; `add_series` requires confirmation because Python's default can start searches for missing episodes. Credential-like fields in routed responses are redacted.

The action catalog intentionally excludes only the HTTP helper and UI/non-domain methods: `request`, `get_api`, login/logout, the `.ics` calendar feed, `/ping`, and generic root/content paths (`get_ping`, `get_content_path`, `get_`, `get_path`). These exclusions are asserted by the Python/Go parity test.

## Run locally

```sh
SONARR_URL=http://127.0.0.1:8989 \
SONARR_API_KEY='...' \
go run ./cmd/sonarr-mcp
```

Build with `go build ./cmd/sonarr-mcp`. To build the isolated image, run `docker build -f docker/sonarr.Dockerfile -t sonarr-mcp .` and provide the credentials at runtime through the deployment secret manager. The container runs as a non-root user with no Python runtime, and declares `HEALTHCHECK CMD ["/sonarr-mcp", "--healthcheck"]`, which is the same command a Kubernetes `livenessProbe`/`readinessProbe` should use.

## Validation

The default tests are offline and use mocked Sonarr responses. Run `go test ./...`, `go vet ./...`, and `go test -race ./...`. No live Sonarr instance is required. Live validation is read-only by default; add, monitoring, and search against a real instance require explicit authorization. The validated upstream release is Sonarr `4.0.20.3014-ls329` through `/api/v3`.
