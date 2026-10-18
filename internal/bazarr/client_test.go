package bazarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// spec: CONN-ARR-MCP-BAZARR-R002
func TestConfigValidation(t *testing.T) {
	for _, cfg := range []Config{
		{},
		{BaseURL: "ftp://localhost", APIKey: "key"},
		{BaseURL: "http://user:pass@localhost", APIKey: "key"},
		{BaseURL: "http://localhost/bazarr?x=1", APIKey: "key"},
		{BaseURL: "http://localhost#frag", APIKey: "key"},
		{BaseURL: "http://localhost"},
	} {
		if err := validateConfig(cfg); err == nil {
			t.Fatalf("invalid config accepted: %#v", cfg)
		}
	}
	if err := validateConfig(Config{BaseURL: "http://localhost:6767/bazarr", APIKey: "key"}); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

// spec: CONN-ARR-MCP-BAZARR-R002, CONN-ARR-MCP-BAZARR-R003
func TestAPIKeyPrefixAndStatusRedaction(t *testing.T) {
	const secret = "bazarr_test-secret"
	mux := http.NewServeMux()
	mux.HandleFunc("/prefix/api/system/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != secret {
			t.Errorf("missing X-API-KEY header")
		}
		_, _ = w.Write([]byte(`{"data":{"bazarr_version":"1.6.2","package_version":"LinuxServer","sonarr_version":"4.0","radarr_version":"6.0","bazarr_directory":"/secret/path","database_engine":"SQLite private","python_version":"3.12"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL + "/prefix", APIKey: secret})
	status, err := client.Status(context.Background())
	if err != nil || status.Data.BazarrVersion != "1.6.2" || status.Data.SonarrVersion != "4.0" {
		t.Fatalf("unexpected status: %#v, %v", status, err)
	}
	encoded, err := json.Marshal(StatusOutput{BazarrVersion: status.Data.BazarrVersion, PackageVersion: status.Data.PackageVersion, SonarrVersion: status.Data.SonarrVersion, RadarrVersion: status.Data.RadarrVersion})
	if err != nil {
		t.Fatal(err)
	}
	for _, sensitive := range []string{"secret/path", "SQLite private", "python_version"} {
		if strings.Contains(string(encoded), sensitive) {
			t.Fatalf("status leaked %q: %s", sensitive, encoded)
		}
	}
}

// spec: CONN-ARR-MCP-BAZARR-R003, CONN-ARR-MCP-BAZARR-R004, CONN-ARR-MCP-BAZARR-R005, CONN-ARR-MCP-BAZARR-R006
func TestReadEndpointsAndPaging(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/languages", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"English","code2":"en","code3":"eng","enabled":true}]`))
	})
	mux.HandleFunc("/api/system/languages/profiles", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"profileId":2,"name":"English","cutoff":0,"items":[{"language":"en","hi":false,"forced":false}]}]`))
	})
	mux.HandleFunc("/api/series", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start") != "5" || r.URL.Query().Get("length") != "25" {
			t.Errorf("unexpected series paging query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"data":[{"sonarrSeriesId":12,"title":"Show","tvdbId":34,"profileId":2}],"total":13}`))
	})
	mux.HandleFunc("/api/episodes", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("seriesid[]") != "12" {
			t.Errorf("unexpected series filter: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"data":[{"sonarrSeriesId":12,"sonarrEpisodeId":123,"season":1,"episode":2,"title":"Pilot"}]}`))
	})
	mux.HandleFunc("/api/episodes/wanted", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("length") != "10" {
			t.Errorf("unexpected wanted paging: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"data":[{"sonarrSeriesId":12,"sonarrEpisodeId":123,"seriesTitle":"Show","episodeTitle":"Pilot"}],"total":4}`))
	})
	mux.HandleFunc("/api/movies", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("radarrid[]") == "9" {
			_, _ = w.Write([]byte(`{"data":[{"radarrId":9,"title":"Film","profileId":2}],"total":1}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"radarrId":9,"title":"Film"}],"total":17}`))
	})
	mux.HandleFunc("/api/movies/wanted", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"radarrId":9,"title":"Film","missing_subtitles":[]}],"total":1}`))
	})
	mux.HandleFunc("/api/providers", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("history") != "true" {
			t.Errorf("unexpected providers query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"data":[{"name":"OpenSubtitles","status":"Good","retry":"-"}]}`))
	})
	mux.HandleFunc("/api/episodes/history", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("episodeid") != "123" {
			t.Errorf("unexpected episode history query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":1,"provider":"OpenSubtitles"}],"total":1}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	languages, err := client.Languages(context.Background())
	if err != nil || len(languages) != 1 || languages[0].Code2 != "en" {
		t.Fatalf("unexpected languages: %#v, %v", languages, err)
	}
	profiles, err := client.LanguageProfiles(context.Background())
	if err != nil || len(profiles) != 1 || profiles[0]["name"] != "English" {
		t.Fatalf("unexpected profiles: %#v, %v", profiles, err)
	}
	page, err := client.Series(context.Background(), urlValues("start", "5", "length", "25"))
	if err != nil || page.Total != 13 || page.Data[0].SeriesID != 12 {
		t.Fatalf("unexpected series page: %#v, %v", page, err)
	}
	episodes, err := client.Episodes(context.Background(), urlValues("seriesid[]", "12"))
	if err != nil || len(episodes) != 1 || episodes[0].EpisodeID != 123 {
		t.Fatalf("unexpected episodes: %#v, %v", episodes, err)
	}
	wantedEpisodes, err := client.WantedEpisodes(context.Background(), urlValues("start", "0", "length", "10"))
	if err != nil || wantedEpisodes.Total != 4 || wantedEpisodes.Data[0].SeriesID != 12 {
		t.Fatalf("unexpected wanted episodes: %#v, %v", wantedEpisodes, err)
	}
	movies, err := client.Movies(context.Background(), urlValues("radarrid[]", "9"))
	if err != nil || movies.Total != 1 || movies.Data[0].MovieID != 9 {
		t.Fatalf("unexpected movie page: %#v, %v", movies, err)
	}
	wantedMovies, err := client.WantedMovies(context.Background(), urlValues("start", "0", "length", "25"))
	if err != nil || wantedMovies.Total != 1 || wantedMovies.Data[0].MovieID != 9 {
		t.Fatalf("unexpected wanted movies: %#v, %v", wantedMovies, err)
	}
	providers, err := client.Providers(context.Background(), urlValues("history", "true"))
	if err != nil || len(providers) != 1 || providers[0].Name != "OpenSubtitles" {
		t.Fatalf("unexpected providers: %#v, %v", providers, err)
	}
	history, err := client.History(context.Background(), "episode", urlValues("episodeid", "123", "start", "0", "length", "25"))
	if err != nil || history.Total != 1 || history.Data[0]["id"] != float64(1) {
		t.Fatalf("unexpected history: %#v, %v", history, err)
	}
}

// spec: CONN-ARR-MCP-BAZARR-R007, CONN-ARR-MCP-BAZARR-R008
func TestWriteEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/series", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if r.URL.Query().Get("seriesid") != "12" || r.URL.Query().Get("profileid") != "none" {
				t.Errorf("unexpected series profile query: %s", r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Query().Get("seriesid") != "12" || r.URL.Query().Get("action") != "search-missing" {
			t.Errorf("unexpected series search query: %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/movies", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("radarrid") != "9" || r.URL.Query().Get("profileid") != "3" {
			t.Errorf("unexpected movie profile query: %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/episodes/subtitles", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Method != http.MethodPatch || q.Get("seriesid") != "12" || q.Get("episodeid") != "123" || q.Get("language") != "pt-BR" || q.Get("forced") != "true" || q.Get("hi") != "false" {
			t.Errorf("unexpected episode download request: %s %s", r.Method, r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	if err := client.SetProfile(context.Background(), "series", 12, "none"); err != nil {
		t.Fatal(err)
	}
	if err := client.SetProfile(context.Background(), "movie", 9, "3"); err != nil {
		t.Fatal(err)
	}
	if err := client.SearchSubtitles(context.Background(), "series", 12); err != nil {
		t.Fatal(err)
	}
	if err := client.DownloadEpisodeSubtitle(context.Background(), 12, 123, "pt-BR", true, false); err != nil {
		t.Fatal(err)
	}
}

// spec: CONN-ARR-MCP-BAZARR-R010
func TestCancellationTLSRedirectAndRedaction(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	if client.http.Timeout != 15*time.Second || client.http.Transport != nil || client.http.CheckRedirect == nil {
		t.Fatal("client must retain timeout, default TLS transport, and redirect protection")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := client.Languages(ctx)
		done <- err
	}()
	<-started
	cancel()
	if err := <-done; err == nil {
		t.Fatal("cancelled request unexpectedly succeeded")
	}

	leaked := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != "" {
			leaked++
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirector.Close()
	redirectClient := NewClient(Config{BaseURL: redirector.URL, APIKey: "key"})
	if _, err := redirectClient.Languages(context.Background()); err == nil {
		t.Fatal("redirected request unexpectedly succeeded")
	}
	if leaked != 0 {
		t.Fatal("API key was forwarded to redirect target")
	}
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "secret=do-not-leak", http.StatusInternalServerError)
	}))
	defer errorServer.Close()
	_, err := NewClient(Config{BaseURL: errorServer.URL, APIKey: "key"}).Languages(context.Background())
	if err == nil || strings.Contains(err.Error(), "do-not-leak") || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("upstream error not sanitized: %v", err)
	}
}

func urlValues(pairs ...string) url.Values {
	values := url.Values{}
	for i := 0; i < len(pairs); i += 2 {
		values.Set(pairs[i], pairs[i+1])
	}
	return values
}
