package bazarr

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
	return c.baseURL + "/api" + path
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, payload any) ([]byte, error) {
	endpoint, err := url.Parse(c.endpoint(path))
	if err != nil {
		return nil, fmt.Errorf("create Bazarr %s request", path)
	}
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode Bazarr %s request", path)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create Bazarr %s request", path)
	}
	request.Header.Set("X-API-KEY", c.apiKey)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Bazarr %s request failed", path)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Bazarr %s response", path)
	}
	if len(responseBody) > maxResponseBytes {
		return nil, fmt.Errorf("Bazarr %s response exceeded size limit", path)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Bazarr %s returned HTTP %d", path, response.StatusCode)
	}
	return responseBody, nil
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, target any) error {
	body, err := c.request(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode Bazarr %s response", path)
	}
	return nil
}

func (c *Client) send(ctx context.Context, method, path string, query url.Values) error {
	_, err := c.request(ctx, method, path, query, nil)
	return err
}

type Page[T any] struct {
	Data  []T `json:"data"`
	Total int `json:"total"`
}

type Language struct {
	Name    string `json:"name"`
	Code2   string `json:"code2"`
	Code3   string `json:"code3"`
	Enabled bool   `json:"enabled"`
}

type Series struct {
	AlternativeTitles   []string `json:"alternativeTitles,omitempty"`
	AudioLanguage       any      `json:"audio_language,omitempty"`
	EpisodeFileCount    int      `json:"episodeFileCount"`
	EpisodeMissingCount int      `json:"episodeMissingCount"`
	Ended               bool     `json:"ended"`
	Fanart              string   `json:"fanart,omitempty"`
	ImdbID              string   `json:"imdbId,omitempty"`
	LastAired           string   `json:"lastAired,omitempty"`
	Monitored           bool     `json:"monitored"`
	Overview            string   `json:"overview,omitempty"`
	Path                string   `json:"path,omitempty"`
	Poster              string   `json:"poster,omitempty"`
	ProfileID           int      `json:"profileId"`
	SeriesType          string   `json:"seriesType"`
	SeriesID            int      `json:"sonarrSeriesId"`
	Tags                []string `json:"tags,omitempty"`
	Title               string   `json:"title"`
	TvdbID              int      `json:"tvdbId"`
	Year                string   `json:"year"`
}

type Episode struct {
	AudioLanguage    any    `json:"audio_language,omitempty"`
	Episode          int    `json:"episode"`
	MissingSubtitles any    `json:"missing_subtitles,omitempty"`
	Monitored        bool   `json:"monitored"`
	Path             string `json:"path"`
	Season           int    `json:"season"`
	EpisodeID        int    `json:"sonarrEpisodeId"`
	SeriesID         int    `json:"sonarrSeriesId"`
	Subtitles        any    `json:"subtitles,omitempty"`
	Title            string `json:"title"`
	SceneName        string `json:"sceneName,omitempty"`
}

type Movie struct {
	AlternativeTitles []string `json:"alternativeTitles,omitempty"`
	AudioLanguage     any      `json:"audio_language,omitempty"`
	CreatedAt         string   `json:"created_at_timestamp"`
	Fanart            string   `json:"fanart,omitempty"`
	ImdbID            string   `json:"imdbId,omitempty"`
	MissingSubtitles  any      `json:"missing_subtitles,omitempty"`
	Monitored         bool     `json:"monitored"`
	Overview          string   `json:"overview,omitempty"`
	Path              string   `json:"path,omitempty"`
	Poster            string   `json:"poster,omitempty"`
	ProfileID         int      `json:"profileId"`
	MovieID           int      `json:"radarrId"`
	SceneName         string   `json:"sceneName,omitempty"`
	Subtitles         any      `json:"subtitles,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	Title             string   `json:"title"`
	Year              string   `json:"year"`
}

type WantedEpisode struct {
	SeriesTitle      string   `json:"seriesTitle"`
	EpisodeNumber    string   `json:"episode_number"`
	EpisodeTitle     string   `json:"episodeTitle"`
	MissingSubtitles any      `json:"missing_subtitles,omitempty"`
	SeriesID         int      `json:"sonarrSeriesId"`
	EpisodeID        int      `json:"sonarrEpisodeId"`
	SceneName        string   `json:"sceneName,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	SeriesType       string   `json:"seriesType"`
}

type WantedMovie struct {
	Title            string   `json:"title"`
	MissingSubtitles any      `json:"missing_subtitles,omitempty"`
	MovieID          int      `json:"radarrId"`
	SceneName        string   `json:"sceneName,omitempty"`
	Tags             []string `json:"tags,omitempty"`
}

type Provider struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Retry  string `json:"retry"`
}

type HistoryItem map[string]any
type LanguageProfile map[string]any

type StatusResponse struct {
	Data struct {
		BazarrVersion  string `json:"bazarr_version"`
		PackageVersion string `json:"package_version"`
		SonarrVersion  string `json:"sonarr_version"`
		RadarrVersion  string `json:"radarr_version"`
	} `json:"data"`
}

func pagingQuery(start, length int) (url.Values, error) {
	if start < 0 {
		return nil, fmt.Errorf("start cannot be negative")
	}
	if length == 0 {
		length = 25
	}
	if length < 1 || length > 100 {
		return nil, fmt.Errorf("length must be between 1 and 100")
	}
	return url.Values{"start": {strconv.Itoa(start)}, "length": {strconv.Itoa(length)}}, nil
}

func validLanguage(language string) bool {
	if len(language) < 2 || len(language) > 12 {
		return false
	}
	for i, r := range language {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '-') {
			return false
		}
		if r == '-' && (i == 0 || i == len(language)-1 || language[i-1] == '-') {
			return false
		}
	}
	return true
}

func (c *Client) Status(ctx context.Context) (StatusResponse, error) {
	var status StatusResponse
	err := c.getJSON(ctx, "/system/status", nil, &status)
	return status, err
}

func (c *Client) Languages(ctx context.Context) ([]Language, error) {
	var languages []Language
	err := c.getJSON(ctx, "/system/languages", nil, &languages)
	return languages, err
}

func (c *Client) LanguageProfiles(ctx context.Context) ([]LanguageProfile, error) {
	var profiles []LanguageProfile
	err := c.getJSON(ctx, "/system/languages/profiles", nil, &profiles)
	return profiles, err
}

func (c *Client) Series(ctx context.Context, query url.Values) (Page[Series], error) {
	var page Page[Series]
	err := c.getJSON(ctx, "/series", query, &page)
	return page, err
}

func (c *Client) Episodes(ctx context.Context, query url.Values) ([]Episode, error) {
	var result struct {
		Data []Episode `json:"data"`
	}
	err := c.getJSON(ctx, "/episodes", query, &result)
	return result.Data, err
}

func (c *Client) WantedEpisodes(ctx context.Context, query url.Values) (Page[WantedEpisode], error) {
	var page Page[WantedEpisode]
	err := c.getJSON(ctx, "/episodes/wanted", query, &page)
	return page, err
}

func (c *Client) Movies(ctx context.Context, query url.Values) (Page[Movie], error) {
	var page Page[Movie]
	err := c.getJSON(ctx, "/movies", query, &page)
	return page, err
}

func (c *Client) WantedMovies(ctx context.Context, query url.Values) (Page[WantedMovie], error) {
	var page Page[WantedMovie]
	err := c.getJSON(ctx, "/movies/wanted", query, &page)
	return page, err
}

func (c *Client) Providers(ctx context.Context, query url.Values) ([]Provider, error) {
	var result struct {
		Data []Provider `json:"data"`
	}
	err := c.getJSON(ctx, "/providers", query, &result)
	return result.Data, err
}

func (c *Client) History(ctx context.Context, mediaType string, query url.Values) (Page[HistoryItem], error) {
	var page Page[HistoryItem]
	path := "/episodes/history"
	if mediaType == "movie" {
		path = "/movies/history"
	}
	err := c.getJSON(ctx, path, query, &page)
	return page, err
}

func (c *Client) SetProfile(ctx context.Context, mediaType string, id int, profileID string) error {
	query := url.Values{}
	path, idKey := "/series", "seriesid"
	if mediaType == "movie" {
		path, idKey = "/movies", "radarrid"
	}
	query.Set(idKey, strconv.Itoa(id))
	query.Set("profileid", profileID)
	return c.send(ctx, http.MethodPost, path, query)
}

func (c *Client) SearchSubtitles(ctx context.Context, mediaType string, id int) error {
	path, idKey := "/series", "seriesid"
	if mediaType == "movie" {
		path, idKey = "/movies", "radarrid"
	}
	return c.send(ctx, http.MethodPatch, path, url.Values{idKey: {strconv.Itoa(id)}, "action": {"search-missing"}})
}

func (c *Client) DownloadEpisodeSubtitle(ctx context.Context, seriesID, episodeID int, language string, forced, hi bool) error {
	query := url.Values{
		"seriesid":  {strconv.Itoa(seriesID)},
		"episodeid": {strconv.Itoa(episodeID)},
		"language":  {language},
		"forced":    {strconv.FormatBool(forced)},
		"hi":        {strconv.FormatBool(hi)},
	}
	return c.send(ctx, http.MethodPatch, "/episodes/subtitles", query)
}
