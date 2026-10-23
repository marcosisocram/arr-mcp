package qbittorrent

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var torrentHashPattern = regexp.MustCompile(`^(?:[a-fA-F0-9]{40}|[a-fA-F0-9]{64})$`)

type ListTorrentsInput struct {
	Filter   string `json:"filter,omitempty" jsonschema:"Filter: all, downloading, seeding, completed, stopped, active, inactive, running, stalled, stalled_uploading, stalled_downloading, or errored."`
	Category string `json:"category,omitempty" jsonschema:"Filter by category."`
	Tag      string `json:"tag,omitempty" jsonschema:"Filter by tag."`
	Limit    int    `json:"limit,omitempty" jsonschema:"Maximum number of results (1-500; default 100)."`
	Offset   int    `json:"offset,omitempty" jsonschema:"Number of results to skip."`
}

type TorrentPropertiesInput struct {
	Hash string `json:"hash" jsonschema:"Torrent hash (40 or 64 hexadecimal characters)."`
}

type TorrentQueryInput struct {
	Hash  string `json:"hash" jsonschema:"Torrent hash (40 or 64 hexadecimal characters)."`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum results (files 1-5000, peers 1-1000; defaults are 500 and 200)."`
}

type DeleteTorrentsInput struct {
	Hashes      []string `json:"hashes" jsonschema:"Explicit list of 1-100 torrent hashes; wildcard all is not supported."`
	Confirm     bool     `json:"confirm,omitempty" jsonschema:"Must be true to remove the explicitly listed torrents; omission cancels the operation."`
	DeleteFiles bool     `json:"delete_files,omitempty" jsonschema:"Permanently delete downloaded files as well; defaults to false."`
}

type DeleteTorrentsOutput struct {
	Status    string   `json:"status,omitempty"`
	Cancelled bool     `json:"cancelled,omitempty"`
	Operation string   `json:"operation,omitempty"`
	Hashes    []string `json:"hashes,omitempty"`
}

type BoundedFilesOutput struct {
	Files     []TorrentFile `json:"files"`
	Truncated bool          `json:"truncated"`
}

type BoundedPeersOutput struct {
	Peers     []Peer `json:"peers"`
	Truncated bool   `json:"truncated"`
}

type TrackersOutput struct {
	Trackers []Tracker `json:"trackers"`
}
type CategoriesOutput struct {
	Categories []Category `json:"categories"`
}
type TagsOutput struct {
	Tags []string `json:"tags"`
}
type AppVersionOutput struct {
	Version       string `json:"version"`
	WebAPIVersion string `json:"webapi_version"`
}
type ServerStateOutput struct {
	ServerState map[string]any `json:"server_state"`
}

type TorrentHashesInput struct {
	Hashes []string `json:"hashes" jsonschema:"Explicit list of torrent hashes to pause or resume; wildcard all is not supported."`
}

type SetTransferLimitsInput struct {
	DownloadLimit int64 `json:"download_limit" jsonschema:"Global download limit in bytes per second; zero means unlimited."`
	UploadLimit   int64 `json:"upload_limit" jsonschema:"Global upload limit in bytes per second; zero means unlimited."`
}

type EmptyInput struct{}

type ActionOutput struct {
	Status string `json:"status" jsonschema:"Operation result."`
}

type TransferLimitsOutput struct {
	DownloadLimit int64 `json:"download_limit" jsonschema:"Download limit in bytes per second; zero means unlimited."`
	UploadLimit   int64 `json:"upload_limit" jsonschema:"Upload limit in bytes per second; zero means unlimited."`
}

type ListTorrentsOutput struct {
	Torrents []Torrent `json:"torrents" jsonschema:"Torrents matching the filters."`
}

type SpeedLimitsModeOutput struct {
	Enabled bool `json:"enabled" jsonschema:"Whether alternative speed limits are enabled."`
}

func NewMCPServer(client *Client) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "qbittorrent-mcp", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_list_torrents", Description: "List torrents with optional status, category, tag, and paging filters."}, func(ctx context.Context, _ *mcp.CallToolRequest, input ListTorrentsInput) (*mcp.CallToolResult, ListTorrentsOutput, error) {
		query, err := torrentListQuery(input)
		if err != nil {
			return nil, ListTorrentsOutput{}, err
		}
		result, err := client.ListTorrents(ctx, query)
		return nil, ListTorrentsOutput{Torrents: result}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_torrent_properties", Description: "Read operational properties for one torrent."}, func(ctx context.Context, _ *mcp.CallToolRequest, input TorrentPropertiesInput) (*mcp.CallToolResult, TorrentProperties, error) {
		if !torrentHashPattern.MatchString(input.Hash) {
			return nil, TorrentProperties{}, fmt.Errorf("hash must contain 40 or 64 hexadecimal characters")
		}
		result, err := client.TorrentProperties(ctx, input.Hash)
		return nil, result, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_transfer_info", Description: "Read global transfer rates, totals, limits, and connection state."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, TransferInfo, error) {
		result, err := client.TransferInfo(ctx)
		return nil, result, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_get_transfer_limits", Description: "Read global download and upload speed limits in bytes per second."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, TransferLimitsOutput, error) {
		download, upload, err := client.TransferLimits(ctx)
		return nil, TransferLimitsOutput{DownloadLimit: download, UploadLimit: upload}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_get_speed_limits_mode", Description: "Read whether qBittorrent alternative speed limits are enabled."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, SpeedLimitsModeOutput, error) {
		mode, err := client.SpeedLimitsMode(ctx)
		return nil, SpeedLimitsModeOutput{Enabled: mode == 1}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_pause_torrents", Description: "Pause only the explicitly listed torrents; the all-torrents wildcard is not accepted."}, func(ctx context.Context, _ *mcp.CallToolRequest, input TorrentHashesInput) (*mcp.CallToolResult, ActionOutput, error) {
		if err := validateHashes(input.Hashes); err != nil {
			return nil, ActionOutput{}, err
		}
		if err := client.SetTorrentsPaused(ctx, input.Hashes, true); err != nil {
			return nil, ActionOutput{}, err
		}
		return nil, ActionOutput{Status: "paused"}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_resume_torrents", Description: "Resume only the explicitly listed torrents; the all-torrents wildcard is not accepted."}, func(ctx context.Context, _ *mcp.CallToolRequest, input TorrentHashesInput) (*mcp.CallToolResult, ActionOutput, error) {
		if err := validateHashes(input.Hashes); err != nil {
			return nil, ActionOutput{}, err
		}
		if err := client.SetTorrentsPaused(ctx, input.Hashes, false); err != nil {
			return nil, ActionOutput{}, err
		}
		return nil, ActionOutput{Status: "resumed"}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_set_transfer_limits", Description: "Set global download and upload speed limits in bytes per second; zero means unlimited."}, func(ctx context.Context, _ *mcp.CallToolRequest, input SetTransferLimitsInput) (*mcp.CallToolResult, ActionOutput, error) {
		if input.DownloadLimit < 0 || input.UploadLimit < 0 {
			return nil, ActionOutput{}, fmt.Errorf("transfer limits cannot be negative")
		}
		if err := client.SetTransferLimits(ctx, input.DownloadLimit, input.UploadLimit); err != nil {
			return nil, ActionOutput{}, err
		}
		return nil, ActionOutput{Status: "transfer limits updated"}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_toggle_speed_limits", Description: "Toggle qBittorrent alternative speed limits on or off."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, ActionOutput, error) {
		if err := client.ToggleSpeedLimitsMode(ctx); err != nil {
			return nil, ActionOutput{}, err
		}
		return nil, ActionOutput{Status: "alternative speed limits toggled"}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_list_torrent_files", Description: "List files in one torrent, capped at 500 by default and 5000 maximum; reports truncation."}, func(ctx context.Context, _ *mcp.CallToolRequest, input TorrentQueryInput) (*mcp.CallToolResult, BoundedFilesOutput, error) {
		if err := validateTorrentHash(input.Hash); err != nil {
			return nil, BoundedFilesOutput{}, err
		}
		limit, err := boundedLimit(input.Limit, 500, 5000)
		if err != nil {
			return nil, BoundedFilesOutput{}, err
		}
		files, err := client.TorrentFiles(ctx, input.Hash)
		if err != nil {
			return nil, BoundedFilesOutput{}, err
		}
		truncated := len(files) > limit
		if truncated {
			files = files[:limit]
		}
		return nil, BoundedFilesOutput{Files: files, Truncated: truncated}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_list_torrent_trackers", Description: "List trackers for one torrent; userinfo and sensitive URL query values are redacted."}, func(ctx context.Context, _ *mcp.CallToolRequest, input TorrentPropertiesInput) (*mcp.CallToolResult, TrackersOutput, error) {
		if err := validateTorrentHash(input.Hash); err != nil {
			return nil, TrackersOutput{}, err
		}
		result, err := client.TorrentTrackers(ctx, input.Hash)
		return nil, TrackersOutput{Trackers: result}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_list_torrent_peers", Description: "List connected peers for one torrent, capped at 200 by default and 1000 maximum."}, func(ctx context.Context, _ *mcp.CallToolRequest, input TorrentQueryInput) (*mcp.CallToolResult, BoundedPeersOutput, error) {
		if err := validateTorrentHash(input.Hash); err != nil {
			return nil, BoundedPeersOutput{}, err
		}
		limit, err := boundedLimit(input.Limit, 200, 1000)
		if err != nil {
			return nil, BoundedPeersOutput{}, err
		}
		peers, err := client.TorrentPeers(ctx, input.Hash)
		if err != nil {
			return nil, BoundedPeersOutput{}, err
		}
		truncated := len(peers) > limit
		if truncated {
			peers = peers[:limit]
		}
		return nil, BoundedPeersOutput{Peers: peers, Truncated: truncated}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_list_categories", Description: "List configured torrent categories and save paths."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, CategoriesOutput, error) {
		result, err := client.Categories(ctx)
		return nil, CategoriesOutput{Categories: result}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_list_tags", Description: "List configured torrent tags."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, TagsOutput, error) {
		result, err := client.Tags(ctx)
		return nil, TagsOutput{Tags: result}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_app_version", Description: "Read qBittorrent application and WebAPI versions."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, AppVersionOutput, error) {
		version, webapi, err := client.AppVersion(ctx)
		return nil, AppVersionOutput{Version: version, WebAPIVersion: webapi}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_server_state", Description: "Read an allowlisted snapshot of qBittorrent server state."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, ServerStateOutput, error) {
		result, err := client.ServerState(ctx)
		return nil, ServerStateOutput{ServerState: result}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "qbittorrent_delete_torrents", Description: "Remove 1-100 explicitly listed torrents. Requires confirm=true. delete_files defaults to false; when true, downloaded data is permanently deleted."}, func(ctx context.Context, _ *mcp.CallToolRequest, input DeleteTorrentsInput) (*mcp.CallToolResult, DeleteTorrentsOutput, error) {
		if err := validateHashes(input.Hashes); err != nil {
			return nil, DeleteTorrentsOutput{}, err
		}
		if !input.Confirm {
			return nil, DeleteTorrentsOutput{Cancelled: true, Operation: "qbittorrent_delete_torrents", Hashes: input.Hashes}, nil
		}
		if err := client.DeleteTorrents(ctx, input.Hashes, input.DeleteFiles); err != nil {
			return nil, DeleteTorrentsOutput{}, err
		}
		return nil, DeleteTorrentsOutput{Status: "deleted"}, nil
	})
	return server
}

func validateTorrentHash(hash string) error {
	if !torrentHashPattern.MatchString(hash) {
		return fmt.Errorf("hash must contain 40 or 64 hexadecimal characters")
	}
	return nil
}

func boundedLimit(requested, defaultValue, maximum int) (int, error) {
	if requested == 0 {
		return defaultValue, nil
	}
	if requested < 1 || requested > maximum {
		return 0, fmt.Errorf("limit must be between 1 and %d", maximum)
	}
	return requested, nil
}

func torrentListQuery(input ListTorrentsInput) (url.Values, error) {
	filters := map[string]bool{
		"": true, "all": true, "downloading": true, "seeding": true,
		"completed": true, "stopped": true, "active": true, "inactive": true,
		"running": true, "stalled": true, "stalled_uploading": true,
		"stalled_downloading": true, "errored": true,
	}
	if !filters[input.Filter] {
		return nil, fmt.Errorf("unsupported torrent filter")
	}
	if input.Offset < 0 {
		return nil, fmt.Errorf("offset cannot be negative")
	}
	limit := input.Limit
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 500 {
		return nil, fmt.Errorf("limit must be between 1 and 500")
	}
	query := url.Values{"limit": {fmt.Sprint(limit)}, "offset": {fmt.Sprint(input.Offset)}}
	if input.Filter != "" {
		query.Set("filter", input.Filter)
	}
	if input.Category != "" {
		query.Set("category", input.Category)
	}
	if input.Tag != "" {
		query.Set("tag", input.Tag)
	}
	return query, nil
}

func validateHashes(hashes []string) error {
	if len(hashes) == 0 || len(hashes) > 100 {
		return fmt.Errorf("provide between 1 and 100 explicit torrent hashes")
	}
	for _, hash := range hashes {
		if strings.EqualFold(hash, "all") || !torrentHashPattern.MatchString(hash) {
			return fmt.Errorf("each hash must be 40 or 64 hexadecimal characters; wildcard all is not supported")
		}
	}
	return nil
}
