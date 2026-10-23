package qbittorrent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// spec: CONN-ARR-MCP-R002, CONN-ARR-MCP-R003, CONN-ARR-MCP-R004
func TestAPIKeyAndTorrentReads(t *testing.T) {
	const secret = "qbt_test-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+secret {
			t.Errorf("missing API key bearer header")
		}
		switch r.URL.Path {
		case "/api/v2/torrents/info":
			if r.URL.Query().Get("limit") != "100" || r.URL.Query().Get("filter") != "active" {
				t.Errorf("unexpected list query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"hash":"0123456789012345678901234567890123456789","name":"example","state":"downloading","progress":0.5,"dlspeed":100,"upspeed":20,"eta":30,"size":200,"category":"","tags":""}]`))
		case "/api/v2/transfer/info":
			_, _ = w.Write([]byte(`{"dl_info_speed":10,"up_info_speed":20,"dl_info_data":30,"up_info_data":40,"dl_rate_limit":50,"up_rate_limit":60,"dht_nodes":7,"connection_status":"connected"}`))
		default:
			t.Errorf("unexpected endpoint: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, APIKey: secret})
	if err != nil {
		t.Fatal(err)
	}
	query, err := torrentListQuery(ListTorrentsInput{Filter: "active"})
	if err != nil {
		t.Fatal(err)
	}
	torrents, err := client.ListTorrents(context.Background(), query)
	if err != nil || len(torrents) != 1 || torrents[0].Name != "example" {
		t.Fatalf("unexpected torrent result: %#v, %v", torrents, err)
	}
	transfer, err := client.TransferInfo(context.Background())
	if err != nil || transfer.DHTNodes != 7 || transfer.DLSpeed != 10 {
		t.Fatalf("unexpected transfer result: %#v, %v", transfer, err)
	}
}

// spec: CONN-ARR-MCP-R002
func TestPasswordLoginRenewsSessionOnce(t *testing.T) {
	logins, calls := 0, 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/auth/login":
			logins++
			if r.Header.Get("Referer") != server.URL {
				t.Errorf("unexpected login Referer: %q", r.Header.Get("Referer"))
			}
			if err := r.ParseForm(); err != nil || r.Form.Get("username") != "user" || r.Form.Get("password") != "password" {
				t.Error("login form fields were not sent")
			}
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "session", Path: "/"})
			_, _ = w.Write([]byte("Ok."))
		case "/api/v2/transfer/downloadLimit":
			calls++
			if calls == 1 {
				http.Error(w, "auth failed; password=do-not-leak", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte("0"))
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, Username: "user", Password: "password"})
	if err != nil {
		t.Fatal(err)
	}
	limit, err := client.getInt(context.Background(), "/transfer/downloadLimit")
	if err != nil || limit != 0 {
		t.Fatalf("unexpected limit result %d, %v", limit, err)
	}
	if logins != 2 || calls != 2 {
		t.Fatalf("expected one initial login and one renewal; logins=%d calls=%d", logins, calls)
	}
}

// spec: CONN-ARR-MCP-R007
func TestCancellationAndTLSDefaults(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	if err != nil {
		t.Fatal(err)
	}
	if client.http.Timeout != 15*time.Second {
		t.Fatalf("unexpected request timeout: %s", client.http.Timeout)
	}
	if client.http.Transport != nil {
		t.Fatal("client must retain the standard transport and TLS verification defaults")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, requestErr := client.request(ctx, http.MethodGet, "/wait", nil, nil)
		done <- requestErr
	}()
	<-started
	cancel()
	if err := <-done; err == nil {
		t.Fatal("cancelled request unexpectedly succeeded")
	}
}

// spec: CONN-ARR-MCP-R005, CONN-ARR-MCP-R007
func TestMutationFormsAndErrorRedaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/torrents/stop" {
			if err := r.ParseForm(); err != nil || r.Form.Get("hashes") != "0123456789012345678901234567890123456789|abcdefabcdefabcdefabcdefabcdefabcdefabcd" {
				t.Errorf("unexpected mutation form: %s", r.Form.Encode())
			}
			_, _ = w.Write([]byte("Ok."))
			return
		}
		http.Error(w, "password=secret", http.StatusInternalServerError)
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, APIKey: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	hashes := []string{"0123456789012345678901234567890123456789", "abcdefabcdefabcdefabcdefabcdefabcdefabcd"}
	if err := client.SetTorrentsPaused(context.Background(), hashes, true); err != nil {
		t.Fatal(err)
	}
	_, err = client.request(context.Background(), http.MethodGet, "/private", nil, nil)
	if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "password") {
		t.Fatalf("upstream error was not safely sanitized: %v", err)
	}
}

// spec: CONN-ARR-MCP-R003, CONN-ARR-MCP-R005, CONN-ARR-MCP-R006
func TestToolValidation(t *testing.T) {
	if _, err := torrentListQuery(ListTorrentsInput{Filter: "bogus"}); err == nil {
		t.Fatal("invalid filter should fail")
	}
	if _, err := torrentListQuery(ListTorrentsInput{Limit: 501}); err == nil {
		t.Fatal("oversized page should fail")
	}
	for _, hashes := range [][]string{nil, {"all"}, {"not-a-hash"}} {
		if err := validateHashes(hashes); err == nil {
			t.Fatalf("invalid hashes accepted: %#v", hashes)
		}
	}
	if err := validateHashes([]string{"0123456789012345678901234567890123456789"}); err != nil {
		t.Fatalf("valid hash rejected: %v", err)
	}
}

// spec: CONN-ARR-MCP-R002
func TestConfigValidation(t *testing.T) {
	for _, cfg := range []Config{
		{},
		{BaseURL: "ftp://localhost", APIKey: "key"},
		{BaseURL: "http://user:pass@localhost", APIKey: "key"},
		{BaseURL: "http://localhost", Username: "user"},
	} {
		if err := validateConfig(cfg); err == nil {
			t.Fatalf("invalid config accepted: %#v", cfg)
		}
	}
	if err := validateConfig(Config{BaseURL: "http://localhost:8080", APIKey: "key"}); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

// spec: CONN-ARR-MCP-R003
func TestTorrentJSONFields(t *testing.T) {
	var torrent Torrent
	if err := json.Unmarshal([]byte(`{"hash":"h","name":"n","state":"stopped","dlspeed":9}`), &torrent); err != nil || torrent.DLSpeed != 9 {
		t.Fatalf("failed decoding torrent: %#v %v", torrent, err)
	}
}

// spec: CONN-ARR-MCP-R009, CONN-ARR-MCP-R010, CONN-ARR-MCP-R011
func TestAdditionalReadsAndRedaction(t *testing.T) {
	const hash = "0123456789012345678901234567890123456789"
	deleteCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/torrents/files":
			_, _ = w.Write([]byte(`[{"name":"a","size":1},{"name":"b","size":2}]`))
		case "/api/v2/torrents/trackers":
			_, _ = w.Write([]byte(`[{"url":"https://user:password@tracker.example/announce?passkey=secret&token=hidden&foo=ok"}]`))
		case "/api/v2/torrents/peers":
			_, _ = w.Write([]byte(`{"peers":{"2.0.0.1:2":{"client":"b"},"1.0.0.1:1":{"client":"a"}}}`))
		case "/api/v2/torrents/categories":
			_, _ = w.Write([]byte(`{"z":{"savePath":"/z"},"a":{"savePath":"/a"}}`))
		case "/api/v2/torrents/tags":
			_, _ = w.Write([]byte(`["z","a"]`))
		case "/api/v2/app/version":
			_, _ = w.Write([]byte("5.2.0\n"))
		case "/api/v2/app/webapiVersion":
			_, _ = w.Write([]byte("2.14.1\n"))
		case "/api/v2/sync/maindata":
			_, _ = w.Write([]byte(`{"server_state":{"free_space_on_disk":42,"password":"do-not-return"}}`))
		case "/api/v2/torrents/delete":
			deleteCalls++
			if err := r.ParseForm(); err != nil || r.Form.Get("hashes") != hash || r.Form.Get("deleteFiles") != "true" {
				t.Errorf("unexpected delete form: %s", r.Form.Encode())
			}
			_, _ = w.Write([]byte("Ok."))
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, APIKey: "key"})
	if err != nil {
		t.Fatal(err)
	}

	files, err := client.TorrentFiles(context.Background(), hash)
	if err != nil || len(files) != 2 {
		t.Fatalf("files: %#v %v", files, err)
	}
	trackers, err := client.TorrentTrackers(context.Background(), hash)
	if err != nil {
		t.Fatal(err)
	}
	trackerJSON, _ := json.Marshal(trackers)
	for _, secret := range []string{"user", "password", "secret", "hidden"} {
		if strings.Contains(string(trackerJSON), secret) {
			t.Fatalf("tracker secret leaked: %s", trackerJSON)
		}
	}
	if !strings.Contains(string(trackerJSON), "foo=ok") {
		t.Fatalf("non-sensitive tracker query was lost: %s", trackerJSON)
	}
	peers, err := client.TorrentPeers(context.Background(), hash)
	if err != nil || len(peers) != 2 || peers[0].Address != "1.0.0.1:1" {
		t.Fatalf("peers not decoded/sorted: %#v %v", peers, err)
	}
	categories, err := client.Categories(context.Background())
	if err != nil || len(categories) != 2 || categories[0].Name != "a" {
		t.Fatalf("categories not sorted: %#v %v", categories, err)
	}
	tags, err := client.Tags(context.Background())
	if err != nil || len(tags) != 2 || tags[0] != "a" {
		t.Fatalf("tags not sorted: %#v %v", tags, err)
	}
	version, webapi, err := client.AppVersion(context.Background())
	if err != nil || version != "5.2.0" || webapi != "2.14.1" {
		t.Fatalf("versions: %q %q %v", version, webapi, err)
	}
	state, err := client.ServerState(context.Background())
	if err != nil || state["free_space_on_disk"] != float64(42) || state["password"] != nil {
		t.Fatalf("unsafe server state: %#v %v", state, err)
	}
	if err := client.DeleteTorrents(context.Background(), []string{hash}, true); err != nil || deleteCalls != 1 {
		t.Fatalf("delete request failed: calls=%d err=%v", deleteCalls, err)
	}
}

// spec: CONN-ARR-MCP-R010
func TestDeleteTorrentsConfirmationValidation(t *testing.T) {
	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer upstream.Close()
	api, err := NewClient(Config{BaseURL: upstream.URL, APIKey: "key"})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateHashes(nil); err == nil {
		t.Fatal("empty hashes accepted")
	}
	if err := validateHashes([]string{"all"}); err == nil {
		t.Fatal("wildcard accepted")
	}
	if requests != 0 {
		t.Fatalf("validation unexpectedly caused upstream request: %d", requests)
	}
	_ = api
}

// spec: CONN-ARR-MCP-R009
func TestBoundedReadLimits(t *testing.T) {
	for _, test := range []struct{ input, defaultValue, maximum, want int }{
		{0, 500, 5000, 500},
		{1, 500, 5000, 1},
		{5000, 500, 5000, 5000},
		{0, 200, 1000, 200},
	} {
		got, err := boundedLimit(test.input, test.defaultValue, test.maximum)
		if err != nil || got != test.want {
			t.Fatalf("boundedLimit(%d) = %d, %v; want %d", test.input, got, err, test.want)
		}
	}
	for _, input := range []int{-1, 5001} {
		if _, err := boundedLimit(input, 500, 5000); err == nil {
			t.Fatalf("invalid limit %d accepted", input)
		}
	}
}
