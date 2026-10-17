package qbittorrent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// spec: CONN-ARR-MCP-R001, CONN-ARR-MCP-R003, CONN-ARR-MCP-R004, CONN-ARR-MCP-R005, CONN-ARR-MCP-R006, CONN-ARR-MCP-R008
func TestMCPReadAndControlContract(t *testing.T) {
	const hash = "0123456789012345678901234567890123456789"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf("missing API key")
		}
		switch r.URL.Path {
		case "/api/v2/torrents/info":
			_, _ = w.Write([]byte(`[{"hash":"` + hash + `","name":"sample","state":"downloading"}]`))
		case "/api/v2/torrents/stop":
			if err := r.ParseForm(); err != nil || r.Form.Get("hashes") != hash {
				t.Errorf("pause tool sent unexpected form: %s", r.Form.Encode())
			}
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/torrents/properties":
			if r.URL.Query().Get("hash") != hash {
				t.Errorf("unexpected properties hash: %s", r.URL.Query().Get("hash"))
			}
			_, _ = w.Write([]byte(`{"save_path":"/downloads","total_size":42}`))
		case "/api/v2/transfer/downloadLimit":
			_, _ = w.Write([]byte("0"))
		case "/api/v2/transfer/uploadLimit":
			_, _ = w.Write([]byte("500"))
		case "/api/v2/transfer/speedLimitsMode":
			_, _ = w.Write([]byte("1"))
		case "/api/v2/transfer/setDownloadLimit", "/api/v2/transfer/setUploadLimit":
			if err := r.ParseForm(); err != nil || r.Form.Get("limit") == "" {
				t.Errorf("transfer limit form missing: %s", r.Form.Encode())
			}
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/transfer/toggleSpeedLimitsMode", "/api/v2/torrents/start":
			_, _ = w.Write([]byte("Ok."))
		default:
			t.Errorf("unexpected upstream path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	api, err := NewClient(Config{BaseURL: upstream.URL, APIKey: "key"})
	if err != nil {
		t.Fatal(err)
	}
	mcpHTTP := httptest.NewServer(NewHTTPHandler(api))
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
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: mcpHTTP.URL + "/mcp/qbittorrent"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 9 {
		t.Fatalf("got %d tools, want 9", len(listed.Tools))
	}
	for _, tool := range listed.Tools {
		schema, err := json.Marshal(tool.OutputSchema)
		if err != nil {
			t.Fatalf("%s output schema is not JSON: %v", tool.Name, err)
		}
		if !strings.Contains(string(schema), `"type":"object"`) {
			t.Fatalf("%s output schema is not a top-level object (Python SDK clients reject it): %s", tool.Name, schema)
		}
	}
	properties, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "qbittorrent_torrent_properties",
		Arguments: map[string]any{"hash": hash},
	})
	if err != nil || properties.IsError {
		t.Fatalf("properties tool failed: result=%#v err=%v", properties, err)
	}
	limits, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "qbittorrent_get_transfer_limits", Arguments: map[string]any{}})
	if err != nil || limits.IsError {
		t.Fatalf("transfer limits tool failed: result=%#v err=%v", limits, err)
	}
	mode, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "qbittorrent_get_speed_limits_mode", Arguments: map[string]any{}})
	if err != nil || mode.IsError {
		t.Fatalf("speed limits mode tool failed: result=%#v err=%v", mode, err)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "qbittorrent_list_torrents",
		Arguments: map[string]any{"filter": "active"},
	})
	if err != nil || result.IsError {
		t.Fatalf("list tool failed: result=%#v err=%v", result, err)
	}
	if result.StructuredContent == nil {
		t.Fatal("list tool returned no structured content")
	}
	paused, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "qbittorrent_pause_torrents",
		Arguments: map[string]any{"hashes": []any{hash}},
	})
	if err != nil || paused.IsError {
		t.Fatalf("pause tool failed: result=%#v err=%v", paused, err)
	}
	invalid, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "qbittorrent_pause_torrents",
		Arguments: map[string]any{"hashes": []any{"all"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !invalid.IsError {
		t.Fatal("wildcard all should be rejected as a tool error")
	}
	resumed, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "qbittorrent_resume_torrents",
		Arguments: map[string]any{"hashes": []any{hash}},
	})
	if err != nil || resumed.IsError {
		t.Fatalf("resume tool failed: result=%#v err=%v", resumed, err)
	}
	setLimits, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "qbittorrent_set_transfer_limits",
		Arguments: map[string]any{"download_limit": 0, "upload_limit": 1000},
	})
	if err != nil || setLimits.IsError {
		t.Fatalf("set limits tool failed: result=%#v err=%v", setLimits, err)
	}
	toggled, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "qbittorrent_toggle_speed_limits", Arguments: map[string]any{}})
	if err != nil || toggled.IsError {
		t.Fatalf("toggle speed mode tool failed: result=%#v err=%v", toggled, err)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil || len(encoded) == 0 {
		t.Fatalf("structured tool result is not JSON: %s (%v)", encoded, err)
	}
}
