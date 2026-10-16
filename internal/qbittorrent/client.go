package qbittorrent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
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

func (c *Client) SetTorrentsPaused(ctx context.Context, hashes []string, paused bool) error {
	endpoint := "/torrents/start"
	if paused {
		endpoint = "/torrents/stop"
	}
	return c.post(ctx, endpoint, url.Values{"hashes": {strings.Join(hashes, "|")}})
}
