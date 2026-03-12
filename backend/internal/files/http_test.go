package files

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"slices"
	"strings"
	"testing"

	"github.com/abhishek/pen-drive/backend/internal/storage"
	"github.com/gin-gonic/gin"
)

func TestUploadHandlerRejectsInvalidPath(t *testing.T) {
	t.Parallel()

	response := performUploadRequest(t, uploadRequest{
		filename:    "report.pdf",
		content:     "payload",
		contentType: "application/pdf",
		path:        "../secrets",
	})

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}

	if !strings.Contains(response.Body.String(), `"code":"invalid_input"`) {
		t.Fatalf("expected invalid_input error, got %s", response.Body.String())
	}
}

func TestUploadHandlerRejectsDuplicate(t *testing.T) {
	t.Parallel()

	response := performUploadRequest(t, uploadRequest{
		filename:    "report.pdf",
		content:     "payload",
		contentType: "application/pdf",
		path:        "docs",
	}, func(service *Service) {
		service.storage = &stubStorageClient{putErr: storage.ErrObjectAlreadyExists}
	})

	if response.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", response.Code)
	}

	if !strings.Contains(response.Body.String(), `"code":"file_exists"`) {
		t.Fatalf("expected file_exists error, got %s", response.Body.String())
	}
}

func TestUploadHandlerReturnsCreated(t *testing.T) {
	t.Parallel()

	response := performUploadRequest(t, uploadRequest{
		filename:    "report.pdf",
		content:     "payload",
		contentType: "application/pdf",
		path:        "docs/reports",
	})

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", response.Code)
	}

	body := response.Body.String()
	if !strings.Contains(body, `"path":"docs/reports/report.pdf"`) {
		t.Fatalf("expected normalized upload path in response, got %s", body)
	}
}

func TestUploadFolderHandlerRejectsMismatchedArrays(t *testing.T) {
	t.Parallel()

	response := performUploadFolderRequest(t, folderUploadRequest{
		files: []folderUploadFile{
			{name: "report.pdf", content: "payload", contentType: "application/pdf", relativePath: "docs/report.pdf"},
			{name: "chart.pdf", content: "payload", contentType: "application/pdf"},
		},
		path: "uploads",
	})

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}

	if !strings.Contains(response.Body.String(), `"code":"invalid_input"`) {
		t.Fatalf("expected invalid_input error, got %s", response.Body.String())
	}
}

func TestUploadFolderHandlerRejectsTraversal(t *testing.T) {
	t.Parallel()

	response := performUploadFolderRequest(t, folderUploadRequest{
		files: []folderUploadFile{
			{name: "report.pdf", content: "payload", contentType: "application/pdf", relativePath: "../report.pdf"},
		},
		path: "uploads",
	})

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}

	if !strings.Contains(response.Body.String(), `"code":"invalid_input"`) {
		t.Fatalf("expected invalid_input error, got %s", response.Body.String())
	}
}

func TestUploadFolderHandlerRejectsDuplicate(t *testing.T) {
	t.Parallel()

	response := performUploadFolderRequest(t, folderUploadRequest{
		files: []folderUploadFile{
			{name: "report.pdf", content: "payload", contentType: "application/pdf", relativePath: "reports/report.pdf"},
		},
		path: "docs",
	}, func(service *Service) {
		service.storage = &stubStorageClient{
			putErrFor: map[string]error{
				"docs/reports/report.pdf": storage.ErrObjectAlreadyExists,
			},
		}
	})

	if response.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", response.Code)
	}

	if !strings.Contains(response.Body.String(), `"code":"file_exists"`) {
		t.Fatalf("expected file_exists error, got %s", response.Body.String())
	}
}

func TestUploadFolderHandlerReturnsCreated(t *testing.T) {
	t.Parallel()

	storageClient := &stubStorageClient{}
	response := performUploadFolderRequest(t, folderUploadRequest{
		files: []folderUploadFile{
			{name: "b.txt", content: "bbb", contentType: "text/plain", relativePath: "z-dir/b.txt"},
			{name: "a.txt", content: "aaa", contentType: "text/plain", relativePath: "a-dir/a.txt"},
		},
		path: "docs",
	}, func(service *Service) {
		service.storage = storageClient
	})

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", response.Code)
	}

	body := response.Body.String()
	if !strings.Contains(body, `"path":"docs/a-dir/a.txt"`) || !strings.Contains(body, `"path":"docs/z-dir/b.txt"`) {
		t.Fatalf("expected uploaded paths in response, got %s", body)
	}

	gotKeys := []string{storageClient.putInputs[0].Key, storageClient.putInputs[1].Key}
	wantKeys := []string{"docs/a-dir/a.txt", "docs/z-dir/b.txt"}
	if !slices.Equal(gotKeys, wantKeys) {
		t.Fatalf("expected deterministic upload order %v, got %v", wantKeys, gotKeys)
	}
}

type uploadRequest struct {
	filename    string
	content     string
	contentType string
	path        string
}

type folderUploadFile struct {
	name         string
	content      string
	contentType  string
	relativePath string
}

type folderUploadRequest struct {
	files []folderUploadFile
	path  string
}

func performUploadRequest(t *testing.T, req uploadRequest, mutateService ...func(*Service)) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	storageClient := &stubStorageClient{}
	service := NewService(storageClient)
	for _, mutate := range mutateService {
		mutate(service)
	}

	handler := NewHandler(service)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("auth.user_id", "user-123")
		c.Next()
	})
	router.POST("/upload", handler.Upload)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, req.filename))
	if req.contentType != "" {
		partHeader.Set("Content-Type", req.contentType)
	}

	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("failed to create multipart part: %v", err)
	}

	if _, err := io.WriteString(part, req.content); err != nil {
		t.Fatalf("failed to write multipart content: %v", err)
	}

	if req.path != "" {
		if err := writer.WriteField("path", req.path); err != nil {
			t.Fatalf("failed to write path field: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/upload", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func performUploadFolderRequest(t *testing.T, req folderUploadRequest, mutateService ...func(*Service)) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	storageClient := &stubStorageClient{}
	service := NewService(storageClient)
	for _, mutate := range mutateService {
		mutate(service)
	}

	handler := NewHandler(service)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("auth.user_id", "user-123")
		c.Next()
	})
	router.POST("/upload-folder", handler.UploadFolder)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for _, file := range req.files {
		partHeader := make(textproto.MIMEHeader)
		partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files"; filename="%s"`, file.name))
		if file.contentType != "" {
			partHeader.Set("Content-Type", file.contentType)
		}

		part, err := writer.CreatePart(partHeader)
		if err != nil {
			t.Fatalf("failed to create multipart file part: %v", err)
		}
		if _, err := io.WriteString(part, file.content); err != nil {
			t.Fatalf("failed to write multipart file content: %v", err)
		}

		if file.relativePath != "" {
			if err := writer.WriteField("relative_paths", file.relativePath); err != nil {
				t.Fatalf("failed to write relative path field: %v", err)
			}
		}
	}

	if req.path != "" {
		if err := writer.WriteField("path", req.path); err != nil {
			t.Fatalf("failed to write path field: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/upload-folder", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
