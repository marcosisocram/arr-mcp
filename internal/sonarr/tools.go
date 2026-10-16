package sonarr

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var monitorTypes = map[string]bool{
	"unknown": true, "all": true, "future": true, "missing": true,
	"existing": true, "pilot": true, "firstSeason": true,
	"lastSeason": true, "latestSeason": true, "none": true,
}

var seriesTypeValues = map[string]bool{
	"standard": true, "daily": true, "anime": true,
}

type ListSeriesInput struct {
	Monitored *bool `json:"monitored,omitempty" jsonschema:"Return only series with this monitored state when set."`
	Limit     int   `json:"limit,omitempty" jsonschema:"Maximum series to return after filtering (1-500; default 100)."`
	Offset    int   `json:"offset,omitempty" jsonschema:"Series to skip after filtering."`
}

type SeriesIDInput struct {
	SeriesID int `json:"series_id" jsonschema:"Internal Sonarr series ID from sonarr_list_series or sonarr_add_series."`
}

type LookupSeriesInput struct {
	Term  string `json:"term" jsonschema:"Title search term, or a tvdb:12345 identifier."`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum lookup results to return (1-50; default 20)."`
}

type ListEpisodesInput struct {
	SeriesID     int   `json:"series_id" jsonschema:"Internal Sonarr series ID whose episodes should be listed."`
	SeasonNumber *int  `json:"season_number,omitempty" jsonschema:"Return only episodes of this season when set."`
	Monitored    *bool `json:"monitored,omitempty" jsonschema:"Return only episodes with this monitored state when set."`
	HasFile      *bool `json:"has_file,omitempty" jsonschema:"Return only episodes with this file state when set."`
	Limit        int   `json:"limit,omitempty" jsonschema:"Maximum episodes to return after filtering (1-500; default 100)."`
	Offset       int   `json:"offset,omitempty" jsonschema:"Episodes to skip after filtering."`
}

type EpisodeIDInput struct {
	EpisodeID int `json:"episode_id" jsonschema:"Internal Sonarr episode ID from sonarr_list_episodes."`
}

type AddSeriesInput struct {
	TvdbID           int    `json:"tvdb_id" jsonschema:"The Movie Database (TVDB) identifier of the series to add."`
	QualityProfileID int    `json:"quality_profile_id" jsonschema:"Quality profile ID from sonarr_list_quality_profiles."`
	RootFolderPath   string `json:"root_folder_path" jsonschema:"Root folder path from sonarr_list_root_folders."`
	Monitored        *bool  `json:"monitored,omitempty" jsonschema:"Monitor the series after adding (default true)."`
	SeasonFolder     *bool  `json:"season_folder,omitempty" jsonschema:"Use a folder per season (default true)."`
	Monitor          string `json:"monitor,omitempty" jsonschema:"Which episodes to monitor: all, future, missing, existing, pilot, firstSeason, lastSeason, latestSeason, or none (default all)."`
	SeriesType       string `json:"series_type,omitempty" jsonschema:"standard, daily, or anime (default standard)."`
}

type SetSeriesMonitoredInput struct {
	SeriesID  int  `json:"series_id" jsonschema:"Internal Sonarr series ID to update."`
	Monitored bool `json:"monitored" jsonschema:"New monitored state for this series only."`
}

type SetEpisodesMonitoredInput struct {
	EpisodeIDs []int `json:"episode_ids" jsonschema:"Internal Sonarr episode IDs to update (1-100)."`
	Monitored  bool  `json:"monitored" jsonschema:"New monitored state for these episodes only."`
}

type SearchSeriesInput struct {
	SeriesID int `json:"series_id" jsonschema:"Internal Sonarr series ID whose releases should be searched."`
}

type SearchSeasonInput struct {
	SeriesID     int `json:"series_id" jsonschema:"Internal Sonarr series ID whose season should be searched."`
	SeasonNumber int `json:"season_number" jsonschema:"Season number to search."`
}

type SearchEpisodesInput struct {
	EpisodeIDs []int `json:"episode_ids" jsonschema:"Internal Sonarr episode IDs whose releases should be searched (1-100)."`
}

type CommandInput struct {
	CommandID int `json:"command_id" jsonschema:"Command ID returned by a sonarr_search_* tool."`
}

type QueueInput struct {
	Page     int `json:"page,omitempty" jsonschema:"Page number starting at 1 (default 1)."`
	PageSize int `json:"page_size,omitempty" jsonschema:"Records per page (1-100; default 25)."`
}

type HistoryInput struct {
	Page     int `json:"page,omitempty" jsonschema:"Page number starting at 1 (default 1)."`
	PageSize int `json:"page_size,omitempty" jsonschema:"Records per page (1-100; default 25)."`
}

type AddSeriesOutput struct {
	Status string `json:"status" jsonschema:"created when a new series was added, or already_exists when the series was already in the library."`
	Series Series `json:"series" jsonschema:"The Sonarr series now in the library."`
}

type SetEpisodesMonitoredOutput struct {
	Monitored  bool  `json:"monitored" jsonschema:"The monitored state that was applied."`
	EpisodeIDs []int `json:"episode_ids" jsonschema:"The episode IDs that were updated."`
}

type EmptyInput struct{}

func NewMCPServer(client *Client) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "sonarr-mcp", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_list_series", Description: "List series in the Sonarr library with an optional monitored filter and local paging."}, func(ctx context.Context, _ *mcp.CallToolRequest, input ListSeriesInput) (*mcp.CallToolResult, []Series, error) {
		library, err := client.ListSeries(ctx)
		if err != nil {
			return nil, nil, err
		}
		filtered, err := seriesListFilter(library, input.Monitored, input.Offset, input.Limit)
		if err != nil {
			return nil, nil, err
		}
		return nil, filtered, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_get_series", Description: "Read one series from the Sonarr library by its internal Sonarr ID, including per-season monitoring flags."}, func(ctx context.Context, _ *mcp.CallToolRequest, input SeriesIDInput) (*mcp.CallToolResult, Series, error) {
		if input.SeriesID <= 0 {
			return nil, Series{}, fmt.Errorf("series_id must be a positive integer")
		}
		series, err := client.SeriesByID(ctx, input.SeriesID)
		return nil, series, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_lookup_series", Description: "Search Sonarr's series metadata source for candidate series to add; returns candidate TVDB identifiers only."}, func(ctx context.Context, _ *mcp.CallToolRequest, input LookupSeriesInput) (*mcp.CallToolResult, []Series, error) {
		term := strings.TrimSpace(input.Term)
		if term == "" {
			return nil, nil, fmt.Errorf("term is required")
		}
		limit := input.Limit
		if limit == 0 {
			limit = 20
		}
		if limit < 1 || limit > 50 {
			return nil, nil, fmt.Errorf("limit must be between 1 and 50")
		}
		library, err := client.LookupSeries(ctx, term)
		if err != nil {
			return nil, nil, err
		}
		if len(library) > limit {
			library = library[:limit]
		}
		return nil, library, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_list_quality_profiles", Description: "List the quality profiles available when adding a series."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, []QualityProfile, error) {
		profiles, err := client.QualityProfiles(ctx)
		return nil, profiles, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_list_root_folders", Description: "List the root folders available when adding a series."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, []RootFolder, error) {
		folders, err := client.RootFolders(ctx)
		return nil, folders, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_list_episodes", Description: "List episodes of one explicit series with optional season, monitored, and has-file filters and local paging."}, func(ctx context.Context, _ *mcp.CallToolRequest, input ListEpisodesInput) (*mcp.CallToolResult, []Episode, error) {
		episodes, err := listEpisodes(ctx, client, input)
		return nil, episodes, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_get_episode", Description: "Read one episode from the Sonarr library by its internal Sonarr episode ID."}, func(ctx context.Context, _ *mcp.CallToolRequest, input EpisodeIDInput) (*mcp.CallToolResult, Episode, error) {
		if input.EpisodeID <= 0 {
			return nil, Episode{}, fmt.Errorf("episode_id must be a positive integer")
		}
		episode, err := client.EpisodeByID(ctx, input.EpisodeID)
		return nil, episode, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_add_series", Description: "Add one series by TVDB ID with an explicit quality profile and root folder. Never starts a release search and never creates a duplicate."}, func(ctx context.Context, _ *mcp.CallToolRequest, input AddSeriesInput) (*mcp.CallToolResult, AddSeriesOutput, error) {
		output, err := addSeries(ctx, client, input)
		return nil, output, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_set_series_monitored", Description: "Set the monitored flag of one explicit series and change no other field."}, func(ctx context.Context, _ *mcp.CallToolRequest, input SetSeriesMonitoredInput) (*mcp.CallToolResult, Series, error) {
		series, err := setSeriesMonitored(ctx, client, input.SeriesID, input.Monitored)
		return nil, series, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_set_episode_monitored", Description: "Set the monitored flag of explicit episodes and change no other field."}, func(ctx context.Context, _ *mcp.CallToolRequest, input SetEpisodesMonitoredInput) (*mcp.CallToolResult, SetEpisodesMonitoredOutput, error) {
		if err := validateEpisodeIDs(input.EpisodeIDs); err != nil {
			return nil, SetEpisodesMonitoredOutput{}, err
		}
		if err := client.SetEpisodesMonitored(ctx, input.EpisodeIDs, input.Monitored); err != nil {
			return nil, SetEpisodesMonitoredOutput{}, err
		}
		return nil, SetEpisodesMonitoredOutput{Monitored: input.Monitored, EpisodeIDs: input.EpisodeIDs}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_search_series", Description: "Ask Sonarr to search indexers for releases of one explicit series. This can start downloads."}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchSeriesInput) (*mcp.CallToolResult, Command, error) {
		if input.SeriesID <= 0 {
			return nil, Command{}, fmt.Errorf("series_id must be a positive integer")
		}
		command, err := client.SearchSeries(ctx, input.SeriesID)
		return nil, command, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_search_season", Description: "Ask Sonarr to search indexers for releases of one explicit season. This can start downloads."}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchSeasonInput) (*mcp.CallToolResult, Command, error) {
		if input.SeriesID <= 0 {
			return nil, Command{}, fmt.Errorf("series_id must be a positive integer")
		}
		if input.SeasonNumber < 0 {
			return nil, Command{}, fmt.Errorf("season_number cannot be negative")
		}
		command, err := client.SearchSeason(ctx, input.SeriesID, input.SeasonNumber)
		return nil, command, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_search_episodes", Description: "Ask Sonarr to search indexers for releases of explicit episodes. This can start downloads."}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchEpisodesInput) (*mcp.CallToolResult, Command, error) {
		if err := validateEpisodeIDs(input.EpisodeIDs); err != nil {
			return nil, Command{}, err
		}
		command, err := client.SearchEpisodes(ctx, input.EpisodeIDs)
		return nil, command, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_get_command", Description: "Read the current state of a Sonarr command, such as the one returned by a sonarr_search_* tool."}, func(ctx context.Context, _ *mcp.CallToolRequest, input CommandInput) (*mcp.CallToolResult, Command, error) {
		if input.CommandID <= 0 {
			return nil, Command{}, fmt.Errorf("command_id must be a positive integer")
		}
		command, err := client.CommandByID(ctx, input.CommandID)
		return nil, command, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_get_queue", Description: "Read one page of the Sonarr download queue."}, func(ctx context.Context, _ *mcp.CallToolRequest, input QueueInput) (*mcp.CallToolResult, QueuePage, error) {
		query, err := pageQuery(input.Page, input.PageSize)
		if err != nil {
			return nil, QueuePage{}, err
		}
		page, err := client.Queue(ctx, query)
		return nil, page, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "sonarr_get_history", Description: "Read one page of the Sonarr episode history."}, func(ctx context.Context, _ *mcp.CallToolRequest, input HistoryInput) (*mcp.CallToolResult, HistoryPage, error) {
		query, err := pageQuery(input.Page, input.PageSize)
		if err != nil {
			return nil, HistoryPage{}, err
		}
		page, err := client.History(ctx, query)
		return nil, page, err
	})
	return server
}

func addSeries(ctx context.Context, client *Client, input AddSeriesInput) (AddSeriesOutput, error) {
	if input.TvdbID <= 0 {
		return AddSeriesOutput{}, fmt.Errorf("tvdb_id must be a positive integer")
	}
	if input.QualityProfileID <= 0 {
		return AddSeriesOutput{}, fmt.Errorf("quality_profile_id must be a positive integer")
	}
	if strings.TrimSpace(input.RootFolderPath) == "" {
		return AddSeriesOutput{}, fmt.Errorf("root_folder_path is required")
	}
	monitored := true
	if input.Monitored != nil {
		monitored = *input.Monitored
	}
	seasonFolder := true
	if input.SeasonFolder != nil {
		seasonFolder = *input.SeasonFolder
	}
	monitor := input.Monitor
	if monitor == "" {
		monitor = "all"
	}
	if !monitorTypes[monitor] {
		return AddSeriesOutput{}, fmt.Errorf("monitor must be unknown, all, future, missing, existing, pilot, firstSeason, lastSeason, latestSeason, or none")
	}
	seriesType := input.SeriesType
	if seriesType == "" {
		seriesType = "standard"
	}
	if !seriesTypeValues[seriesType] {
		return AddSeriesOutput{}, fmt.Errorf("series_type must be standard, daily, or anime")
	}
	existing, err := client.SeriesByTvdbID(ctx, input.TvdbID)
	if err != nil {
		return AddSeriesOutput{}, err
	}
	if len(existing) > 0 {
		return AddSeriesOutput{Status: "already_exists", Series: existing[0]}, nil
	}
	resource, err := client.LookupSeriesByTvdbID(ctx, input.TvdbID)
	if err != nil {
		return AddSeriesOutput{}, err
	}
	if _, found := resource["title"]; !found {
		return AddSeriesOutput{}, fmt.Errorf("Sonarr returned no series metadata for tvdb_id %d", input.TvdbID)
	}
	resource["qualityProfileId"] = input.QualityProfileID
	resource["rootFolderPath"] = input.RootFolderPath
	resource["monitored"] = monitored
	resource["seasonFolder"] = seasonFolder
	resource["seriesType"] = seriesType
	resource["addOptions"] = map[string]any{
		"monitor":                      monitor,
		"searchForMissingEpisodes":     false,
		"searchForCutoffUnmetEpisodes": false,
	}
	series, err := client.AddSeries(ctx, resource)
	if err != nil {
		return AddSeriesOutput{}, err
	}
	return AddSeriesOutput{Status: "created", Series: series}, nil
}

func setSeriesMonitored(ctx context.Context, client *Client, seriesID int, monitored bool) (Series, error) {
	if seriesID <= 0 {
		return Series{}, fmt.Errorf("series_id must be a positive integer")
	}
	resource, err := client.SeriesResource(ctx, seriesID)
	if err != nil {
		return Series{}, err
	}
	resource["monitored"] = monitored
	return client.UpdateSeries(ctx, seriesID, resource)
}

func listEpisodes(ctx context.Context, client *Client, input ListEpisodesInput) ([]Episode, error) {
	if input.SeriesID <= 0 {
		return nil, fmt.Errorf("series_id must be a positive integer")
	}
	query := url.Values{"seriesId": {strconv.Itoa(input.SeriesID)}}
	episodes, err := client.ListEpisodes(ctx, query)
	if err != nil {
		return nil, err
	}
	return episodeListFilter(episodes, input.SeasonNumber, input.Monitored, input.HasFile, input.Offset, input.Limit)
}

func validateEpisodeIDs(ids []int) error {
	if len(ids) == 0 {
		return fmt.Errorf("episode_ids must contain at least one episode ID")
	}
	if len(ids) > 100 {
		return fmt.Errorf("episode_ids must contain at most 100 episode IDs")
	}
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("episode_ids must contain only positive integers")
		}
	}
	return nil
}

func pageQuery(page, pageSize int) (url.Values, error) {
	if page == 0 {
		page = 1
	}
	if page < 1 {
		return nil, fmt.Errorf("page must be at least 1")
	}
	if pageSize == 0 {
		pageSize = 25
	}
	if pageSize < 1 || pageSize > 100 {
		return nil, fmt.Errorf("page_size must be between 1 and 100")
	}
	return url.Values{"page": {strconv.Itoa(page)}, "pageSize": {strconv.Itoa(pageSize)}}, nil
}

func seriesListFilter(library []Series, monitored *bool, offset, limit int) ([]Series, error) {
	if offset < 0 {
		return nil, fmt.Errorf("offset cannot be negative")
	}
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 500 {
		return nil, fmt.Errorf("limit must be between 1 and 500")
	}
	filtered := make([]Series, 0, len(library))
	for _, series := range library {
		if monitored != nil && series.Monitored != *monitored {
			continue
		}
		filtered = append(filtered, series)
	}
	if offset >= len(filtered) {
		return []Series{}, nil
	}
	filtered = filtered[offset:]
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func episodeListFilter(episodes []Episode, seasonNumber *int, monitored, hasFile *bool, offset, limit int) ([]Episode, error) {
	if offset < 0 {
		return nil, fmt.Errorf("offset cannot be negative")
	}
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 500 {
		return nil, fmt.Errorf("limit must be between 1 and 500")
	}
	if seasonNumber != nil && *seasonNumber < 0 {
		return nil, fmt.Errorf("season_number cannot be negative")
	}
	filtered := make([]Episode, 0, len(episodes))
	for _, episode := range episodes {
		if seasonNumber != nil && episode.SeasonNumber != *seasonNumber {
			continue
		}
		if monitored != nil && episode.Monitored != *monitored {
			continue
		}
		if hasFile != nil && episode.HasFile != *hasFile {
			continue
		}
		filtered = append(filtered, episode)
	}
	if offset >= len(filtered) {
		return []Episode{}, nil
	}
	filtered = filtered[offset:]
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}
