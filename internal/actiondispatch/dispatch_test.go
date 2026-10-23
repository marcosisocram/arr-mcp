package actiondispatch

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"testing"
)

func TestListActionsAndConfirmedRequest(t *testing.T) {
	actions := []Action{{Name: "delete_movie_id", Method: "DELETE", Path: "/movie/{id}", PathParams: []string{"id"}, QueryParams: map[string]string{}, BodyParams: map[string]string{}, Defaults: map[string]any{}, Destructive: true}}
	requests := 0
	request := func(_ context.Context, method, path string, query url.Values, payload any) ([]byte, error) {
		requests++
		if method != "DELETE" || path != "/movie/42" || len(query) != 0 || payload != nil {
			t.Fatalf("unexpected request: %s %s %v %#v", method, path, query, payload)
		}
		return []byte(`{"status":"deleted"}`), nil
	}
	result, err := Execute(context.Background(), "radarr", "list_actions", "{}", false, actions, request)
	if err != nil || !reflect.DeepEqual(result.Actions, []string{"delete_movie_id"}) || result.Service != "radarr" {
		t.Fatalf("list actions = %#v, %v", result, err)
	}
	result, err = Execute(context.Background(), "radarr", "delete_movie_id", `{"id":42}`, false, actions, request)
	if err != nil || !result.Cancelled || result.Operation != "delete_movie_id" || requests != 0 {
		t.Fatalf("unconfirmed action = %#v, %v; requests %d", result, err, requests)
	}
	result, err = Execute(context.Background(), "radarr", "delete_movie_id", `{"id":42}`, true, actions, request)
	if err != nil || result.Result == nil || requests != 1 {
		t.Fatalf("confirmed action = %#v, %v; requests %d", result, err, requests)
	}
}

func TestMappedQueryAndBodyAndValidation(t *testing.T) {
	action := Action{
		Name: "post_example", Method: "POST", Path: "/example/{id}",
		PathParams: []string{"id"}, QueryParams: map[string]string{"ids": "ids", "enabled": "enabled"},
		BodyParams: map[string]string{"title": "movieTitle"}, Defaults: map[string]any{"enabled": true},
	}
	request := func(_ context.Context, method, path string, query url.Values, payload any) ([]byte, error) {
		if method != "POST" || path != "/example/a%2Fb" || !reflect.DeepEqual(query["ids"], []string{"1", "2"}) || query.Get("enabled") != "true" {
			t.Fatalf("request = %s %s %v", method, path, query)
		}
		if !reflect.DeepEqual(payload, map[string]any{"movieTitle": "Example"}) {
			t.Fatalf("payload = %#v", payload)
		}
		return []byte(`{}`), nil
	}
	if _, err := Execute(context.Background(), "radarr", action.Name, `{"id":"a/b","ids":[1,2],"title":"Example"}`, true, []Action{action}, request); err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(context.Background(), "radarr", action.Name, `{"id":1,"bogus":true}`, true, []Action{action}, request); err == nil {
		t.Fatal("unknown parameter was accepted")
	}
	if _, err := Execute(context.Background(), "radarr", action.Name, `[]`, true, []Action{action}, request); err == nil {
		t.Fatal("non-object JSON was accepted")
	}
}

func TestRequestErrorsPropagate(t *testing.T) {
	want := errors.New("request failed")
	action := Action{Name: "get_status", Method: "GET", Path: "/system/status"}
	_, err := Execute(context.Background(), "bazarr", action.Name, "{}", true, []Action{action}, func(context.Context, string, string, url.Values, any) ([]byte, error) {
		return nil, want
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestSensitiveResponseFieldsAreRedacted(t *testing.T) {
	action := Action{Name: "get_settings", Method: "GET", Path: "/settings"}
	result, err := Execute(context.Background(), "radarr", action.Name, "{}", false, []Action{action}, func(context.Context, string, string, url.Values, any) ([]byte, error) {
		return []byte(`{"apiKey":"key","downloadClient":{"password":"pw","name":"client"},"nested":[{"access_token":"token"}]}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"apiKey":         "[REDACTED]",
		"downloadClient": map[string]any{"password": "[REDACTED]", "name": "client"},
		"nested":         []any{map[string]any{"access_token": "[REDACTED]"}},
	}
	if !reflect.DeepEqual(result.Result, want) {
		t.Fatalf("result = %#v, want %#v", result.Result, want)
	}
}

func TestVariantRoute(t *testing.T) {
	action := Action{
		Name: "search_subtitles", Method: "POST", Path: "/episodes/search",
		PathParams: []string{}, QueryParams: map[string]string{},
		BodyParams: map[string]string{"episode_id": "episodeId"},
		Defaults:   map[string]any{},
		Variant:    &Variant{Parameter: "episode_id", Truthy: true, IgnoreParams: []string{"series_id"}, Otherwise: &VariantRoute{Path: "/series/search", BodyParams: map[string]string{"series_id": "seriesId"}}},
		Required:   []string{"series_id"},
	}
	requests := 0
	request := func(_ context.Context, method, path string, _ url.Values, payload any) ([]byte, error) {
		requests++
		if method != "POST" {
			t.Fatalf("method = %s", method)
		}
		switch requests {
		case 1:
			if path != "/series/search" || !reflect.DeepEqual(payload, map[string]any{"seriesId": json.Number("7")}) {
				t.Fatalf("series route = %s %#v", path, payload)
			}
		case 2:
			if path != "/episodes/search" || !reflect.DeepEqual(payload, map[string]any{"episodeId": json.Number("23")}) {
				t.Fatalf("episode route = %s %#v", path, payload)
			}
		}
		return []byte(`{"ok":true}`), nil
	}
	if _, err := Execute(context.Background(), "bazarr", action.Name, `{"series_id":7}`, true, []Action{action}, request); err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(context.Background(), "bazarr", action.Name, `{"series_id":7,"episode_id":23}`, true, []Action{action}, request); err != nil {
		t.Fatal(err)
	}
}
