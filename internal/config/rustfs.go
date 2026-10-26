package config

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/minio/minio-go/v7/pkg/s3utils"
)

const S3ProviderExternal = "external"
const S3ProviderRustFS = "rustfs"

// RustFSConfig is an optional managed store; credentials are exclusively Vault-owned.
type RustFSConfig struct {
	Enabled     bool              `yaml:"enabled" json:"enabled"`
	Bucket      string            `yaml:"bucket" json:"bucket"`
	AgentAccess bool              `yaml:"agent_access" json:"agent_access"`
	Permissions RustFSPermissions `yaml:"permissions" json:"permissions"`
}
type RustFSLocation struct {
	Bucket   string   `yaml:"bucket" json:"bucket"`
	Prefixes []string `yaml:"prefixes" json:"prefixes"`
}
type RustFSPermissions struct {
	AllowRead        bool             `yaml:"allow_read" json:"allow_read"`
	AllowWrite       bool             `yaml:"allow_write" json:"allow_write"`
	AllowDelete      bool             `yaml:"allow_delete" json:"allow_delete"`
	AllowedLocations []RustFSLocation `yaml:"allowed_locations" json:"allowed_locations"`
}

// RustFSS3Lease binds credentials and publication to one admitted target generation.
// Publish may only contain the short final local rename, never network/streaming work.
type RustFSS3Lease struct {
	Context     context.Context          `json:"-"`
	Managed     bool                     `json:"-"`
	Endpoint    string                   `json:"-"`
	Region      string                   `json:"-"`
	Bucket      string                   `json:"-"`
	AccessKey   string                   `json:"-"`
	SecretKey   string                   `json:"-"`
	Generation  string                   `json:"-"`
	Permissions RustFSPermissions        `json:"-"`
	Release     func()                   `json:"-"`
	Check       func() error             `json:"-"`
	Publish     func(func() error) error `json:"-"`
}

func NormalizeRustFSConfig(c *Config) error {
	if c == nil {
		return fmt.Errorf("configuration is required")
	}
	if c.S3.Provider == "" {
		c.S3.Provider = S3ProviderExternal
	}
	if c.S3.Provider != S3ProviderExternal && c.S3.Provider != S3ProviderRustFS {
		return fmt.Errorf("s3.provider must be external or rustfs")
	}
	if c.RustFS.Bucket == "" {
		c.RustFS.Bucket = "aurago"
	}
	if err := s3utils.CheckValidBucketNameStrict(c.RustFS.Bucket); err != nil {
		return fmt.Errorf("rustfs.bucket: %w", err)
	}
	if len(c.RustFS.Permissions.AllowedLocations) > 64 {
		return fmt.Errorf("rustfs allows at most 64 bucket locations")
	}
	seen := map[string]bool{}
	for _, location := range c.RustFS.Permissions.AllowedLocations {
		if err := s3utils.CheckValidBucketNameStrict(location.Bucket); err != nil {
			return fmt.Errorf("rustfs allowed bucket: %w", err)
		}
		if seen[location.Bucket] {
			return fmt.Errorf("rustfs bucket locations must be unique")
		}
		seen[location.Bucket] = true
		if len(location.Prefixes) > 64 {
			return fmt.Errorf("rustfs allows at most 64 prefixes per bucket")
		}
		for _, prefix := range location.Prefixes {
			if len(prefix) > 1024 || !utf8.ValidString(prefix) || strings.ContainsAny(prefix, "*?") || strings.Contains(prefix, "${") {
				return fmt.Errorf("rustfs prefixes must be literal UTF-8 object prefixes without IAM wildcards or variables")
			}
			for _, r := range prefix {
				if r < 32 || r == 127 {
					return fmt.Errorf("rustfs prefixes must not contain control characters")
				}
			}
		}
	}
	return nil
}

// Allows treats object keys as opaque strings, never filesystem paths.
func (p RustFSPermissions) Allows(bucket, key, access string) bool {
	switch access {
	case "read":
		if !p.AllowRead {
			return false
		}
	case "write":
		if !p.AllowWrite {
			return false
		}
	case "delete":
		if !p.AllowDelete {
			return false
		}
	default:
		return false
	}
	for _, location := range p.AllowedLocations {
		if location.Bucket != bucket {
			continue
		}
		for _, prefix := range location.Prefixes {
			if strings.HasPrefix(key, prefix) {
				return true
			}
		}
	}
	return false
}

// ListingPrefix never widens an omitted prefix into an unrestricted bucket scan.
func (p RustFSPermissions) ListingPrefix(bucket, prefix string) (string, error) {
	if !p.AllowRead {
		return "", fmt.Errorf("rustfs read permission is required")
	}
	for _, location := range p.AllowedLocations {
		if location.Bucket != bucket {
			continue
		}
		if prefix != "" && p.Allows(bucket, prefix, "read") {
			return prefix, nil
		}
		if prefix == "" {
			for _, allowed := range location.Prefixes {
				if allowed == "" {
					return "", nil
				}
			}
			if len(location.Prefixes) == 1 {
				return location.Prefixes[0], nil
			}
		}
	}
	return "", fmt.Errorf("select an allowed rustfs bucket and object prefix")
}
