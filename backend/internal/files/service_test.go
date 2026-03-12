package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/textproto"
	"slices"
	"testing"

	"github.com/abhishek/pen-drive/backend/internal/api/dto"
	"github.com/abhishek/pen-drive/backend/internal/storage"
)

type stubStorageClient struct {
	putInputs              []storage.PutObjectInput
	putErr                 error
	putErrFor              map[string]error
	existingKeys           map[string]bool
	startMultipartInput    storage.StartMultipartUploadInput
	startMultipartErr      error
	uploadPartInputs       []storage.UploadPartInput
	uploadPartErr          error
	completeMultipartInput storage.CompleteMultipartUploadInput
	completeMultipartErr   error
	abortMultipartInput    storage.AbortMultipartUploadInput
	abortMultipartErr      error
	listObjectKeys         []string
	listObjectKeysErr      error
	copyInputs             []storage.CopyObjectInput
	copyErr                error
	deleteInputs           []storage.DeleteObjectInput
	deleteErr              error
}

func (s *stubStorageClient) ListPath(context.Context, storage.ListPathInput) (storage.ListPathResult, error) {
	return storage.ListPathResult{}, nil
}

func (s *stubStorageClient) PutObject(_ context.Context, input storage.PutObjectInput) (storage.PutObjectResult, error) {
	s.putInputs = append(s.putInputs, input)
	if s.putErrFor != nil {
		if err, ok := s.putErrFor[input.Key]; ok {
			return storage.PutObjectResult{}, err
		}
	}
	if s.putErr != nil {
		return storage.PutObjectResult{}, s.putErr
	}

	return storage.PutObjectResult{Key: input.Key}, nil
}

func (s *stubStorageClient) StartMultipartUpload(_ context.Context, input storage.StartMultipartUploadInput) (storage.StartMultipartUploadResult, error) {
	s.startMultipartInput = input
	if s.startMultipartErr != nil {
		return storage.StartMultipartUploadResult{}, s.startMultipartErr
	}
	return storage.StartMultipartUploadResult{
		UploadID: "upload-123",
		Key:      input.Key,
	}, nil
}

func (s *stubStorageClient) UploadPart(_ context.Context, input storage.UploadPartInput) (storage.UploadPartResult, error) {
	s.uploadPartInputs = append(s.uploadPartInputs, input)
	if s.uploadPartErr != nil {
		return storage.UploadPartResult{}, s.uploadPartErr
	}
	return storage.UploadPartResult{
		ETag:       "\"etag-part\"",
		PartNumber: input.PartNumber,
	}, nil
}

func (s *stubStorageClient) CompleteMultipartUpload(_ context.Context, input storage.CompleteMultipartUploadInput) (storage.PutObjectResult, error) {
	s.completeMultipartInput = input
	if s.completeMultipartErr != nil {
		return storage.PutObjectResult{}, s.completeMultipartErr
	}
	return storage.PutObjectResult{
		Key:  input.Key,
		ETag: "\"etag-final\"",
	}, nil
}

func (s *stubStorageClient) AbortMultipartUpload(_ context.Context, input storage.AbortMultipartUploadInput) error {
	s.abortMultipartInput = input
	return s.abortMultipartErr
}

func (s *stubStorageClient) ObjectExists(_ context.Context, input storage.ObjectExistsInput) (bool, error) {
	if s.existingKeys == nil {
		return false, nil
	}
	return s.existingKeys[input.Key], nil
}

func (s *stubStorageClient) ListObjectKeys(_ context.Context, _ storage.ListObjectKeysInput) ([]string, error) {
	return s.listObjectKeys, s.listObjectKeysErr
}

func (s *stubStorageClient) CopyObject(_ context.Context, input storage.CopyObjectInput) error {
	s.copyInputs = append(s.copyInputs, input)
	return s.copyErr
}

func (s *stubStorageClient) DeleteObject(_ context.Context, input storage.DeleteObjectInput) error {
	s.deleteInputs = append(s.deleteInputs, input)
	return s.deleteErr
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
		dto.DuplicateConflictPolicyReject,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Path != "docs/reports/report.pdf" {
		t.Fatalf("expected normalized path, got %q", result.Path)
	}

	if len(storageClient.putInputs) != 1 {
		t.Fatalf("expected one upload call, got %d", len(storageClient.putInputs))
	}

	if storageClient.putInputs[0].ContentType != "application/pdf" {
		t.Fatalf("expected content type to be preserved, got %q", storageClient.putInputs[0].ContentType)
	}

	if storageClient.putInputs[0].Metadata["original-filename"] != " report.pdf " {
		t.Fatalf("expected original filename metadata to be preserved")
	}

	if storageClient.putInputs[0].Metadata["stored-filename"] != "report.pdf" {
		t.Fatalf("expected stored filename metadata to be trimmed")
	}

	if storageClient.putInputs[0].Metadata["uploaded-by-user-id"] != "user-123" {
		t.Fatalf("expected uploader metadata to be set")
	}

	if storageClient.putInputs[0].Metadata["uploaded-at"] == "" {
		t.Fatalf("expected uploaded-at metadata to be set")
	}
}

func TestUploadRejectsDuplicate(t *testing.T) {
	t.Parallel()

	service := NewService(&stubStorageClient{existingKeys: map[string]bool{"docs/report.pdf": true}})

	_, err := service.Upload(
		context.Background(),
		"user-123",
		"docs",
		"report.pdf",
		"application/pdf",
		bytes.NewBufferString("payload"),
		int64(len("payload")),
		dto.DuplicateConflictPolicyReject,
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
		dto.DuplicateConflictPolicyReject,
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
		dto.DuplicateConflictPolicyReject,
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected storage error to propagate, got %v", err)
	}
}

func TestPreviewDuplicatesReturnsImpactedPathsAndRenameTargets(t *testing.T) {
	t.Parallel()

	service := NewService(&stubStorageClient{
		existingKeys: map[string]bool{
			"docs/report.pdf":     true,
			"docs/report_(1).pdf": true,
		},
	})

	response, err := service.PreviewDuplicates(
		context.Background(),
		"user-123",
		"docs",
		"",
		[]string{"report.pdf", "notes.txt"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !response.HasConflicts {
		t.Fatalf("expected conflicts to be reported")
	}
	if len(response.ImpactedPaths) != 1 || response.ImpactedPaths[0] != "docs/report.pdf" {
		t.Fatalf("unexpected impacted paths: %+v", response.ImpactedPaths)
	}
	if response.Items[0].RenamePath != "docs/report_(2).pdf" {
		t.Fatalf("expected next rename path, got %+v", response.Items[0])
	}
	if response.Items[1].Conflict {
		t.Fatalf("expected non-conflicting item to remain available")
	}
}

func TestUploadRenameResolvesDuplicateWithSuffix(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{
		existingKeys: map[string]bool{
			"docs/report.pdf":     true,
			"docs/report_(1).pdf": true,
		},
	}
	service := NewService(storageClient)

	result, err := service.Upload(
		context.Background(),
		"user-123",
		"docs",
		"report.pdf",
		"application/pdf",
		bytes.NewBufferString("payload"),
		int64(len("payload")),
		dto.DuplicateConflictPolicyRename,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Path != "docs/report_(2).pdf" {
		t.Fatalf("expected renamed path, got %q", result.Path)
	}
	if storageClient.putInputs[0].Key != "docs/report_(2).pdf" {
		t.Fatalf("expected renamed upload key, got %q", storageClient.putInputs[0].Key)
	}
}

func TestUploadReplaceMovesExistingObjectToTrash(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{
		existingKeys: map[string]bool{
			"docs/report.pdf": true,
		},
	}
	service := NewService(storageClient)

	result, err := service.Upload(
		context.Background(),
		"user-123",
		"docs",
		"report.pdf",
		"application/pdf",
		bytes.NewBufferString("payload"),
		int64(len("payload")),
		dto.DuplicateConflictPolicyReplace,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Path != "docs/report.pdf" {
		t.Fatalf("expected replace to keep original key, got %q", result.Path)
	}
	if len(storageClient.copyInputs) != 1 || storageClient.copyInputs[0].DestinationKey != "trash/docs/report.pdf" {
		t.Fatalf("expected trash copy before write, got %+v", storageClient.copyInputs)
	}
	if len(storageClient.deleteInputs) != 1 || storageClient.deleteInputs[0].Key != "docs/report.pdf" {
		t.Fatalf("expected source delete after trash copy, got %+v", storageClient.deleteInputs)
	}
}

func TestValidateRelativePath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "simple file", input: "file.pdf", want: "file.pdf"},
		{name: "nested path", input: "reports/q1/file.pdf", want: "reports/q1/file.pdf"},
		{name: "backslash normalized", input: "reports\\q1\\file.pdf", want: "reports/q1/file.pdf"},
		{name: "mixed separators normalized", input: "reports/q1\\file.pdf", want: "reports/q1/file.pdf"},
		{name: "double slashes removed", input: "reports//q1///file.pdf", want: "reports/q1/file.pdf"},
		{name: "leading dot removed", input: "./reports/file.pdf", want: "reports/file.pdf"},
		{name: "embedded dot removed", input: "reports/./file.pdf", want: "reports/file.pdf"},
		{name: "trailing slash normalized", input: "reports/folder/file.pdf", want: "reports/folder/file.pdf"},
		{name: "empty rejected", input: "", wantErr: true},
		{name: "whitespace only rejected", input: "  ", wantErr: true},
		{name: "absolute path rejected", input: "/reports/file.pdf", wantErr: true},
		{name: "traversal rejected", input: "../file.pdf", wantErr: true},
		{name: "nested traversal rejected", input: "reports/../../file.pdf", wantErr: true},
		{name: "dots only rejected", input: ".", wantErr: true},
		{name: "dots only rejected", input: "..", wantErr: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := validateRelativePath(tc.input)
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

func TestBuildFinalObjectKey(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		destination string
		relative    string
		want        string
	}{
		{name: "root destination", destination: "", relative: "file.pdf", want: "file.pdf"},
		{name: "nested destination", destination: "docs/reports", relative: "file.pdf", want: "docs/reports/file.pdf"},
		{name: "nested relative", destination: "", relative: "docs/q1/file.pdf", want: "docs/q1/file.pdf"},
		{name: "both nested", destination: "uploads", relative: "reports/q1/file.pdf", want: "uploads/reports/q1/file.pdf"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := buildFinalObjectKey(tc.destination, tc.relative)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
func TestUploadFolderRejectsMismatchedArrays(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{}
	service := NewService(storageClient)

	files := []*multipart.FileHeader{
		mustCreateFileHeader(t, "files", "file1.pdf", "application/pdf", "first"),
		mustCreateFileHeader(t, "files", "file2.pdf", "application/pdf", "second"),
	}
	relativePaths := []string{"file1.pdf"}

	results, err := service.UploadFolder(
		context.Background(),
		"user-123",
		"docs",
		files,
		relativePaths,
		dto.DuplicateConflictPolicyReject,
	)

	// Should fail on the second file (index 1) due to missing relative path
	if err == nil {
		t.Fatalf("expected error for mismatched arrays, got nil")
	}
	if !errors.Is(err, ErrInvalidUploadInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}

	if results != nil {
		t.Fatalf("expected nil results on error")
	}
	if len(storageClient.putInputs) != 0 {
		t.Fatalf("expected no writes on validation failure")
	}
}

func TestUploadFolderRejectsZeroByte(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{}
	service := NewService(storageClient)

	files := []*multipart.FileHeader{
		{Size: 0},
	}
	relativePaths := []string{"file.pdf"}

	results, err := service.UploadFolder(
		context.Background(),
		"user-123",
		"",
		files,
		relativePaths,
		dto.DuplicateConflictPolicyReject,
	)

	if err == nil {
		t.Fatalf("expected zero-byte rejection, got nil")
	}

	if !errors.Is(err, ErrZeroByteFile) {
		t.Fatalf("expected zero-byte error, got %v", err)
	}

	if results != nil {
		t.Fatalf("expected nil results on error")
	}
	if len(storageClient.putInputs) != 0 {
		t.Fatalf("expected no writes on validation failure")
	}
}

func TestUploadFolderRejectsTraversal(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{}
	service := NewService(storageClient)

	files := []*multipart.FileHeader{
		mustCreateFileHeader(t, "files", "report.pdf", "application/pdf", "payload"),
	}

	results, err := service.UploadFolder(
		context.Background(),
		"user-123",
		"docs",
		files,
		[]string{"../report.pdf"},
		dto.DuplicateConflictPolicyReject,
	)

	if err == nil {
		t.Fatalf("expected traversal rejection")
	}
	if !errors.Is(err, ErrInvalidUploadInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil results on error")
	}
	if len(storageClient.putInputs) != 0 {
		t.Fatalf("expected no writes on validation failure")
	}
}

func TestUploadFolderRejectsDuplicateTargetKeyInBatch(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{}
	service := NewService(storageClient)

	files := []*multipart.FileHeader{
		mustCreateFileHeader(t, "files", "first.pdf", "application/pdf", "first"),
		mustCreateFileHeader(t, "files", "second.pdf", "application/pdf", "second"),
	}

	results, err := service.UploadFolder(
		context.Background(),
		"user-123",
		"docs",
		files,
		[]string{"reports//q1/file.pdf", "reports/q1/file.pdf"},
		dto.DuplicateConflictPolicyReject,
	)

	if err == nil {
		t.Fatalf("expected duplicate batch key rejection")
	}
	if !errors.Is(err, ErrDuplicateBatchKey) {
		t.Fatalf("expected duplicate batch key error, got %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil results on error")
	}
	if len(storageClient.putInputs) != 0 {
		t.Fatalf("expected no writes on validation failure")
	}
}

func TestUploadFolderReturnsConflictWhenTargetExists(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{
		existingKeys: map[string]bool{
			"docs/reports/file.pdf": true,
		},
	}
	service := NewService(storageClient)

	files := []*multipart.FileHeader{
		mustCreateFileHeader(t, "files", "file.pdf", "application/pdf", "payload"),
	}

	results, err := service.UploadFolder(
		context.Background(),
		"user-123",
		"docs",
		files,
		[]string{"reports/file.pdf"},
		dto.DuplicateConflictPolicyReject,
	)

	if err == nil {
		t.Fatalf("expected duplicate target conflict")
	}
	if !errors.Is(err, storage.ErrObjectAlreadyExists) {
		t.Fatalf("expected storage duplicate error, got %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil results on error")
	}
}

func TestUploadFolderUploadsInDeterministicOrderAndPreservesMetadata(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{}
	service := NewService(storageClient)

	files := []*multipart.FileHeader{
		mustCreateFileHeader(t, "files", "b.txt", "text/plain", "bbb"),
		mustCreateFileHeader(t, "files", "a.txt", "text/plain", "aaa"),
	}

	results, err := service.UploadFolder(
		context.Background(),
		"user-123",
		"docs",
		files,
		[]string{"z-dir/b.txt", "a-dir/a.txt"},
		dto.DuplicateConflictPolicyReject,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected two results, got %d", len(results))
	}

	gotKeys := []string{
		storageClient.putInputs[0].Key,
		storageClient.putInputs[1].Key,
	}
	wantKeys := []string{"docs/a-dir/a.txt", "docs/z-dir/b.txt"}
	if !slices.Equal(gotKeys, wantKeys) {
		t.Fatalf("expected deterministic upload order %v, got %v", wantKeys, gotKeys)
	}

	if results[0].Path != "docs/a-dir/a.txt" || results[1].Path != "docs/z-dir/b.txt" {
		t.Fatalf("unexpected result paths: %+v", results)
	}

	if storageClient.putInputs[0].ContentType != "text/plain" {
		t.Fatalf("expected content type to be preserved, got %q", storageClient.putInputs[0].ContentType)
	}

	metadata := storageClient.putInputs[0].Metadata
	if metadata["original-filename"] != "a.txt" {
		t.Fatalf("expected basename metadata, got %q", metadata["original-filename"])
	}
	if metadata["stored-filename"] != "a.txt" {
		t.Fatalf("expected stored filename metadata, got %q", metadata["stored-filename"])
	}
	if metadata["uploaded-by-user-id"] != "user-123" {
		t.Fatalf("expected uploader metadata, got %q", metadata["uploaded-by-user-id"])
	}
	if metadata["uploaded-at"] == "" {
		t.Fatalf("expected uploaded-at metadata")
	}
}

func TestUploadFolderRenameResolvesConflicts(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{
		existingKeys: map[string]bool{
			"docs/reports/file.pdf":     true,
			"docs/reports/file_(1).pdf": true,
		},
	}
	service := NewService(storageClient)

	files := []*multipart.FileHeader{
		mustCreateFileHeader(t, "files", "file.pdf", "application/pdf", "payload"),
	}

	results, err := service.UploadFolder(
		context.Background(),
		"user-123",
		"docs",
		files,
		[]string{"reports/file.pdf"},
		dto.DuplicateConflictPolicyRename,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if results[0].Path != "docs/reports/file_(2).pdf" {
		t.Fatalf("expected renamed folder upload path, got %+v", results[0])
	}
}

func TestInitiateMultipartUploadPassesMetadata(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{}
	service := NewService(storageClient)

	session, err := service.InitiateMultipartUpload(
		context.Background(),
		"user-123",
		"videos",
		" clip .mp4 ",
		"video/mp4",
		dto.DuplicateConflictPolicyReject,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if session.Key != "videos/clip .mp4" {
		t.Fatalf("expected sanitized key, got %q", session.Key)
	}
	if session.PartSize != multipartPartSize {
		t.Fatalf("expected multipart part size %d, got %d", multipartPartSize, session.PartSize)
	}
	if storageClient.startMultipartInput.Metadata["original-filename"] != " clip .mp4 " {
		t.Fatalf("expected original filename metadata")
	}
	if storageClient.startMultipartInput.Metadata["stored-filename"] != "clip .mp4" {
		t.Fatalf("expected stored filename metadata")
	}
}

func TestInitiateMultipartUploadRenameResolvesConflicts(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{
		existingKeys: map[string]bool{
			"videos/clip.mp4": true,
		},
	}
	service := NewService(storageClient)

	session, err := service.InitiateMultipartUpload(
		context.Background(),
		"user-123",
		"videos",
		"clip.mp4",
		"video/mp4",
		dto.DuplicateConflictPolicyRename,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if session.Key != "videos/clip_(1).mp4" {
		t.Fatalf("expected renamed multipart key, got %q", session.Key)
	}
}

func TestUploadMultipartPartRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	service := NewService(&stubStorageClient{})

	_, err := service.UploadMultipartPart(
		context.Background(),
		"user-123",
		"",
		"upload-123",
		1,
		bytes.NewBufferString("chunk"),
		int64(len("chunk")),
	)
	if err == nil || !errors.Is(err, ErrInvalidUploadInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
}

func TestUploadMultipartPartPassesChunk(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{}
	service := NewService(storageClient)

	result, err := service.UploadMultipartPart(
		context.Background(),
		"user-123",
		"videos/clip.mp4",
		"upload-123",
		2,
		bytes.NewBufferString("chunk"),
		int64(len("chunk")),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PartNumber != 2 {
		t.Fatalf("expected part number 2, got %d", result.PartNumber)
	}
	if len(storageClient.uploadPartInputs) != 1 {
		t.Fatalf("expected one part upload, got %d", len(storageClient.uploadPartInputs))
	}
}

func TestCompleteMultipartUploadSortsParts(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{}
	service := NewService(storageClient)

	result, err := service.CompleteMultipartUpload(
		context.Background(),
		"user-123",
		"videos/clip.mp4",
		"upload-123",
		[]storage.CompletedPart{
			{ETag: "\"two\"", PartNumber: 2},
			{ETag: "\"one\"", PartNumber: 1},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Path != "videos/clip.mp4" {
		t.Fatalf("expected completed path, got %q", result.Path)
	}
	if storageClient.completeMultipartInput.Parts[0].PartNumber != 1 {
		t.Fatalf("expected sorted parts, got %+v", storageClient.completeMultipartInput.Parts)
	}
}

func TestAbortMultipartUploadPassesIdentifiers(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{}
	service := NewService(storageClient)

	if err := service.AbortMultipartUpload(context.Background(), "user-123", "videos/clip.mp4", "upload-123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if storageClient.abortMultipartInput.Key != "videos/clip.mp4" || storageClient.abortMultipartInput.UploadID != "upload-123" {
		t.Fatalf("unexpected abort input: %+v", storageClient.abortMultipartInput)
	}
}

func mustCreateFileHeader(t *testing.T, fieldName, filename, contentType, content string) *multipart.FileHeader {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="`+fieldName+`"; filename="`+filename+`"`)
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}

	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("failed to create multipart part: %v", err)
	}
	if _, err := io.WriteString(part, content); err != nil {
		t.Fatalf("failed to write multipart content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	reader := multipart.NewReader(bytes.NewReader(body.Bytes()), writer.Boundary())
	form, err := reader.ReadForm(int64(len(body.Bytes())))
	if err != nil {
		t.Fatalf("failed to read multipart form: %v", err)
	}

	headers := form.File[fieldName]
	if len(headers) != 1 {
		t.Fatalf("expected one file header, got %d", len(headers))
	}

	return headers[0]
}
