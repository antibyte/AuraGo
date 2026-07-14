package virtualcomputers

import (
	"context"
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type StorageTestConfig struct {
	Endpoint    string
	Bucket      string
	Region      string
	AccessKeyID string
	SecretKey   string
	UseSSL      bool
}

func TestStorageConnection(ctx context.Context, cfg StorageTestConfig) error {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return fmt.Errorf("S3 endpoint is required")
	}
	if strings.Contains(endpoint, "://") {
		return fmt.Errorf("S3 endpoint must be host:port without a URL scheme")
	}
	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return fmt.Errorf("S3 bucket is required")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretKey) == "" {
		return fmt.Errorf("S3 access key and secret key are required")
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(strings.TrimSpace(cfg.AccessKeyID), strings.TrimSpace(cfg.SecretKey), ""),
		Secure: cfg.UseSSL,
		Region: region,
	})
	if err != nil {
		return fmt.Errorf("configure S3 client: %w", err)
	}
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check S3 bucket: %w", err)
	}
	if !exists {
		return fmt.Errorf("S3 bucket %q was not found", bucket)
	}
	return nil
}
