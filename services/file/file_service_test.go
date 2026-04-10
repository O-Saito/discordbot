package file

import (
	"os"
	"path/filepath"
	"testing"

	"mydiscordbot/domain"
)

func TestIsAudioOrVideo(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		{"mp3 is audio", ".mp3", true},
		{"wav is audio", ".wav", true},
		{"flac is audio", ".flac", true},
		{"ogg is audio", ".ogg", true},
		{"m4a is audio", ".m4a", true},
		{"aac is audio", ".aac", true},
		{"wma is audio", ".wma", true},
		{"mp4 is video", ".mp4", true},
		{"avi is video", ".avi", true},
		{"mkv is video", ".mkv", true},
		{"mov is video", ".mov", true},
		{"webm is video", ".webm", true},
		{"txt is not media", ".txt", false},
		{"pdf is not media", ".pdf", false},
		{"jpg is not media", ".jpg", false},
		{"PNG is png image (not video)", ".PNG", false},
		{"MP3 is audio (case insensitive)", ".MP3", true},
		{"empty extension", "", false},
	}

	s := &FileService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.isAudioOrVideo(tt.ext)
			if result != tt.expected {
				t.Errorf("isAudioOrVideo(%q) = %v, want %v", tt.ext, result, tt.expected)
			}
		})
	}
}

func TestCollectFiles(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "song.mp3"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "video.mp4"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("content"), 0644)

	subDir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "nested.mp3"), []byte("content"), 0644)

	s := &FileService{}

	t.Run("non-recursive collects only top level", func(t *testing.T) {
		files := s.collectFiles(tmpDir, false)
		if len(files) != 2 {
			t.Errorf("expected 2 files, got %d", len(files))
		}
	})

	t.Run("recursive collects all", func(t *testing.T) {
		files := s.collectFiles(tmpDir, true)
		if len(files) != 3 {
			t.Errorf("expected 3 files, got %d", len(files))
		}
	})

	t.Run("non-existent directory", func(t *testing.T) {
		files := s.collectFiles("/nonexistent/path", false)
		if len(files) != 0 {
			t.Errorf("expected 0 files, got %d", len(files))
		}
	})
}

func TestSearch(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "mySong.mp3"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "anotherSong.flac"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "myVideo.mp4"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("content"), 0644)

	subDir := filepath.Join(tmpDir, "music")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "classical.mp3"), []byte("content"), 0644)

	s := New()

	t.Run("empty folders returns nil", func(t *testing.T) {
		result, err := s.Search([]string{}, "test", false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("empty query returns nil", func(t *testing.T) {
		result, err := s.Search([]string{tmpDir}, "", false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("finds matching files", func(t *testing.T) {
		result, err := s.Search([]string{tmpDir}, "song", false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 results, got %d", len(result))
		}
	})

	t.Run("case insensitive search", func(t *testing.T) {
		result, err := s.Search([]string{tmpDir}, "MYSONG", false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("expected 1 result, got %d", len(result))
		}
	})

	t.Run("recursive search includes subdirs", func(t *testing.T) {
		result, err := s.Search([]string{tmpDir}, "classical", true)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("expected 1 result, got %d", len(result))
		}
	})

	t.Run("non-recursive excludes subdirs", func(t *testing.T) {
		result, err := s.Search([]string{tmpDir}, "classical", false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected 0 results, got %d", len(result))
		}
	})

	t.Run("search by video extension", func(t *testing.T) {
		result, err := s.Search([]string{tmpDir}, "video", false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("expected 1 result, got %d", len(result))
		}
		if result[0].Type != domain.TrackTypeFile {
			t.Errorf("expected TrackTypeFile, got %v", result[0].Type)
		}
	})
}

func TestListAll(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "song1.mp3"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "song2.flac"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "video.mp4"), []byte("content"), 0644)

	subDir := filepath.Join(tmpDir, "sub")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "nested.mp3"), []byte("content"), 0644)

	s := New()

	t.Run("empty folders returns nil", func(t *testing.T) {
		result, err := s.ListAll([]string{}, false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("lists all files non-recursive", func(t *testing.T) {
		result, err := s.ListAll([]string{tmpDir}, false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("expected 3 results, got %d", len(result))
		}
	})

	t.Run("lists all files recursive", func(t *testing.T) {
		result, err := s.ListAll([]string{tmpDir}, true)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 4 {
			t.Errorf("expected 4 results, got %d", len(result))
		}
	})
}

func TestNewReturnsFileService(t *testing.T) {
	svc := New()
	if svc == nil {
		t.Error("expected non-nil FileService")
	}
}
