package sonarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// spec: CONN-ARR-MCP-SONARR-R001, CONN-ARR-MCP-SONARR-R003, CONN-ARR-MCP-SONARR-R004, CONN-ARR-MCP-SONARR-R005, CONN-ARR-MCP-SONARR-R006, CONN-ARR-MCP-SONARR-R007, CONN-ARR-MCP-SONARR-R008, CONN-ARR-MCP-SONARR-R009
func TestMCPContract(t *testing.T) {
	added := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "key" {
			t.Errorf("missing API key")
		}
		if r.Method == http.MethodGet {
			if r.URL.Query().Get("tvdbId") == "81189" {
				if added {
					_, _ = w.Write([]byte(`[{"id":12,"title":"Breaking Bad","tvdbId":81189,"monitored":true}]`))
					return
				}
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":12,"title":"Breaking Bad","year":2008,"tvdbId":81189,"monitored":true,"seasons":[{"seasonNumber":1,"monitored":true}]}]`))
			return
		}
		added = true
		_, _ = w.Write([]byte(`{"id":12,"title":"Breaking Bad","tvdbId":81189,"monitored":true,"qualityProfileId":3,"rootFolderPath":"/tv"}`))
	})
	mux.HandleFunc("/api/v3/series/12", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_, _ = w.Write([]byte(`{"id":12,"title":"Breaking Bad","tvdbId":81189,"monitored":false}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":12,"title":"Breaking Bad","year":2008,"tvdbId":81189,"monitored":true,"seasons":[{"seasonNumber":1,"monitored":true}]}`))
	})
	mux.HandleFunc("/api/v3/series/lookup", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"title":"Breaking Bad","year":2008,"tvdbId":81189}]`))
	})
	mux.HandleFunc("/api/v3/qualityprofile", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":3,"name":"HD-1080p","upgradeAllowed":true,"cutoff":7}]`))
	})
	mux.HandleFunc("/api/v3/rootfolder", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":1,"path":"/tv","accessible":true,"freeSpace":1024}]`))
	})
	mux.HandleFunc("/api/v3/episode", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":101,"seriesId":12,"seasonNumber":1,"episodeNumber":1,"title":"Pilot","monitored":true,"hasFile":true}]`))
	})
	mux.HandleFunc("/api/v3/episode/101", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":101,"seriesId":12,"seasonNumber":1,"episodeNumber":1,"title":"Pilot","monitored":true,"hasFile":true}`))
	})
	mux.HandleFunc("/api/v3/episode/monitor", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("/api/v3/command", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":42,"name":"SeriesSearch","status":"queued"}`))
	})
	mux.HandleFunc("/api/v3/command/42", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":42,"name":"SeriesSearch","status":"completed","result":"successful"}`))
	})
	mux.HandleFunc("/api/v3/queue", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"page":1,"pageSize":25,"totalRecords":1,"records":[{"id":1,"seriesId":12,"episodeId":101,"title":"Breaking Bad","status":"downloading"}]}`))
	})
	mux.HandleFunc("/api/v3/history", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"page":1,"pageSize":25,"totalRecords":1,"records":[{"id":5,"seriesId":12,"episodeId":101,"eventType":"grabbed"}]}`))
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
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: mcpHTTP.URL + "/mcp/sonarr"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 16 {
		t.Fatalf("got %d tools, want 16", len(listed.Tools))
	}
	for _, tool := range listed.Tools {
		if tool.Name != "sonarr_list_series" {
			continue
		}
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("list series input schema is not JSON: %v", err)
		}
		if !strings.Contains(string(schema), `"monitored"`) || !strings.Contains(string(schema), `"boolean"`) {
			t.Fatalf("list series input schema is not typed: %s", schema)
		}
	}

	call := func(name string, arguments map[string]any) *mcp.CallToolResult {
		t.Helper()
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
		if err != nil {
			t.Fatalf("%s call failed: %v", name, err)
		}
		return result
	}

	if result := call("sonarr_list_series", map[string]any{"monitored": true}); result.IsError || result.StructuredContent == nil {
		t.Fatalf("list series failed: %#v", result)
	}
	if result := call("sonarr_get_series", map[string]any{"series_id": 12}); result.IsError {
		t.Fatalf("get series failed: %#v", result)
	}
	if result := call("sonarr_lookup_series", map[string]any{"term": "breaking bad"}); result.IsError {
		t.Fatalf("lookup series failed: %#v", result)
	}
	if result := call("sonarr_list_quality_profiles", map[string]any{}); result.IsError {
		t.Fatalf("quality profiles failed: %#v", result)
	}
	if result := call("sonarr_list_root_folders", map[string]any{}); result.IsError {
		t.Fatalf("root folders failed: %#v", result)
	}
	if result := call("sonarr_list_episodes", map[string]any{"series_id": 12, "season_number": 1}); result.IsError {
		t.Fatalf("list episodes failed: %#v", result)
	}
	if result := call("sonarr_get_episode", map[string]any{"episode_id": 101}); result.IsError {
		t.Fatalf("get episode failed: %#v", result)
	}
	created := call("sonarr_add_series", map[string]any{"tvdb_id": 81189, "quality_profile_id": 3, "root_folder_path": "/tv"})
	if created.IsError {
		t.Fatalf("add series failed: %#v", created)
	}
	encoded, err := json.Marshal(created.StructuredContent)
	if err != nil || !strings.Contains(string(encoded), "created") {
		t.Fatalf("add series did not report creation: %s (%v)", encoded, err)
	}
	duplicate := call("sonarr_add_series", map[string]any{"tvdb_id": 81189, "quality_profile_id": 3, "root_folder_path": "/tv"})
	if duplicate.IsError {
		t.Fatalf("duplicate add failed: %#v", duplicate)
	}
	encoded, err = json.Marshal(duplicate.StructuredContent)
	if err != nil || !strings.Contains(string(encoded), "already_exists") {
		t.Fatalf("duplicate add was not reported: %s (%v)", encoded, err)
	}
	if result := call("sonarr_set_series_monitored", map[string]any{"series_id": 12, "monitored": false}); result.IsError {
		t.Fatalf("set series monitored failed: %#v", result)
	}
	if result := call("sonarr_set_episode_monitored", map[string]any{"episode_ids": []int{101}, "monitored": false}); result.IsError {
		t.Fatalf("set episode monitored failed: %#v", result)
	}
	search := call("sonarr_search_series", map[string]any{"series_id": 12})
	if search.IsError || search.StructuredContent == nil {
		t.Fatalf("search series failed: %#v", search)
	}
	if result := call("sonarr_search_season", map[string]any{"series_id": 12, "season_number": 1}); result.IsError {
		t.Fatalf("search season failed: %#v", result)
	}
	if result := call("sonarr_search_episodes", map[string]any{"episode_ids": []int{101}}); result.IsError {
		t.Fatalf("search episodes failed: %#v", result)
	}
	if result := call("sonarr_get_command", map[string]any{"command_id": 42}); result.IsError {
		t.Fatalf("get command failed: %#v", result)
	}
	if result := call("sonarr_get_queue", map[string]any{"page": 1, "page_size": 25}); result.IsError {
		t.Fatalf("get queue failed: %#v", result)
	}
	if result := call("sonarr_get_history", map[string]any{}); result.IsError {
		t.Fatalf("get history failed: %#v", result)
	}

	invalid := call("sonarr_add_series", map[string]any{"tvdb_id": 81189, "quality_profile_id": 3, "root_folder_path": "/tv", "monitor": "bogus"})
	if !invalid.IsError {
		t.Fatal("unknown monitor type should be rejected as a tool error")
	}
	invalid = call("sonarr_search_series", map[string]any{"series_id": 0})
	if !invalid.IsError {
		t.Fatal("non-positive series_id should be rejected as a tool error")
	}
	invalid = call("sonarr_list_episodes", map[string]any{})
	if !invalid.IsError {
		t.Fatal("missing series_id should be rejected as a tool error")
	}
	invalid = call("sonarr_get_queue", map[string]any{"page_size": 1000})
	if !invalid.IsError {
		t.Fatal("oversized page_size should be rejected as a tool error")
	}
}
