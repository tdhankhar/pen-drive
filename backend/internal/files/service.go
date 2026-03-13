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
	ErrPathNotFound       = errors.New("path not found")
)

const multipartPartSize int64 = 5 * 1024 * 1024
const trashNamespace = "trash"

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

type duplicateResolution struct {
	RequestedKey string
	FinalKey     string
	Conflict     bool
}

type storageClient interface {
	ListPath(ctx context.Context, input storage.ListPathInput) (storage.ListPathResult, error)
	PutObject(ctx context.Context, input storage.PutObjectInput) (storage.PutObjectResult, error)
	StartMultipartUpload(ctx context.Context, input storage.StartMultipartUploadInput) (storage.StartMultipartUploadResult, error)
	UploadPart(ctx context.Context, input storage.UploadPartInput) (storage.UploadPartResult, error)
	CompleteMultipartUpload(ctx context.Context, input storage.CompleteMultipartUploadInput) (storage.PutObjectResult, error)
	AbortMultipartUpload(ctx context.Context, input storage.AbortMultipartUploadInput) error
	ObjectExists(ctx context.Context, input storage.ObjectExistsInput) (bool, error)
	ListObjectKeys(ctx context.Context, input storage.ListObjectKeysInput) ([]string, error)
	CopyObject(ctx context.Context, input storage.CopyObjectInput) error
	DeleteObject(ctx context.Context, input storage.DeleteObjectInput) error
	GetPresignedURL(ctx context.Context, input storage.PresignedURLInput) (string, error)
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
		if isTrashPath(folder.Path) {
			continue
		}
		entries = append(entries, dto.FileSystemEntry{
			Name: folder.Name,
			Path: folder.Path,
			Type: dto.FileSystemEntryTypeFolder,
		})
	}

	for _, file := range result.Files {
		if isTrashPath(file.Path) {
			continue
		}
		entry := dto.FileSystemEntry{
			Name: file.Name,
			Path: file.Path,
			Type: dto.FileSystemEntryTypeFile,
			Size: file.Size,
		}
		if !file.LastModified.IsZero() {
			entry.LastModified = file.LastModified.UTC().Format(time.RFC3339)
		}
		// Generate presigned URL for file download
		presignedURL, err := s.storage.GetPresignedURL(ctx, storage.PresignedURLInput{
			Bucket:     userID,
			Key:        file.Path,
			Expiration: 1 * time.Hour,
		})
		if err == nil {
			entry.PresignedURL = presignedURL
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

func (s *Service) Delete(ctx context.Context, userID, targetPath, entryType string) ([]string, error) {
	validatedPath, err := validateRelativePath(targetPath)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid path: %v", ErrInvalidUploadInput, err)
	}

	switch strings.TrimSpace(entryType) {
	case dto.FileSystemEntryTypeFile:
		exists, err := s.objectExists(ctx, userID, validatedPath)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrPathNotFound
		}
		if err := s.moveObjectToTrash(ctx, userID, validatedPath); err != nil {
			return nil, err
		}
		return []string{validatedPath}, nil
	case dto.FileSystemEntryTypeFolder:
		keys, err := s.storage.ListObjectKeys(ctx, storage.ListObjectKeysInput{
			Bucket: userID,
			Prefix: toPrefix(validatedPath),
		})
		if err != nil {
			return nil, err
		}
		keys = filterTrashKeys(keys)
		if len(keys) == 0 {
			return nil, ErrPathNotFound
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := s.moveObjectToTrash(ctx, userID, key); err != nil {
				return nil, err
			}
		}
		return keys, nil
	default:
		return nil, fmt.Errorf("%w: unsupported entry type %q", ErrInvalidUploadInput, entryType)
	}
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

// Upload handles single-file uploads with path validation, duplicate resolution, and filename sanitization.
func (s *Service) Upload(
	ctx context.Context,
	userID,
	destinationPath,
	filename,
	contentType string,
	body io.Reader,
	size int64,
	conflictPolicy dto.DuplicateConflictPolicy,
) (UploadResult, error) {
	normalizedPolicy, err := parseConflictPolicy(string(conflictPolicy))
	if err != nil {
		return UploadResult{}, err
	}

	requestedKey, originalName, _, err := buildSingleUploadRequest(destinationPath, filename)
	if err != nil {
		return UploadResult{}, err
	}

	resolution, err := s.resolveUploadKey(ctx, userID, requestedKey, normalizedPolicy, nil)
	if err != nil {
		if errors.Is(err, storage.ErrObjectAlreadyExists) {
			return UploadResult{}, fmt.Errorf("object already exists: %s", requestedKey)
		}
		return UploadResult{}, err
	}

	if resolution.Conflict && normalizedPolicy == dto.DuplicateConflictPolicyReplace {
		if err := s.moveObjectToTrash(ctx, userID, requestedKey); err != nil {
			return UploadResult{}, err
		}
	}

	now := time.Now().UTC()
	uploadedAt := now.Format(time.RFC3339)
	storedFilename := path.Base(resolution.FinalKey)
	metadata := map[string]string{
		"original-filename":   originalName,
		"stored-filename":     storedFilename,
		"uploaded-by-user-id": userID,
		"uploaded-at":         uploadedAt,
	}

	_, err = s.storage.PutObject(ctx, storage.PutObjectInput{
		Bucket:      userID,
		Key:         resolution.FinalKey,
		Body:        body,
		Size:        size,
		ContentType: contentType,
		Metadata:    metadata,
	})
	if err != nil {
		if errors.Is(err, storage.ErrObjectAlreadyExists) {
			return UploadResult{}, fmt.Errorf("object already exists: %s", resolution.FinalKey)
		}
		return UploadResult{}, err
	}

	return UploadResult{
		Name:       storedFilename,
		Path:       resolution.FinalKey,
		Size:       size,
		UploadedAt: uploadedAt,
	}, nil
}

func (s *Service) PreviewDuplicates(
	ctx context.Context,
	userID string,
	destinationPath string,
	filename string,
	relativePaths []string,
) (dto.DuplicatePreviewResponse, error) {
	if strings.TrimSpace(filename) == "" && len(relativePaths) == 0 {
		return dto.DuplicatePreviewResponse{}, fmt.Errorf("%w: filename or relative_paths is required", ErrInvalidUploadInput)
	}

	reserved := make(map[string]bool)
	items := make([]dto.DuplicatePreviewItem, 0, max(1, len(relativePaths)))

	if strings.TrimSpace(filename) != "" {
		requestedKey, _, _, err := buildSingleUploadRequest(destinationPath, filename)
		if err != nil {
			return dto.DuplicatePreviewResponse{}, fmt.Errorf("%w: %v", ErrInvalidUploadInput, err)
		}

		item, err := s.buildDuplicatePreviewItem(ctx, userID, requestedKey, reserved)
		if err != nil {
			return dto.DuplicatePreviewResponse{}, err
		}
		items = append(items, item)
	} else {
		requestedKeys, err := buildFolderRequestedKeys(destinationPath, relativePaths)
		if err != nil {
			return dto.DuplicatePreviewResponse{}, err
		}

		for _, requestedKey := range requestedKeys {
			item, err := s.buildDuplicatePreviewItem(ctx, userID, requestedKey, reserved)
			if err != nil {
				return dto.DuplicatePreviewResponse{}, err
			}
			items = append(items, item)
		}
	}

	impactedPaths := make([]string, 0)
	hasConflicts := false
	for _, item := range items {
		if !item.Conflict {
			continue
		}
		hasConflicts = true
		impactedPaths = append(impactedPaths, item.ExistingPath)
	}

	return dto.DuplicatePreviewResponse{
		HasConflicts:  hasConflicts,
		ImpactedPaths: impactedPaths,
		Items:         items,
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
	if isTrashPath(cleaned) {
		return "", errors.New("path cannot target the internal trash namespace")
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
	if isTrashPath(result) {
		return "", errors.New("path cannot target the internal trash namespace")
	}
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
	conflictPolicy dto.DuplicateConflictPolicy,
) (MultipartUploadSession, error) {
	normalizedPolicy, err := parseConflictPolicy(string(conflictPolicy))
	if err != nil {
		return MultipartUploadSession{}, err
	}

	requestedKey, originalName, _, err := buildSingleUploadRequest(destinationPath, filename)
	if err != nil {
		return MultipartUploadSession{}, fmt.Errorf("%w: %v", ErrInvalidUploadInput, err)
	}

	resolution, err := s.resolveUploadKey(ctx, userID, requestedKey, normalizedPolicy, nil)
	if err != nil {
		if errors.Is(err, storage.ErrObjectAlreadyExists) {
			return MultipartUploadSession{}, fmt.Errorf("%w: %s", storage.ErrObjectAlreadyExists, requestedKey)
		}
		return MultipartUploadSession{}, err
	}

	if resolution.Conflict && normalizedPolicy == dto.DuplicateConflictPolicyReplace {
		if err := s.moveObjectToTrash(ctx, userID, requestedKey); err != nil {
			return MultipartUploadSession{}, err
		}
	}

	uploadedAt := time.Now().UTC().Format(time.RFC3339)
	result, err := s.storage.StartMultipartUpload(ctx, storage.StartMultipartUploadInput{
		Bucket:      userID,
		Key:         resolution.FinalKey,
		ContentType: contentType,
		Metadata: map[string]string{
			"original-filename":   originalName,
			"stored-filename":     path.Base(resolution.FinalKey),
			"uploaded-by-user-id": userID,
			"uploaded-at":         uploadedAt,
		},
	})
	if err != nil {
		if errors.Is(err, storage.ErrObjectAlreadyExists) {
			return MultipartUploadSession{}, fmt.Errorf("%w: %s", storage.ErrObjectAlreadyExists, resolution.FinalKey)
		}
		return MultipartUploadSession{}, err
	}

	return MultipartUploadSession{
		UploadID: result.UploadID,
		Key:      result.Key,
		Name:     path.Base(resolution.FinalKey),
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
	conflictPolicy dto.DuplicateConflictPolicy,
) ([]UploadResult, error) {
	normalizedPolicy, err := parseConflictPolicy(string(conflictPolicy))
	if err != nil {
		return nil, err
	}

	if len(multipartFiles) == 0 {
		return nil, fmt.Errorf("%w: files field is required", ErrInvalidUploadInput)
	}
	if len(relativePaths) == 0 {
		return nil, fmt.Errorf("%w: relative_paths field is required", ErrInvalidUploadInput)
	}
	if len(multipartFiles) != len(relativePaths) {
		return nil, fmt.Errorf("%w: files and relative_paths counts must match", ErrInvalidUploadInput)
	}

	requestedKeys, err := buildFolderRequestedKeys(destinationPath, relativePaths)
	if err != nil {
		return nil, err
	}

	uploadItems := make([]uploadItemInternal, 0, len(multipartFiles))
	now := time.Now().UTC()
	uploadedAt := now.Format(time.RFC3339)
	reservedKeys := make(map[string]bool)

	for i, mfh := range multipartFiles {
		if mfh.Size == 0 {
			return nil, fmt.Errorf("%w: file at index %d", ErrZeroByteFile, i)
		}

		validatedRelPath, err := validateRelativePath(relativePaths[i])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid relative path at index %d: %v", ErrInvalidUploadInput, i, err)
		}

		segments := strings.Split(validatedRelPath, "/")
		originalName := segments[len(segments)-1]
		requestedKey := requestedKeys[i]
		resolution, err := s.resolveUploadKey(ctx, userID, requestedKey, normalizedPolicy, reservedKeys)
		if err != nil {
			if errors.Is(err, storage.ErrObjectAlreadyExists) {
				return nil, fmt.Errorf("%w: %s", storage.ErrObjectAlreadyExists, requestedKey)
			}
			return nil, err
		}

		item := uploadItemInternal{
			RequestedKey:    requestedKey,
			FinalKey:        resolution.FinalKey,
			Filename:        path.Base(resolution.FinalKey),
			OriginalName:    originalName,
			ContentType:     mfh.Header.Get("Content-Type"),
			Size:            mfh.Size,
			MultipartHeader: mfh,
			UploadedAt:      uploadedAt,
			Conflict:        resolution.Conflict,
		}
		uploadItems = append(uploadItems, item)
	}

	sort.Slice(uploadItems, func(i, j int) bool {
		return uploadItems[i].FinalKey < uploadItems[j].FinalKey
	})

	results := make([]UploadResult, 0, len(uploadItems))
	for _, item := range uploadItems {
		if item.Conflict && normalizedPolicy == dto.DuplicateConflictPolicyReplace {
			if err := s.moveObjectToTrash(ctx, userID, item.RequestedKey); err != nil {
				return nil, err
			}
		}

		src, err := item.MultipartHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", item.Filename, err)
		}
		defer src.Close()

		metadata := map[string]string{
			"original-filename":   item.OriginalName,
			"stored-filename":     item.Filename,
			"uploaded-by-user-id": userID,
			"uploaded-at":         item.UploadedAt,
		}

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
	RequestedKey    string
	FinalKey        string
	Filename        string
	OriginalName    string
	ContentType     string
	Size            int64
	MultipartHeader *multipart.FileHeader
	UploadedAt      string
	Conflict        bool
}

func buildSingleUploadRequest(destinationPath, filename string) (string, string, string, error) {
	normalizedPath, err := validateDestinationPath(destinationPath)
	if err != nil {
		return "", "", "", err
	}

	sanitizedFilename, err := SanitizeFilename(filename)
	if err != nil {
		return "", "", "", err
	}

	requestedKey := buildFinalObjectKey(normalizedPath, sanitizedFilename.Stored)
	return requestedKey, sanitizedFilename.Original, sanitizedFilename.Stored, nil
}

func buildFolderRequestedKeys(destinationPath string, relativePaths []string) ([]string, error) {
	normalizedDestPath, err := validateDestinationPath(destinationPath)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid destination path: %v", ErrInvalidUploadInput, err)
	}

	requestedKeys := make([]string, 0, len(relativePaths))
	seenKeys := make(map[string]bool, len(relativePaths))

	for i, relativePath := range relativePaths {
		validatedRelPath, err := validateRelativePath(relativePath)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid relative path at index %d: %v", ErrInvalidUploadInput, i, err)
		}

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

		requestedKey := buildFinalObjectKey(normalizedDestPath, sanitizedRelPath)
		if seenKeys[requestedKey] {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateBatchKey, requestedKey)
		}
		seenKeys[requestedKey] = true
		requestedKeys = append(requestedKeys, requestedKey)
	}

	return requestedKeys, nil
}

func parseConflictPolicy(rawPolicy string) (dto.DuplicateConflictPolicy, error) {
	policy := dto.DuplicateConflictPolicy(strings.TrimSpace(rawPolicy))
	if policy == "" {
		return dto.DuplicateConflictPolicyReject, nil
	}

	switch policy {
	case dto.DuplicateConflictPolicyReject, dto.DuplicateConflictPolicyRename, dto.DuplicateConflictPolicyReplace:
		return policy, nil
	default:
		return "", fmt.Errorf("%w: unsupported conflict policy %q", ErrInvalidUploadInput, rawPolicy)
	}
}

func (s *Service) buildDuplicatePreviewItem(
	ctx context.Context,
	userID string,
	requestedKey string,
	reservedKeys map[string]bool,
) (dto.DuplicatePreviewItem, error) {
	exists, err := s.objectExists(ctx, userID, requestedKey)
	if err != nil {
		return dto.DuplicatePreviewItem{}, err
	}

	item := dto.DuplicatePreviewItem{
		RequestedPath: requestedKey,
		Conflict:      exists,
	}
	if !exists {
		reservedKeys[requestedKey] = true
		return item, nil
	}

	item.ExistingPath = requestedKey
	renamePath, err := s.findNextAvailableKey(ctx, userID, requestedKey, reservedKeys)
	if err != nil {
		return dto.DuplicatePreviewItem{}, err
	}
	item.RenamePath = renamePath
	return item, nil
}

func (s *Service) resolveUploadKey(
	ctx context.Context,
	userID string,
	requestedKey string,
	conflictPolicy dto.DuplicateConflictPolicy,
	reservedKeys map[string]bool,
) (duplicateResolution, error) {
	if reservedKeys == nil {
		reservedKeys = make(map[string]bool)
	}

	exists, err := s.objectExists(ctx, userID, requestedKey)
	if err != nil {
		return duplicateResolution{}, err
	}

	if !exists && !reservedKeys[requestedKey] {
		reservedKeys[requestedKey] = true
		return duplicateResolution{
			RequestedKey: requestedKey,
			FinalKey:     requestedKey,
			Conflict:     false,
		}, nil
	}

	switch conflictPolicy {
	case dto.DuplicateConflictPolicyReplace:
		reservedKeys[requestedKey] = true
		return duplicateResolution{
			RequestedKey: requestedKey,
			FinalKey:     requestedKey,
			Conflict:     exists,
		}, nil
	case dto.DuplicateConflictPolicyRename:
		finalKey, err := s.findNextAvailableKey(ctx, userID, requestedKey, reservedKeys)
		if err != nil {
			return duplicateResolution{}, err
		}
		return duplicateResolution{
			RequestedKey: requestedKey,
			FinalKey:     finalKey,
			Conflict:     true,
		}, nil
	case dto.DuplicateConflictPolicyReject:
		fallthrough
	default:
		return duplicateResolution{}, storage.ErrObjectAlreadyExists
	}
}

func (s *Service) findNextAvailableKey(
	ctx context.Context,
	userID string,
	requestedKey string,
	reservedKeys map[string]bool,
) (string, error) {
	dir := path.Dir(requestedKey)
	if dir == "." {
		dir = ""
	}
	filename := path.Base(requestedKey)
	ext := path.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	for suffix := 1; ; suffix++ {
		candidateName := fmt.Sprintf("%s_(%d)%s", base, suffix, ext)
		candidateKey := candidateName
		if dir != "" {
			candidateKey = dir + "/" + candidateName
		}

		if reservedKeys[candidateKey] {
			continue
		}

		exists, err := s.objectExists(ctx, userID, candidateKey)
		if err != nil {
			return "", err
		}
		if exists {
			continue
		}

		reservedKeys[candidateKey] = true
		return candidateKey, nil
	}
}

func (s *Service) objectExists(ctx context.Context, userID string, key string) (bool, error) {
	return s.storage.ObjectExists(ctx, storage.ObjectExistsInput{
		Bucket: userID,
		Key:    key,
	})
}

func (s *Service) moveObjectToTrash(ctx context.Context, userID string, key string) error {
	trashKey := path.Join("trash", key)
	if err := s.storage.CopyObject(ctx, storage.CopyObjectInput{
		Bucket:         userID,
		SourceKey:      key,
		DestinationKey: trashKey,
	}); err != nil {
		return err
	}

	if err := s.storage.DeleteObject(ctx, storage.DeleteObjectInput{
		Bucket: userID,
		Key:    key,
	}); err != nil {
		return err
	}

	return nil
}

func isTrashPath(key string) bool {
	normalized := normalizePath(key)
	return normalized == trashNamespace || strings.HasPrefix(normalized, trashNamespace+"/")
}

func filterTrashKeys(keys []string) []string {
	filtered := make([]string, 0, len(keys))
	for _, key := range keys {
		if isTrashPath(key) {
			continue
		}
		filtered = append(filtered, key)
	}
	return filtered
}
