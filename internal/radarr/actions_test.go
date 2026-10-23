package radarr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCompositeAddMovieRequiresConfirmationAndUsesExpectedAPI(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("X-Api-Key") != "secret" {
			t.Errorf("API key header missing")
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/movie/lookup":
			if r.URL.Query().Get("term") != "matrix" {
				t.Errorf("lookup query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"title":"The Matrix","tmdbId":603,"year":1999,"titleSlug":"the-matrix","images":[]}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/movie":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode add body: %v", err)
			}
			if body["tmdbId"] != float64(603) || body["qualityProfileId"] != float64(3) || body["rootFolderPath"] != "/movies" {
				t.Errorf("add body = %#v", body)
			}
			options := body["addOptions"].(map[string]any)
			if options["searchForMovie"] != true {
				t.Errorf("default search option = %#v", options)
			}
			_, _ = w.Write([]byte(`{"id":10,"title":"The Matrix"}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "secret"})

	result, err := dispatchAction(t.Context(), client, ActionInput{Action: "add_movie", ParamsJSON: `{"term":"matrix","root_folder_path":"/movies","quality_profile_id":3}`})
	if err != nil || !result.Cancelled || requests != 0 {
		t.Fatalf("without confirmation = %#v, %v; requests=%d", result, err, requests)
	}
	result, err = dispatchAction(t.Context(), client, ActionInput{Action: "add_movie", Confirm: true, ParamsJSON: `{"term":"matrix","root_folder_path":"/movies","quality_profile_id":3}`})
	if err != nil || result.Result == nil || requests != 2 {
		t.Fatalf("confirmed add = %#v, %v; requests=%d", result, err, requests)
	}
	if !reflect.DeepEqual(result.Result, map[string]any{"id": float64(10), "title": "The Matrix"}) {
		t.Fatalf("result = %#v", result.Result)
	}
}
