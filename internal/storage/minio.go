// Package storage provides MinIO object storage connectivity.
package storage

import (
        "context"
        "fmt"
        "io"
        "log"
        "net/url"
        "time"

        "github.com/minio/minio-go/v7"
        "github.com/minio/minio-go/v7/pkg/credentials"

        "github.com/priyansx01/smartfm-lms/internal/config"
)

// Client wraps the MinIO SDK with LMS-specific operations.
type Client struct {
        mc           *minio.Client
        rawBucket    string
        hlsBucket    string
        thumbBucket  string
}

// NewClient creates a MinIO client and ensures required buckets exist.
func NewClient(cfg config.MinIOConfig) (*Client, error) {
        mc, err := minio.New(cfg.Endpoint, &minio.Options{
                Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
                Secure: cfg.UseSSL,
        })
        if err != nil {
                return nil, fmt.Errorf("minio connect: %w", err)
        }

        client := &Client{
                mc:          mc,
                rawBucket:   cfg.RawBucket,
                hlsBucket:   cfg.HLSBucket,
                thumbBucket: cfg.ThumbnailsBucket,
        }

        // Ensure buckets exist
        ctx := context.Background()
        for _, bucket := range []string{cfg.RawBucket, cfg.HLSBucket, cfg.ThumbnailsBucket} {
                exists, err := mc.BucketExists(ctx, bucket)
                if err != nil {
                        return nil, fmt.Errorf("check bucket %s: %w", bucket, err)
                }
                if !exists {
                        if err := mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
                                return nil, fmt.Errorf("create bucket %s: %w", bucket, err)
                        }
                        log.Printf("✓ Created MinIO bucket: %s", bucket)
                }
        }

        log.Printf("✓ Connected to MinIO (%s)", cfg.Endpoint)
        return client, nil
}

// UploadFile uploads a file directly to the raw bucket bypassing presigned URLs.
func (c *Client) UploadFile(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
        _, err := c.mc.PutObject(ctx, c.rawBucket, objectKey, reader, size, minio.PutObjectOptions{ContentType: contentType})
        if err != nil {
                return fmt.Errorf("upload put: %w", err)
        }
        return nil
}

// PresignedUpload generates a presigned PUT URL for direct browser upload.// The instructor uploads raw video directly to MinIO, bypassing the API server.
func (c *Client) PresignedUpload(objectKey string, ttl time.Duration) (string, error) {
	presignedURL, err := c.mc.PresignedPutObject(
		context.Background(),
		c.rawBucket,
		objectKey,
		ttl,
	)
	if err != nil {
		return "", fmt.Errorf("presigned put: %w", err)
	}
	return presignedURL.String(), nil
}

// PresignedDownload generates a presigned GET URL for video playback.
// Used until CloudFront is integrated.
func (c *Client) PresignedDownload(bucket, objectKey string, ttl time.Duration) (string, error) {
	reqParams := make(url.Values)
	presignedURL, err := c.mc.PresignedGetObject(
		context.Background(),
		bucket,
		objectKey,
		ttl,
		reqParams,
	)
	if err != nil {
		return "", fmt.Errorf("presigned get: %w", err)
	}
	return presignedURL.String(), nil
}

// HLSPlaybackURL returns a presigned URL for the HLS master playlist.
func (c *Client) HLSPlaybackURL(courseID, moduleID string, ttl time.Duration) (string, error) {
	objectKey := fmt.Sprintf("courses/%s/%s/master.m3u8", courseID, moduleID)
	return c.PresignedDownload(c.hlsBucket, objectKey, ttl)
}
