package bazarr

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type EmptyInput struct{}

type StatusOutput struct {
	BazarrVersion  string `json:"bazarr_version"`
	PackageVersion string `json:"package_version"`
	SonarrVersion  string `json:"sonarr_version"`
	RadarrVersion  string `json:"radarr_version"`
}

type LanguagesOutput struct {
	Languages []Language `json:"languages"`
}

type LanguageProfilesOutput struct {
	Profiles []LanguageProfile `json:"profiles"`
}

type ListInput struct {
	Start  int `json:"start,omitempty" jsonschema:"Number of records to skip (default 0)."`
	Length int `json:"length,omitempty" jsonschema:"Maximum records to return (1-100; default 25)."`
}

type SeriesListInput struct {
	Start  int `json:"start,omitempty" jsonschema:"Number of records to skip (default 0)."`
	Length int `json:"length,omitempty" jsonschema:"Maximum records to return (1-100; default 25)."`
}

type SeriesIDInput struct {
	SeriesID int `json:"series_id" jsonschema:"Sonarr series ID."`
}

type MovieIDInput struct {
	MovieID int `json:"movie_id" jsonschema:"Radarr movie ID."`
}

type EpisodeListInput struct {
	SeriesID int `json:"series_id" jsonschema:"Sonarr series ID whose episodes should be listed."`
}

type PageOutput[T any] struct {
	Data  []T `json:"data" jsonschema:"Page of matching Bazarr records."`
	Total int `json:"total" jsonschema:"Total number of matching records."`
}

type EpisodesOutput struct {
	Data []Episode `json:"data" jsonschema:"Episodes belonging to the requested series."`
}

type SeriesOutput struct {
	Series Series `json:"series"`
}

type MovieOutput struct {
	Movie Movie `json:"movie"`
}

type HistoryInput struct {
	MediaType string `json:"media_type" jsonschema:"History type: episode or movie."`
	MediaID   int    `json:"media_id,omitempty" jsonschema:"Optional Sonarr episode ID or Radarr movie ID filter."`
	Start     int    `json:"start,omitempty" jsonschema:"Number of records to skip (default 0)."`
	Length    int    `json:"length,omitempty" jsonschema:"Maximum records to return (1-100; default 25)."`
}

type ProfileInput struct {
	SeriesID  int    `json:"series_id,omitempty" jsonschema:"Sonarr series ID; use for series profile updates."`
	MovieID   int    `json:"movie_id,omitempty" jsonschema:"Radarr movie ID; use for movie profile updates."`
	ProfileID string `json:"profile_id" jsonschema:"Explicit language profile ID from bazarr_list_language_profiles, or none to clear the profile."`
}

type SearchInput struct {
	SeriesID int `json:"series_id,omitempty" jsonschema:"Sonarr series ID to search subtitles for."`
	MovieID  int `json:"movie_id,omitempty" jsonschema:"Radarr movie ID to search subtitles for."`
}

type DownloadEpisodeInput struct {
	SeriesID  int    `json:"series_id" jsonschema:"Sonarr series ID containing the episode."`
	EpisodeID int    `json:"episode_id" jsonschema:"Sonarr episode ID to download a subtitle for."`
	Language  string `json:"language" jsonschema:"Language code (e.g. en, pt-BR) from bazarr_list_languages."`
	Forced    bool   `json:"forced,omitempty" jsonschema:"Request forced subtitles (default false)."`
	HI        bool   `json:"hi,omitempty" jsonschema:"Request hearing-impaired subtitles (default false)."`
}

type ActionOutput struct {
	Status string `json:"status"`
}

func NewMCPServer(client *Client) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "bazarr-mcp", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_get_status", Description: "Read Bazarr and connected Sonarr/Radarr version information. Local filesystem paths and database/runtime details are deliberately omitted."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, StatusOutput, error) {
		status, err := client.Status(ctx)
		return nil, StatusOutput{BazarrVersion: status.Data.BazarrVersion, PackageVersion: status.Data.PackageVersion, SonarrVersion: status.Data.SonarrVersion, RadarrVersion: status.Data.RadarrVersion}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_list_languages", Description: "List languages available in Bazarr for subtitle filtering."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, LanguagesOutput, error) {
		languages, err := client.Languages(ctx)
		return nil, LanguagesOutput{Languages: languages}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_list_language_profiles", Description: "List configured subtitle language profiles."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, LanguageProfilesOutput, error) {
		profiles, err := client.LanguageProfiles(ctx)
		return nil, LanguageProfilesOutput{Profiles: profiles}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_list_series", Description: "List Bazarr series subtitle metadata using bounded server-side paging."}, func(ctx context.Context, _ *mcp.CallToolRequest, input SeriesListInput) (*mcp.CallToolResult, PageOutput[Series], error) {
		query, err := pagingQuery(input.Start, input.Length)
		if err != nil {
			return nil, PageOutput[Series]{}, err
		}
		page, err := client.Series(ctx, query)
		return nil, PageOutput[Series]{Data: page.Data, Total: page.Total}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_get_series", Description: "Read Bazarr subtitle metadata for one explicit Sonarr series ID."}, func(ctx context.Context, _ *mcp.CallToolRequest, input SeriesIDInput) (*mcp.CallToolResult, SeriesOutput, error) {
		if input.SeriesID <= 0 {
			return nil, SeriesOutput{}, fmt.Errorf("series_id must be a positive integer")
		}
		query := url.Values{"seriesid[]": {strconv.Itoa(input.SeriesID)}}
		page, err := client.Series(ctx, query)
		if err != nil {
			return nil, SeriesOutput{}, err
		}
		if len(page.Data) == 0 {
			return nil, SeriesOutput{}, fmt.Errorf("Bazarr returned no series for series_id %d", input.SeriesID)
		}
		return nil, SeriesOutput{Series: page.Data[0]}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_list_episodes", Description: "List Bazarr subtitle metadata for episodes of one explicit Sonarr series."}, func(ctx context.Context, _ *mcp.CallToolRequest, input EpisodeListInput) (*mcp.CallToolResult, EpisodesOutput, error) {
		if input.SeriesID <= 0 {
			return nil, EpisodesOutput{}, fmt.Errorf("series_id must be a positive integer")
		}
		query := url.Values{"seriesid[]": {strconv.Itoa(input.SeriesID)}}
		episodes, err := client.Episodes(ctx, query)
		return nil, EpisodesOutput{Data: episodes}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_list_wanted_episodes", Description: "List episodes with missing subtitles using bounded server-side paging."}, func(ctx context.Context, _ *mcp.CallToolRequest, input ListInput) (*mcp.CallToolResult, PageOutput[WantedEpisode], error) {
		query, err := pagingQuery(input.Start, input.Length)
		if err != nil {
			return nil, PageOutput[WantedEpisode]{}, err
		}
		page, err := client.WantedEpisodes(ctx, query)
		return nil, PageOutput[WantedEpisode]{Data: page.Data, Total: page.Total}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_list_movies", Description: "List Bazarr movie subtitle metadata using bounded server-side paging."}, func(ctx context.Context, _ *mcp.CallToolRequest, input ListInput) (*mcp.CallToolResult, PageOutput[Movie], error) {
		query, err := pagingQuery(input.Start, input.Length)
		if err != nil {
			return nil, PageOutput[Movie]{}, err
		}
		page, err := client.Movies(ctx, query)
		return nil, PageOutput[Movie]{Data: page.Data, Total: page.Total}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_get_movie", Description: "Read Bazarr subtitle metadata for one explicit Radarr movie ID."}, func(ctx context.Context, _ *mcp.CallToolRequest, input MovieIDInput) (*mcp.CallToolResult, MovieOutput, error) {
		if input.MovieID <= 0 {
			return nil, MovieOutput{}, fmt.Errorf("movie_id must be a positive integer")
		}
		query := url.Values{"radarrid[]": {strconv.Itoa(input.MovieID)}}
		page, err := client.Movies(ctx, query)
		if err != nil {
			return nil, MovieOutput{}, err
		}
		if len(page.Data) == 0 {
			return nil, MovieOutput{}, fmt.Errorf("Bazarr returned no movie for movie_id %d", input.MovieID)
		}
		return nil, MovieOutput{Movie: page.Data[0]}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_list_wanted_movies", Description: "List movies with missing subtitles using bounded server-side paging."}, func(ctx context.Context, _ *mcp.CallToolRequest, input ListInput) (*mcp.CallToolResult, PageOutput[WantedMovie], error) {
		query, err := pagingQuery(input.Start, input.Length)
		if err != nil {
			return nil, PageOutput[WantedMovie]{}, err
		}
		page, err := client.WantedMovies(ctx, query)
		return nil, PageOutput[WantedMovie]{Data: page.Data, Total: page.Total}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_list_providers", Description: "List subtitle providers and their current availability/throttle status."}, func(ctx context.Context, _ *mcp.CallToolRequest, input ProviderInput) (*mcp.CallToolResult, ProvidersOutput, error) {
		query := url.Values{}
		if input.History {
			query.Set("history", "true")
		}
		providers, err := client.Providers(ctx, query)
		return nil, ProvidersOutput{Providers: providers}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_get_history", Description: "Read bounded subtitle history for episodes or movies; optionally filter to one explicit media ID."}, func(ctx context.Context, _ *mcp.CallToolRequest, input HistoryInput) (*mcp.CallToolResult, HistoryOutput, error) {
		if input.MediaType != "episode" && input.MediaType != "movie" {
			return nil, HistoryOutput{}, fmt.Errorf("media_type must be episode or movie")
		}
		query, err := pagingQuery(input.Start, input.Length)
		if err != nil {
			return nil, HistoryOutput{}, err
		}
		if input.MediaID < 0 {
			return nil, HistoryOutput{}, fmt.Errorf("media_id cannot be negative")
		}
		if input.MediaID > 0 {
			key := "episodeid"
			if input.MediaType == "movie" {
				key = "radarrid"
			}
			query.Set(key, strconv.Itoa(input.MediaID))
		}
		page, err := client.History(ctx, input.MediaType, query)
		return nil, HistoryOutput{Data: page.Data, Total: page.Total}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_set_series_profile", Description: "Set or clear the language profile of one explicit Sonarr series; does not trigger a subtitle search."}, func(ctx context.Context, _ *mcp.CallToolRequest, input ProfileInput) (*mcp.CallToolResult, ActionOutput, error) {
		if input.SeriesID <= 0 || input.MovieID != 0 {
			return nil, ActionOutput{}, fmt.Errorf("provide only a positive series_id")
		}
		profileID, err := validateProfileID(input.ProfileID)
		if err != nil {
			return nil, ActionOutput{}, err
		}
		if err := client.SetProfile(ctx, "series", input.SeriesID, profileID); err != nil {
			return nil, ActionOutput{}, err
		}
		return nil, ActionOutput{Status: "series profile updated"}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_set_movie_profile", Description: "Set or clear the language profile of one explicit Radarr movie; does not trigger a subtitle search."}, func(ctx context.Context, _ *mcp.CallToolRequest, input ProfileInput) (*mcp.CallToolResult, ActionOutput, error) {
		if input.MovieID <= 0 || input.SeriesID != 0 {
			return nil, ActionOutput{}, fmt.Errorf("provide only a positive movie_id")
		}
		profileID, err := validateProfileID(input.ProfileID)
		if err != nil {
			return nil, ActionOutput{}, err
		}
		if err := client.SetProfile(ctx, "movie", input.MovieID, profileID); err != nil {
			return nil, ActionOutput{}, err
		}
		return nil, ActionOutput{Status: "movie profile updated"}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_search_series_subtitles", Description: "Search/download missing subtitles for one explicit series. This can trigger provider searches and download subtitle files."}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, ActionOutput, error) {
		if input.SeriesID <= 0 || input.MovieID != 0 {
			return nil, ActionOutput{}, fmt.Errorf("provide only a positive series_id")
		}
		if err := client.SearchSubtitles(ctx, "series", input.SeriesID); err != nil {
			return nil, ActionOutput{}, err
		}
		return nil, ActionOutput{Status: "series subtitle search requested"}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_search_movie_subtitles", Description: "Search/download missing subtitles for one explicit movie. This can trigger provider searches and download subtitle files."}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, ActionOutput, error) {
		if input.MovieID <= 0 || input.SeriesID != 0 {
			return nil, ActionOutput{}, fmt.Errorf("provide only a positive movie_id")
		}
		if err := client.SearchSubtitles(ctx, "movie", input.MovieID); err != nil {
			return nil, ActionOutput{}, err
		}
		return nil, ActionOutput{Status: "movie subtitle search requested"}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "bazarr_download_episode_subtitle", Description: "Request a subtitle download for one explicit episode and language. This may write a subtitle file beside the episode media."}, func(ctx context.Context, _ *mcp.CallToolRequest, input DownloadEpisodeInput) (*mcp.CallToolResult, ActionOutput, error) {
		if input.SeriesID <= 0 || input.EpisodeID <= 0 {
			return nil, ActionOutput{}, fmt.Errorf("series_id and episode_id must be positive integers")
		}
		language := strings.TrimSpace(input.Language)
		if !validLanguage(language) {
			return nil, ActionOutput{}, fmt.Errorf("language must be a valid language code")
		}
		if err := client.DownloadEpisodeSubtitle(ctx, input.SeriesID, input.EpisodeID, language, input.Forced, input.HI); err != nil {
			return nil, ActionOutput{}, err
		}
		return nil, ActionOutput{Status: "episode subtitle download requested"}, nil
	})
	return server
}

type ProviderInput struct {
	History bool `json:"history,omitempty" jsonschema:"Include providers that appear in subtitle history instead of current throttled-provider status."`
}

type ProvidersOutput struct {
	Providers []Provider `json:"providers"`
}

type HistoryOutput struct {
	Data  []HistoryItem `json:"data"`
	Total int           `json:"total"`
}

func validateProfileID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "none") {
		return "none", nil
	}
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return "", fmt.Errorf("profile_id must be a positive integer or none")
	}
	return strconv.Itoa(id), nil
}
