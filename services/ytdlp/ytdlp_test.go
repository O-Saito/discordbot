package ytdlp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mydiscordbot/domain"
)

type mockRunner struct {
	output string
	err    error
}

func (m *mockRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	return m.output, m.err
}

func TestFindFirstJSONLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "finds JSON line",
			input:    "Downloading...\n{\"title\": \"Test\", \"url\": \"http://example.com\"}\nOther text",
			expected: "{\"title\": \"Test\", \"url\": \"http://example.com\"}",
		},
		{
			name:     "no JSON returns empty",
			input:    "Just some text\nNo JSON here",
			expected: "",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "JSON at start",
			input:    "{\"key\": \"value\"}",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "whitespace before JSON",
			input:    "   {\"key\": \"value\"}",
			expected: "{\"key\": \"value\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findFirstJSONLine(tt.input)
			if result != tt.expected {
				t.Errorf("findFirstJSONLine(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDurationToString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "float64 seconds",
			input:    125.0,
			expected: "2:05",
		},
		{
			name:     "float64 zero",
			input:    0.0,
			expected: "0:00",
		},
		{
			name:     "float64 large",
			input:    3661.0,
			expected: "61:01",
		},
		{
			name:     "string duration",
			input:    "1:30",
			expected: "1:30",
		},
		{
			name:     "nil returns empty",
			input:    nil,
			expected: "",
		},
		{
			name:     "int returns empty",
			input:    100,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := durationToString(tt.input)
			if result != tt.expected {
				t.Errorf("durationToString(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNew(t *testing.T) {
	svc := New()
	if svc == nil {
		t.Error("expected non-nil YouTubeService")
	}

	yt, ok := svc.(*YouTubeService)
	if !ok {
		t.Error("expected YouTubeService type")
	}

	if yt.binaryPath != "yt-dlp" {
		t.Errorf("expected binary path 'yt-dlp', got %q", yt.binaryPath)
	}

	if yt.cmdRunner == nil {
		t.Error("expected non-nil command runner")
	}
}

func TestNewWithRunner(t *testing.T) {
	mock := &mockRunner{output: "test"}
	svc := NewWithRunner(mock)

	yt, ok := svc.(*YouTubeService)
	if !ok {
		t.Error("expected YouTubeService type")
	}

	if yt.cmdRunner != mock {
		t.Error("expected custom runner to be used")
	}
}

func TestNewWithBinaryPath(t *testing.T) {
	svc := NewWithBinaryPath("/custom/path/yt-dlp")

	yt, ok := svc.(*YouTubeService)
	if !ok {
		t.Error("expected YouTubeService type")
	}

	if yt.binaryPath != "/custom/path/yt-dlp" {
		t.Errorf("expected binary path '/custom/path/yt-dlp', got %q", yt.binaryPath)
	}
}

func TestParseURL(t *testing.T) {
	jsonOutput := `{"webpage_url": "https://youtube.com/watch?v=abc123", "title": "Test Video", "description": "Description here"}`

	t.Run("success", func(t *testing.T) {
		mock := &mockRunner{output: jsonOutput}
		svc := &YouTubeService{cmdRunner: mock, binaryPath: "yt-dlp"}

		done := make(chan bool)
		var resultTrack domain.Track
		var resultErr error

		svc.ParseURL(context.Background(), "https://youtube.com/watch?v=abc123", func(track domain.Track, err error) {
			resultTrack = track
			resultErr = err
			done <- true
		})

		<-done

		if resultErr != nil {
			t.Errorf("unexpected error: %v", resultErr)
		}

		if resultTrack.Title() != "Test Video" {
			t.Errorf("expected title 'Test Video', got %q", resultTrack.Title())
		}
		if resultTrack.URL() != "https://youtube.com/watch?v=abc123" {
			t.Errorf("expected URL 'https://youtube.com/watch?v=abc123', got %q", resultTrack.URL())
		}
	})

	t.Run("yt-dlp error returns error", func(t *testing.T) {
		mock := &mockRunner{output: "Error message", err: errors.New("command failed")}
		svc := &YouTubeService{cmdRunner: mock, binaryPath: "yt-dlp"}

		done := make(chan bool)
		var resultErr error

		svc.ParseURL(context.Background(), "https://youtube.com/watch?v=abc123", func(track domain.Track, err error) {
			resultErr = err
			done <- true
		})

		<-done

		if resultErr == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("no valid JSON returns error", func(t *testing.T) {
		mock := &mockRunner{output: "Not a JSON output"}
		svc := &YouTubeService{cmdRunner: mock, binaryPath: "yt-dlp"}

		done := make(chan bool)
		var resultErr error

		svc.ParseURL(context.Background(), "https://youtube.com/watch?v=abc123", func(track domain.Track, err error) {
			resultErr = err
			done <- true
		})

		<-done

		if resultErr == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("timeout returns timeout error", func(t *testing.T) {
		mock := &mockRunner{output: "", err: context.DeadlineExceeded}
		svc := &YouTubeService{cmdRunner: mock, binaryPath: "yt-dlp"}

		done := make(chan bool)
		var resultErr error

		svc.ParseURL(context.Background(), "https://youtube.com/watch?v=abc123", func(track domain.Track, err error) {
			resultErr = err
			done <- true
		})

		<-done

		if resultErr == nil {
			t.Error("expected error, got nil")
		}
		if !strings.Contains(resultErr.Error(), "deadline") {
			t.Errorf("expected deadline error, got %v", resultErr)
		}
	})
}

func TestGetAudioURL(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &mockRunner{output: "https://example.com/audio.m4a"}
		svc := &YouTubeService{cmdRunner: mock, binaryPath: "yt-dlp"}

		done := make(chan bool)
		var resultURL string
		var resultErr error

		svc.GetAudioURL(context.Background(), "https://youtube.com/watch?v=abc123", func(url string, err error) {
			resultURL = url
			resultErr = err
			done <- true
		})

		<-done

		if resultErr != nil {
			t.Errorf("unexpected error: %v", resultErr)
		}

		if resultURL != "https://example.com/audio.m4a" {
			t.Errorf("expected audio URL, got %q", resultURL)
		}
	})

	t.Run("empty output returns error", func(t *testing.T) {
		mock := &mockRunner{output: ""}
		svc := &YouTubeService{cmdRunner: mock, binaryPath: "yt-dlp"}

		done := make(chan bool)
		var resultErr error

		svc.GetAudioURL(context.Background(), "https://youtube.com/watch?v=abc123", func(url string, err error) {
			resultErr = err
			done <- true
		})

		<-done

		if resultErr == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("command error returns error", func(t *testing.T) {
		mock := &mockRunner{output: "", err: errors.New("command failed")}
		svc := &YouTubeService{cmdRunner: mock, binaryPath: "yt-dlp"}

		done := make(chan bool)
		var resultErr error

		svc.GetAudioURL(context.Background(), "https://youtube.com/watch?v=abc123", func(url string, err error) {
			resultErr = err
			done <- true
		})

		<-done

		if resultErr == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestSearch(t *testing.T) {
	jsonOutput := `{"webpage_url": "https://youtube.com/watch?v=1", "title": "Video 1", "duration": 120}
{"webpage_url": "https://youtube.com/watch?v=2", "title": "Video 2", "duration": 180}`

	t.Run("success parses results", func(t *testing.T) {
		mock := &mockRunner{output: jsonOutput}
		svc := &YouTubeService{cmdRunner: mock, binaryPath: "yt-dlp"}

		done := make(chan bool)
		var results []domain.SearchResult
		var resultErr error

		svc.Search(context.Background(), "test query", 5, func(searchResults []domain.SearchResult, err error) {
			results = searchResults
			resultErr = err
			done <- true
		})

		<-done

		if resultErr != nil {
			t.Errorf("unexpected error: %v", resultErr)
		}

		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}

		if results[0].Title != "Video 1" {
			t.Errorf("expected title 'Video 1', got %q", results[0].Title)
		}
		if results[0].URL != "https://youtube.com/watch?v=1" {
			t.Errorf("expected URL 'https://youtube.com/watch?v=1', got %q", results[0].URL)
		}
		if results[0].Duration != "2:00" {
			t.Errorf("expected duration '2:00', got %q", results[0].Duration)
		}
	})

	t.Run("empty output returns empty slice", func(t *testing.T) {
		mock := &mockRunner{output: ""}
		svc := &YouTubeService{cmdRunner: mock, binaryPath: "yt-dlp"}

		done := make(chan bool)
		var results []domain.SearchResult
		var resultErr error

		svc.Search(context.Background(), "test query", 5, func(searchResults []domain.SearchResult, err error) {
			results = searchResults
			resultErr = err
			done <- true
		})

		<-done

		if resultErr != nil {
			t.Errorf("unexpected error: %v", resultErr)
		}

		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("error returns error", func(t *testing.T) {
		mock := &mockRunner{output: "", err: errors.New("command failed")}
		svc := &YouTubeService{cmdRunner: mock, binaryPath: "yt-dlp"}

		done := make(chan bool)
		var resultErr error

		svc.Search(context.Background(), "test query", 5, func(results []domain.SearchResult, err error) {
			resultErr = err
			done <- true
		})

		<-done

		if resultErr == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestParsePlaylist(t *testing.T) {
	jsonOutput := `{"webpage_url": "https://youtube.com/watch?v=1", "title": "Song 1", "description": "Desc 1"}
{"webpage_url": "https://youtube.com/watch?v=2", "title": "Song 2", "description": "Desc 2"}`

	t.Run("success parses tracks", func(t *testing.T) {
		mock := &mockRunner{output: jsonOutput}
		svc := &YouTubeService{cmdRunner: mock, binaryPath: "yt-dlp"}

		done := make(chan bool)
		var tracks []domain.Track
		var resultErr error

		svc.ParsePlaylist(context.Background(), "https://youtube.com/playlist?list=PL123", func(resultTracks []domain.Track, err error) {
			tracks = resultTracks
			resultErr = err
			done <- true
		})

		<-done

		if resultErr != nil {
			t.Errorf("unexpected error: %v", resultErr)
		}

		if len(tracks) != 2 {
			t.Errorf("expected 2 tracks, got %d", len(tracks))
		}

		if tracks[0].Title() != "Song 1" {
			t.Errorf("expected title 'Song 1', got %q", tracks[0].Title())
		}
		if tracks[1].Title() != "Song 2" {
			t.Errorf("expected title 'Song 2', got %q", tracks[1].Title())
		}
	})

	t.Run("error returns error", func(t *testing.T) {
		mock := &mockRunner{output: "", err: errors.New("command failed")}
		svc := &YouTubeService{cmdRunner: mock, binaryPath: "yt-dlp"}

		done := make(chan bool)
		var resultErr error

		svc.ParsePlaylist(context.Background(), "https://youtube.com/playlist?list=PL123", func(tracks []domain.Track, err error) {
			resultErr = err
			done <- true
		})

		<-done

		if resultErr == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("timeout returns timeout error", func(t *testing.T) {
		mock := &mockRunner{output: "", err: context.DeadlineExceeded}
		svc := &YouTubeService{cmdRunner: mock, binaryPath: "yt-dlp"}

		done := make(chan bool)
		var resultErr error

		svc.ParsePlaylist(context.Background(), "https://youtube.com/playlist?list=PL123", func(tracks []domain.Track, err error) {
			resultErr = err
			done <- true
		})

		<-done

		if resultErr == nil {
			t.Error("expected error, got nil")
		}
		if !errors.Is(resultErr, context.DeadlineExceeded) {
			t.Errorf("expected DeadlineExceeded error, got %v", resultErr)
		}
	})
}

func TestYouTubeServiceImplementsInterface(t *testing.T) {
	var _ domain.YouTubeService = (*YouTubeService)(nil)
}

func TestRealCommandRunner(t *testing.T) {
	t.Run("returns error for non-existent command", func(t *testing.T) {
		runner := &realCommandRunner{}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := runner.Run(ctx, "nonexistent-command-xyz")
		if err == nil {
			t.Error("expected error for non-existent command")
		}
	})
}
