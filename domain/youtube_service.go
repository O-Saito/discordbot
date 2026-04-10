package domain

import "context"

type YouTubeService interface {
	ParseURL(ctx context.Context, url string) (Track, error)
	GetAudioURL(ctx context.Context, url string) (string, error)
	Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error)
	ParsePlaylist(ctx context.Context, url string) ([]Track, error)
}

type SearchResult struct {
	Title    string
	URL      string
	Duration string
}
