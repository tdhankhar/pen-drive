package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/abhishek/pen-drive/backend/internal/config"
)

type Client struct {
	s3         *s3.Client
	pingBucket string
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
