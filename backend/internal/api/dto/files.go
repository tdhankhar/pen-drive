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

// FileUploadRequest represents a single-file upload request
// The actual file is in the multipart form, but this DTO documents the response
type FileUploadRequest struct {
	// File is the binary file content (multipart form field)
	File []byte
	// Path is the destination path within the user's bucket (optional)
	Path string
	// Filename is an optional override for the uploaded filename
	Filename string
}

// UploadedFileInfo contains metadata about an uploaded file
type UploadedFileInfo struct {
	Name       string `json:"name" example:"document.pdf"`
	Path       string `json:"path" example:"uploads/document.pdf"`
	Size       int64  `json:"size" example:"1024576"`
	UploadedAt string `json:"uploaded_at" example:"2026-03-12T16:00:00Z"`
}

// FileUploadResponse is the response for a successful file upload
type FileUploadResponse struct {
	File UploadedFileInfo `json:"file"`
}
