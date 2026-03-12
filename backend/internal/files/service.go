package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/abhishek/pen-drive/backend/internal/api/dto"
	"github.com/abhishek/pen-drive/backend/internal/storage"
)

type Service struct {
	storage storageClient
}

type UploadResult struct {
	Name       string
	Path       string
	Size       int64
	UploadedAt string
}

type storageClient interface {
	ListPath(ctx context.Context, input storage.ListPathInput) (storage.ListPathResult, error)
	PutObject(ctx context.Context, input storage.PutObjectInput) (storage.PutObjectResult, error)
}

func NewService(storageClient storageClient) *Service {
	return &Service{storage: storageClient}
}

func (s *Service) List(ctx context.Context, userID, rawPath, continuationToken string, limit int32) (dto.FileListResponse, error) {
	path := normalizePath(rawPath)
	prefix := toPrefix(path)

	result, err := s.storage.ListPath(ctx, storage.ListPathInput{
		Bucket:            userID,
		Prefix:            prefix,
		ContinuationToken: continuationToken,
		MaxKeys:           limit,
	})
	if err != nil {
		return dto.FileListResponse{}, err
	}

	entries := make([]dto.FileSystemEntry, 0, len(result.Folders)+len(result.Files))
	for _, folder := range result.Folders {
		entries = append(entries, dto.FileSystemEntry{
			Name: folder.Name,
			Path: folder.Path,
			Type: "folder",
		})
	}

	for _, file := range result.Files {
		entry := dto.FileSystemEntry{
			Name: file.Name,
			Path: file.Path,
			Type: "file",
			Size: file.Size,
		}
		if !file.LastModified.IsZero() {
			entry.LastModified = file.LastModified.UTC().Format(time.RFC3339)
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type == entries[j].Type {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Type == "folder"
	})

	return dto.FileListResponse{
		Path:                  path,
		Entries:               entries,
		NextContinuationToken: result.NextContinuationToken,
		HasMore:               result.HasMore,
	}, nil
}

func normalizePath(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "." {
		return ""
	}
	return trimmed
}

func toPrefix(path string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s/", path)
}

// Upload handles single-file uploads with path validation and minimal filename validation.
func (s *Service) Upload(ctx context.Context, userID, destinationPath, filename, contentType string, body io.Reader, size int64) (UploadResult, error) {
	normalizedPath, err := validateDestinationPath(destinationPath)
	if err != nil {
		return UploadResult{}, err
	}

	// Validate filename (minimal sanitization for issue #1)
	validatedFilename, err := validateFilename(filename)
	if err != nil {
		return UploadResult{}, err
	}

	// Construct S3 key: path/filename
	var s3Key string
	if normalizedPath == "" {
		s3Key = validatedFilename
	} else {
		s3Key = fmt.Sprintf("%s/%s", normalizedPath, validatedFilename)
	}

	// Prepare metadata
	now := time.Now().UTC()
	uploadedAt := now.Format(time.RFC3339)
	metadata := map[string]string{
		"original-filename":   filename,
		"stored-filename":     validatedFilename,
		"uploaded-by-user-id": userID,
		"uploaded-at":         uploadedAt,
	}

	// Upload to storage
	putInput := storage.PutObjectInput{
		Bucket:      userID,
		Key:         s3Key,
		Body:        body,
		Size:        size,
		ContentType: contentType,
		Metadata:    metadata,
	}

	_, err = s.storage.PutObject(ctx, putInput)
	if err != nil {
		if errors.Is(err, storage.ErrObjectAlreadyExists) {
			return UploadResult{}, fmt.Errorf("object already exists: %s", s3Key)
		}
		return UploadResult{}, err
	}

	return UploadResult{
		Name:       validatedFilename,
		Path:       s3Key,
		Size:       size,
		UploadedAt: uploadedAt,
	}, nil
}

// validateFilename applies minimal filename validation for issue #1
// Rules:
// 1. Use provided filename, trim whitespace
// 2. Reject path separators and traversal fragments (/, \, ..)
// 3. Reject empty filenames
func validateFilename(filename string) (string, error) {
	// Trim whitespace
	cleaned := strings.TrimSpace(filename)

	// Reject empty
	if cleaned == "" {
		return "", errors.New("filename cannot be empty")
	}

	// Reject path separators
	if strings.Contains(cleaned, "/") || strings.Contains(cleaned, "\\") {
		return "", errors.New("filename cannot contain path separators")
	}

	// Reject traversal attempts
	if strings.Contains(cleaned, "..") {
		return "", errors.New("filename cannot contain traversal sequences")
	}

	return cleaned, nil
}

func validateDestinationPath(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", nil
	}

	normalizedSeparators := strings.ReplaceAll(trimmed, "\\", "/")
	parts := strings.Split(normalizedSeparators, "/")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", errors.New("path cannot contain traversal sequences")
		default:
			normalized = append(normalized, part)
		}
	}

	cleaned := path.Clean(strings.Join(normalized, "/"))
	if cleaned == "." {
		return "", nil
	}

	return cleaned, nil
}
