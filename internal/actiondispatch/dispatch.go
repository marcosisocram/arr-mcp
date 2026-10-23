package actiondispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

const maxInputBytes = 64 << 10

type Action struct {
	Name        string            `json:"name"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	PathParams  []string          `json:"path_params"`
	QueryParams map[string]string `json:"query_params"`
	BodyParams  map[string]string `json:"body_params"`
	BodyDirect  string            `json:"body_direct"`
	Defaults    map[string]any    `json:"defaults"`
	Required    []string          `json:"required_params"`
	Destructive bool              `json:"destructive"`
	Composite   bool              `json:"composite"`
	Variant     *Variant          `json:"variant,omitempty"`
}

type Variant struct {
	Parameter    string        `json:"parameter"`
	Truthy       bool          `json:"truthy"`
	IgnoreParams []string      `json:"ignore_params"`
	Otherwise    *VariantRoute `json:"otherwise"`
}

type VariantRoute struct {
	Path       string            `json:"path"`
	BodyParams map[string]string `json:"body_params"`
}

type Result struct {
	Result    any      `json:"result,omitempty"`
	Cancelled bool     `json:"cancelled,omitempty"`
	Operation string   `json:"operation,omitempty"`
	Actions   []string `json:"actions,omitempty"`
	Service   string   `json:"service,omitempty"`
}

// OutputSchema describes the fixed action envelope while allowing result to
// contain any JSON value using an object-form schema accepted by strict clients.
func OutputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"result": {AnyOf: []*jsonschema.Schema{
				{Type: "object"},
				{Type: "array"},
				{Type: "string"},
				{Type: "number"},
				{Type: "integer"},
				{Type: "boolean"},
				{Type: "null"},
			}},
			"cancelled": {Type: "boolean"},
			"operation": {Type: "string"},
			"actions":   {Type: "array", Items: &jsonschema.Schema{Type: "string"}},
			"service":   {Type: "string"},
		},
	}
}

type RequestFunc func(context.Context, string, string, url.Values, any) ([]byte, error)

func Names(actions []Action) []string {
	names := make([]string, 0, len(actions))
	for _, action := range actions {
		names = append(names, action.Name)
	}
	sort.Strings(names)
	return names
}

func Execute(ctx context.Context, service, actionName, paramsJSON string, confirm bool, actions []Action, request RequestFunc) (Result, error) {
	if actionName == "list_actions" || actionName == "actions" || actionName == "help" {
		return Result{Service: service, Actions: Names(actions)}, nil
	}
	var action *Action
	for i := range actions {
		if actions[i].Name == actionName {
			action = &actions[i]
			break
		}
	}
	if action == nil {
		return Result{}, unknownAction(actionName, actions)
	}
	if action.Destructive && !confirm {
		return Result{Cancelled: true, Operation: action.Name}, nil
	}
	if len(paramsJSON) > maxInputBytes {
		return Result{}, fmt.Errorf("params_json exceeds the %d-byte limit", maxInputBytes)
	}
	params := map[string]any{}
	if strings.TrimSpace(paramsJSON) != "" {
		decoder := json.NewDecoder(strings.NewReader(paramsJSON))
		decoder.UseNumber()
		if err := decoder.Decode(&params); err != nil || params == nil {
			return Result{}, fmt.Errorf("params_json must be a JSON object")
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return Result{}, fmt.Errorf("params_json must contain one JSON object")
		}
	}
	path := action.Path
	bodyParams := action.BodyParams
	if action.Variant != nil && truthy(params[action.Variant.Parameter]) != action.Variant.Truthy {
		if action.Variant.Otherwise == nil {
			return Result{}, fmt.Errorf("invalid variant metadata for action %q", action.Name)
		}
		path = action.Variant.Otherwise.Path
		bodyParams = action.Variant.Otherwise.BodyParams
	}

	allowed := map[string]bool{}
	for _, name := range action.PathParams {
		allowed[name] = true
	}
	for name := range action.QueryParams {
		allowed[name] = true
	}
	for name := range action.BodyParams {
		allowed[name] = true
	}
	for name := range bodyParams {
		allowed[name] = true
	}
	if action.Variant != nil && action.Variant.Otherwise != nil {
		for name := range action.Variant.Otherwise.BodyParams {
			allowed[name] = true
		}
	}
	if action.BodyDirect != "" {
		allowed[action.BodyDirect] = true
	}
	for name := range action.Defaults {
		allowed[name] = true
	}
	for name := range params {
		if name == "confirm" || !allowed[name] {
			return Result{}, fmt.Errorf("unknown parameter %q for action %q", name, action.Name)
		}
	}
	for name, value := range action.Defaults {
		if _, exists := params[name]; !exists && value != nil {
			params[name] = value
		}
	}
	for _, name := range action.Required {
		if value, exists := params[name]; !exists || value == nil {
			return Result{}, fmt.Errorf("parameter %q is required for action %q", name, action.Name)
		}
	}

	for _, name := range action.PathParams {
		value, exists := params[name]
		if !exists || value == nil || fmt.Sprint(value) == "" {
			return Result{}, fmt.Errorf("parameter %q is required for action %q", name, action.Name)
		}
		path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(fmt.Sprint(value)))
		delete(params, name)
	}
	query := url.Values{}
	for name, wireName := range action.QueryParams {
		value, exists := params[name]
		if !exists || value == nil {
			continue
		}
		addQueryValues(query, wireName, value)
		delete(params, name)
	}

	var payload any
	if action.BodyDirect != "" {
		payload = params[action.BodyDirect]
		delete(params, action.BodyDirect)
	} else if len(bodyParams) > 0 {
		body := map[string]any{}
		for name, wireName := range bodyParams {
			if value, exists := params[name]; exists && value != nil {
				body[wireName] = value
				delete(params, name)
			}
		}
		payload = body
	}
	if action.Variant != nil {
		for _, name := range action.Variant.IgnoreParams {
			delete(params, name)
		}
	}
	if len(params) > 0 {
		return Result{}, fmt.Errorf("unsupported parameter(s) for action %q", action.Name)
	}

	body, err := request(ctx, action.Method, path, query, payload)
	if err != nil {
		return Result{}, err
	}
	var result any
	if len(bytes.TrimSpace(body)) == 0 {
		result = map[string]string{"status": "success"}
	} else if err := json.Unmarshal(body, &result); err != nil {
		return Result{}, fmt.Errorf("decode %s action %q response", service, action.Name)
	}
	result = redactSecrets(result)
	if service != "bazarr" {
		if _, isArray := result.([]any); isArray {
			result = map[string]any{"result": result}
		}
	}
	return Result{Result: result}, nil
}

func redactSecrets(value any) any {
	switch item := value.(type) {
	case map[string]any:
		for key, nested := range item {
			name := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
			if strings.Contains(name, "apikey") || strings.Contains(name, "password") || strings.Contains(name, "secret") || strings.Contains(name, "token") || strings.Contains(name, "authorization") || strings.Contains(name, "cookie") {
				if nested != nil && nested != "" {
					item[key] = "[REDACTED]"
				}
				continue
			}
			item[key] = redactSecrets(nested)
		}
	case []any:
		for i, nested := range item {
			item[i] = redactSecrets(nested)
		}
	}
	return value
}

func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case json.Number:
		return v != "0"
	default:
		return true
	}
}

func addQueryValues(query url.Values, name string, value any) {
	switch items := value.(type) {
	case []any:
		for _, item := range items {
			query.Add(name, fmt.Sprint(item))
		}
	default:
		query.Add(name, fmt.Sprint(value))
	}
}

func unknownAction(name string, actions []Action) error {
	valid := Names(actions)
	var suggestions []string
	for _, candidate := range valid {
		if strings.HasPrefix(candidate, name) || strings.HasPrefix(name, candidate) {
			suggestions = append(suggestions, candidate)
			if len(suggestions) == 3 {
				break
			}
		}
	}
	if len(suggestions) > 0 {
		return fmt.Errorf("unknown action %q; possible actions: %s; use action=\"list_actions\" to discover all %d", name, strings.Join(suggestions, ", "), len(valid))
	}
	return fmt.Errorf("unknown action %q; use action=\"list_actions\" to discover all %d", name, len(valid))
}
