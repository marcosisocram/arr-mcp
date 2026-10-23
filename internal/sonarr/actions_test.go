package sonarr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompositeAddSeriesRequiresConfirmationAndUsesExpectedAPI(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("X-Api-Key") != "secret" {
			t.Errorf("API key header missing")
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/series/lookup":
			if r.URL.Query().Get("term") != "the office" {
				t.Errorf("lookup query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"title":"The Office","tvdbId":73244,"year":2005,"titleSlug":"the-office","images":[]}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/series":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode add body: %v", err)
			}
			if body["tvdbId"] != float64(73244) || body["qualityProfileId"] != float64(4) || body["rootFolderPath"] != "/tv" {
				t.Errorf("add body = %#v", body)
			}
			options := body["addOptions"].(map[string]any)
			if options["searchForMissingEpisodes"] != true {
				t.Errorf("default search option = %#v", options)
			}
			_, _ = w.Write([]byte(`{"id":15,"title":"The Office"}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "secret"})
	params := `{"term":"the office","root_folder_path":"/tv","quality_profile_id":4}`
	result, err := dispatchAction(t.Context(), client, ActionInput{Action: "add_series", ParamsJSON: params})
	if err != nil || !result.Cancelled || requests != 0 {
		t.Fatalf("without confirmation = %#v, %v; requests=%d", result, err, requests)
	}
	result, err = dispatchAction(t.Context(), client, ActionInput{Action: "add_series", Confirm: true, ParamsJSON: params})
	if err != nil || result.Result == nil || requests != 2 {
		t.Fatalf("confirmed add = %#v, %v; requests=%d", result, err, requests)
	}
}
