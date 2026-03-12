package files

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/abhishek/pen-drive/backend/internal/storage"
)

type stubStorageClient struct {
	putInput storage.PutObjectInput
	putErr   error
}

func (s *stubStorageClient) ListPath(context.Context, storage.ListPathInput) (storage.ListPathResult, error) {
	return storage.ListPathResult{}, nil
}

func (s *stubStorageClient) PutObject(_ context.Context, input storage.PutObjectInput) (storage.PutObjectResult, error) {
	s.putInput = input
	if s.putErr != nil {
		return storage.PutObjectResult{}, s.putErr
	}

	return storage.PutObjectResult{Key: input.Key}, nil
}

func TestValidateDestinationPath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty path", input: "", want: ""},
		{name: "trim and normalize", input: " /docs//reports/ ", want: "docs/reports"},
		{name: "dot segments are removed", input: "./docs/./reports", want: "docs/reports"},
		{name: "traversal rejected", input: "../docs", wantErr: true},
		{name: "nested traversal rejected", input: "docs/../../secrets", wantErr: true},
		{name: "backslash traversal rejected", input: `..\docs`, wantErr: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := validateDestinationPath(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestUploadPassesContentTypeAndMetadata(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{}
	service := NewService(storageClient)

	result, err := service.Upload(
		context.Background(),
		"user-123",
		"docs//reports",
		" report.pdf ",
		"application/pdf",
		bytes.NewBufferString("payload"),
		int64(len("payload")),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Path != "docs/reports/report.pdf" {
		t.Fatalf("expected normalized path, got %q", result.Path)
	}

	if storageClient.putInput.ContentType != "application/pdf" {
		t.Fatalf("expected content type to be preserved, got %q", storageClient.putInput.ContentType)
	}

	if storageClient.putInput.Metadata["original-filename"] != " report.pdf " {
		t.Fatalf("expected original filename metadata to be preserved")
	}

	if storageClient.putInput.Metadata["stored-filename"] != "report.pdf" {
		t.Fatalf("expected stored filename metadata to be trimmed")
	}

	if storageClient.putInput.Metadata["uploaded-by-user-id"] != "user-123" {
		t.Fatalf("expected uploader metadata to be set")
	}

	if storageClient.putInput.Metadata["uploaded-at"] == "" {
		t.Fatalf("expected uploaded-at metadata to be set")
	}
}

func TestUploadRejectsDuplicate(t *testing.T) {
	t.Parallel()

	service := NewService(&stubStorageClient{putErr: storage.ErrObjectAlreadyExists})

	_, err := service.Upload(
		context.Background(),
		"user-123",
		"docs",
		"report.pdf",
		"application/pdf",
		bytes.NewBufferString("payload"),
		int64(len("payload")),
	)
	if err == nil {
		t.Fatalf("expected duplicate error, got nil")
	}

	if got := err.Error(); got != "object already exists: docs/report.pdf" {
		t.Fatalf("unexpected error message: %q", got)
	}
}

func TestUploadRejectsInvalidDestinationPath(t *testing.T) {
	t.Parallel()

	service := NewService(&stubStorageClient{})

	_, err := service.Upload(
		context.Background(),
		"user-123",
		"../secrets",
		"report.pdf",
		"application/pdf",
		bytes.NewBufferString("payload"),
		int64(len("payload")),
	)
	if err == nil {
		t.Fatalf("expected invalid path error, got nil")
	}
}

func TestUploadPropagatesStorageErrors(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("storage failed")
	service := NewService(&stubStorageClient{putErr: expectedErr})

	_, err := service.Upload(
		context.Background(),
		"user-123",
		"docs",
		"report.pdf",
		"application/pdf",
		bytes.NewBufferString("payload"),
		int64(len("payload")),
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected storage error to propagate, got %v", err)
	}
}
