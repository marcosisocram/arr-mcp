package sonarr

import (
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func NewHTTPHandler(client *Client) http.Handler {
	server := NewMCPServer(client)
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{SessionTimeout: 30 * time.Minute})
	mux := http.NewServeMux()
	mux.Handle("/mcp/sonarr", mcpHandler)
	mux.Handle("/mcp/sonarr/", mcpHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return http.NewCrossOriginProtection().Handler(mux)
}
