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

func TestDeleteHandlerReturnsDeletedPaths(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	service := NewService(&stubStorageClient{
		existingKeys: map[string]bool{
			"docs/report.pdf": true,
		},
	})
	handler := NewHandler(service)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("auth.user_id", "user-123")
		c.Next()
	})
	router.DELETE("/files", handler.Delete)

	request := httptest.NewRequest(http.MethodDelete, "/files?path=docs/report.pdf&type=file", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"deleted_paths":["docs/report.pdf"]`) {
		t.Fatalf("expected deleted path in response, got %s", response.Body.String())
	}
}

func TestDeleteHandlerReturnsNotFound(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	service := NewService(&stubStorageClient{})
	handler := NewHandler(service)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("auth.user_id", "user-123")
		c.Next()
	})
	router.DELETE("/files", handler.Delete)

	request := httptest.NewRequest(http.MethodDelete, "/files?path=docs/report.pdf&type=file", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("expected not_found error, got %s", response.Body.String())
	}
}

func TestInitiateMultipartUploadHandlerReturnsCreated(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	service := NewService(&stubStorageClient{})
	handler := NewHandler(service)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("auth.user_id", "user-123")
		c.Next()
	})
	router.POST("/upload-multipart/initiate", handler.InitiateMultipartUpload)

	request := httptest.NewRequest(
		http.MethodPost,
		"/upload-multipart/initiate",
		strings.NewReader(`{"filename":"video.mp4","path":"clips","content_type":"video/mp4","size":7340032}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"upload_id":"upload-123"`) {
		t.Fatalf("expected upload id in response, got %s", response.Body.String())
	}
}

func TestUploadMultipartPartHandlerReturnsETag(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	service := NewService(&stubStorageClient{})
	handler := NewHandler(service)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("auth.user_id", "user-123")
		c.Next()
	})
	router.POST("/upload-multipart/part", handler.UploadMultipartPart)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("upload_id", "upload-123"); err != nil {
		t.Fatalf("failed to write upload_id field: %v", err)
	}
	if err := writer.WriteField("key", "clips/video.mp4"); err != nil {
		t.Fatalf("failed to write key field: %v", err)
	}
	if err := writer.WriteField("part_number", "1"); err != nil {
		t.Fatalf("failed to write part_number field: %v", err)
	}

	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="part"; filename="chunk.bin"`)
	partHeader.Set("Content-Type", "application/octet-stream")
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		t.Fatalf("failed to create multipart part: %v", err)
	}
	if _, err := io.WriteString(part, "chunk"); err != nil {
		t.Fatalf("failed to write chunk payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/upload-multipart/part", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"etag":"\"etag-part\""`) {
		t.Fatalf("expected etag in response, got %s", response.Body.String())
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
