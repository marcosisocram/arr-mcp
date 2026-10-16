package radarr

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var monitorTypes = map[string]bool{"movieOnly": true, "movieAndCollection": true, "none": true}

var minimumAvailabilityValues = map[string]bool{
	"tba": true, "announced": true, "inCinemas": true, "released": true, "deleted": true,
}

type ListMoviesInput struct {
	Monitored *bool `json:"monitored,omitempty" jsonschema:"Return only movies with this monitored state when set."`
	HasFile   *bool `json:"has_file,omitempty" jsonschema:"Return only movies with this file state when set."`
	Limit     int   `json:"limit,omitempty" jsonschema:"Maximum movies to return after filtering (1-500; default 100)."`
	Offset    int   `json:"offset,omitempty" jsonschema:"Movies to skip after filtering."`
}

type MovieIDInput struct {
	MovieID int `json:"movie_id" jsonschema:"Internal Radarr movie ID from radarr_list_movies or radarr_add_movie."`
}

type LookupMoviesInput struct {
	Term  string `json:"term" jsonschema:"Title search term, or an imdb:tt... or tmdb:... identifier."`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum lookup results to return (1-50; default 20)."`
}

type AddMovieInput struct {
	TmdbID              int    `json:"tmdb_id" jsonschema:"The Movie Database (TMDB) identifier of the movie to add."`
	QualityProfileID    int    `json:"quality_profile_id" jsonschema:"Quality profile ID from radarr_list_quality_profiles."`
	RootFolderPath      string `json:"root_folder_path" jsonschema:"Root folder path from radarr_list_root_folders."`
	Monitored           *bool  `json:"monitored,omitempty" jsonschema:"Monitor the movie after adding (default true)."`
	MinimumAvailability string `json:"minimum_availability,omitempty" jsonschema:"tba, announced, inCinemas, released, or deleted (default released)."`
	Monitor             string `json:"monitor,omitempty" jsonschema:"movieOnly, movieAndCollection, or none (default movieOnly)."`
}

type SetMovieMonitoredInput struct {
	MovieID   int  `json:"movie_id" jsonschema:"Internal Radarr movie ID to update."`
	Monitored bool `json:"monitored" jsonschema:"New monitored state for this movie only."`
}

type SearchMovieInput struct {
	MovieID int `json:"movie_id" jsonschema:"Internal Radarr movie ID whose releases should be searched."`
}

type CommandInput struct {
	CommandID int `json:"command_id" jsonschema:"Command ID returned by radarr_search_movie."`
}

type QueueInput struct {
	Page         int  `json:"page,omitempty" jsonschema:"Page number starting at 1 (default 1)."`
	PageSize     int  `json:"page_size,omitempty" jsonschema:"Records per page (1-100; default 25)."`
	IncludeMovie bool `json:"include_movie,omitempty" jsonschema:"Embed the full movie resource in each record."`
}

type HistoryInput struct {
	Page     int `json:"page,omitempty" jsonschema:"Page number starting at 1 (default 1)."`
	PageSize int `json:"page_size,omitempty" jsonschema:"Records per page (1-100; default 25)."`
}

type AddMovieOutput struct {
	Status string `json:"status" jsonschema:"created when a new movie was added, or already_exists when the movie was already in the library."`
	Movie  Movie  `json:"movie" jsonschema:"The Radarr movie now in the library."`
}

type EmptyInput struct{}

func NewMCPServer(client *Client) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "radarr-mcp", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "radarr_list_movies", Description: "List movies in the Radarr library with optional monitored and has-file filters and local paging."}, func(ctx context.Context, _ *mcp.CallToolRequest, input ListMoviesInput) (*mcp.CallToolResult, []Movie, error) {
		movies, err := client.ListMovies(ctx)
		if err != nil {
			return nil, nil, err
		}
		filtered, err := movieListFilter(movies, input.Monitored, input.HasFile, input.Offset, input.Limit)
		if err != nil {
			return nil, nil, err
		}
		return nil, filtered, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "radarr_get_movie", Description: "Read one movie from the Radarr library by its internal Radarr ID."}, func(ctx context.Context, _ *mcp.CallToolRequest, input MovieIDInput) (*mcp.CallToolResult, Movie, error) {
		if input.MovieID <= 0 {
			return nil, Movie{}, fmt.Errorf("movie_id must be a positive integer")
		}
		movie, err := client.MovieByID(ctx, input.MovieID)
		return nil, movie, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "radarr_lookup_movies", Description: "Search Radarr's movie metadata source for candidate movies to add; returns candidate TMDB identifiers only."}, func(ctx context.Context, _ *mcp.CallToolRequest, input LookupMoviesInput) (*mcp.CallToolResult, []Movie, error) {
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
		movies, err := client.LookupMovies(ctx, term)
		if err != nil {
			return nil, nil, err
		}
		if len(movies) > limit {
			movies = movies[:limit]
		}
		return nil, movies, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "radarr_list_quality_profiles", Description: "List the quality profiles available when adding a movie."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, []QualityProfile, error) {
		profiles, err := client.QualityProfiles(ctx)
		return nil, profiles, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "radarr_list_root_folders", Description: "List the root folders available when adding a movie."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, []RootFolder, error) {
		folders, err := client.RootFolders(ctx)
		return nil, folders, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "radarr_add_movie", Description: "Add one movie by TMDB ID with an explicit quality profile and root folder. Never starts a release search and never creates a duplicate."}, func(ctx context.Context, _ *mcp.CallToolRequest, input AddMovieInput) (*mcp.CallToolResult, AddMovieOutput, error) {
		output, err := addMovie(ctx, client, input)
		return nil, output, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "radarr_set_movie_monitored", Description: "Set the monitored flag of one explicit movie and change no other field."}, func(ctx context.Context, _ *mcp.CallToolRequest, input SetMovieMonitoredInput) (*mcp.CallToolResult, Movie, error) {
		movie, err := setMovieMonitored(ctx, client, input.MovieID, input.Monitored)
		return nil, movie, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "radarr_search_movie", Description: "Ask Radarr to search indexers for releases of one explicit movie. This can start a download."}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchMovieInput) (*mcp.CallToolResult, Command, error) {
		if input.MovieID <= 0 {
			return nil, Command{}, fmt.Errorf("movie_id must be a positive integer")
		}
		command, err := client.SearchMovies(ctx, input.MovieID)
		return nil, command, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "radarr_get_command", Description: "Read the current state of a Radarr command, such as the one returned by radarr_search_movie."}, func(ctx context.Context, _ *mcp.CallToolRequest, input CommandInput) (*mcp.CallToolResult, Command, error) {
		if input.CommandID <= 0 {
			return nil, Command{}, fmt.Errorf("command_id must be a positive integer")
		}
		command, err := client.CommandByID(ctx, input.CommandID)
		return nil, command, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "radarr_get_queue", Description: "Read one page of the Radarr download queue."}, func(ctx context.Context, _ *mcp.CallToolRequest, input QueueInput) (*mcp.CallToolResult, QueuePage, error) {
		query, err := pageQuery(input.Page, input.PageSize)
		if err != nil {
			return nil, QueuePage{}, err
		}
		if input.IncludeMovie {
			query.Set("includeMovie", "true")
		}
		page, err := client.Queue(ctx, query)
		return nil, page, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "radarr_get_history", Description: "Read one page of the Radarr movie history."}, func(ctx context.Context, _ *mcp.CallToolRequest, input HistoryInput) (*mcp.CallToolResult, HistoryPage, error) {
		query, err := pageQuery(input.Page, input.PageSize)
		if err != nil {
			return nil, HistoryPage{}, err
		}
		page, err := client.History(ctx, query)
		return nil, page, err
	})
	return server
}

func addMovie(ctx context.Context, client *Client, input AddMovieInput) (AddMovieOutput, error) {
	if input.TmdbID <= 0 {
		return AddMovieOutput{}, fmt.Errorf("tmdb_id must be a positive integer")
	}
	if input.QualityProfileID <= 0 {
		return AddMovieOutput{}, fmt.Errorf("quality_profile_id must be a positive integer")
	}
	if strings.TrimSpace(input.RootFolderPath) == "" {
		return AddMovieOutput{}, fmt.Errorf("root_folder_path is required")
	}
	monitored := true
	if input.Monitored != nil {
		monitored = *input.Monitored
	}
	availability := input.MinimumAvailability
	if availability == "" {
		availability = "released"
	}
	if !minimumAvailabilityValues[availability] {
		return AddMovieOutput{}, fmt.Errorf("minimum_availability must be tba, announced, inCinemas, released, or deleted")
	}
	monitor := input.Monitor
	if monitor == "" {
		monitor = "movieOnly"
	}
	if !monitorTypes[monitor] {
		return AddMovieOutput{}, fmt.Errorf("monitor must be movieOnly, movieAndCollection, or none")
	}
	existing, err := client.MoviesByTmdbID(ctx, input.TmdbID)
	if err != nil {
		return AddMovieOutput{}, err
	}
	if len(existing) > 0 {
		return AddMovieOutput{Status: "already_exists", Movie: existing[0]}, nil
	}
	resource, err := client.LookupMovieByTmdbID(ctx, input.TmdbID)
	if err != nil {
		return AddMovieOutput{}, err
	}
	if _, found := resource["title"]; !found {
		return AddMovieOutput{}, fmt.Errorf("Radarr returned no movie metadata for tmdb_id %d", input.TmdbID)
	}
	resource["qualityProfileId"] = input.QualityProfileID
	resource["rootFolderPath"] = input.RootFolderPath
	resource["monitored"] = monitored
	resource["minimumAvailability"] = availability
	resource["addOptions"] = map[string]any{"monitor": monitor, "searchForMovie": false}
	movie, err := client.AddMovie(ctx, resource)
	if err != nil {
		return AddMovieOutput{}, err
	}
	return AddMovieOutput{Status: "created", Movie: movie}, nil
}

func setMovieMonitored(ctx context.Context, client *Client, movieID int, monitored bool) (Movie, error) {
	if movieID <= 0 {
		return Movie{}, fmt.Errorf("movie_id must be a positive integer")
	}
	resource, err := client.MovieResource(ctx, movieID)
	if err != nil {
		return Movie{}, err
	}
	resource["monitored"] = monitored
	return client.UpdateMovie(ctx, movieID, resource)
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

func movieListFilter(movies []Movie, monitored, hasFile *bool, offset, limit int) ([]Movie, error) {
	if offset < 0 {
		return nil, fmt.Errorf("offset cannot be negative")
	}
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 500 {
		return nil, fmt.Errorf("limit must be between 1 and 500")
	}
	filtered := make([]Movie, 0, len(movies))
	for _, movie := range movies {
		if monitored != nil && movie.Monitored != *monitored {
			continue
		}
		if hasFile != nil && movie.HasFile != *hasFile {
			continue
		}
		filtered = append(filtered, movie)
	}
	if offset >= len(filtered) {
		return []Movie{}, nil
	}
	filtered = filtered[offset:]
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}
