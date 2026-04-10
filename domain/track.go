package domain

import (
	"errors"
	"path/filepath"
	"strings"
)

type TrackType string

const (
	TrackTypeYouTube TrackType = "YouTube"
	TrackTypeFile    TrackType = "File"
)

func (t TrackType) String() string {
	switch t {
	case TrackTypeYouTube:
		return "YouTube"
	case TrackTypeFile:
		return "File"
	default:
		return string(t)
	}
}

type Track struct {
	url         string
	title       string
	description string
	audioURL    string
	Type        TrackType
}

var (
	ErrEmptyURL      = errors.New("URL cannot be empty")
	ErrEmptyTitle    = errors.New("title cannot be empty")
	ErrEmptyAudioURL = errors.New("audio URL cannot be empty")
	ErrEmptyType     = errors.New("track type cannot be empty")
)

func NewTrack(url, title, description, audioURL string, trackType TrackType) Track {
	return Track{
		url:         url,
		title:       title,
		description: description,
		audioURL:    audioURL,
		Type:        trackType,
	}
}

func NewTrackFromYouTube(url, title, description, audioURL string) Track {
	return NewTrack(url, title, description, audioURL, TrackTypeYouTube)
}

func NewTrackFromFile(path string) Track {
	return NewTrackFromFileWithFolder(path, "")
}

func NewTrackFromFileWithFolder(path, relativeFolder string) Track {
	title := filepath.Base(path)
	if relativeFolder != "" {
		title = relativeFolder + "/" + title
	}
	return NewTrack(path, title, "", path, TrackTypeFile)
}

func (t *Track) IsValid() error {
	if strings.TrimSpace(t.url) == "" {
		return ErrEmptyURL
	}
	if strings.TrimSpace(t.title) == "" {
		return ErrEmptyTitle
	}
	if strings.TrimSpace(t.audioURL) == "" {
		return ErrEmptyAudioURL
	}
	if strings.TrimSpace(string(t.Type)) == "" {
		return ErrEmptyType
	}
	return nil
}

func (t Track) Title() string       { return t.title }
func (t Track) URL() string         { return t.url }
func (t Track) AudioURL() string    { return t.audioURL }
func (t Track) Description() string { return t.description }

func (t *Track) SetAudioURL(url string) {
	t.audioURL = url
}
