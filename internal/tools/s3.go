// Package tools – s3: S3 storage operations (AWS S3 + S3-compatible: MinIO, Wasabi, Backblaze B2).
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Config holds the resolved S3 connection parameters.
type S3Config struct {
	Endpoint             string
	Region               string
	Bucket               string
	AccessKey            string
	SecretKey            string
	UsePathStyle         bool
	Insecure             bool
	ReadOnly             bool
	WorkspaceDir         string
	DataDir              string
	AllowFilesystemWrite bool
	Config               *config.Config
	MaxDownloadBytes     int64
	MaxOutputBytes       int
	ctx                  context.Context
	lease                *config.RustFSS3Lease
}

// s3Result is the JSON payload returned by S3 operations.
type s3Result struct {
	Status    string      `json:"status"`
	Code      string      `json:"code,omitempty"`
	Truncated bool        `json:"truncated,omitempty"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
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
	r.Message = security.Scrub(r.Message)
	b, _ := json.Marshal(r)
	return security.Scrub(string(b))
}

// parseEndpoint strips an http/https scheme and enforces explicit opt-in for HTTP.
func parseEndpoint(endpoint string, insecure bool) (string, bool, error) {
	endpoint = strings.TrimSpace(endpoint)
	lower := strings.ToLower(endpoint)
	if strings.HasPrefix(lower, "https://") {
		return endpoint[len("https://"):], true, nil
	}
	if strings.HasPrefix(lower, "http://") {
		if !insecure {
			return "", false, fmt.Errorf("HTTP S3 endpoints require s3.insecure=true")
		}
		return endpoint[len("http://"):], false, nil
	}
	return endpoint, !insecure, nil
}

// newS3Client creates a configured minio client from the given config.
func newS3Client(cfg S3Config) (*minio.Client, error) {
	security.RegisterSensitive(cfg.AccessKey)
	security.RegisterSensitive(cfg.SecretKey)
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}

	host, secure, err := parseEndpoint(endpoint, cfg.Insecure)
	if err != nil {
		return nil, err
	}

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

	if cfg.managed() {
		opts.MaxRetries = 1
	} // The SDK defines one attempt as retries disabled.
	return minio.New(host, opts)
}

// ExecuteS3 dispatches S3 operations.
// Operations: list_buckets, list_objects, upload, download, delete, copy, move.
func ExecuteS3(cfg S3Config, operation, bucket, key, localPath, prefix, destBucket, destKey string) string {
	return ExecuteS3Context(context.Background(), cfg, operation, bucket, key, localPath, prefix, destBucket, destKey)
}

// ExecuteS3Context keeps one target and permission generation for the whole operation.
func ExecuteS3Context(ctx context.Context, cfg S3Config, operation, bucket, key, localPath, prefix, destBucket, destKey string) string {
	operation = strings.ToLower(strings.TrimSpace(operation))
	if operation == "list" {
		operation = "list_objects"
	}
	switch operation {
	case "list_buckets", "list_objects", "upload", "download", "delete", "copy", "move":
	default:
		return s3Encode(s3Result{Status: "error", Message: "operation must be: list_buckets, list_objects, upload, download, delete, copy, or move"})
	}
	if cfg.ReadOnly && s3Mutation(operation) {
		return s3Encode(s3Result{Status: "error", Message: "S3 is in read-only mode. Disable s3.readonly to allow changes."})
	}
	cfg, release, err := acquireS3Config(ctx, cfg, true)
	if err != nil {
		return s3Encode(s3Result{Status: "error", Code: "s3_access_unavailable", Message: err.Error()})
	}
	defer release()
	if cfg.ReadOnly && s3Mutation(operation) {
		return s3Encode(s3Result{Status: "policy_denied", Code: "s3_readonly", Message: "S3 is in read-only mode"})
	}
	bucket = resolveBucket(bucket, cfg.Bucket)
	destBucket = resolveS3DestinationBucket(bucket, destBucket)
	prefix, err = authorizeS3Operation(cfg, operation, bucket, key, prefix, destBucket, destKey)
	if err != nil {
		return s3Encode(s3Result{Status: "policy_denied", Code: "s3_permission_denied", Message: err.Error()})
	}
	if cfg.managed() && operation == "list_buckets" {
		return s3ManagedBuckets(cfg)
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
		return s3ListBuckets(client, cfg)
	case "list_objects":
		return s3ListObjects(client, cfg, bucket, prefix)
	case "upload":
		return s3Upload(client, cfg, bucket, key, localPath)
	case "download":
		return s3Download(client, cfg, bucket, key, localPath)
	case "delete":
		return s3Delete(client, cfg, bucket, key)
	case "copy":
		return s3Copy(client, cfg, bucket, key, destBucket, destKey)
	default:
		return s3Move(client, cfg, bucket, key, destBucket, destKey)
	}
}

func resolveBucket(explicit, fallback string) string {
	if explicit != "" {
		return explicit
	}
	return fallback
}

func resolveS3DestinationBucket(srcBucket, explicitDestBucket string) string {
	if strings.TrimSpace(explicitDestBucket) != "" {
		return explicitDestBucket
	}
	return srcBucket
}

func resolveS3DownloadDestination(cfg S3Config, key, localPath string) (string, error) {
	perms, configured := EffectiveRuntimePermissions(cfg.requestContext())
	if !cfg.AllowFilesystemWrite || configured && !perms.AllowFilesystemWrite {
		return "", fmt.Errorf("filesystem write is disabled by runtime permissions")
	}
	if strings.TrimSpace(cfg.WorkspaceDir) == "" {
		return "", fmt.Errorf("workspace_dir is not configured")
	}
	dest := strings.TrimSpace(localPath)
	if dest == "" {
		dest = filepath.Base(key)
	}
	if dest == "" || dest == "." || dest == string(filepath.Separator) {
		return "", fmt.Errorf("local_path or key filename is required for download")
	}
	resolved, err := resolveS3PathWithinRoot(cfg.WorkspaceDir, dest, true, "workspace")
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		return "", fmt.Errorf("download destination is a directory")
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("download destination: %w", err)
	}
	return resolved, nil
}

func resolveS3UploadSource(cfg S3Config, localPath string) (string, error) {
	if strings.TrimSpace(localPath) == "" {
		return "", fmt.Errorf("local_path is required for upload")
	}

	var lastErr error
	if strings.TrimSpace(cfg.WorkspaceDir) != "" {
		if resolved, err := resolveS3PathWithinRoot(cfg.WorkspaceDir, localPath, true, "workspace"); err == nil {
			return validateS3UploadSource(resolved)
		} else {
			lastErr = err
		}
	}
	if strings.TrimSpace(cfg.DataDir) != "" {
		if resolved, err := resolveS3PathWithinRoot(cfg.DataDir, localPath, false, "data"); err == nil {
			return validateS3UploadSource(resolved)
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("local_path must be within workspace or data directory: %w", lastErr)
	}
	return "", fmt.Errorf("workspace or data directory is not configured")
}

func validateS3UploadSource(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("upload source: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("upload source is a directory")
	}
	return path, nil
}

func resolveS3PathWithinRoot(root, userPath string, normalizeWorkspace bool, rootName string) (string, error) {
	absRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		absRoot, err = filepath.Abs(root)
		if err != nil {
			return "", fmt.Errorf("resolve %s root: %w", rootName, err)
		}
	}

	candidate := strings.TrimSpace(userPath)
	if normalizeWorkspace {
		candidate = normalizeS3WorkspacePath(candidate)
	}

	var full string
	if filepath.IsAbs(candidate) {
		full = filepath.Clean(candidate)
	} else {
		full = filepath.Clean(filepath.Join(absRoot, candidate))
	}
	resolved, err := secureResolveFinalPath(full)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if !s3PathWithinRoot(absRoot, resolved) {
		return "", fmt.Errorf("path must stay within %s root", rootName)
	}
	return resolved, nil
}

func normalizeS3WorkspacePath(path string) string {
	slash := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	switch {
	case slash == "workdir" || slash == "/workdir" || slash == "agent_workspace/workdir" || slash == "/agent_workspace/workdir":
		return "."
	case strings.HasPrefix(slash, "workdir/"):
		return filepath.FromSlash(strings.TrimPrefix(slash, "workdir/"))
	case strings.HasPrefix(slash, "/workdir/"):
		return filepath.FromSlash(strings.TrimPrefix(slash, "/workdir/"))
	case strings.HasPrefix(slash, "agent_workspace/workdir/"):
		return filepath.FromSlash(strings.TrimPrefix(slash, "agent_workspace/workdir/"))
	case strings.HasPrefix(slash, "/agent_workspace/workdir/"):
		return filepath.FromSlash(strings.TrimPrefix(slash, "/agent_workspace/workdir/"))
	default:
		return filepath.FromSlash(slash)
	}
}

func s3PathWithinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

// ── Operations ───────────────────────────────────────────────────────────────

func s3ListBuckets(client *minio.Client, cfg S3Config) string {
	ctx, cancel := context.WithTimeout(cfg.requestContext(), 30*time.Second)
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

func s3ListObjects(client *minio.Client, cfg S3Config, bucket, prefix string) string {
	if bucket == "" {
		return s3Encode(s3Result{Status: "error", Message: "bucket is required"})
	}

	ctx, cancel := context.WithTimeout(cfg.requestContext(), 60*time.Second)
	defer cancel()

	opts := minio.ListObjectsOptions{
		Prefix:  prefix,
		MaxKeys: 1000,
	}

	objects := make([]s3ObjectInfo, 0, 64)
	outputBytes := 0
	truncated := false
	for obj := range client.ListObjects(ctx, bucket, opts) {
		if obj.Err != nil {
			return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("list objects: %v", obj.Err)})
		}
		if cfg.managed() && !cfg.allows(bucket, obj.Key, "read") {
			return s3Encode(s3Result{Status: "policy_denied", Code: "s3_permission_denied", Message: "S3 returned an object outside the permitted prefix"})
		}
		info := s3ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			StorageClass: obj.StorageClass,
		}
		if !obj.LastModified.IsZero() {
			info.LastModified = obj.LastModified.Format(time.RFC3339)
		}
		if cfg.managed() {
			encoded, _ := json.Marshal(info)
			if len(objects) >= 1000 || outputBytes+len(encoded)+1 > cfg.outputBudget() {
				truncated = true
				cancel()
				break
			}
			outputBytes += len(encoded) + 1
		}
		objects = append(objects, info)
	}

	msg := fmt.Sprintf("%d object(s) in %s", len(objects), bucket)
	if prefix != "" && !cfg.managed() {
		msg += fmt.Sprintf(" (prefix: %s)", prefix)
	}
	if truncated {
		msg += "; results truncated, use a narrower allowed prefix"
	}
	return s3Encode(s3Result{Status: "success", Message: msg, Data: objects, Truncated: truncated})
}

func s3Upload(client *minio.Client, cfg S3Config, bucket, key, localPath string) string {
	if bucket == "" {
		return s3Encode(s3Result{Status: "error", Message: "bucket is required"})
	}
	if key == "" {
		return s3Encode(s3Result{Status: "error", Message: "key is required"})
	}
	ctx, cancel := context.WithTimeout(cfg.requestContext(), 5*time.Minute)
	defer cancel()
	var sourcePath string
	var err error
	if cfg.managed() {
		var source *os.File
		if normalized := normalizeS3WorkspacePath(localPath); !filepath.IsAbs(normalized) && normalized != filepath.Clean(strings.TrimSpace(localPath)) {
			localPath = filepath.Join(cfg.WorkspaceDir, normalized)
		}
		source, sourcePath, err = OpenOutgoingAttachment(localPath, cfg.Config)
		if err == nil {
			defer source.Close()
			var info os.FileInfo
			info, err = source.Stat()
			if err == nil {
				err = cfg.check()
			}
			if err == nil {
				_, err = client.PutObject(ctx, bucket, key, source, info.Size(), minio.PutObjectOptions{})
			}
		}
	} else {
		sourcePath, err = resolveS3UploadSource(cfg, localPath)
		if err == nil {
			err = cfg.check()
		}
		if err == nil {
			_, err = client.FPutObject(ctx, bucket, key, sourcePath, minio.PutObjectOptions{})
		}
	}
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("upload: %v", err)})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("uploaded %s → s3://%s/%s", filepath.Base(sourcePath), bucket, key)})
}

func s3Download(client *minio.Client, cfg S3Config, bucket, key, localPath string) string {
	if bucket == "" {
		return s3Encode(s3Result{Status: "error", Message: "bucket is required"})
	}
	if key == "" {
		return s3Encode(s3Result{Status: "error", Message: "key is required"})
	}
	destPath, err := resolveS3DownloadDestination(cfg, key, localPath)
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: err.Error()})
	}

	ctx, cancel := context.WithTimeout(cfg.requestContext(), 5*time.Minute)
	defer cancel()

	obj, err := client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("download: %v", err)})
	}
	defer obj.Close()

	info, err := obj.Stat()
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("download stat: %v", err)})
	}
	written, err := writeS3DownloadRooted(cfg, destPath, obj, info.Size)
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("write file: %v", err)})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("downloaded s3://%s/%s → %s (%d bytes)", bucket, key, destPath, written)})
}

func s3Delete(client *minio.Client, cfg S3Config, bucket, key string) string {
	if bucket == "" {
		return s3Encode(s3Result{Status: "error", Message: "bucket is required"})
	}
	if key == "" {
		return s3Encode(s3Result{Status: "error", Message: "key is required"})
	}

	ctx, cancel := context.WithTimeout(cfg.requestContext(), 30*time.Second)
	defer cancel()

	if err := cfg.check(); err != nil {
		return s3Encode(s3Result{Status: "error", Message: err.Error()})
	}
	err := client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("delete: %v", err)})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("deleted s3://%s/%s", bucket, key)})
}

func s3Copy(client *minio.Client, cfg S3Config, srcBucket, srcKey, dstBucket, dstKey string) string {
	if srcBucket == "" || srcKey == "" {
		return s3Encode(s3Result{Status: "error", Message: "source bucket and key are required"})
	}
	if dstBucket == "" || dstKey == "" {
		return s3Encode(s3Result{Status: "error", Message: "destination_bucket and destination_key are required"})
	}

	ctx, cancel := context.WithTimeout(cfg.requestContext(), 2*time.Minute)
	defer cancel()

	dst := minio.CopyDestOptions{Bucket: dstBucket, Object: dstKey}
	src := minio.CopySrcOptions{Bucket: srcBucket, Object: srcKey}
	if err := cfg.check(); err != nil {
		return s3Encode(s3Result{Status: "error", Message: err.Error()})
	}
	_, err := client.CopyObject(ctx, dst, src)
	if err != nil {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("copy: %v", err)})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("copied s3://%s/%s → s3://%s/%s", srcBucket, srcKey, dstBucket, dstKey)})
}

func s3Move(client *minio.Client, cfg S3Config, srcBucket, srcKey, dstBucket, dstKey string) string {
	// Copy then delete
	result := s3Copy(client, cfg, srcBucket, srcKey, dstBucket, dstKey)
	var r s3Result
	if err := json.Unmarshal([]byte(result), &r); err != nil || r.Status != "success" {
		return result
	}

	delResult := s3Delete(client, cfg, srcBucket, srcKey)
	var dr s3Result
	if err := json.Unmarshal([]byte(delResult), &dr); err != nil || dr.Status != "success" {
		return s3Encode(s3Result{Status: "error", Message: fmt.Sprintf("copy succeeded but delete failed: %s", dr.Message)})
	}
	return s3Encode(s3Result{Status: "success", Message: fmt.Sprintf("moved s3://%s/%s → s3://%s/%s", srcBucket, srcKey, dstBucket, dstKey)})
}
