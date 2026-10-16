package sonarr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxResponseBytes = 8 << 20

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(cfg Config) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		http: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) endpoint(path string) string {
	return c.baseURL + "/api/v3" + path
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, payload any) ([]byte, error) {
	endpoint, err := url.Parse(c.endpoint(path))
	if err != nil {
		return nil, fmt.Errorf("create Sonarr %s request", path)
	}
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode Sonarr %s request", path)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create Sonarr %s request", path)
	}
	request.Header.Set("X-Api-Key", c.apiKey)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Sonarr %s request failed", path)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Sonarr %s response", path)
	}
	if len(responseBody) > maxResponseBytes {
		return nil, fmt.Errorf("Sonarr %s response exceeded size limit", path)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Sonarr %s returned HTTP %d", path, response.StatusCode)
	}
	return responseBody, nil
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, target any) error {
	body, err := c.request(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode Sonarr %s response", path)
	}
	return nil
}

func (c *Client) sendJSON(ctx context.Context, method, path string, query url.Values, payload, target any) error {
	body, err := c.request(ctx, method, path, query, payload)
	if err != nil {
		return err
	}
	if target == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode Sonarr %s response", path)
	}
	return nil
}

type Season struct {
	SeasonNumber int  `json:"seasonNumber"`
	Monitored    bool `json:"monitored"`
}

type Series struct {
	ID               int      `json:"id"`
	Title            string   `json:"title"`
	Year             int      `json:"year"`
	TvdbID           int      `json:"tvdbId"`
	ImdbID           string   `json:"imdbId"`
	Monitored        bool     `json:"monitored"`
	Status           string   `json:"status"`
	SeriesType       string   `json:"seriesType"`
	QualityProfileID int      `json:"qualityProfileId"`
	RootFolderPath   string   `json:"rootFolderPath"`
	Path             string   `json:"path"`
	SeasonFolder     bool     `json:"seasonFolder"`
	Seasons          []Season `json:"seasons"`
	Added            string   `json:"added"`
	LastSearchTime   string   `json:"lastSearchTime"`
}

type Episode struct {
	ID            int    `json:"id"`
	SeriesID      int    `json:"seriesId"`
	TvdbID        int    `json:"tvdbId"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	Title         string `json:"title"`
	Monitored     bool   `json:"monitored"`
	HasFile       bool   `json:"hasFile"`
	AirDate       string `json:"airDate"`
	AirDateUTC    string `json:"airDateUtc"`
}

type QualityProfile struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	UpgradeAllowed bool   `json:"upgradeAllowed"`
	Cutoff         int    `json:"cutoff"`
}

type RootFolder struct {
	ID         int    `json:"id"`
	Path       string `json:"path"`
	Accessible bool   `json:"accessible"`
	FreeSpace  int64  `json:"freeSpace"`
}

type Command struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	CommandName string `json:"commandName"`
	Status      string `json:"status"`
	Result      string `json:"result"`
	Message     string `json:"message"`
	Queued      string `json:"queued"`
	Started     string `json:"started"`
	Ended       string `json:"ended"`
}

type QueueEntry struct {
	ID                      int     `json:"id"`
	SeriesID                int     `json:"seriesId"`
	EpisodeID               int     `json:"episodeId"`
	Title                   string  `json:"title"`
	Status                  string  `json:"status"`
	TrackedDownloadStatus   string  `json:"trackedDownloadStatus"`
	TrackedDownloadState    string  `json:"trackedDownloadState"`
	Size                    float64 `json:"size"`
	Protocol                string  `json:"protocol"`
	DownloadClient          string  `json:"downloadClient"`
	Indexer                 string  `json:"indexer"`
	OutputPath              string  `json:"outputPath"`
	ErrorMessage            string  `json:"errorMessage"`
	Added                   string  `json:"added"`
	EstimatedCompletionTime string  `json:"estimatedCompletionTime"`
}

type QueuePage struct {
	Page         int          `json:"page"`
	PageSize     int          `json:"pageSize"`
	TotalRecords int          `json:"totalRecords"`
	Records      []QueueEntry `json:"records"`
}

type HistoryEntry struct {
	ID                  int    `json:"id"`
	SeriesID            int    `json:"seriesId"`
	EpisodeID           int    `json:"episodeId"`
	SourceTitle         string `json:"sourceTitle"`
	EventType           string `json:"eventType"`
	Date                string `json:"date"`
	DownloadID          string `json:"downloadId"`
	QualityCutoffNotMet bool   `json:"qualityCutoffNotMet"`
}

type HistoryPage struct {
	Page         int            `json:"page"`
	PageSize     int            `json:"pageSize"`
	TotalRecords int            `json:"totalRecords"`
	Records      []HistoryEntry `json:"records"`
}

func (c *Client) ListSeries(ctx context.Context) ([]Series, error) {
	var series []Series
	err := c.getJSON(ctx, "/series", nil, &series)
	return series, err
}

func (c *Client) SeriesByID(ctx context.Context, id int) (Series, error) {
	var series Series
	err := c.getJSON(ctx, "/series/"+strconv.Itoa(id), nil, &series)
	return series, err
}

func (c *Client) SeriesByTvdbID(ctx context.Context, tvdbID int) ([]Series, error) {
	var series []Series
	query := url.Values{"tvdbId": {strconv.Itoa(tvdbID)}}
	err := c.getJSON(ctx, "/series", query, &series)
	return series, err
}

func (c *Client) LookupSeries(ctx context.Context, term string) ([]Series, error) {
	var series []Series
	err := c.getJSON(ctx, "/series/lookup", url.Values{"term": {term}}, &series)
	return series, err
}

// LookupSeriesByTvdbID returns the raw lookup resource so a series can be added
// with the full metadata Sonarr expects, without exposing the resource to a tool.
func (c *Client) LookupSeriesByTvdbID(ctx context.Context, tvdbID int) (map[string]any, error) {
	var resources []map[string]any
	query := url.Values{"term": {"tvdb:" + strconv.Itoa(tvdbID)}}
	if err := c.getJSON(ctx, "/series/lookup", query, &resources); err != nil {
		return nil, err
	}
	for _, resource := range resources {
		if id, ok := resource["tvdbId"].(float64); ok && int(id) == tvdbID {
			return resource, nil
		}
	}
	return nil, fmt.Errorf("Sonarr returned no series metadata for tvdb_id %d", tvdbID)
}

// SeriesResource returns the raw series resource so an update preserves every
// field Sonarr sent, not only the trimmed subset the tools expose.
func (c *Client) SeriesResource(ctx context.Context, id int) (map[string]any, error) {
	resource := map[string]any{}
	err := c.getJSON(ctx, "/series/"+strconv.Itoa(id), nil, &resource)
	return resource, err
}

func (c *Client) QualityProfiles(ctx context.Context) ([]QualityProfile, error) {
	var profiles []QualityProfile
	err := c.getJSON(ctx, "/qualityprofile", nil, &profiles)
	return profiles, err
}

func (c *Client) RootFolders(ctx context.Context) ([]RootFolder, error) {
	var folders []RootFolder
	err := c.getJSON(ctx, "/rootfolder", nil, &folders)
	return folders, err
}

func (c *Client) ListEpisodes(ctx context.Context, query url.Values) ([]Episode, error) {
	var episodes []Episode
	err := c.getJSON(ctx, "/episode", query, &episodes)
	return episodes, err
}

func (c *Client) EpisodeByID(ctx context.Context, id int) (Episode, error) {
	var episode Episode
	err := c.getJSON(ctx, "/episode/"+strconv.Itoa(id), nil, &episode)
	return episode, err
}

func (c *Client) AddSeries(ctx context.Context, payload map[string]any) (Series, error) {
	var series Series
	err := c.sendJSON(ctx, http.MethodPost, "/series", nil, payload, &series)
	return series, err
}

func (c *Client) UpdateSeries(ctx context.Context, id int, payload map[string]any) (Series, error) {
	var series Series
	query := url.Values{"moveFiles": {"false"}}
	err := c.sendJSON(ctx, http.MethodPut, "/series/"+strconv.Itoa(id), query, payload, &series)
	return series, err
}

func (c *Client) SetEpisodesMonitored(ctx context.Context, episodeIDs []int, monitored bool) error {
	payload := map[string]any{"episodeIds": episodeIDs, "monitored": monitored}
	return c.sendJSON(ctx, http.MethodPut, "/episode/monitor", nil, payload, nil)
}

func (c *Client) SearchSeries(ctx context.Context, seriesID int) (Command, error) {
	var command Command
	payload := map[string]any{"name": "SeriesSearch", "seriesId": seriesID}
	err := c.sendJSON(ctx, http.MethodPost, "/command", nil, payload, &command)
	return command, err
}

func (c *Client) SearchSeason(ctx context.Context, seriesID, seasonNumber int) (Command, error) {
	var command Command
	payload := map[string]any{"name": "SeasonSearch", "seriesId": seriesID, "seasonNumber": seasonNumber}
	err := c.sendJSON(ctx, http.MethodPost, "/command", nil, payload, &command)
	return command, err
}

func (c *Client) SearchEpisodes(ctx context.Context, episodeIDs []int) (Command, error) {
	var command Command
	payload := map[string]any{"name": "EpisodeSearch", "episodeIds": episodeIDs}
	err := c.sendJSON(ctx, http.MethodPost, "/command", nil, payload, &command)
	return command, err
}

func (c *Client) CommandByID(ctx context.Context, id int) (Command, error) {
	var command Command
	err := c.getJSON(ctx, "/command/"+strconv.Itoa(id), nil, &command)
	return command, err
}

func (c *Client) Queue(ctx context.Context, query url.Values) (QueuePage, error) {
	var page QueuePage
	err := c.getJSON(ctx, "/queue", query, &page)
	return page, err
}

func (c *Client) History(ctx context.Context, query url.Values) (HistoryPage, error) {
	var page HistoryPage
	err := c.getJSON(ctx, "/history", query, &page)
	return page, err
}
