package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"aurago/internal/config"
)

var ErrS3ReadVerificationUnavailable = errors.New("S3 data-read verification is unavailable for the configured permissions")

// S3ConfigFromConfig keeps provider selection and private admission attached to the caller snapshot.
func S3ConfigFromConfig(c *config.Config) S3Config {
	if c == nil {
		return S3Config{}
	}
	maxMB := c.VirtualDesktop.MaxFileSizeMB
	if maxMB <= 0 {
		maxMB = 50
	}
	return S3Config{
		Config: c, Endpoint: c.S3.Endpoint, Region: c.S3.Region, Bucket: c.S3.Bucket,
		AccessKey: c.S3.AccessKey, SecretKey: c.S3.SecretKey, UsePathStyle: c.S3.UsePathStyle,
		Insecure: c.S3.Insecure, ReadOnly: c.S3.ReadOnly, WorkspaceDir: c.Directories.WorkspaceDir,
		DataDir: c.Directories.DataDir, AllowFilesystemWrite: c.Agent.AllowFilesystemWrite,
		MaxDownloadBytes: int64(maxMB) << 20, MaxOutputBytes: c.Agent.ToolOutputLimit,
	}
}

func s3Provider(c *config.Config) string {
	if c == nil || c.S3.Provider == "" {
		return config.S3ProviderExternal
	}
	return c.S3.Provider
}

// S3AgentEnabled is config-only; runtime readiness is checked separately without network I/O.
func S3AgentEnabled(c *config.Config) bool {
	if c == nil || !c.S3.Enabled {
		return false
	}
	switch s3Provider(c) {
	case config.S3ProviderExternal:
		return true
	case config.S3ProviderRustFS:
		return c.RustFS.Enabled && c.RustFS.AgentAccess
	default:
		return false
	}
}

func S3RuntimeAvailability(c *config.Config) config.FeatureAvailability {
	if c == nil {
		return config.FeatureAvailability{Reason: "S3 configuration is unavailable"}
	}
	if s3Provider(c) == config.S3ProviderExternal {
		return config.FeatureAvailability{Available: true}
	}
	if s3Provider(c) != config.S3ProviderRustFS {
		return config.FeatureAvailability{Reason: "Unknown S3 provider"}
	}
	if c.AcquireS3 == nil || c.S3Availability == nil {
		return config.FeatureAvailability{Reason: "Managed RustFS is not initialized"}
	}
	return c.S3Availability(c)
}

func acquireS3Config(ctx context.Context, cfg S3Config, agent bool) (S3Config, func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg.ctx = ctx
	release := func() {}
	if err := ctx.Err(); err != nil {
		return cfg, release, err
	}
	c := cfg.Config
	if c == nil {
		return cfg, release, nil
	}
	if !c.S3.Enabled {
		return cfg, release, fmt.Errorf("S3 integration is disabled")
	}
	cfg.ReadOnly = cfg.ReadOnly || c.S3.ReadOnly
	cfg.AllowFilesystemWrite = cfg.AllowFilesystemWrite && c.Agent.AllowFilesystemWrite
	provider := s3Provider(c)
	if provider != config.S3ProviderExternal && provider != config.S3ProviderRustFS {
		return cfg, release, fmt.Errorf("unknown S3 provider")
	}
	if provider == config.S3ProviderRustFS && (!c.RustFS.Enabled || agent && !c.RustFS.AgentAccess) {
		return cfg, release, fmt.Errorf("managed RustFS agent access is disabled")
	}
	if c.AcquireS3 == nil {
		if provider == config.S3ProviderRustFS {
			return cfg, release, fmt.Errorf("managed RustFS is not initialized")
		}
		identity := s3ExternalIdentity(c)
		cfg.lease = &config.RustFSS3Lease{Context: ctx, Check: func() error {
			if c.AuthorizationSnapshots != nil {
				_, current := c.AuthorizationSnapshots()
				if current == nil || s3ExternalIdentity(current) != identity {
					return fmt.Errorf("S3 authorization changed; start a new request")
				}
			}
			return ctx.Err()
		}}
		return cfg, release, cfg.check()
	}
	lease, err := c.AcquireS3(ctx, c, agent)
	if err != nil {
		return cfg, release, err
	}
	if lease != nil && lease.Release != nil {
		release = lease.Release
	}
	if lease == nil || lease.Context == nil || lease.Check == nil || lease.Publish == nil || lease.Release == nil || lease.Managed != (provider == config.S3ProviderRustFS) {
		release()
		return cfg, func() {}, fmt.Errorf("S3 runtime did not return a valid scoped target")
	}
	cfg.ctx, cfg.lease = lease.Context, lease
	cfg.Endpoint, cfg.Region, cfg.Bucket = lease.Endpoint, lease.Region, lease.Bucket
	cfg.AccessKey, cfg.SecretKey = lease.AccessKey, lease.SecretKey
	if lease.Managed {
		cfg.UsePathStyle, cfg.Insecure = true, true
		if cfg.Endpoint == "" || cfg.Region == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
			release()
			return cfg, func() {}, fmt.Errorf("managed RustFS target is incomplete")
		}
	}
	if err := cfg.check(); err != nil {
		release()
		return cfg, func() {}, err
	}
	return cfg, release, nil
}

func s3ExternalIdentity(c *config.Config) [32]byte {
	// Credentials enter a digest only; no identity string contains secret material.
	target := c.S3
	target.Provider = s3Provider(c)
	value, _ := json.Marshal([]any{target, c.Agent.AllowFilesystemWrite})
	return sha256.Sum256(value)
}

func (cfg S3Config) managed() bool { return cfg.lease != nil && cfg.lease.Managed }
func (cfg S3Config) requestContext() context.Context {
	if cfg.ctx != nil {
		return cfg.ctx
	}
	return context.Background()
}
func (cfg S3Config) check() error {
	if err := cfg.requestContext().Err(); err != nil {
		return err
	}
	if cfg.lease != nil && cfg.lease.Check != nil {
		return cfg.lease.Check()
	}
	return nil
}
func (cfg S3Config) publish(commit func() error) error {
	if err := cfg.check(); err != nil {
		return err
	}
	if cfg.lease != nil && cfg.lease.Publish != nil {
		return cfg.lease.Publish(commit)
	}
	return commit()
}
func (cfg S3Config) outputBudget() int {
	limit := cfg.MaxOutputBytes
	if limit <= 0 {
		limit = 50000
	}
	// Reserve the result envelope, notice, and caller's tool prefix.
	return max(0, limit-512)
}
func s3Mutation(op string) bool {
	return op == "upload" || op == "copy" || op == "move" || op == "delete"
}
func (cfg S3Config) allows(bucket, key, access string) bool {
	return cfg.Config != nil && cfg.Config.RustFS.Permissions.Allows(bucket, key, access) && cfg.lease.Permissions.Allows(bucket, key, access)
}

func authorizeS3Operation(cfg S3Config, op, bucket, key, prefix, destBucket, destKey string) (string, error) {
	if err := cfg.check(); err != nil {
		return prefix, err
	}
	if !cfg.managed() {
		return prefix, nil
	}
	denied := func() (string, error) {
		return prefix, fmt.Errorf("managed RustFS permission does not allow this operation and bucket/key")
	}
	switch op {
	case "list_buckets":
		if !cfg.Config.RustFS.Permissions.AllowRead || !cfg.lease.Permissions.AllowRead {
			return denied()
		}
	case "list_objects":
		selected, err := cfg.Config.RustFS.Permissions.ListingPrefix(bucket, prefix)
		if err != nil {
			return prefix, err
		}
		return cfg.lease.Permissions.ListingPrefix(bucket, selected)
	case "download":
		if !cfg.allows(bucket, key, "read") {
			return denied()
		}
	case "upload":
		if !cfg.allows(bucket, key, "write") {
			return denied()
		}
	case "delete":
		if !cfg.allows(bucket, key, "delete") {
			return denied()
		}
	case "copy", "move":
		if !cfg.allows(bucket, key, "read") || !cfg.allows(destBucket, destKey, "write") || op == "move" && !cfg.allows(bucket, key, "delete") {
			return denied()
		}
	default:
		return denied()
	}
	return prefix, nil
}

func s3ManagedBuckets(cfg S3Config) string {
	names := map[string]bool{}
	for _, location := range cfg.Config.RustFS.Permissions.AllowedLocations {
		for _, prefix := range location.Prefixes {
			for _, leased := range cfg.lease.Permissions.AllowedLocations {
				if location.Bucket != leased.Bucket {
					continue
				}
				for _, allowed := range leased.Prefixes {
					if cfg.allows(location.Bucket, prefix, "read") || cfg.allows(location.Bucket, allowed, "read") {
						names[location.Bucket] = true
					}
				}
			}
		}
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	buckets := make([]s3BucketInfo, 0, len(sorted))
	size, truncated := 0, false
	for _, name := range sorted {
		item := s3BucketInfo{Name: name}
		encoded, _ := json.Marshal(item)
		if len(buckets) >= 1000 || size+len(encoded)+1 > cfg.outputBudget() {
			truncated = true
			break
		}
		size += len(encoded) + 1
		buckets = append(buckets, item)
	}
	return s3Encode(s3Result{Status: "success", Data: buckets, Truncated: truncated, Message: fmt.Sprintf("%d permitted bucket(s)", len(buckets))})
}
