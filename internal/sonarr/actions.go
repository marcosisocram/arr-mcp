package sonarr

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/marcosisocram/arr-mcp/internal/actiondispatch"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed actions.json
var actionCatalogJSON []byte

var actionCatalog = mustLoadActionCatalog()

func mustLoadActionCatalog() []actiondispatch.Action {
	var actions []actiondispatch.Action
	if err := json.Unmarshal(actionCatalogJSON, &actions); err != nil {
		panic("decode embedded Sonarr action catalog: " + err.Error())
	}
	return actions
}

type ActionInput struct {
	Action     string `json:"action" jsonschema:"Sonarr API client action name; use list_actions to discover the complete domain catalog."`
	ParamsJSON string `json:"params_json,omitempty" jsonschema:"JSON object containing the action's parameters."`
	Confirm    bool   `json:"confirm,omitempty" jsonschema:"Required for every mutating or destructive action; false means no upstream request is made."`
}

func registerActionTool(server *mcp.Server, client *Client) {
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_action", Description: "Execute one allowlisted Sonarr domain API action. Use action=list_actions to discover actions. Mutating actions require confirm=true.", OutputSchema: actiondispatch.OutputSchema()}, func(ctx context.Context, _ *mcp.CallToolRequest, input ActionInput) (*mcp.CallToolResult, actiondispatch.Result, error) {
		result, err := dispatchAction(ctx, client, input)
		return nil, result, err
	})
}

func dispatchAction(ctx context.Context, client *Client, input ActionInput) (actiondispatch.Result, error) {
	if input.Action == "lookup_series" || input.Action == "add_series" {
		return dispatchSeriesComposite(ctx, client, input)
	}
	return actiondispatch.Execute(ctx, "sonarr", input.Action, input.ParamsJSON, input.Confirm, actionCatalog,
		func(ctx context.Context, method, path string, query url.Values, payload any) ([]byte, error) {
			return client.request(ctx, method, path, query, payload)
		})
}

func dispatchSeriesComposite(ctx context.Context, client *Client, input ActionInput) (actiondispatch.Result, error) {
	if input.Action == "add_series" && !input.Confirm {
		return actiondispatch.Result{Cancelled: true, Operation: input.Action}, nil
	}
	if len(input.ParamsJSON) > 64<<10 {
		return actiondispatch.Result{}, fmt.Errorf("params_json exceeds the 65536-byte limit")
	}
	var params struct {
		Term                     string `json:"term"`
		RootFolderPath           string `json:"root_folder_path"`
		QualityProfileID         int    `json:"quality_profile_id"`
		Monitored                *bool  `json:"monitored"`
		SearchForMissingEpisodes *bool  `json:"search_for_missing_episodes"`
	}
	decoder := json.NewDecoder(strings.NewReader(input.ParamsJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&params); err != nil {
		return actiondispatch.Result{}, fmt.Errorf("params_json is invalid for action %q", input.Action)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return actiondispatch.Result{}, fmt.Errorf("params_json must contain one JSON object")
	}
	if strings.TrimSpace(params.Term) == "" {
		return actiondispatch.Result{}, fmt.Errorf("term is required")
	}
	lookupBody, err := client.request(ctx, "GET", "/series/lookup", url.Values{"term": {params.Term}}, nil)
	if err != nil {
		return actiondispatch.Result{}, err
	}
	var candidates []map[string]any
	if err := json.Unmarshal(lookupBody, &candidates); err != nil {
		return actiondispatch.Result{}, fmt.Errorf("decode Sonarr series lookup response")
	}
	if input.Action == "lookup_series" {
		if params.RootFolderPath != "" || params.QualityProfileID != 0 || params.Monitored != nil || params.SearchForMissingEpisodes != nil {
			return actiondispatch.Result{}, fmt.Errorf("lookup_series accepts only term")
		}
		return actiondispatch.Result{Result: map[string]any{"result": candidates}}, nil
	}
	if len(candidates) == 0 {
		return actiondispatch.Result{Result: map[string]string{"error": fmt.Sprintf("No series found for term: %s", params.Term)}}, nil
	}
	if params.RootFolderPath == "" || params.QualityProfileID <= 0 {
		return actiondispatch.Result{}, fmt.Errorf("root_folder_path and positive quality_profile_id are required")
	}
	monitored := true
	if params.Monitored != nil {
		monitored = *params.Monitored
	}
	search := true
	if params.SearchForMissingEpisodes != nil {
		search = *params.SearchForMissingEpisodes
	}
	series := candidates[0]
	payload := map[string]any{
		"title": series["title"], "qualityProfileId": params.QualityProfileID,
		"rootFolderPath": params.RootFolderPath, "monitored": monitored,
		"tvdbId": series["tvdbId"], "year": series["year"], "titleSlug": series["titleSlug"],
		"images": series["images"], "addOptions": map[string]any{"searchForMissingEpisodes": search},
	}
	if payload["images"] == nil {
		payload["images"] = []any{}
	}
	body, err := client.request(ctx, "POST", "/series", nil, payload)
	if err != nil {
		return actiondispatch.Result{}, err
	}
	var created any
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &created); err != nil {
			return actiondispatch.Result{}, fmt.Errorf("decode Sonarr add series response")
		}
	}
	return actiondispatch.Result{Result: created}, nil
}
