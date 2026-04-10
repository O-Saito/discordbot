package domain

type FileService interface {
	Search(folders []string, query string, recursive bool) ([]Track, error)
	ListAll(folders []string, recursive bool) ([]Track, error)
}

type FileInfo struct {
	Path    string
	Title   string
	IsAudio bool
	IsVideo bool
}
