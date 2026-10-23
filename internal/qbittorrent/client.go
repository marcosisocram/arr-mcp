package qbittorrent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxResponseBytes = 8 << 20

type Client struct {
	baseURL  string
	apiKey   string
	username string
	password string
	http     *http.Client
	authMu   sync.Mutex
	loggedIn bool
}

func NewClient(cfg Config) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create qBittorrent cookie jar: %w", err)
	}
	return &Client{
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:   cfg.APIKey,
		username: cfg.Username,
		password: cfg.Password,
		http:     &http.Client{Timeout: 15 * time.Second, Jar: jar},
	}, nil
}

func (c *Client) endpoint(path string) string {
	return c.baseURL + "/api/v2" + path
}

func (c *Client) login(ctx context.Context) error {
	form := url.Values{"username": {c.username}, "password": {c.password}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/auth/login"), strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create qBittorrent login request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", c.baseURL)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("qBittorrent login request failed")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qBittorrent login failed with HTTP %d", resp.StatusCode)
	}
	parsed, _ := url.Parse(c.baseURL)
	for _, cookie := range c.http.Jar.Cookies(parsed) {
		if cookie.Name == "SID" && cookie.Value != "" {
			c.loggedIn = true
			return nil
		}
	}
	return fmt.Errorf("qBittorrent login failed: no session cookie returned")
}

func (c *Client) request(ctx context.Context, method, path string, query, form url.Values) ([]byte, error) {
	return c.requestWithAuth(ctx, method, path, query, form, false)
}

func (c *Client) requestWithAuth(ctx context.Context, method, path string, query, form url.Values, retried bool) ([]byte, error) {
	c.authMu.Lock()
	if c.apiKey == "" && !c.loggedIn {
		if err := c.login(ctx); err != nil {
			c.authMu.Unlock()
			return nil, err
		}
	}
	c.authMu.Unlock()

	endpoint, _ := url.Parse(c.endpoint(path))
	endpoint.RawQuery = query.Encode()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create qBittorrent %s request", path)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qBittorrent %s request failed", path)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read qBittorrent %s response", path)
	}
	if len(responseBody) > maxResponseBytes {
		return nil, fmt.Errorf("qBittorrent %s response exceeded size limit", path)
	}
	if (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) && c.apiKey == "" && !retried {
		c.authMu.Lock()
		c.loggedIn = false
		err := c.login(ctx)
		c.authMu.Unlock()
		if err != nil {
			return nil, err
		}
		return c.requestWithAuth(ctx, method, path, query, form, true)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("qBittorrent %s returned HTTP %d", path, resp.StatusCode)
	}
	return responseBody, nil
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, target any) error {
	body, err := c.request(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode qBittorrent %s response", path)
	}
	return nil
}

func (c *Client) getInt(ctx context.Context, path string) (int64, error) {
	body, err := c.request(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(body)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("decode qBittorrent %s response", path)
	}
	return value, nil
}

func (c *Client) getText(ctx context.Context, path string) (string, error) {
	body, err := c.request(ctx, http.MethodGet, path, nil, nil)
	return strings.TrimSpace(string(body)), err
}

func (c *Client) post(ctx context.Context, path string, form url.Values) error {
	_, err := c.request(ctx, http.MethodPost, path, nil, form)
	return err
}

type Torrent struct {
	Hash     string  `json:"hash"`
	Name     string  `json:"name"`
	State    string  `json:"state"`
	Progress float64 `json:"progress"`
	DLSpeed  int64   `json:"dlspeed"`
	UPSpeed  int64   `json:"upspeed"`
	ETA      int64   `json:"eta"`
	Size     int64   `json:"size"`
	Category string  `json:"category"`
	Tags     string  `json:"tags"`
}

type TorrentProperties struct {
	SavePath        string  `json:"save_path"`
	CreationDate    int64   `json:"creation_date"`
	TotalSize       int64   `json:"total_size"`
	TotalUploaded   int64   `json:"total_uploaded"`
	TotalDownloaded int64   `json:"total_downloaded"`
	UploadLimit     int64   `json:"upload_limit"`
	DownloadLimit   int64   `json:"dl_limit"`
	SeedingTime     int64   `json:"seeding_time"`
	Connections     int64   `json:"nb_connections"`
	ShareRatio      float64 `json:"share_ratio"`
}

type TransferInfo struct {
	DLSpeed          int64  `json:"dl_info_speed"`
	UPSpeed          int64  `json:"up_info_speed"`
	Downloaded       int64  `json:"dl_info_data"`
	Uploaded         int64  `json:"up_info_data"`
	DownloadLimit    int64  `json:"dl_rate_limit"`
	UploadLimit      int64  `json:"up_rate_limit"`
	DHTNodes         int64  `json:"dht_nodes"`
	ConnectionStatus string `json:"connection_status"`
}

type TorrentFile struct {
	Name         string  `json:"name"`
	Size         int64   `json:"size"`
	Progress     float64 `json:"progress"`
	Priority     int     `json:"priority"`
	IsSeed       bool    `json:"is_seed"`
	PieceRange   []int   `json:"piece_range"`
	Availability float64 `json:"availability"`
}

type Tracker struct {
	URL        string `json:"url"`
	Status     int    `json:"status"`
	Tier       int    `json:"tier"`
	NumPeers   int    `json:"num_peers"`
	NumSeeds   int    `json:"num_seeds"`
	NumLeeches int    `json:"num_leeches"`
	Message    string `json:"msg"`
}

type Peer struct {
	Address    string  `json:"address"`
	Client     string  `json:"client"`
	Connection string  `json:"connection"`
	Flags      string  `json:"flags"`
	Progress   float64 `json:"progress"`
	DLSpeed    int64   `json:"dl_speed"`
	UPSpeed    int64   `json:"up_speed"`
	Downloaded int64   `json:"downloaded"`
	Uploaded   int64   `json:"uploaded"`
	Relevance  float64 `json:"relevance"`
	Files      string  `json:"files"`
}

type Category struct {
	Name     string `json:"name"`
	SavePath string `json:"save_path"`
}

type PeerResponse struct {
	Peers map[string]Peer `json:"peers"`
}

type MainDataResponse struct {
	ServerState map[string]any `json:"server_state"`
}

var serverStateFields = map[string]bool{
	"alltime_dl": true, "alltime_ul": true, "average_time_queue": true,
	"connection_status": true, "dht_nodes": true, "dl_info_data": true,
	"dl_info_speed": true, "dl_rate_limit": true, "free_space_on_disk": true,
	"global_ratio": true, "queued_io_jobs": true, "queueing": true,
	"read_cache_hits": true, "read_cache_overload": true, "refresh_interval": true,
	"total_buffers_size": true, "total_peer_connections": true, "total_queued_size": true,
	"total_wasted_session": true, "up_info_data": true, "up_info_speed": true,
	"up_rate_limit": true, "use_alt_speed_limits": true, "write_cache_overload": true,
}

func (c *Client) ListTorrents(ctx context.Context, query url.Values) ([]Torrent, error) {
	var torrents []Torrent
	err := c.getJSON(ctx, "/torrents/info", query, &torrents)
	return torrents, err
}

func (c *Client) TorrentProperties(ctx context.Context, hash string) (TorrentProperties, error) {
	var result TorrentProperties
	err := c.getJSON(ctx, "/torrents/properties", url.Values{"hash": {hash}}, &result)
	return result, err
}

func (c *Client) TransferInfo(ctx context.Context) (TransferInfo, error) {
	var result TransferInfo
	err := c.getJSON(ctx, "/transfer/info", nil, &result)
	return result, err
}

func (c *Client) TransferLimits(ctx context.Context) (download, upload int64, err error) {
	download, err = c.getInt(ctx, "/transfer/downloadLimit")
	if err != nil {
		return
	}
	upload, err = c.getInt(ctx, "/transfer/uploadLimit")
	return
}

func (c *Client) SpeedLimitsMode(ctx context.Context) (int64, error) {
	return c.getInt(ctx, "/transfer/speedLimitsMode")
}

func (c *Client) SetTransferLimits(ctx context.Context, download, upload int64) error {
	if err := c.post(ctx, "/transfer/setDownloadLimit", url.Values{"limit": {strconv.FormatInt(download, 10)}}); err != nil {
		return err
	}
	return c.post(ctx, "/transfer/setUploadLimit", url.Values{"limit": {strconv.FormatInt(upload, 10)}})
}

func (c *Client) ToggleSpeedLimitsMode(ctx context.Context) error {
	return c.post(ctx, "/transfer/toggleSpeedLimitsMode", url.Values{})
}

func (c *Client) TorrentFiles(ctx context.Context, hash string) ([]TorrentFile, error) {
	var result []TorrentFile
	err := c.getJSON(ctx, "/torrents/files", url.Values{"hash": {hash}}, &result)
	return result, err
}

func (c *Client) TorrentTrackers(ctx context.Context, hash string) ([]Tracker, error) {
	var result []Tracker
	if err := c.getJSON(ctx, "/torrents/trackers", url.Values{"hash": {hash}}, &result); err != nil {
		return nil, err
	}
	for i := range result {
		result[i].URL = redactTrackerURL(result[i].URL)
	}
	return result, nil
}

func (c *Client) TorrentPeers(ctx context.Context, hash string) ([]Peer, error) {
	var response PeerResponse
	if err := c.getJSON(ctx, "/torrents/peers", url.Values{"hash": {hash}}, &response); err != nil {
		return nil, err
	}
	peers := make([]Peer, 0, len(response.Peers))
	for address, peer := range response.Peers {
		peer.Address = address
		peers = append(peers, peer)
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].Address < peers[j].Address })
	return peers, nil
}

func (c *Client) Categories(ctx context.Context) ([]Category, error) {
	var result map[string]struct {
		SavePath string `json:"savePath"`
	}
	if err := c.getJSON(ctx, "/torrents/categories", nil, &result); err != nil {
		return nil, err
	}
	categories := make([]Category, 0, len(result))
	for name, category := range result {
		categories = append(categories, Category{Name: name, SavePath: category.SavePath})
	}
	sort.Slice(categories, func(i, j int) bool { return categories[i].Name < categories[j].Name })
	return categories, nil
}

func (c *Client) Tags(ctx context.Context) ([]string, error) {
	var result []string
	if err := c.getJSON(ctx, "/torrents/tags", nil, &result); err != nil {
		return nil, err
	}
	sort.Strings(result)
	return result, nil
}

func (c *Client) AppVersion(ctx context.Context) (string, string, error) {
	version, err := c.getText(ctx, "/app/version")
	if err != nil {
		return "", "", err
	}
	webapiVersion, err := c.getText(ctx, "/app/webapiVersion")
	return version, webapiVersion, err
}

func (c *Client) ServerState(ctx context.Context) (map[string]any, error) {
	var response MainDataResponse
	if err := c.getJSON(ctx, "/sync/maindata", url.Values{"rid": {"0"}}, &response); err != nil {
		return nil, err
	}
	state := make(map[string]any)
	for key, value := range response.ServerState {
		if serverStateFields[key] {
			state[key] = value
		}
	}
	return state, nil
}

func (c *Client) DeleteTorrents(ctx context.Context, hashes []string, deleteFiles bool) error {
	return c.post(ctx, "/torrents/delete", url.Values{
		"hashes":      {strings.Join(hashes, "|")},
		"deleteFiles": {strconv.FormatBool(deleteFiles)},
	})
}

func redactTrackerURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "[redacted tracker URL]"
	}
	if parsed.User != nil {
		parsed.User = nil
	}
	query := parsed.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "passkey") || strings.Contains(lower, "authkey") || strings.Contains(lower, "apikey") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || lower == "key" {
			query.Set(key, "[redacted]")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (c *Client) SetTorrentsPaused(ctx context.Context, hashes []string, paused bool) error {
	endpoint := "/torrents/start"
	if paused {
		endpoint = "/torrents/stop"
	}
	return c.post(ctx, endpoint, url.Values{"hashes": {strings.Join(hashes, "|")}})
}
