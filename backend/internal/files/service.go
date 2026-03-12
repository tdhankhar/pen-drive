package files

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/abhishek/pen-drive/backend/internal/api/dto"
	"github.com/abhishek/pen-drive/backend/internal/storage"
)

type Service struct {
	storage *storage.Client
}

func NewService(storageClient *storage.Client) *Service {
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
