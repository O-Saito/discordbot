package domain

import "context"

type YouTubeServiceCallback func(Track, error)
type YouTubeServiceSearchCallback func([]SearchResult, error)
type YouTubeServicePlaylistCallback func([]Track, error)

type YouTubeService interface {
	ParseURL(ctx context.Context, url string, callback YouTubeServiceCallback)
	GetAudioURL(ctx context.Context, url string, callback func(string, error))
	Search(ctx context.Context, query string, maxResults int, callback YouTubeServiceSearchCallback)
	ParsePlaylist(ctx context.Context, url string, callback YouTubeServicePlaylistCallback)
}

type SearchResult struct {
	Title    string
	URL      string
	Duration string
}
