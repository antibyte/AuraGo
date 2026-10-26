package rustfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/dockerutil"

	"aurago/internal/security"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const maxAdminBody = 2 << 20

type adminClient struct {
	endpoint, access, secret string
	http                     *http.Client
}

func newAdminClient(endpoint, access, secret string, base *http.Client) (*adminClient, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" && parsed.Path != "/" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, failure("invalid_endpoint", nil)
	}
	security.RegisterSensitive(access)
	security.RegisterSensitive(secret)
	if base == nil {
		base = &http.Client{Timeout: 20 * time.Second}
	}
	client := *base
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &adminClient{strings.TrimRight(endpoint, "/"), access, secret, &client}, nil
}

// Native RustFS admin routes accept SigV4 JSON; MinIO's encrypted admin prefix is deliberately unused.
func (a *adminClient) call(ctx context.Context, method, path string, query url.Values, payload, result any) (int, error) {
	var data []byte
	var err error
	if payload != nil {
		data, err = json.Marshal(payload)
		if err != nil {
			return 0, failure("invalid_admin_request", err)
		}
	}
	address := a.endpoint + path
	if len(query) > 0 {
		address += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, address, bytes.NewReader(data))
	if err != nil {
		return 0, failure("invalid_admin_request", nil)
	}
	req.Header.Set("Content-Type", "application/json")
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	req.Header.Set("X-Amz-Content-Sha256", hash)
	if err = v4.NewSigner().SignHTTP(ctx, aws.Credentials{AccessKeyID: a.access, SecretAccessKey: a.secret}, req, hash, "s3", Region, time.Now()); err != nil {
		return 0, failure("admin_sign_failed", nil)
	}
	response, err := a.http.Do(req)
	if err != nil {
		return 0, failure("admin_unavailable", err)
	}
	defer response.Body.Close()
	data, err = io.ReadAll(io.LimitReader(response.Body, maxAdminBody+1))
	if err != nil || len(data) > maxAdminBody {
		return response.StatusCode, failure("admin_response_failed", nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var rejection struct{ Code, Message string }
		_ = xml.Unmarshal(data, &rejection)
		detail := dockerutil.SanitizeOneLine(security.Scrub(rejection.Code+": "+rejection.Message), 256)
		return response.StatusCode, failure("admin_rejected", fmt.Errorf("%s HTTP %d (%s)", path, response.StatusCode, detail))
	}
	if result != nil && len(data) > 0 {
		// Register credentials before decoding errors or diagnostics can expose a provider response.
		var envelope struct {
			Credentials struct {
				Access string `json:"accessKey"`
				Secret string `json:"secretKey"`
			} `json:"credentials"`
		}
		if json.Unmarshal(data, &envelope) == nil {
			security.RegisterSensitive(envelope.Credentials.Access)
			security.RegisterSensitive(envelope.Credentials.Secret)
		}
		if err = json.Unmarshal(data, result); err != nil {
			return response.StatusCode, failure("admin_response_failed", nil)
		}
	}
	return response.StatusCode, nil
}
func (a *adminClient) native(ctx context.Context, method, name string, q url.Values, body, result any) (int, error) {
	return a.call(ctx, method, "/rustfs/admin/v3/"+name, q, body, result)
}
func newDataClient(endpoint, access, secret string, base *http.Client) (*minio.Client, error) {
	security.RegisterSensitive(access)
	security.RegisterSensitive(secret)
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, failure("invalid_endpoint", nil)
	}
	var transport http.RoundTripper
	if base != nil {
		transport = base.Transport
	}
	return minio.New(u.Host, &minio.Options{Creds: credentials.NewStaticV4(access, secret, ""), Secure: u.Scheme == "https", Region: Region, BucketLookup: minio.BucketLookupPath, MaxRetries: 1, Transport: transport})
}
