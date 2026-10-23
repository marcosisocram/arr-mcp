package radarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// spec: CONN-ARR-MCP-RADARR-R001, CONN-ARR-MCP-RADARR-R003, CONN-ARR-MCP-RADARR-R004, CONN-ARR-MCP-RADARR-R005, CONN-ARR-MCP-RADARR-R006, CONN-ARR-MCP-RADARR-R007, CONN-ARR-MCP-RADARR-R008
func TestMCPContract(t *testing.T) {
	added := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/movie", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "key" {
			t.Errorf("missing API key")
		}
		if r.Method == http.MethodGet {
			if r.URL.Query().Get("tmdbId") == "603" {
				if added {
					_, _ = w.Write([]byte(`[{"id":7,"title":"The Matrix","tmdbId":603,"monitored":true}]`))
					return
				}
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":7,"title":"The Matrix","year":1999,"tmdbId":603,"monitored":true,"hasFile":true,"qualityProfileId":3}]`))
			return
		}
		added = true
		_, _ = w.Write([]byte(`{"id":7,"title":"The Matrix","tmdbId":603,"monitored":true,"qualityProfileId":3,"rootFolderPath":"/movies"}`))
	})
	mux.HandleFunc("/api/v3/movie/7", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_, _ = w.Write([]byte(`{"id":7,"title":"The Matrix","tmdbId":603,"monitored":false}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":7,"title":"The Matrix","year":1999,"tmdbId":603,"monitored":true,"hasFile":true}`))
	})
	mux.HandleFunc("/api/v3/movie/lookup", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"title":"The Matrix","year":1999,"tmdbId":603}]`))
	})
	mux.HandleFunc("/api/v3/movie/lookup/tmdb", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"title":"The Matrix","year":1999,"tmdbId":603,"titleSlug":"the-matrix-603"}`))
	})
	mux.HandleFunc("/api/v3/qualityprofile", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":3,"name":"HD-1080p","upgradeAllowed":true,"cutoff":7}]`))
	})
	mux.HandleFunc("/api/v3/rootfolder", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":1,"path":"/movies","accessible":true,"freeSpace":1024}]`))
	})
	mux.HandleFunc("/api/v3/command", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":42,"name":"MoviesSearch","status":"queued"}`))
	})
	mux.HandleFunc("/api/v3/command/42", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":42,"name":"MoviesSearch","status":"completed","result":"successful"}`))
	})
	mux.HandleFunc("/api/v3/queue", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"page":1,"pageSize":25,"totalRecords":1,"records":[{"id":1,"movieId":7,"title":"The Matrix","status":"downloading"}]}`))
	})
	mux.HandleFunc("/api/v3/history", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"page":1,"pageSize":25,"totalRecords":1,"records":[{"id":5,"movieId":7,"eventType":"grabbed"}]}`))
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
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: mcpHTTP.URL + "/mcp/radarr"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 12 {
		t.Fatalf("got %d tools, want 12", len(listed.Tools))
	}
	for _, tool := range listed.Tools {
		outputSchema, err := json.Marshal(tool.OutputSchema)
		if err != nil {
			t.Fatalf("%s output schema is not JSON: %v", tool.Name, err)
		}
		if !strings.Contains(string(outputSchema), `"type":"object"`) {
			t.Fatalf("%s output schema is not a top-level object (Python SDK clients reject it): %s", tool.Name, outputSchema)
		}
		if tool.Name == "radarr_action" {
			var schema map[string]any
			if err := json.Unmarshal(outputSchema, &schema); err != nil {
				t.Fatal(err)
			}
			properties, _ := schema["properties"].(map[string]any)
			if _, ok := properties["result"].(map[string]any); !ok {
				t.Fatalf("radarr_action result schema must be an object, got %s", outputSchema)
			}
		}
		if tool.Name != "radarr_list_movies" {
			continue
		}
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("list movies input schema is not JSON: %v", err)
		}
		if !strings.Contains(string(schema), `"monitored"`) || !strings.Contains(string(schema), `"boolean"`) {
			t.Fatalf("list movies input schema is not typed: %s", schema)
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

	if result := call("radarr_list_movies", map[string]any{"monitored": true}); result.IsError || result.StructuredContent == nil {
		t.Fatalf("list movies failed: %#v", result)
	}
	if result := call("radarr_get_movie", map[string]any{"movie_id": 7}); result.IsError {
		t.Fatalf("get movie failed: %#v", result)
	}
	if result := call("radarr_lookup_movies", map[string]any{"term": "matrix"}); result.IsError {
		t.Fatalf("lookup movies failed: %#v", result)
	}
	if result := call("radarr_list_quality_profiles", map[string]any{}); result.IsError {
		t.Fatalf("quality profiles failed: %#v", result)
	}
	if result := call("radarr_list_root_folders", map[string]any{}); result.IsError {
		t.Fatalf("root folders failed: %#v", result)
	}
	created := call("radarr_add_movie", map[string]any{"tmdb_id": 603, "quality_profile_id": 3, "root_folder_path": "/movies"})
	if created.IsError {
		t.Fatalf("add movie failed: %#v", created)
	}
	encoded, err := json.Marshal(created.StructuredContent)
	if err != nil || !strings.Contains(string(encoded), "created") {
		t.Fatalf("add movie did not report creation: %s (%v)", encoded, err)
	}
	duplicate := call("radarr_add_movie", map[string]any{"tmdb_id": 603, "quality_profile_id": 3, "root_folder_path": "/movies"})
	if duplicate.IsError {
		t.Fatalf("duplicate add failed: %#v", duplicate)
	}
	encoded, err = json.Marshal(duplicate.StructuredContent)
	if err != nil || !strings.Contains(string(encoded), "already_exists") {
		t.Fatalf("duplicate add was not reported: %s (%v)", encoded, err)
	}
	if result := call("radarr_set_movie_monitored", map[string]any{"movie_id": 7, "monitored": false}); result.IsError {
		t.Fatalf("set monitored failed: %#v", result)
	}
	search := call("radarr_search_movie", map[string]any{"movie_id": 7})
	if search.IsError || search.StructuredContent == nil {
		t.Fatalf("search movie failed: %#v", search)
	}
	if result := call("radarr_get_command", map[string]any{"command_id": 42}); result.IsError {
		t.Fatalf("get command failed: %#v", result)
	}
	if result := call("radarr_get_queue", map[string]any{"page": 1, "page_size": 25}); result.IsError {
		t.Fatalf("get queue failed: %#v", result)
	}
	if result := call("radarr_get_history", map[string]any{}); result.IsError {
		t.Fatalf("get history failed: %#v", result)
	}

	invalid := call("radarr_add_movie", map[string]any{"tmdb_id": 603, "quality_profile_id": 3, "root_folder_path": "/movies", "monitor": "bogus"})
	if !invalid.IsError {
		t.Fatal("unknown monitor type should be rejected as a tool error")
	}
	invalid = call("radarr_search_movie", map[string]any{"movie_id": 0})
	if !invalid.IsError {
		t.Fatal("non-positive movie_id should be rejected as a tool error")
	}
	invalid = call("radarr_get_queue", map[string]any{"page_size": 1000})
	if !invalid.IsError {
		t.Fatal("oversized page_size should be rejected as a tool error")
	}
	actions := call("radarr_action", map[string]any{"action": "list_actions"})
	encoded, err = json.Marshal(actions.StructuredContent)
	if actions.IsError || err != nil || !strings.Contains(string(encoded), "add_movie") || len(actionCatalog) != 230 {
		t.Fatalf("action catalog discovery failed: %s (%v)", encoded, err)
	}
	unconfirmed := call("radarr_action", map[string]any{"action": "delete_movie_id", "params_json": `{"id":1}`})
	encoded, err = json.Marshal(unconfirmed.StructuredContent)
	if unconfirmed.IsError || err != nil || !strings.Contains(string(encoded), `"cancelled":true`) {
		t.Fatalf("unconfirmed destructive action was not cancelled: %s (%v)", encoded, err)
	}
}
