package sonarr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// spec: CONN-ARR-MCP-SONARR-R002
func TestConfigValidation(t *testing.T) {
	for _, cfg := range []Config{
		{},
		{BaseURL: "ftp://localhost", APIKey: "key"},
		{BaseURL: "http://user:pass@localhost", APIKey: "key"},
		{BaseURL: "http://localhost/sonarr?x=1", APIKey: "key"},
		{BaseURL: "http://localhost#frag", APIKey: "key"},
		{BaseURL: "http://localhost"},
	} {
		if err := validateConfig(cfg); err == nil {
			t.Fatalf("invalid config accepted: %#v", cfg)
		}
	}
	if err := validateConfig(Config{BaseURL: "http://localhost:8989/sonarr", APIKey: "key"}); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

// spec: CONN-ARR-MCP-SONARR-R002, CONN-ARR-MCP-SONARR-R003
func TestAPIKeyAndSeriesReads(t *testing.T) {
	const secret = "sonarr_test-secret"
	mux := http.NewServeMux()
	mux.HandleFunc("/sonarr/api/v3/series", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != secret {
			t.Errorf("missing X-Api-Key header")
		}
		if r.URL.Query().Get("tvdbId") != "" {
			t.Errorf("unexpected tvdbId filter: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":12,"title":"Breaking Bad","year":2008,"tvdbId":81189,"monitored":true,"status":"ended","seriesType":"standard","qualityProfileId":3,"rootFolderPath":"/tv","seasons":[{"seasonNumber":1,"monitored":true}]}]`))
	})
	mux.HandleFunc("/sonarr/api/v3/series/12", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":12,"title":"Breaking Bad","year":2008,"tvdbId":81189,"monitored":true,"status":"ended","seasons":[{"seasonNumber":1,"monitored":true},{"seasonNumber":2,"monitored":false}]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL + "/sonarr", APIKey: secret})
	library, err := client.ListSeries(context.Background())
	if err != nil || len(library) != 1 || library[0].Title != "Breaking Bad" || library[0].TvdbID != 81189 {
		t.Fatalf("unexpected list result: %#v, %v", library, err)
	}
	if len(library[0].Seasons) != 1 || library[0].Seasons[0].SeasonNumber != 1 {
		t.Fatalf("unexpected season data: %#v", library[0].Seasons)
	}
	series, err := client.SeriesByID(context.Background(), 12)
	if err != nil || series.Status != "ended" || len(series.Seasons) != 2 {
		t.Fatalf("unexpected series result: %#v, %v", series, err)
	}
}

// spec: CONN-ARR-MCP-SONARR-R004
func TestLookupProfilesAndRootFolders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series/lookup", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("term") != "breaking bad" {
			t.Errorf("unexpected lookup term: %s", r.URL.Query().Get("term"))
		}
		_, _ = w.Write([]byte(`[{"title":"Breaking Bad","year":2008,"tvdbId":81189,"imdbId":"tt0903747"}]`))
	})
	mux.HandleFunc("/api/v3/qualityprofile", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":3,"name":"HD-1080p","upgradeAllowed":true,"cutoff":7}]`))
	})
	mux.HandleFunc("/api/v3/rootfolder", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":1,"path":"/tv","accessible":true,"freeSpace":1024}]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	found, err := client.LookupSeries(context.Background(), "breaking bad")
	if err != nil || len(found) != 1 || found[0].TvdbID != 81189 {
		t.Fatalf("unexpected lookup result: %#v, %v", found, err)
	}
	profiles, err := client.QualityProfiles(context.Background())
	if err != nil || len(profiles) != 1 || profiles[0].Name != "HD-1080p" || !profiles[0].UpgradeAllowed {
		t.Fatalf("unexpected profile result: %#v, %v", profiles, err)
	}
	folders, err := client.RootFolders(context.Background())
	if err != nil || len(folders) != 1 || folders[0].Path != "/tv" || folders[0].FreeSpace != 1024 {
		t.Fatalf("unexpected root folder result: %#v, %v", folders, err)
	}
}

// spec: CONN-ARR-MCP-SONARR-R005
func TestEpisodeReads(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/episode", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("seriesId") != "12" {
			t.Errorf("unexpected seriesId query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":101,"seriesId":12,"tvdbId":1,"seasonNumber":1,"episodeNumber":1,"title":"Pilot","monitored":true,"hasFile":true},{"id":102,"seriesId":12,"tvdbId":2,"seasonNumber":2,"episodeNumber":1,"title":"Seven Thirty-Seven","monitored":false,"hasFile":false}]`))
	})
	mux.HandleFunc("/api/v3/episode/101", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":101,"seriesId":12,"seasonNumber":1,"episodeNumber":1,"title":"Pilot","monitored":true,"hasFile":true,"airDate":"2008-01-20"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	season := 1
	episodes, err := listEpisodes(context.Background(), client, ListEpisodesInput{SeriesID: 12, SeasonNumber: &season})
	if err != nil || len(episodes) != 1 || episodes[0].ID != 101 {
		t.Fatalf("unexpected season-scoped episodes: %#v, %v", episodes, err)
	}
	monitored := false
	episodes, err = listEpisodes(context.Background(), client, ListEpisodesInput{SeriesID: 12, Monitored: &monitored})
	if err != nil || len(episodes) != 1 || episodes[0].ID != 102 {
		t.Fatalf("unexpected monitored filter: %#v, %v", episodes, err)
	}
	episode, err := client.EpisodeByID(context.Background(), 101)
	if err != nil || episode.Title != "Pilot" || episode.AirDate != "2008-01-20" {
		t.Fatalf("unexpected episode result: %#v, %v", episode, err)
	}
	if _, err := listEpisodes(context.Background(), client, ListEpisodesInput{}); err == nil {
		t.Fatal("missing series_id accepted")
	}
	if _, err := listEpisodes(context.Background(), client, ListEpisodesInput{SeriesID: -1}); err == nil {
		t.Fatal("negative series_id accepted")
	}
}

// spec: CONN-ARR-MCP-SONARR-R006
func TestAddSeriesIsDuplicateSafe(t *testing.T) {
	var posted map[string]any
	existing := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if r.URL.Query().Get("tvdbId") != "81189" {
				t.Errorf("unexpected tvdbId query: %s", r.URL.RawQuery)
			}
			if existing {
				_, _ = w.Write([]byte(`[{"id":12,"title":"Breaking Bad","tvdbId":81189,"monitored":true}]`))
				return
			}
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
			t.Errorf("decode add body: %v", err)
		}
		existing = true
		_, _ = w.Write([]byte(`{"id":12,"title":"Breaking Bad","tvdbId":81189,"monitored":true,"qualityProfileId":3,"rootFolderPath":"/tv"}`))
	})
	mux.HandleFunc("/api/v3/series/lookup", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("term") != "tvdb:81189" {
			t.Errorf("unexpected lookup term: %s", r.URL.Query().Get("term"))
		}
		_, _ = w.Write([]byte(`[{"title":"Breaking Bad","year":2008,"tvdbId":81189,"titleSlug":"breaking-bad"}]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	input := AddSeriesInput{TvdbID: 81189, QualityProfileID: 3, RootFolderPath: "/tv"}
	first, err := addSeries(context.Background(), client, input)
	if err != nil || first.Status != "created" || first.Series.ID != 12 {
		t.Fatalf("unexpected add result: %#v, %v", first, err)
	}
	options, ok := posted["addOptions"].(map[string]any)
	if !ok || options["searchForMissingEpisodes"] != false || options["searchForCutoffUnmetEpisodes"] != false || options["monitor"] != "all" {
		t.Fatalf("unexpected addOptions: %#v", posted["addOptions"])
	}
	if posted["title"] != "Breaking Bad" || posted["seriesType"] != "standard" || posted["monitored"] != true || posted["seasonFolder"] != true {
		t.Fatalf("add body lost lookup metadata or defaults: %#v", posted)
	}
	if posted["qualityProfileId"] != float64(3) || posted["rootFolderPath"] != "/tv" {
		t.Fatalf("add body lost the explicit profile/folder: %#v", posted)
	}
	second, err := addSeries(context.Background(), client, input)
	if err != nil || second.Status != "already_exists" || second.Series.ID != 12 {
		t.Fatalf("duplicate add was not detected: %#v, %v", second, err)
	}
}

// spec: CONN-ARR-MCP-SONARR-R007, CONN-ARR-MCP-SONARR-R010
func TestMutationBodiesAndErrorRedaction(t *testing.T) {
	var putBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series/12", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":12,"title":"Kept","year":2008,"tvdbId":81189,"monitored":true,"qualityProfileId":3,"rootFolderPath":"/tv"}`))
		case http.MethodPut:
			if r.URL.Query().Get("moveFiles") != "false" {
				t.Errorf("unexpected moveFiles: %s", r.URL.Query().Get("moveFiles"))
			}
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Errorf("decode PUT body: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":12,"title":"Kept","year":2008,"tvdbId":81189,"monitored":false,"qualityProfileId":3}`))
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
	series, err := setSeriesMonitored(context.Background(), client, 12, false)
	if err != nil || series.Monitored {
		t.Fatalf("unexpected monitored result: %#v, %v", series, err)
	}
	if putBody["title"] != "Kept" || putBody["qualityProfileId"] != float64(3) || putBody["tvdbId"] != float64(81189) {
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

// spec: CONN-ARR-MCP-SONARR-R007
func TestEpisodeMonitorBody(t *testing.T) {
	var patched map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/episode/monitor", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&patched); err != nil {
			t.Errorf("decode monitor body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	if err := client.SetEpisodesMonitored(context.Background(), []int{101, 102}, true); err != nil {
		t.Fatalf("episode monitor failed: %v", err)
	}
	ids, ok := patched["episodeIds"].([]any)
	if !ok || len(ids) != 2 || ids[0] != float64(101) || ids[1] != float64(102) {
		t.Fatalf("unexpected episodeIds: %#v", patched["episodeIds"])
	}
	if monitored, ok := patched["monitored"].(bool); !ok || !monitored {
		t.Fatalf("unexpected monitored flag: %#v", patched["monitored"])
	}
	for _, ids := range [][]int{nil, {}, {0}, {101, -2}, make([]int, 101)} {
		if err := validateEpisodeIDs(ids); err == nil {
			t.Fatalf("invalid episode_ids accepted: %v", ids)
		}
	}
}

// spec: CONN-ARR-MCP-SONARR-R008, CONN-ARR-MCP-SONARR-R009
func TestSearchQueueAndHistory(t *testing.T) {
	var commands []map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/command", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s", r.Method)
		}
		var command map[string]any
		if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
			t.Errorf("decode command body: %v", err)
		}
		commands = append(commands, command)
		_, _ = w.Write([]byte(`{"id":42,"name":"SeriesSearch","status":"queued"}`))
	})
	mux.HandleFunc("/api/v3/command/42", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":42,"name":"SeriesSearch","status":"completed","result":"successful"}`))
	})
	mux.HandleFunc("/api/v3/queue", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "1" || r.URL.Query().Get("pageSize") != "25" {
			t.Errorf("unexpected queue paging: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"page":1,"pageSize":25,"totalRecords":1,"records":[{"id":1,"seriesId":12,"episodeId":101,"title":"Breaking Bad","status":"downloading","size":100.5,"protocol":"torrent"}]}`))
	})
	mux.HandleFunc("/api/v3/history", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"page":1,"pageSize":25,"totalRecords":1,"records":[{"id":5,"seriesId":12,"episodeId":101,"sourceTitle":"Breaking.Bad.S01E01","eventType":"grabbed","date":"2026-10-15T00:00:00Z"}]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	query, err := pageQuery(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	search, err := client.SearchSeries(context.Background(), 12)
	if err != nil || search.ID != 42 || search.Status != "queued" {
		t.Fatalf("unexpected series search command: %#v, %v", search, err)
	}
	if _, err := client.SearchSeason(context.Background(), 12, 1); err != nil {
		t.Fatalf("season search failed: %v", err)
	}
	if _, err := client.SearchEpisodes(context.Background(), []int{101, 102}); err != nil {
		t.Fatalf("episode search failed: %v", err)
	}
	if len(commands) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(commands))
	}
	if name, ok := commands[0]["name"].(string); !ok || name != "SeriesSearch" {
		t.Fatalf("unexpected series command payload: %#v", commands[0])
	}
	if id, ok := commands[0]["seriesId"].(float64); !ok || id != 12 {
		t.Fatalf("unexpected series command seriesId: %#v", commands[0]["seriesId"])
	}
	if name, ok := commands[1]["name"].(string); !ok || name != "SeasonSearch" {
		t.Fatalf("unexpected season command payload: %#v", commands[1])
	}
	if season, ok := commands[1]["seasonNumber"].(float64); !ok || season != 1 {
		t.Fatalf("unexpected season command seasonNumber: %#v", commands[1]["seasonNumber"])
	}
	if name, ok := commands[2]["name"].(string); !ok || name != "EpisodeSearch" {
		t.Fatalf("unexpected episode command payload: %#v", commands[2])
	}
	ids, ok := commands[2]["episodeIds"].([]any)
	if !ok || len(ids) != 2 || ids[0] != float64(101) {
		t.Fatalf("unexpected episode command episodeIds: %#v", commands[2]["episodeIds"])
	}
	tracked, err := client.CommandByID(context.Background(), 42)
	if err != nil || tracked.Status != "completed" || tracked.Result != "successful" {
		t.Fatalf("unexpected tracked command: %#v, %v", tracked, err)
	}
	queue, err := client.Queue(context.Background(), query)
	if err != nil || queue.TotalRecords != 1 || queue.Records[0].Title != "Breaking Bad" {
		t.Fatalf("unexpected queue: %#v, %v", queue, err)
	}
	history, err := client.History(context.Background(), query)
	if err != nil || history.Records[0].EventType != "grabbed" {
		t.Fatalf("unexpected history: %#v, %v", history, err)
	}
}

// spec: CONN-ARR-MCP-SONARR-R010
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
		_, err := client.ListSeries(ctx)
		done <- err
	}()
	<-started
	cancel()
	if err := <-done; err == nil {
		t.Fatal("cancelled request unexpectedly succeeded")
	}
}

// spec: CONN-ARR-MCP-SONARR-R002, CONN-ARR-MCP-SONARR-R010
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
	if _, err := client.ListSeries(context.Background()); err == nil {
		t.Fatal("redirected request unexpectedly succeeded")
	}
	if leaked != 0 {
		t.Fatal("API key was forwarded across a redirect")
	}
}

// spec: CONN-ARR-MCP-SONARR-R003, CONN-ARR-MCP-SONARR-R006, CONN-ARR-MCP-SONARR-R009
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
	library := []Series{{ID: 1, Monitored: true}, {ID: 2, Monitored: false}}
	monitored := true
	filtered, err := seriesListFilter(library, &monitored, 0, 0)
	if err != nil || len(filtered) != 1 || filtered[0].ID != 1 {
		t.Fatalf("unexpected monitored filter: %#v, %v", filtered, err)
	}
	filtered, err = seriesListFilter(library, nil, 5, 0)
	if err != nil || len(filtered) != 0 {
		t.Fatalf("offset past the end should return no series: %#v, %v", filtered, err)
	}
	if _, err := seriesListFilter(library, nil, -1, 0); err == nil {
		t.Fatal("negative offset accepted")
	}
	if _, err := seriesListFilter(library, nil, 0, 501); err == nil {
		t.Fatal("oversized limit accepted")
	}
	episodes := []Episode{
		{ID: 101, SeasonNumber: 1, Monitored: true, HasFile: true},
		{ID: 102, SeasonNumber: 2, Monitored: false, HasFile: false},
	}
	season := 2
	seasoned, err := episodeListFilter(episodes, &season, nil, nil, 0, 0)
	if err != nil || len(seasoned) != 1 || seasoned[0].ID != 102 {
		t.Fatalf("unexpected season filter: %#v, %v", seasoned, err)
	}
	hasFile := true
	withFiles, err := episodeListFilter(episodes, nil, nil, &hasFile, 0, 0)
	if err != nil || len(withFiles) != 1 || withFiles[0].ID != 101 {
		t.Fatalf("unexpected has-file filter: %#v, %v", withFiles, err)
	}
	negativeSeason := -1
	if _, err := episodeListFilter(episodes, &negativeSeason, nil, nil, 0, 0); err == nil {
		t.Fatal("negative season accepted")
	}
	for _, input := range []AddSeriesInput{
		{},
		{TvdbID: 81189, QualityProfileID: 3},
		{TvdbID: 81189, RootFolderPath: "/tv"},
		{TvdbID: 81189, QualityProfileID: 3, RootFolderPath: "/tv", Monitor: "bogus"},
		{TvdbID: 81189, QualityProfileID: 3, RootFolderPath: "/tv", SeriesType: "movie"},
	} {
		if _, err := addSeries(context.Background(), nil, input); err == nil {
			t.Fatalf("invalid add accepted: %#v", input)
		}
	}
}

// spec: CONN-ARR-MCP-SONARR-R003
func TestSeriesAndPageJSONFields(t *testing.T) {
	var series Series
	if err := json.Unmarshal([]byte(`{"id":12,"title":"t","year":2008,"tvdbId":81189,"monitored":true,"seasons":[{"seasonNumber":1,"monitored":true}]}`), &series); err != nil || series.TvdbID != 81189 || len(series.Seasons) != 1 {
		t.Fatalf("failed decoding series: %#v %v", series, err)
	}
	var episode Episode
	if err := json.Unmarshal([]byte(`{"id":101,"seriesId":12,"seasonNumber":1,"episodeNumber":1,"monitored":true,"hasFile":true}`), &episode); err != nil || episode.SeasonNumber != 1 || !episode.HasFile {
		t.Fatalf("failed decoding episode: %#v %v", episode, err)
	}
	var page QueuePage
	if err := json.Unmarshal([]byte(`{"page":1,"pageSize":10,"totalRecords":1,"records":[{"id":3,"seriesId":12,"episodeId":101,"status":"downloading","size":2048.5}]}`), &page); err != nil || page.TotalRecords != 1 || page.Records[0].Size != 2048.5 {
		t.Fatalf("failed decoding queue page: %#v %v", page, err)
	}
}
