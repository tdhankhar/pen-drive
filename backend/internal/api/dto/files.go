package dto

type FileSystemEntry struct {
	Name         string `json:"name" example:"photo.jpg"`
	Path         string `json:"path" example:"photos/photo.jpg"`
	Type         string `json:"type" example:"file"`
	Size         int64  `json:"size,omitempty" example:"2048"`
	LastModified string `json:"last_modified,omitempty" example:"2026-03-12T16:00:00Z"`
	PresignedURL string `json:"presigned_url,omitempty" example:"https://s3.example.com/..."`
}

const (
	FileSystemEntryTypeFile   = "file"
	FileSystemEntryTypeFolder = "folder"
)

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

type DuplicateConflictPolicy string

const (
	DuplicateConflictPolicyReject  DuplicateConflictPolicy = "reject"
	DuplicateConflictPolicyRename  DuplicateConflictPolicy = "rename"
	DuplicateConflictPolicyReplace DuplicateConflictPolicy = "replace"
)

type DuplicatePreviewRequest struct {
	Path          string   `json:"path,omitempty" example:"uploads/docs"`
	Filename      string   `json:"filename,omitempty" example:"report.pdf"`
	RelativePaths []string `json:"relative_paths,omitempty" example:"reports/q1/report.pdf"`
}

type DuplicatePreviewItem struct {
	RequestedPath string `json:"requested_path" example:"uploads/docs/report.pdf"`
	Conflict      bool   `json:"conflict" example:"true"`
	ExistingPath  string `json:"existing_path,omitempty" example:"uploads/docs/report.pdf"`
	RenamePath    string `json:"rename_path,omitempty" example:"uploads/docs/report_(1).pdf"`
}

type DuplicatePreviewResponse struct {
	HasConflicts  bool                   `json:"has_conflicts" example:"true"`
	ImpactedPaths []string               `json:"impacted_paths,omitempty" example:"uploads/docs/report.pdf"`
	Items         []DuplicatePreviewItem `json:"items"`
}

type DeleteResponse struct {
	DeletedPaths []string `json:"deleted_paths" example:"docs/report.pdf"`
}

// FileUploadResponse is the response for a successful file upload
type FileUploadResponse struct {
	File UploadedFileInfo `json:"file"`
}

// FolderUploadRequest represents a folder upload request with multiple files
// The actual files are in multipart form, but this DTO documents the structure
type FolderUploadRequest struct {
	// Files are the binary file contents (multipart form fields, repeated)
	Files [][]byte
	// RelativePaths are the relative paths for each file in the folder structure (repeated)
	RelativePaths []string
	// Path is the destination folder path within the user's bucket (optional)
	Path string
}

// FolderUploadResponse is the response for a successful folder upload
type FolderUploadResponse struct {
	Files []UploadedFileInfo `json:"files"`
}

type MultipartUploadInitiateRequest struct {
	Filename       string                  `json:"filename" example:"video.mp4"`
	Path           string                  `json:"path,omitempty" example:"uploads/videos"`
	ContentType    string                  `json:"content_type,omitempty" example:"video/mp4"`
	Size           int64                   `json:"size" example:"7340032"`
	ConflictPolicy DuplicateConflictPolicy `json:"conflict_policy,omitempty" example:"reject"`
}

type MultipartUploadInitiateResponse struct {
	UploadID string `json:"upload_id" example:"upload-id"`
	Key      string `json:"key" example:"uploads/videos/video.mp4"`
	Name     string `json:"name" example:"video.mp4"`
	PartSize int64  `json:"part_size" example:"5242880"`
}

type MultipartUploadPartResponse struct {
	PartNumber int32  `json:"part_number" example:"1"`
	ETag       string `json:"etag" example:"\"etag-value\""`
}

type MultipartCompletedPart struct {
	PartNumber int32  `json:"part_number" example:"1"`
	ETag       string `json:"etag" example:"\"etag-value\""`
}

type MultipartUploadCompleteRequest struct {
	UploadID string                   `json:"upload_id" example:"upload-id"`
	Key      string                   `json:"key" example:"uploads/videos/video.mp4"`
	Parts    []MultipartCompletedPart `json:"parts"`
}

type MultipartUploadAbortRequest struct {
	UploadID string `json:"upload_id" example:"upload-id"`
	Key      string `json:"key" example:"uploads/videos/video.mp4"`
}
