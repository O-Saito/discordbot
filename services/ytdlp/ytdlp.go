package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"mydiscordbot/domain"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type realCommandRunner struct{}

func (r *realCommandRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stderr.String(), err
	}
	return stdout.String(), nil
}

type YouTubeService struct {
	cmdRunner  CommandRunner
	binaryPath string
}

func New() domain.YouTubeService {
	return &YouTubeService{
		cmdRunner:  &realCommandRunner{},
		binaryPath: "yt-dlp",
	}
}

func NewWithRunner(runner CommandRunner) domain.YouTubeService {
	return &YouTubeService{
		cmdRunner:  runner,
		binaryPath: "yt-dlp",
	}
}

func NewWithBinaryPath(binaryPath string) domain.YouTubeService {
	return &YouTubeService{
		cmdRunner:  &realCommandRunner{},
		binaryPath: binaryPath,
	}
}

func findFirstJSONLine(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "{") {
			return line
		}
	}
	return ""
}

func (s *YouTubeService) ParseURL(ctx context.Context, url string, callback domain.YouTubeServiceCallback) {
	go func() {
		output, err := s.cmdRunner.Run(ctx, s.binaryPath,
			"--no-warnings",
			"--skip-download",
			"--dump-json",
			url)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				callback(domain.Track{}, fmt.Errorf("timeout: %w", ctx.Err()))
				return
			}

			jsonLine := findFirstJSONLine(output)
			if jsonLine == "" {
				errMsg := strings.TrimSpace(output)
				if errMsg == "" {
					errMsg = err.Error()
				}
				callback(domain.Track{}, fmt.Errorf("yt-dlp error: %s", errMsg))
				return
			}

			callback(domain.Track{}, fmt.Errorf("failed to execute yt-dlp: %w", err))
			return
		}

		jsonLine := findFirstJSONLine(output)
		if jsonLine == "" {
			callback(domain.Track{}, fmt.Errorf("no valid JSON in yt-dlp output: %s", output))
			return
		}

		var video ytDlpVideo
		if err := json.Unmarshal([]byte(jsonLine), &video); err != nil {
			callback(domain.Track{}, fmt.Errorf("failed to parse json: %w", err))
			return
		}

		callback(domain.NewTrackFromYouTube(
			video.WebpageURL,
			video.Title,
			video.Description,
			"",
		), nil)
	}()
}

func (s *YouTubeService) GetAudioURL(ctx context.Context, url string, callback func(string, error)) {
	go func() {
		output, err := s.cmdRunner.Run(ctx, s.binaryPath,
			"--no-warnings",
			"--skip-download",
			"--get-url",
			"-f", "bestaudio",
			url)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				callback("", fmt.Errorf("timeout: %w", ctx.Err()))
				return
			}

			errMsg := strings.TrimSpace(output)
			if errMsg == "" {
				errMsg = err.Error()
			}
			callback("", fmt.Errorf("yt-dlp error: %s", errMsg))
			return
		}

		output = strings.TrimSpace(output)
		if output == "" {
			callback("", fmt.Errorf("no audio URL returned"))
			return
		}

		callback(output, nil)
	}()
}

func durationToString(d interface{}) string {
	switch v := d.(type) {
	case float64:
		seconds := int(v)
		minutes := seconds / 60
		secs := seconds % 60
		return fmt.Sprintf("%d:%02d", minutes, secs)
	case string:
		return v
	default:
		return ""
	}
}

func (s *YouTubeService) Search(ctx context.Context, query string, maxResults int, callback domain.YouTubeServiceSearchCallback) {
	go func() {
		output, err := s.cmdRunner.Run(ctx, s.binaryPath,
			"--no-warnings",
			"--no-playlist",
			"--no-check-certificate",
			"--geo-bypass",
			"--flat-playlist",
			"--skip-download",
			"--dump-json",
			fmt.Sprintf("ytsearch%d:%s", maxResults, query))
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				callback(nil, fmt.Errorf("timeout: %w", ctx.Err()))
				return
			}

			jsonLine := findFirstJSONLine(output)
			if jsonLine == "" {
				errMsg := strings.TrimSpace(output)
				if errMsg == "" {
					errMsg = err.Error()
				}
				callback(nil, fmt.Errorf("yt-dlp error: %s", errMsg))
				return
			}

			callback(nil, fmt.Errorf("failed to execute yt-dlp: %w", err))
			return
		}

		lines := strings.Split(strings.TrimSpace(output), "\n")
		results := make([]domain.SearchResult, 0, len(lines))

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var video ytDlpVideo
			if err := json.Unmarshal([]byte(line), &video); err != nil {
				continue
			}

			results = append(results, domain.SearchResult{
				Title:    video.Title,
				URL:      video.WebpageURL,
				Duration: durationToString(video.Duration),
			})
		}

		callback(results, nil)
	}()
}

func (s *YouTubeService) ParsePlaylist(ctx context.Context, url string, callback domain.YouTubeServicePlaylistCallback) {
	go func() {
		output, err := s.cmdRunner.Run(ctx, s.binaryPath,
			"--no-warnings",
			"--flat-playlist",
			"--skip-download",
			"--dump-json",
			url)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				callback(nil, fmt.Errorf("timeout: %w", ctx.Err()))
				return
			}

			callback(nil, fmt.Errorf("failed to execute yt-dlp: %w", err))
			return
		}

		lines := strings.Split(strings.TrimSpace(output), "\n")
		tracks := make([]domain.Track, 0, len(lines))

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var video ytDlpVideo
			if err := json.Unmarshal([]byte(line), &video); err != nil {
				continue
			}

			tracks = append(tracks, domain.NewTrackFromYouTube(
				video.WebpageURL,
				video.Title,
				video.Description,
				"",
			))
		}

		callback(tracks, nil)
	}()
}

type ytDlpVideo struct {
	WebpageURL  string        `json:"webpage_url"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Formats     []ytDlpFormat `json:"formats"`
	Duration    interface{}   `json:"duration"`
}

type ytDlpFormat struct {
	URL        string `json:"url"`
	Resolution string `json:"resolution"`
}
