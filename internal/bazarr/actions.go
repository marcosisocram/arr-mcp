package bazarr

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/url"

	"github.com/marcosisocram/arr-mcp/internal/actiondispatch"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed actions.json
var actionCatalogJSON []byte

var actionCatalog = mustLoadActionCatalog()

func mustLoadActionCatalog() []actiondispatch.Action {
	var actions []actiondispatch.Action
	if err := json.Unmarshal(actionCatalogJSON, &actions); err != nil {
		panic("decode embedded Bazarr action catalog: " + err.Error())
	}
	return actions
}

type ActionInput struct {
	Action     string `json:"action" jsonschema:"Bazarr API client action name; use list_actions to discover the complete domain catalog."`
	ParamsJSON string `json:"params_json,omitempty" jsonschema:"JSON object containing the action's parameters."`
	Confirm    bool   `json:"confirm,omitempty" jsonschema:"Required for every mutating or destructive action; false means no upstream request is made."`
}

func registerActionTool(server *mcp.Server, client *Client) {
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_action", Description: "Execute one allowlisted Bazarr domain API action. Use action=list_actions to discover actions. Mutating actions require confirm=true."}, func(ctx context.Context, _ *mcp.CallToolRequest, input ActionInput) (*mcp.CallToolResult, actiondispatch.Result, error) {
		result, err := dispatchAction(ctx, client, input)
		return nil, result, err
	})
}

func dispatchAction(ctx context.Context, client *Client, input ActionInput) (actiondispatch.Result, error) {
	return actiondispatch.Execute(ctx, "bazarr", input.Action, input.ParamsJSON, input.Confirm, actionCatalog,
		func(ctx context.Context, method, path string, query url.Values, payload any) ([]byte, error) {
			return client.request(ctx, method, path, query, payload)
		})
}
