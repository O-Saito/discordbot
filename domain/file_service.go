package domain

type FileServiceSearchCallback func([]Track, error)

type FileService interface {
	Search(folders []string, query string, recursive bool, callback FileServiceSearchCallback)
	ListAll(folders []string, recursive bool, callback FileServiceSearchCallback)
}

type FileInfo struct {
	Path    string
	Title   string
	IsAudio bool
	IsVideo bool
}
