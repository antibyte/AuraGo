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

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
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

// newS3Client creates a configured S3 client from the given config.
func newS3Client(cfg S3Config) (*s3.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey, cfg.SecretKey, "",
		)),
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	s3Opts := func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
	}

	return s3.NewFromConfig(awsCfg, s3Opts), nil
}

// ExecuteS3 dispatches S3 operations.
// Operations: list_buckets, list_objects, upload, download, delete, copy, move.
func ExecuteS3(cfg S3Config, operation, bucket, key, localPath, prefix, destBucket, destKey string) string {
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return s3Encode(s3Result{Status: "error", Message: "S3 credentials not configured. Store 's3_access_key' and 's3_secret_key' in the secrets vault."})
	}

	client, err := newS3Client(cfg)
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("S3 client init failed: %v", err)})
	}

	operation = strings.ToLower(strings.TrimSpace(operation))
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

func s3ListBuckets(client *s3.Client) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("list buckets: %v", err)})
	}

	buckets := make([]s3BucketInfo, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		info := s3BucketInfo{Name: aws.ToString(b.Name)}
		if b.CreationDate != nil {
			info.Created = b.CreationDate.Format(time.RFC3339)
		}
		buckets = append(buckets, info)
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("%d bucket(s)", len(buckets)), Data: buckets})
}

func s3ListObjects(client *s3.Client, bucket, prefix string) string {
	if bucket == "" {
		return s3Encode(s3Result{Status: "error", Message: "bucket is required"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		MaxKeys: aws.Int32(1000),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}

	out, err := client.ListObjectsV2(ctx, input)
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("list objects: %v", err)})
	}

	objects := make([]s3ObjectInfo, 0, len(out.Contents))
	for _, obj := range out.Contents {
		info := s3ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			StorageClass: string(obj.StorageClass),
		}
		if obj.LastModified != nil {
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

func s3Upload(client *s3.Client, bucket, key, localPath string) string {
	if bucket == "" {
		return s3Encode(s3Result{Status: "error", Message: "bucket is required"})
	}
	if key == "" {
		return s3Encode(s3Result{Status: "error", Message: "key is required"})
	}
	if localPath == "" {
		return s3Encode(s3Result{Status: "error", Message: "local_path is required for upload"})
	}

	file, err := os.Open(localPath)
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("open file: %v", err)})
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("upload: %v", err)})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("uploaded %s → s3://%s/%s", filepath.Base(localPath), bucket, key)})
}

func s3Download(client *s3.Client, bucket, key, localPath string) string {
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

	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("download: %v", err)})
	}
	defer out.Body.Close()

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

	written, err := io.Copy(file, out.Body)
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("write file: %v", err)})
	}

	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("downloaded s3://%s/%s → %s (%d bytes)", bucket, key, localPath, written)})
}

func s3Delete(client *s3.Client, bucket, key string) string {
	if bucket == "" {
		return s3Encode(s3Result{Status: "error", Message: "bucket is required"})
	}
	if key == "" {
		return s3Encode(s3Result{Status: "error", Message: "key is required"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("delete: %v", err)})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("deleted s3://%s/%s", bucket, key)})
}

func s3Copy(client *s3.Client, srcBucket, srcKey, dstBucket, dstKey string) string {
	if srcBucket == "" || srcKey == "" {
		return s3Encode(s3Result{Status: "error", Message: "source bucket and key are required"})
	}
	if dstBucket == "" || dstKey == "" {
		return s3Encode(s3Result{Status: "error", Message: "destination_bucket and destination_key are required"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	copySource := srcBucket + "/" + srcKey
	_, err := client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(dstBucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(copySource),
	})
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("copy: %v", err)})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("copied s3://%s/%s → s3://%s/%s", srcBucket, srcKey, dstBucket, dstKey)})
}

func s3Move(client *s3.Client, srcBucket, srcKey, dstBucket, dstKey string) string {
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

// Ensure unused imports are not flagged.
var _ s3types.ObjectStorageClass
