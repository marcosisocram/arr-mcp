package radarr

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
		return nil, fmt.Errorf("create Radarr %s request", path)
	}
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode Radarr %s request", path)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create Radarr %s request", path)
	}
	request.Header.Set("X-Api-Key", c.apiKey)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Radarr %s request failed", path)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Radarr %s response", path)
	}
	if len(responseBody) > maxResponseBytes {
		return nil, fmt.Errorf("Radarr %s response exceeded size limit", path)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Radarr %s returned HTTP %d", path, response.StatusCode)
	}
	return responseBody, nil
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, target any) error {
	body, err := c.request(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode Radarr %s response", path)
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
		return fmt.Errorf("decode Radarr %s response", path)
	}
	return nil
}

type Movie struct {
	ID                  int    `json:"id"`
	Title               string `json:"title"`
	Year                int    `json:"year"`
	TmdbID              int    `json:"tmdbId"`
	ImdbID              string `json:"imdbId"`
	Monitored           bool   `json:"monitored"`
	HasFile             bool   `json:"hasFile"`
	IsAvailable         bool   `json:"isAvailable"`
	Status              string `json:"status"`
	MinimumAvailability string `json:"minimumAvailability"`
	QualityProfileID    int    `json:"qualityProfileId"`
	RootFolderPath      string `json:"rootFolderPath"`
	Path                string `json:"path"`
	SizeOnDisk          int64  `json:"sizeOnDisk"`
	Runtime             int    `json:"runtime"`
	Added               string `json:"added"`
	LastSearchTime      string `json:"lastSearchTime"`
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
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Result  string `json:"result"`
	Message string `json:"message"`
	Queued  string `json:"queued"`
	Started string `json:"started"`
	Ended   string `json:"ended"`
}

type QueueEntry struct {
	ID                      int     `json:"id"`
	MovieID                 int     `json:"movieId"`
	Title                   string  `json:"title"`
	Status                  string  `json:"status"`
	TrackedDownloadStatus   string  `json:"trackedDownloadStatus"`
	TrackedDownloadState    string  `json:"trackedDownloadState"`
	CustomFormatScore       int     `json:"customFormatScore"`
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
	MovieID             int    `json:"movieId"`
	SourceTitle         string `json:"sourceTitle"`
	EventType           string `json:"eventType"`
	Date                string `json:"date"`
	DownloadID          string `json:"downloadId"`
	CustomFormatScore   int    `json:"customFormatScore"`
	QualityCutoffNotMet bool   `json:"qualityCutoffNotMet"`
}

type HistoryPage struct {
	Page         int            `json:"page"`
	PageSize     int            `json:"pageSize"`
	TotalRecords int            `json:"totalRecords"`
	Records      []HistoryEntry `json:"records"`
}

func (c *Client) ListMovies(ctx context.Context) ([]Movie, error) {
	var movies []Movie
	err := c.getJSON(ctx, "/movie", nil, &movies)
	return movies, err
}

func (c *Client) MovieByID(ctx context.Context, id int) (Movie, error) {
	var movie Movie
	err := c.getJSON(ctx, "/movie/"+strconv.Itoa(id), nil, &movie)
	return movie, err
}

func (c *Client) MoviesByTmdbID(ctx context.Context, tmdbID int) ([]Movie, error) {
	var movies []Movie
	query := url.Values{"tmdbId": {strconv.Itoa(tmdbID)}}
	err := c.getJSON(ctx, "/movie", query, &movies)
	return movies, err
}

func (c *Client) LookupMovies(ctx context.Context, term string) ([]Movie, error) {
	var movies []Movie
	err := c.getJSON(ctx, "/movie/lookup", url.Values{"term": {term}}, &movies)
	return movies, err
}

// LookupMovieByTmdbID returns the raw lookup resource so a movie can be added
// with the full metadata Radarr expects, without exposing the resource to a tool.
func (c *Client) LookupMovieByTmdbID(ctx context.Context, tmdbID int) (map[string]any, error) {
	resource := map[string]any{}
	query := url.Values{"tmdbId": {strconv.Itoa(tmdbID)}}
	err := c.getJSON(ctx, "/movie/lookup/tmdb", query, &resource)
	return resource, err
}

// MovieResource returns the raw movie resource so an update preserves every
// field Radarr sent, not only the trimmed subset the tools expose.
func (c *Client) MovieResource(ctx context.Context, id int) (map[string]any, error) {
	resource := map[string]any{}
	err := c.getJSON(ctx, "/movie/"+strconv.Itoa(id), nil, &resource)
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

func (c *Client) AddMovie(ctx context.Context, payload map[string]any) (Movie, error) {
	var movie Movie
	err := c.sendJSON(ctx, http.MethodPost, "/movie", nil, payload, &movie)
	return movie, err
}

func (c *Client) UpdateMovie(ctx context.Context, id int, payload map[string]any) (Movie, error) {
	var movie Movie
	query := url.Values{"moveFiles": {"false"}}
	err := c.sendJSON(ctx, http.MethodPut, "/movie/"+strconv.Itoa(id), query, payload, &movie)
	return movie, err
}

func (c *Client) SearchMovies(ctx context.Context, movieID int) (Command, error) {
	var command Command
	payload := map[string]any{"name": "MoviesSearch", "movieIds": []int{movieID}}
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
