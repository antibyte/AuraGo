package tools

import (
	"aurago/internal/config"
	"context"
	"errors"
	"fmt"
	"github.com/minio/minio-go/v7"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/security"
)

const integrationCheckBodyLimit int64 = 1 << 20

func validateIntegrationHTTPURL(rawURL string, name string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s URL is invalid", name)
	}
	return nil
}

func drainIntegrationResponse(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, integrationCheckBodyLimit))
}

// CheckHomeAssistantConnection calls the documented read-only /api/ endpoint.
func CheckHomeAssistantConnection(ctx context.Context, cfg HAConfig) error {
	if strings.TrimSpace(cfg.URL) == "" {
		return errors.New("Home Assistant URL is not configured")
	}
	if strings.TrimSpace(cfg.AccessToken) == "" {
		return errors.New("Home Assistant access token is not configured")
	}
	if err := validateIntegrationHTTPURL(cfg.URL, "Home Assistant"); err != nil {
		return err
	}
	security.RegisterSensitive(cfg.AccessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(cfg.URL, "/")+"/api/", nil)
	if err != nil {
		return errors.New("could not create the Home Assistant API request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.AccessToken)

	resp, err := haHTTPClient.Do(req)
	if err != nil {
		return errors.New("Home Assistant API request failed")
	}
	defer resp.Body.Close()
	drainIntegrationResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Home Assistant API returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// CheckProxmoxConnection performs a read-only request against /version.
func CheckProxmoxConnection(ctx context.Context, cfg ProxmoxConfig) error {
	if strings.TrimSpace(cfg.URL) == "" {
		return errors.New("Proxmox URL is not configured")
	}
	if strings.TrimSpace(cfg.TokenID) == "" || strings.TrimSpace(cfg.Secret) == "" {
		return errors.New("Proxmox API credentials are not configured")
	}
	baseURL, err := proxmoxBaseURL(cfg.URL)
	if err != nil {
		return errors.New("Proxmox URL is invalid; HTTPS is required")
	}
	cfg.URL = baseURL
	security.RegisterSensitive(cfg.TokenID)
	security.RegisterSensitive(cfg.Secret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api2/json/version", nil)
	if err != nil {
		return errors.New("could not create the Proxmox API request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "PVEAPIToken="+cfg.TokenID+"="+cfg.Secret)

	resp, err := getProxmoxClient(cfg).Do(req)
	if err != nil {
		return errors.New("Proxmox API request failed")
	}
	defer resp.Body.Close()
	drainIntegrationResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Proxmox API returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// CheckS3Connection checks only the configured bucket. It does not list,
// upload, delete, or modify any object.
func CheckS3Connection(ctx context.Context, cfg S3Config) error {
	if c := cfg.Config; c != nil && s3Provider(c) == config.S3ProviderRustFS {
		readable := false
		for _, location := range c.RustFS.Permissions.AllowedLocations {
			readable = readable || len(location.Prefixes) > 0
		}
		if !c.RustFS.AgentAccess || !c.RustFS.Permissions.AllowRead || !readable {
			return ErrS3ReadVerificationUnavailable
		}
	}
	resolved, release, err := acquireS3Config(ctx, cfg, false)
	if err != nil {
		return err
	}
	defer release()
	cfg = resolved
	bucket, prefix := cfg.Bucket, ""
	if cfg.managed() {
		found := false
		for _, location := range cfg.Config.RustFS.Permissions.AllowedLocations {
			for _, candidate := range location.Prefixes {
				if cfg.allows(location.Bucket, candidate, "read") {
					bucket, prefix, found = location.Bucket, candidate, true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return ErrS3ReadVerificationUnavailable
		}
	}
	if strings.TrimSpace(bucket) == "" {
		return errors.New("S3 bucket is not configured")
	}
	if strings.TrimSpace(cfg.AccessKey) == "" || strings.TrimSpace(cfg.SecretKey) == "" {
		return errors.New("S3 credentials are not configured")
	}
	client, err := newS3Client(cfg)
	if err != nil {
		return fmt.Errorf("S3 client initialization failed: %w", err)
	}
	if cfg.managed() {
		probeCtx, cancel := context.WithCancel(cfg.requestContext())
		defer cancel()
		for object := range client.ListObjects(probeCtx, bucket, minio.ListObjectsOptions{Prefix: prefix, MaxKeys: 1}) {
			if object.Err != nil {
				return errors.New("S3 permitted prefix is not accessible")
			}
			break
		}
		return cfg.check()
	}
	exists, err := client.BucketExists(cfg.requestContext(), bucket)
	if err != nil || !exists {
		return errors.New("S3 bucket is not accessible")
	}
	return cfg.check()
}

// CheckAnsibleConnection calls only the sidecar status endpoint. It never
// starts an Ansible ping, playbook, or ad-hoc command.
func CheckAnsibleConnection(ctx context.Context, cfg AnsibleConfig) error {
	if strings.TrimSpace(cfg.URL) == "" {
		return errors.New("Ansible sidecar URL is not configured")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return errors.New("Ansible sidecar token is not configured")
	}
	if err := validateIntegrationHTTPURL(cfg.URL, "Ansible sidecar"); err != nil {
		return err
	}
	security.RegisterSensitive(cfg.Token)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(cfg.URL, "/")+"/status", nil)
	if err != nil {
		return errors.New("could not create the Ansible sidecar request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("Ansible sidecar request failed")
	}
	defer resp.Body.Close()
	drainIntegrationResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ansible sidecar returned HTTP %d", resp.StatusCode)
	}
	return nil
}
