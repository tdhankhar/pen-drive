package dto

type FileSystemEntry struct {
	Name         string `json:"name" example:"photo.jpg"`
	Path         string `json:"path" example:"photos/photo.jpg"`
	Type         string `json:"type" example:"file"`
	Size         int64  `json:"size,omitempty" example:"2048"`
	LastModified string `json:"last_modified,omitempty" example:"2026-03-12T16:00:00Z"`
}

type FileListResponse struct {
	Path                  string            `json:"path" example:"docs"`
	Entries               []FileSystemEntry `json:"entries"`
	NextContinuationToken string            `json:"next_continuation_token,omitempty" example:"token-123"`
	HasMore               bool              `json:"has_more" example:"false"`
}
