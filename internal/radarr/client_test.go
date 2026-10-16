package radarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// spec: CONN-ARR-MCP-RADARR-R002
func TestConfigValidation(t *testing.T) {
	for _, cfg := range []Config{
		{},
		{BaseURL: "ftp://localhost", APIKey: "key"},
		{BaseURL: "http://user:pass@localhost", APIKey: "key"},
		{BaseURL: "http://localhost/radarr?x=1", APIKey: "key"},
		{BaseURL: "http://localhost#frag", APIKey: "key"},
		{BaseURL: "http://localhost"},
	} {
		if err := validateConfig(cfg); err == nil {
			t.Fatalf("invalid config accepted: %#v", cfg)
		}
	}
	if err := validateConfig(Config{BaseURL: "http://localhost:7878/radarr", APIKey: "key"}); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

// spec: CONN-ARR-MCP-RADARR-R002, CONN-ARR-MCP-RADARR-R003
func TestAPIKeyAndMovieReads(t *testing.T) {
	const secret = "radarr_test-secret"
	mux := http.NewServeMux()
	mux.HandleFunc("/radarr/api/v3/movie", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != secret {
			t.Errorf("missing X-Api-Key header")
		}
		if r.URL.Query().Get("tmdbId") != "" {
			t.Errorf("unexpected tmdbId filter: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":7,"title":"The Matrix","year":1999,"tmdbId":603,"monitored":true,"hasFile":false,"qualityProfileId":3,"rootFolderPath":"/movies","sizeOnDisk":0}]`))
	})
	mux.HandleFunc("/radarr/api/v3/movie/7", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":7,"title":"The Matrix","year":1999,"tmdbId":603,"monitored":true,"hasFile":true,"status":"released"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL + "/radarr", APIKey: secret})
	movies, err := client.ListMovies(context.Background())
	if err != nil || len(movies) != 1 || movies[0].Title != "The Matrix" || movies[0].TmdbID != 603 {
		t.Fatalf("unexpected list result: %#v, %v", movies, err)
	}
	movie, err := client.MovieByID(context.Background(), 7)
	if err != nil || !movie.HasFile || movie.Status != "released" {
		t.Fatalf("unexpected movie result: %#v, %v", movie, err)
	}
}

// spec: CONN-ARR-MCP-RADARR-R004
func TestLookupProfilesAndRootFolders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/movie/lookup", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("term") != "matrix" {
			t.Errorf("unexpected lookup term: %s", r.URL.Query().Get("term"))
		}
		_, _ = w.Write([]byte(`[{"title":"The Matrix","year":1999,"tmdbId":603,"imdbId":"tt0133093"}]`))
	})
	mux.HandleFunc("/api/v3/qualityprofile", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":3,"name":"HD-1080p","upgradeAllowed":true,"cutoff":7}]`))
	})
	mux.HandleFunc("/api/v3/rootfolder", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":1,"path":"/movies","accessible":true,"freeSpace":1024}]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	movies, err := client.LookupMovies(context.Background(), "matrix")
	if err != nil || len(movies) != 1 || movies[0].TmdbID != 603 {
		t.Fatalf("unexpected lookup result: %#v, %v", movies, err)
	}
	profiles, err := client.QualityProfiles(context.Background())
	if err != nil || len(profiles) != 1 || profiles[0].Name != "HD-1080p" || !profiles[0].UpgradeAllowed {
		t.Fatalf("unexpected profile result: %#v, %v", profiles, err)
	}
	folders, err := client.RootFolders(context.Background())
	if err != nil || len(folders) != 1 || folders[0].Path != "/movies" || folders[0].FreeSpace != 1024 {
		t.Fatalf("unexpected root folder result: %#v, %v", folders, err)
	}
}

// spec: CONN-ARR-MCP-RADARR-R005
func TestAddMovieIsDuplicateSafe(t *testing.T) {
	var posted map[string]any
	existing := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/movie", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if r.URL.Query().Get("tmdbId") != "603" {
				t.Errorf("unexpected tmdbId query: %s", r.URL.RawQuery)
			}
			if existing {
				_, _ = w.Write([]byte(`[{"id":7,"title":"The Matrix","tmdbId":603,"monitored":true}]`))
				return
			}
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
			t.Errorf("decode add body: %v", err)
		}
		existing = true
		_, _ = w.Write([]byte(`{"id":7,"title":"The Matrix","tmdbId":603,"monitored":true,"qualityProfileId":3,"rootFolderPath":"/movies"}`))
	})
	mux.HandleFunc("/api/v3/movie/lookup/tmdb", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("tmdbId") != "603" {
			t.Errorf("unexpected lookup tmdbId: %s", r.URL.Query().Get("tmdbId"))
		}
		_, _ = w.Write([]byte(`{"title":"The Matrix","year":1999,"tmdbId":603,"titleSlug":"the-matrix-603"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	input := AddMovieInput{TmdbID: 603, QualityProfileID: 3, RootFolderPath: "/movies"}
	first, err := addMovie(context.Background(), client, input)
	if err != nil || first.Status != "created" || first.Movie.ID != 7 {
		t.Fatalf("unexpected add result: %#v, %v", first, err)
	}
	options, ok := posted["addOptions"].(map[string]any)
	if !ok || options["searchForMovie"] != false || options["monitor"] != "movieOnly" {
		t.Fatalf("unexpected addOptions: %#v", posted["addOptions"])
	}
	if posted["title"] != "The Matrix" || posted["minimumAvailability"] != "released" || posted["monitored"] != true {
		t.Fatalf("add body lost lookup metadata or defaults: %#v", posted)
	}
	if posted["qualityProfileId"] != float64(3) || posted["rootFolderPath"] != "/movies" {
		t.Fatalf("add body lost the explicit profile/folder: %#v", posted)
	}
	second, err := addMovie(context.Background(), client, input)
	if err != nil || second.Status != "already_exists" || second.Movie.ID != 7 {
		t.Fatalf("duplicate add was not detected: %#v, %v", second, err)
	}
}

// spec: CONN-ARR-MCP-RADARR-R006, CONN-ARR-MCP-RADARR-R009
func TestMutationBodiesAndErrorRedaction(t *testing.T) {
	var putBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/movie/7", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":7,"title":"Kept","year":1999,"tmdbId":603,"monitored":true,"qualityProfileId":3,"rootFolderPath":"/movies"}`))
		case http.MethodPut:
			if r.URL.Query().Get("moveFiles") != "false" {
				t.Errorf("unexpected moveFiles: %s", r.URL.Query().Get("moveFiles"))
			}
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Errorf("decode PUT body: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":7,"title":"Kept","year":1999,"tmdbId":603,"monitored":false,"qualityProfileId":3}`))
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	})
	mux.HandleFunc("/api/v3/system/status", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "apiKey=do-not-leak", http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "secret"})
	movie, err := setMovieMonitored(context.Background(), client, 7, false)
	if err != nil || movie.Monitored {
		t.Fatalf("unexpected monitored result: %#v, %v", movie, err)
	}
	if putBody["title"] != "Kept" || putBody["qualityProfileId"] != float64(3) || putBody["tmdbId"] != float64(603) {
		t.Fatalf("update dropped existing fields: %#v", putBody)
	}
	if monitored, ok := putBody["monitored"]; !ok || monitored != false {
		t.Fatalf("update did not set monitored: %#v", putBody)
	}
	_, err = client.request(context.Background(), http.MethodGet, "/system/status", nil, nil)
	if err == nil {
		t.Fatal("upstream failure unexpectedly succeeded")
	}
	if message := err.Error(); strings.Contains(message, "do-not-leak") || !strings.Contains(message, "HTTP 500") {
		t.Fatalf("upstream error was not safely sanitized: %v", err)
	}
}

// spec: CONN-ARR-MCP-RADARR-R007, CONN-ARR-MCP-RADARR-R008
func TestSearchQueueAndHistory(t *testing.T) {
	var command map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/command", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
			t.Errorf("decode command body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":42,"name":"MoviesSearch","status":"queued"}`))
	})
	mux.HandleFunc("/api/v3/command/42", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":42,"name":"MoviesSearch","status":"completed","result":"successful"}`))
	})
	mux.HandleFunc("/api/v3/queue", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "1" || r.URL.Query().Get("pageSize") != "25" {
			t.Errorf("unexpected queue paging: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"page":1,"pageSize":25,"totalRecords":1,"records":[{"id":1,"movieId":7,"title":"The Matrix","status":"downloading","size":100.5,"protocol":"torrent"}]}`))
	})
	mux.HandleFunc("/api/v3/history", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"page":1,"pageSize":25,"totalRecords":1,"records":[{"id":5,"movieId":7,"sourceTitle":"The.Matrix.1999","eventType":"grabbed","date":"2026-10-15T00:00:00Z"}]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	query, err := pageQuery(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	search, err := client.SearchMovies(context.Background(), 7)
	if err != nil || search.ID != 42 || search.Status != "queued" {
		t.Fatalf("unexpected search command: %#v, %v", search, err)
	}
	if name, ok := command["name"].(string); !ok || name != "MoviesSearch" {
		t.Fatalf("unexpected command payload: %#v", command)
	}
	ids, ok := command["movieIds"].([]any)
	if !ok || len(ids) != 1 || ids[0] != float64(7) {
		t.Fatalf("unexpected command movieIds: %#v", command["movieIds"])
	}
	tracked, err := client.CommandByID(context.Background(), 42)
	if err != nil || tracked.Status != "completed" || tracked.Result != "successful" {
		t.Fatalf("unexpected tracked command: %#v, %v", tracked, err)
	}
	queue, err := client.Queue(context.Background(), query)
	if err != nil || queue.TotalRecords != 1 || queue.Records[0].Title != "The Matrix" {
		t.Fatalf("unexpected queue: %#v, %v", queue, err)
	}
	history, err := client.History(context.Background(), query)
	if err != nil || history.Records[0].EventType != "grabbed" {
		t.Fatalf("unexpected history: %#v, %v", history, err)
	}
}

// spec: CONN-ARR-MCP-RADARR-R009
func TestCancellationAndTLSDefaults(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	if client.http.Timeout != 15*time.Second {
		t.Fatalf("unexpected request timeout: %s", client.http.Timeout)
	}
	if client.http.Transport != nil {
		t.Fatal("client must retain the standard transport and TLS verification defaults")
	}
	if client.http.CheckRedirect == nil {
		t.Fatal("client must refuse to follow redirects")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := client.ListMovies(ctx)
		done <- err
	}()
	<-started
	cancel()
	if err := <-done; err == nil {
		t.Fatal("cancelled request unexpectedly succeeded")
	}
}

// spec: CONN-ARR-MCP-RADARR-R002, CONN-ARR-MCP-RADARR-R009
func TestNoRedirectLeaksAPIKey(t *testing.T) {
	leaked := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "" {
			leaked++
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirector.Close()
	client := NewClient(Config{BaseURL: redirector.URL, APIKey: "key"})
	if _, err := client.ListMovies(context.Background()); err == nil {
		t.Fatal("redirected request unexpectedly succeeded")
	}
	if leaked != 0 {
		t.Fatal("API key was forwarded across a redirect")
	}
}

// spec: CONN-ARR-MCP-RADARR-R003, CONN-ARR-MCP-RADARR-R005, CONN-ARR-MCP-RADARR-R008
func TestToolValidation(t *testing.T) {
	if _, err := pageQuery(0, 101); err == nil {
		t.Fatal("oversized page accepted")
	}
	if _, err := pageQuery(0, -1); err == nil {
		t.Fatal("negative page size accepted")
	}
	if _, err := pageQuery(-1, 10); err == nil {
		t.Fatal("negative page accepted")
	}
	query, err := pageQuery(0, 0)
	if err != nil || query.Get("page") != "1" || query.Get("pageSize") != "25" {
		t.Fatalf("default paging rejected: %v, %v", query, err)
	}
	movies := []Movie{{ID: 1, Monitored: true, HasFile: false}, {ID: 2, Monitored: false, HasFile: true}}
	monitored := true
	filtered, err := movieListFilter(movies, &monitored, nil, 0, 0)
	if err != nil || len(filtered) != 1 || filtered[0].ID != 1 {
		t.Fatalf("unexpected monitored filter: %#v, %v", filtered, err)
	}
	hasFile := true
	filtered, err = movieListFilter(movies, nil, &hasFile, 0, 0)
	if err != nil || len(filtered) != 1 || filtered[0].ID != 2 {
		t.Fatalf("unexpected has-file filter: %#v, %v", filtered, err)
	}
	filtered, err = movieListFilter(movies, nil, nil, 5, 0)
	if err != nil || len(filtered) != 0 {
		t.Fatalf("offset past the end should return no movies: %#v, %v", filtered, err)
	}
	if _, err := movieListFilter(movies, nil, nil, -1, 0); err == nil {
		t.Fatal("negative offset accepted")
	}
	if _, err := movieListFilter(movies, nil, nil, 0, 501); err == nil {
		t.Fatal("oversized limit accepted")
	}
	for _, input := range []AddMovieInput{
		{},
		{TmdbID: 603, QualityProfileID: 3},
		{TmdbID: 603, RootFolderPath: "/movies"},
		{TmdbID: 603, QualityProfileID: 3, RootFolderPath: "/movies", Monitor: "bogus"},
		{TmdbID: 603, QualityProfileID: 3, RootFolderPath: "/movies", MinimumAvailability: "soon"},
	} {
		if _, err := addMovie(context.Background(), nil, input); err == nil {
			t.Fatalf("invalid add accepted: %#v", input)
		}
	}
}

// spec: CONN-ARR-MCP-RADARR-R003
func TestMovieAndPageJSONFields(t *testing.T) {
	var movie Movie
	if err := json.Unmarshal([]byte(`{"id":7,"title":"t","year":1999,"tmdbId":603,"monitored":true,"hasFile":true}`), &movie); err != nil || movie.TmdbID != 603 || !movie.HasFile {
		t.Fatalf("failed decoding movie: %#v %v", movie, err)
	}
	var page QueuePage
	if err := json.Unmarshal([]byte(`{"page":1,"pageSize":10,"totalRecords":1,"records":[{"id":3,"movieId":7,"status":"downloading","size":2048.5}]}`), &page); err != nil || page.TotalRecords != 1 || page.Records[0].Size != 2048.5 {
		t.Fatalf("failed decoding queue page: %#v %v", page, err)
	}
}
