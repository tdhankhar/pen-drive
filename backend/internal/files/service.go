package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/abhishek/pen-drive/backend/internal/api/dto"
	"github.com/abhishek/pen-drive/backend/internal/storage"
)

var (
	ErrInvalidUploadInput = errors.New("invalid upload input")
	ErrZeroByteFile       = errors.New("zero byte file")
	ErrDuplicateBatchKey  = errors.New("duplicate target key in batch")
)

const multipartPartSize int64 = 5 * 1024 * 1024

type Service struct {
	storage storageClient
}

type UploadResult struct {
	Name       string
	Path       string
	Size       int64
	UploadedAt string
}

type MultipartUploadSession struct {
	UploadID string
	Key      string
	Name     string
	PartSize int64
}

type storageClient interface {
	ListPath(ctx context.Context, input storage.ListPathInput) (storage.ListPathResult, error)
	PutObject(ctx context.Context, input storage.PutObjectInput) (storage.PutObjectResult, error)
	StartMultipartUpload(ctx context.Context, input storage.StartMultipartUploadInput) (storage.StartMultipartUploadResult, error)
	UploadPart(ctx context.Context, input storage.UploadPartInput) (storage.UploadPartResult, error)
	CompleteMultipartUpload(ctx context.Context, input storage.CompleteMultipartUploadInput) (storage.PutObjectResult, error)
	AbortMultipartUpload(ctx context.Context, input storage.AbortMultipartUploadInput) error
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

// Upload handles single-file uploads with path validation and filename sanitization.
func (s *Service) Upload(ctx context.Context, userID, destinationPath, filename, contentType string, body io.Reader, size int64) (UploadResult, error) {
	normalizedPath, err := validateDestinationPath(destinationPath)
	if err != nil {
		return UploadResult{}, err
	}

	sanitizedFilename, err := SanitizeFilename(filename)
	if err != nil {
		return UploadResult{}, err
	}

	// Construct S3 key: path/filename
	var s3Key string
	if normalizedPath == "" {
		s3Key = sanitizedFilename.Stored
	} else {
		s3Key = fmt.Sprintf("%s/%s", normalizedPath, sanitizedFilename.Stored)
	}

	// Prepare metadata
	now := time.Now().UTC()
	uploadedAt := now.Format(time.RFC3339)
	metadata := map[string]string{
		"original-filename":   sanitizedFilename.Original,
		"stored-filename":     sanitizedFilename.Stored,
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
		Name:       sanitizedFilename.Stored,
		Path:       s3Key,
		Size:       size,
		UploadedAt: uploadedAt,
	}, nil
}

func validateFilename(filename string) (string, error) {
	sanitized, err := SanitizeFilename(filename)
	if err != nil {
		return "", err
	}
	return sanitized.Stored, nil
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

// validateRelativePath validates a relative path within a folder upload
// Rules:
// 1. Normalize backslashes to forward slashes
// 2. Reject path traversal attempts (..)
// 3. Reject empty or absolute paths
// 4. Must have at least one non-empty segment after normalization
func validateRelativePath(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", errors.New("relative path cannot be empty")
	}

	// Reject absolute paths
	if strings.HasPrefix(trimmed, "/") {
		return "", errors.New("relative path cannot be absolute")
	}

	// Normalize separators
	normalized := strings.ReplaceAll(trimmed, "\\", "/")

	// Split and validate segments
	parts := strings.Split(normalized, "/")
	validParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			// Skip empty segments (from double slashes)
			continue
		}
		if part == "." {
			// Skip current directory references
			continue
		}
		if part == ".." {
			// Reject traversal
			return "", errors.New("relative path cannot contain traversal sequences")
		}
		validParts = append(validParts, part)
	}

	// Must have at least one segment after normalization
	if len(validParts) == 0 {
		return "", errors.New("relative path must have at least one file segment")
	}

	result := strings.Join(validParts, "/")
	return result, nil
}

// buildFinalObjectKey constructs the final S3 key from destination and relative path
func buildFinalObjectKey(destinationPath, relativePath string) string {
	if destinationPath == "" {
		return relativePath
	}
	return fmt.Sprintf("%s/%s", destinationPath, relativePath)
}

func (s *Service) InitiateMultipartUpload(
	ctx context.Context,
	userID string,
	destinationPath string,
	filename string,
	contentType string,
) (MultipartUploadSession, error) {
	normalizedPath, err := validateDestinationPath(destinationPath)
	if err != nil {
		return MultipartUploadSession{}, fmt.Errorf("%w: invalid destination path: %v", ErrInvalidUploadInput, err)
	}

	sanitizedFilename, err := SanitizeFilename(filename)
	if err != nil {
		return MultipartUploadSession{}, fmt.Errorf("%w: invalid filename: %v", ErrInvalidUploadInput, err)
	}

	finalKey := sanitizedFilename.Stored
	if normalizedPath != "" {
		finalKey = fmt.Sprintf("%s/%s", normalizedPath, sanitizedFilename.Stored)
	}

	uploadedAt := time.Now().UTC().Format(time.RFC3339)
	result, err := s.storage.StartMultipartUpload(ctx, storage.StartMultipartUploadInput{
		Bucket:      userID,
		Key:         finalKey,
		ContentType: contentType,
		Metadata: map[string]string{
			"original-filename":   sanitizedFilename.Original,
			"stored-filename":     sanitizedFilename.Stored,
			"uploaded-by-user-id": userID,
			"uploaded-at":         uploadedAt,
		},
	})
	if err != nil {
		if errors.Is(err, storage.ErrObjectAlreadyExists) {
			return MultipartUploadSession{}, fmt.Errorf("%w: %s", storage.ErrObjectAlreadyExists, finalKey)
		}
		return MultipartUploadSession{}, err
	}

	return MultipartUploadSession{
		UploadID: result.UploadID,
		Key:      result.Key,
		Name:     sanitizedFilename.Stored,
		PartSize: multipartPartSize,
	}, nil
}

func (s *Service) UploadMultipartPart(
	ctx context.Context,
	userID string,
	key string,
	uploadID string,
	partNumber int32,
	body io.Reader,
	size int64,
) (storage.UploadPartResult, error) {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(uploadID) == "" {
		return storage.UploadPartResult{}, fmt.Errorf("%w: key and upload_id are required", ErrInvalidUploadInput)
	}
	if partNumber <= 0 {
		return storage.UploadPartResult{}, fmt.Errorf("%w: part_number must be greater than zero", ErrInvalidUploadInput)
	}
	if size <= 0 {
		return storage.UploadPartResult{}, fmt.Errorf("%w: multipart part cannot be empty", ErrInvalidUploadInput)
	}

	return s.storage.UploadPart(ctx, storage.UploadPartInput{
		Bucket:     userID,
		Key:        key,
		UploadID:   uploadID,
		PartNumber: partNumber,
		Body:       body,
		Size:       size,
	})
}

func (s *Service) CompleteMultipartUpload(
	ctx context.Context,
	userID string,
	key string,
	uploadID string,
	parts []storage.CompletedPart,
) (UploadResult, error) {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(uploadID) == "" {
		return UploadResult{}, fmt.Errorf("%w: key and upload_id are required", ErrInvalidUploadInput)
	}
	if len(parts) == 0 {
		return UploadResult{}, fmt.Errorf("%w: at least one uploaded part is required", ErrInvalidUploadInput)
	}

	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	_, err := s.storage.CompleteMultipartUpload(ctx, storage.CompleteMultipartUploadInput{
		Bucket:   userID,
		Key:      key,
		UploadID: uploadID,
		Parts:    parts,
	})
	if err != nil {
		return UploadResult{}, err
	}

	uploadedAt := time.Now().UTC().Format(time.RFC3339)
	return UploadResult{
		Name:       path.Base(key),
		Path:       key,
		UploadedAt: uploadedAt,
	}, nil
}

func (s *Service) AbortMultipartUpload(ctx context.Context, userID string, key string, uploadID string) error {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(uploadID) == "" {
		return fmt.Errorf("%w: key and upload_id are required", ErrInvalidUploadInput)
	}

	return s.storage.AbortMultipartUpload(ctx, storage.AbortMultipartUploadInput{
		Bucket:   userID,
		Key:      key,
		UploadID: uploadID,
	})
}

// UploadFolder handles batch folder uploads with relative path preservation
func (s *Service) UploadFolder(
	ctx context.Context,
	userID string,
	destinationPath string,
	multipartFiles []*multipart.FileHeader,
	relativePaths []string,
) ([]UploadResult, error) {
	if len(multipartFiles) == 0 {
		return nil, fmt.Errorf("%w: files field is required", ErrInvalidUploadInput)
	}
	if len(relativePaths) == 0 {
		return nil, fmt.Errorf("%w: relative_paths field is required", ErrInvalidUploadInput)
	}
	if len(multipartFiles) != len(relativePaths) {
		return nil, fmt.Errorf("%w: files and relative_paths counts must match", ErrInvalidUploadInput)
	}

	// Validate destination path once
	normalizedDestPath, err := validateDestinationPath(destinationPath)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid destination path: %v", ErrInvalidUploadInput, err)
	}

	// Phase 1: Validate all inputs and build upload items
	uploadItems := make([]uploadItemInternal, 0, len(multipartFiles))
	seenKeys := make(map[string]bool)
	now := time.Now().UTC()
	uploadedAt := now.Format(time.RFC3339)

	for i, mfh := range multipartFiles {
		// Check zero-byte file
		if mfh.Size == 0 {
			return nil, fmt.Errorf("%w: file at index %d", ErrZeroByteFile, i)
		}

		// Validate relative path
		validatedRelPath, err := validateRelativePath(relativePaths[i])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid relative path at index %d: %v", ErrInvalidUploadInput, i, err)
		}

		// Extract basename (filename) from validated relative path
		segments := strings.Split(validatedRelPath, "/")
		basename := segments[len(segments)-1]
		parentPath := strings.Join(segments[:len(segments)-1], "/")

		sanitizedFilename, err := SanitizeFilename(basename)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid filename at index %d: %v", ErrInvalidUploadInput, i, err)
		}

		sanitizedRelPath := sanitizedFilename.Stored
		if parentPath != "" {
			sanitizedRelPath = parentPath + "/" + sanitizedFilename.Stored
		}

		// Build final object key
		finalKey := buildFinalObjectKey(normalizedDestPath, sanitizedRelPath)

		// Check for duplicates within this batch
		if seenKeys[finalKey] {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateBatchKey, finalKey)
		}
		seenKeys[finalKey] = true

		// Create upload item
		item := uploadItemInternal{
			FinalKey:        finalKey,
			Filename:        sanitizedFilename.Stored,
			OriginalName:    sanitizedFilename.Original,
			ContentType:     mfh.Header.Get("Content-Type"),
			Size:            mfh.Size,
			MultipartHeader: mfh,
			UploadedAt:      uploadedAt,
		}
		uploadItems = append(uploadItems, item)
	}

	// Phase 2: Check for duplicate existing keys in storage
	// Note: This is a simplified check. In production, you might batch-check or rely on PutObject's ErrObjectAlreadyExists
	// For now, we'll let the PutObject call handle it

	// Phase 3: Upload files in deterministic order (sorted by final key)
	sort.Slice(uploadItems, func(i, j int) bool {
		return uploadItems[i].FinalKey < uploadItems[j].FinalKey
	})

	results := make([]UploadResult, 0, len(uploadItems))
	for _, item := range uploadItems {
		// Open file
		src, err := item.MultipartHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", item.Filename, err)
		}
		defer src.Close()

		// Prepare metadata
		metadata := map[string]string{
			"original-filename":   item.OriginalName,
			"stored-filename":     item.Filename,
			"uploaded-by-user-id": userID,
			"uploaded-at":         item.UploadedAt,
		}

		// Upload to storage
		putInput := storage.PutObjectInput{
			Bucket:      userID,
			Key:         item.FinalKey,
			Body:        src,
			Size:        item.Size,
			ContentType: item.ContentType,
			Metadata:    metadata,
		}

		_, err = s.storage.PutObject(ctx, putInput)
		if err != nil {
			if errors.Is(err, storage.ErrObjectAlreadyExists) {
				return nil, fmt.Errorf("%w: %s", storage.ErrObjectAlreadyExists, item.FinalKey)
			}
			return nil, fmt.Errorf("failed to upload %s: %w", item.Filename, err)
		}

		results = append(results, UploadResult{
			Name:       item.Filename,
			Path:       item.FinalKey,
			Size:       item.Size,
			UploadedAt: item.UploadedAt,
		})
	}

	return results, nil
}

// uploadItemInternal represents an internal upload item with all metadata
type uploadItemInternal struct {
	FinalKey        string
	Filename        string
	OriginalName    string
	ContentType     string
	Size            int64
	MultipartHeader *multipart.FileHeader
	UploadedAt      string
}
