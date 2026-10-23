package bazarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// spec: CONN-ARR-MCP-BAZARR-R001, CONN-ARR-MCP-BAZARR-R003, CONN-ARR-MCP-BAZARR-R004, CONN-ARR-MCP-BAZARR-R005, CONN-ARR-MCP-BAZARR-R006, CONN-ARR-MCP-BAZARR-R007, CONN-ARR-MCP-BAZARR-R008, CONN-ARR-MCP-BAZARR-R009
func TestMCPContract(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"bazarr_version":"1.6.2","package_version":"Docker","sonarr_version":"4.0","radarr_version":"6.0","bazarr_directory":"/secret"}}`))
	})
	mux.HandleFunc("/api/system/languages", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"English","code2":"en","code3":"eng","enabled":true}]`))
	})
	mux.HandleFunc("/api/system/languages/profiles", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"profileId":2,"name":"English","items":[]}]`))
	})
	mux.HandleFunc("/api/series", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch || r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"sonarrSeriesId":12,"title":"Show","tvdbId":34,"profileId":2}],"total":1}`))
	})
	mux.HandleFunc("/api/episodes", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"sonarrSeriesId":12,"sonarrEpisodeId":123,"season":1,"episode":2,"title":"Pilot"}]}`))
	})
	mux.HandleFunc("/api/episodes/wanted", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"sonarrSeriesId":12,"sonarrEpisodeId":123,"seriesTitle":"Show","episodeTitle":"Pilot"}],"total":1}`))
	})
	mux.HandleFunc("/api/episodes/subtitles", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/api/episodes/history", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":1,"provider":"Example"}],"total":1}`))
	})
	mux.HandleFunc("/api/movies", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch || r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"radarrId":9,"title":"Film","profileId":2}],"total":1}`))
	})
	mux.HandleFunc("/api/movies/wanted", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"radarrId":9,"title":"Film"}],"total":1}`))
	})
	mux.HandleFunc("/api/movies/history", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":2,"title":"Film"}],"total":1}`))
	})
	mux.HandleFunc("/api/providers", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"name":"Example","status":"Good","retry":"-"}]}`))
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()
	mcpHTTP := httptest.NewServer(NewHTTPHandler(NewClient(Config{BaseURL: upstream.URL, APIKey: "key"})))
	defer mcpHTTP.Close()
	health, err := http.Get(mcpHTTP.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	if health.StatusCode != http.StatusOK {
		t.Fatalf("health endpoint returned HTTP %d", health.StatusCode)
	}
	_ = health.Body.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: mcpHTTP.URL + "/mcp/bazarr"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 18 {
		t.Fatalf("got %d tools, want 18", len(listed.Tools))
	}
	for _, tool := range listed.Tools {
		encoded, err := json.Marshal(tool.OutputSchema)
		if err != nil {
			t.Fatalf("%s output schema is not JSON: %v", tool.Name, err)
		}
		if !strings.Contains(string(encoded), `"type":"object"`) {
			t.Fatalf("%s outputSchema is not a top-level object: %s", tool.Name, encoded)
		}
	}
	call := func(name string, arguments map[string]any) *mcp.CallToolResult {
		t.Helper()
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
		if err != nil {
			t.Fatalf("%s call failed: %v", name, err)
		}
		if result.IsError {
			t.Fatalf("%s tool returned error: %#v", name, result)
		}
		if result.StructuredContent == nil {
			t.Fatalf("%s returned no structured content", name)
		}
		return result
	}
	call("bazarr_get_status", map[string]any{})
	call("bazarr_list_languages", map[string]any{})
	call("bazarr_list_language_profiles", map[string]any{})
	call("bazarr_list_series", map[string]any{"start": 0, "length": 25})
	call("bazarr_get_series", map[string]any{"series_id": 12})
	call("bazarr_list_episodes", map[string]any{"series_id": 12})
	call("bazarr_list_wanted_episodes", map[string]any{})
	call("bazarr_list_movies", map[string]any{})
	call("bazarr_get_movie", map[string]any{"movie_id": 9})
	call("bazarr_list_wanted_movies", map[string]any{})
	call("bazarr_list_providers", map[string]any{"history": true})
	call("bazarr_get_history", map[string]any{"media_type": "episode", "media_id": 123})
	call("bazarr_set_series_profile", map[string]any{"series_id": 12, "profile_id": "none"})
	call("bazarr_set_movie_profile", map[string]any{"movie_id": 9, "profile_id": "2"})
	call("bazarr_search_series_subtitles", map[string]any{"series_id": 12})
	call("bazarr_search_movie_subtitles", map[string]any{"movie_id": 9})
	call("bazarr_download_episode_subtitle", map[string]any{"series_id": 12, "episode_id": 123, "language": "en", "forced": false, "hi": false})

	for _, invalid := range []struct {
		name string
		args map[string]any
	}{
		{"bazarr_get_series", map[string]any{"series_id": 0}},
		{"bazarr_list_episodes", map[string]any{}},
		{"bazarr_get_movie", map[string]any{"movie_id": -1}},
		{"bazarr_get_history", map[string]any{"media_type": "all"}},
		{"bazarr_set_series_profile", map[string]any{"series_id": 12, "profile_id": "0"}},
		{"bazarr_set_movie_profile", map[string]any{"movie_id": 9, "profile_id": "2", "series_id": 12}},
		{"bazarr_search_series_subtitles", map[string]any{"series_id": 0}},
		{"bazarr_search_movie_subtitles", map[string]any{"movie_id": 9, "series_id": 12}},
		{"bazarr_download_episode_subtitle", map[string]any{"series_id": 12, "episode_id": 123, "language": "../secret"}},
		{"bazarr_list_movies", map[string]any{"length": 101}},
	} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: invalid.name, Arguments: invalid.args})
		if err != nil || !result.IsError {
			t.Fatalf("invalid %s input was not rejected: result=%#v err=%v", invalid.name, result, err)
		}
	}
	actions := call("bazarr_action", map[string]any{"action": "list_actions"})
	encoded, err := json.Marshal(actions.StructuredContent)
	if actions.IsError || err != nil || !strings.Contains(string(encoded), "update_settings") || len(actionCatalog) != 25 {
		t.Fatalf("action catalog discovery failed: %s (%v)", encoded, err)
	}
	unconfirmed := call("bazarr_action", map[string]any{"action": "remove_from_blacklist", "params_json": `{"blacklist_id":1}`})
	encoded, err = json.Marshal(unconfirmed.StructuredContent)
	if unconfirmed.IsError || err != nil || !strings.Contains(string(encoded), `"cancelled":true`) {
		t.Fatalf("unconfirmed destructive action was not cancelled: %s (%v)", encoded, err)
	}
}
