package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/abhishek/pen-drive/backend/internal/config"
)

type Client struct {
	s3         *s3.Client
	pingBucket string
}

type ListPathInput struct {
	Bucket            string
	Prefix            string
	ContinuationToken string
	MaxKeys           int32
}

type ListPathResult struct {
	Folders               []FolderEntry
	Files                 []FileEntry
	NextContinuationToken string
	HasMore               bool
}

type FolderEntry struct {
	Name string
	Path string
}

type FileEntry struct {
	Name         string
	Path         string
	Size         int64
	LastModified time.Time
}

type PutObjectInput struct {
	Bucket      string
	Key         string
	Body        io.Reader
	Size        int64
	ContentType string
	Metadata    map[string]string
}

type PutObjectResult struct {
	Key  string
	ETag string
}

type ObjectMetadata struct {
	OriginalFilename string
	StoredFilename   string
	UploadedByUserID string
	UploadedAt       string
}

func NewClient(_ context.Context, cfg config.S3Config) (*Client, error) {
	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccess, "")),
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = cfg.UsePathStyle
		options.BaseEndpoint = &cfg.Endpoint
	})

	return &Client{
		s3:         client,
		pingBucket: cfg.PingBucket,
	}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: &c.pingBucket,
	})
	if err != nil {
		return fmt.Errorf("head bucket %q: %w", c.pingBucket, err)
	}

	return nil
}

func (c *Client) CreateBucket(ctx context.Context, bucket string) error {
	_, err := c.s3.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &bucket,
	})
	if err != nil {
		var responseErr *awshttp.ResponseError
		if errors.As(err, &responseErr) && responseErr.HTTPStatusCode() == 409 {
			return nil
		}
		return fmt.Errorf("create bucket %q: %w", bucket, err)
	}

	return nil
}

func (c *Client) DeleteBucket(ctx context.Context, bucket string) error {
	_, err := c.s3.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: &bucket,
	})
	if err != nil {
		return fmt.Errorf("delete bucket %q: %w", bucket, err)
	}

	return nil
}

func (c *Client) ListPath(ctx context.Context, input ListPathInput) (ListPathResult, error) {
	maxKeys := input.MaxKeys
	if maxKeys <= 0 {
		maxKeys = 100
	}

	response, err := c.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:            &input.Bucket,
		Prefix:            aws.String(input.Prefix),
		Delimiter:         aws.String("/"),
		ContinuationToken: emptyToNil(input.ContinuationToken),
		MaxKeys:           aws.Int32(maxKeys),
	})
	if err != nil {
		return ListPathResult{}, fmt.Errorf("list path %q in bucket %q: %w", input.Prefix, input.Bucket, err)
	}

	result := ListPathResult{
		Folders:               make([]FolderEntry, 0, len(response.CommonPrefixes)),
		Files:                 make([]FileEntry, 0, len(response.Contents)),
		NextContinuationToken: aws.ToString(response.NextContinuationToken),
		HasMore:               response.IsTruncated != nil && *response.IsTruncated,
	}

	for _, prefix := range response.CommonPrefixes {
		fullPrefix := strings.TrimSuffix(aws.ToString(prefix.Prefix), "/")
		if fullPrefix == "" {
			continue
		}

		result.Folders = append(result.Folders, FolderEntry{
			Name: path.Base(fullPrefix),
			Path: fullPrefix,
		})
	}

	for _, object := range response.Contents {
		key := aws.ToString(object.Key)
		if key == "" || key == input.Prefix {
			continue
		}

		result.Files = append(result.Files, FileEntry{
			Name:         path.Base(key),
			Path:         key,
			Size:         aws.ToInt64(object.Size),
			LastModified: aws.ToTime(object.LastModified),
		})
	}

	return result, nil
}

func (c *Client) PutObject(ctx context.Context, input PutObjectInput) (PutObjectResult, error) {
	if input.Size == 0 {
		return PutObjectResult{}, errors.New("zero-byte uploads not allowed")
	}

	// Check if object already exists
	_, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &input.Bucket,
		Key:    &input.Key,
	})
	if err == nil {
		// Object exists
		return PutObjectResult{}, ErrObjectAlreadyExists
	}
	var responseErr *awshttp.ResponseError
	if !errors.As(err, &responseErr) || responseErr.HTTPStatusCode() != 404 {
		// Some other error occurred
		return PutObjectResult{}, fmt.Errorf("check object existence: %w", err)
	}

	// Set ContentType default
	contentType := input.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Prepare metadata
	userMetadata := input.Metadata
	if userMetadata == nil {
		userMetadata = make(map[string]string)
	}

	// Perform the upload
	uploader := manager.NewUploader(c.s3)
	result, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      &input.Bucket,
		Key:         &input.Key,
		Body:        input.Body,
		ContentType: aws.String(contentType),
		Metadata:    userMetadata,
	})
	if err != nil {
		return PutObjectResult{}, fmt.Errorf("upload object %q to bucket %q: %w", input.Key, input.Bucket, err)
	}

	return PutObjectResult{
		Key:  input.Key,
		ETag: aws.ToString(result.ETag),
	}, nil
}

func emptyToNil(value string) *string {
	if value == "" {
		return nil
	}
	return aws.String(value)
}

// ErrObjectAlreadyExists is returned when attempting to upload to a key that already exists
var ErrObjectAlreadyExists = errors.New("object already exists")
