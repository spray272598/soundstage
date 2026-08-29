package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spray272598/soundstage/internal/pkg/config"
)

// Client wraps MinIO client for media storage.
type Client struct {
	client     *minio.Client
	bucket     string
	presignTTL time.Duration
}

// NewMinIOClient creates a new MinIO client.
func NewMinIOClient(cfg config.MinIOConfig) (*Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket exists: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket: %w", err)
		}
	}

	return &Client{
		client:     client,
		bucket:     cfg.Bucket,
		presignTTL: 24 * time.Hour,
	}, nil
}

// Upload uploads an object to MinIO.
func (c *Client) Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) error {
	_, err := c.client.PutObject(ctx, c.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// Download downloads an object from MinIO.
func (c *Client) Download(ctx context.Context, objectName string) (io.ReadCloser, error) {
	obj, err := c.client.GetObject(ctx, c.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// Delete deletes an object from MinIO.
func (c *Client) Delete(ctx context.Context, objectName string) error {
	return c.client.RemoveObject(ctx, c.bucket, objectName, minio.RemoveObjectOptions{})
}

// PresignGetURL generates a presigned URL for downloading an object.
func (c *Client) PresignGetURL(ctx context.Context, objectName string, ttl time.Duration) (string, error) {
	url, err := c.client.PresignedGetObject(ctx, c.bucket, objectName, ttl, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

// PresignPutURL generates a presigned URL for uploading an object.
func (c *Client) PresignPutURL(ctx context.Context, objectName string, ttl time.Duration) (string, error) {
	url, err := c.client.PresignedPutObject(ctx, c.bucket, objectName, ttl)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

// Stat returns object info.
func (c *Client) Stat(ctx context.Context, objectName string) (minio.ObjectInfo, error) {
	return c.client.StatObject(ctx, c.bucket, objectName, minio.StatObjectOptions{})
}

// ListObjects lists objects with prefix.
func (c *Client) ListObjects(ctx context.Context, prefix string, recursive bool) ([]minio.ObjectInfo, error) {
	var objects []minio.ObjectInfo
	ch := c.client.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: recursive,
	})
	for obj := range ch {
		if obj.Err != nil {
			return nil, obj.Err
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

// Bucket returns the bucket name.
func (c *Client) Bucket() string {
	return c.bucket
}

// Close closes the client (no-op for MinIO).
func (c *Client) Close() error {
	return nil
}