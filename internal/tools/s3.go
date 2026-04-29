// Package tools – s3: S3 storage operations (AWS S3 + S3-compatible: MinIO, Wasabi, Backblaze B2).
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Config holds the resolved S3 connection parameters.
type S3Config struct {
	Endpoint     string
	Region       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
	Insecure     bool
	ReadOnly     bool
}

// s3Result is the JSON payload returned by S3 operations.
type s3Result struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// s3BucketInfo represents a bucket in list results.
type s3BucketInfo struct {
	Name    string `json:"name"`
	Created string `json:"created,omitempty"`
}

// s3ObjectInfo represents an object in list results.
type s3ObjectInfo struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified string `json:"last_modified,omitempty"`
	StorageClass string `json:"storage_class,omitempty"`
}

func s3Encode(r s3Result) string {
	b, _ := json.Marshal(r)
	return string(b)
}

// parseEndpoint strips an http/https scheme from an endpoint and returns
// the bare host:port together with whether TLS should be used.
func parseEndpoint(endpoint string, insecure bool) (string, bool) {
	if strings.HasPrefix(endpoint, "https://") {
		return strings.TrimPrefix(endpoint, "https://"), true
	}
	if strings.HasPrefix(endpoint, "http://") {
		return strings.TrimPrefix(endpoint, "http://"), false
	}
	return endpoint, !insecure
}

// newS3Client creates a configured minio client from the given config.
func newS3Client(cfg S3Config) (*minio.Client, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}

	host, secure := parseEndpoint(endpoint, cfg.Insecure)

	lookup := minio.BucketLookupAuto
	if cfg.UsePathStyle {
		lookup = minio.BucketLookupPath
	}

	opts := &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       secure,
		Region:       cfg.Region,
		BucketLookup: lookup,
	}

	return minio.New(host, opts)
}

// ExecuteS3 dispatches S3 operations.
// Operations: list_buckets, list_objects, upload, download, delete, copy, move.
func ExecuteS3(cfg S3Config, operation, bucket, key, localPath, prefix, destBucket, destKey string) string {
	operation = strings.ToLower(strings.TrimSpace(operation))
	if cfg.ReadOnly {
		switch operation {
		case "upload", "delete", "copy", "move":
			return s3Encode(s3Result{Status: "error", Message: "S3 is in read-only mode. Disable s3.readonly to allow changes."})
		}
	}

	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return s3Encode(s3Result{Status: "error", Message: "S3 credentials not configured. Store 's3_access_key' and 's3_secret_key' in the secrets vault."})
	}

	client, err := newS3Client(cfg)
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("S3 client init failed: %v", err)})
	}

	switch operation {
	case "list_buckets":
		return s3ListBuckets(client)
	case "list_objects", "list":
		return s3ListObjects(client, resolveBucket(bucket, cfg.Bucket), prefix)
	case "upload":
		return s3Upload(client, resolveBucket(bucket, cfg.Bucket), key, localPath)
	case "download":
		return s3Download(client, resolveBucket(bucket, cfg.Bucket), key, localPath)
	case "delete":
		return s3Delete(client, resolveBucket(bucket, cfg.Bucket), key)
	case "copy":
		return s3Copy(client, resolveBucket(bucket, cfg.Bucket), key, resolveBucket(destBucket, cfg.Bucket), destKey)
	case "move":
		return s3Move(client, resolveBucket(bucket, cfg.Bucket), key, resolveBucket(destBucket, cfg.Bucket), destKey)
	default:
		return s3Encode(s3Result{Status: "error", Message: "operation must be: list_buckets, list_objects, upload, download, delete, copy, or move"})
	}
}

func resolveBucket(explicit, fallback string) string {
	if explicit != "" {
		return explicit
	}
	return fallback
}

// ── Operations ───────────────────────────────────────────────────────────────

func s3ListBuckets(client *minio.Client) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	list, err := client.ListBuckets(ctx)
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("list buckets: %v", err)})
	}

	buckets := make([]s3BucketInfo, 0, len(list))
	for _, b := range list {
		buckets = append(buckets, s3BucketInfo{
			Name:    b.Name,
			Created: b.CreationDate.Format(time.RFC3339),
		})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("%d bucket(s)", len(buckets)), Data: buckets})
}

func s3ListObjects(client *minio.Client, bucket, prefix string) string {
	if bucket == "" {
		return s3Encode(s3Result{Status: "error", Message: "bucket is required"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	opts := minio.ListObjectsOptions{
		Prefix:  prefix,
		MaxKeys: 1000,
	}

	objects := make([]s3ObjectInfo, 0, 64)
	for obj := range client.ListObjects(ctx, bucket, opts) {
		if obj.Err != nil {
			return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("list objects: %v", obj.Err)})
		}
		info := s3ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			StorageClass: obj.StorageClass,
		}
		if !obj.LastModified.IsZero() {
			info.LastModified = obj.LastModified.Format(time.RFC3339)
		}
		objects = append(objects, info)
	}

	msg := fmt.Sprintf("%d object(s) in %s", len(objects), bucket)
	if prefix != "" {
		msg += fmt.Sprintf(" (prefix: %s)", prefix)
	}
	return s3Encode(s3Result{Status: "success", Message: msg, Data: objects})
}

func s3Upload(client *minio.Client, bucket, key, localPath string) string {
	if bucket == "" {
		return s3Encode(s3Result{Status: "error", Message: "bucket is required"})
	}
	if key == "" {
		return s3Encode(s3Result{Status: "error", Message: "key is required"})
	}
	if localPath == "" {
		return s3Encode(s3Result{Status: "error", Message: "local_path is required for upload"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	_, err := client.FPutObject(ctx, bucket, key, localPath, minio.PutObjectOptions{})
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("upload: %v", err)})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("uploaded %s → s3://%s/%s", filepath.Base(localPath), bucket, key)})
}

func s3Download(client *minio.Client, bucket, key, localPath string) string {
	if bucket == "" {
		return s3Encode(s3Result{Status: "error", Message: "bucket is required"})
	}
	if key == "" {
		return s3Encode(s3Result{Status: "error", Message: "key is required"})
	}
	if localPath == "" {
		localPath = filepath.Base(key)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	obj, err := client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("download: %v", err)})
	}
	defer obj.Close()

	if dir := filepath.Dir(localPath); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("mkdir: %v", err)})
		}
	}

	file, err := os.Create(localPath)
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("create file: %v", err)})
	}
	defer file.Close()

	written, err := io.Copy(file, obj)
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("write file: %v", err)})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("downloaded s3://%s/%s → %s (%d bytes)", bucket, key, localPath, written)})
}

func s3Delete(client *minio.Client, bucket, key string) string {
	if bucket == "" {
		return s3Encode(s3Result{Status: "error", Message: "bucket is required"})
	}
	if key == "" {
		return s3Encode(s3Result{Status: "error", Message: "key is required"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("delete: %v", err)})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("deleted s3://%s/%s", bucket, key)})
}

func s3Copy(client *minio.Client, srcBucket, srcKey, dstBucket, dstKey string) string {
	if srcBucket == "" || srcKey == "" {
		return s3Encode(s3Result{Status: "error", Message: "source bucket and key are required"})
	}
	if dstBucket == "" || dstKey == "" {
		return s3Encode(s3Result{Status: "error", Message: "destination_bucket and destination_key are required"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	dst := minio.CopyDestOptions{Bucket: dstBucket, Object: dstKey}
	src := minio.CopySrcOptions{Bucket: srcBucket, Object: srcKey}
	_, err := client.CopyObject(ctx, dst, src)
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("copy: %v", err)})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("copied s3://%s/%s → s3://%s/%s", srcBucket, srcKey, dstBucket, dstKey)})
}

func s3Move(client *minio.Client, srcBucket, srcKey, dstBucket, dstKey string) string {
	// Copy then delete
	result := s3Copy(client, srcBucket, srcKey, dstBucket, dstKey)
	var r s3Result
	if err := json.Unmarshal([]byte(result), &r); err != nil || r.Status != "success" {
		return result
	}

	delResult := s3Delete(client, srcBucket, srcKey)
	var dr s3Result
	if err := json.Unmarshal([]byte(delResult), &dr); err != nil || dr.Status != "success" {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("copy succeeded but delete failed: %s", dr.Message)})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("moved s3://%s/%s → s3://%s/%s", srcBucket, srcKey, dstBucket, dstKey)})
}
