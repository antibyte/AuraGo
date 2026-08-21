package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/security"
)

const (
	hereNowBaseURL          = "https://here.now"
	hereNowClientHeader     = "aurago/native"
	hereNowMaxFiles         = 1000
	hereNowUploadWorkers    = 4
	hereNowMaxResponseBytes = 8 * 1024 * 1024
	hereNowMaxAttempts      = 4 // initial request plus at most three retries
	hereNowNoAccount        = "\x00"
)

// HereNowConfig contains the fixed here.now connection and AuraGo permission gates.
type HereNowConfig struct {
	APIKey                string
	DefaultAccount        string
	ReadOnly              bool
	AllowPublish          bool
	AllowSiteManagement   bool
	AllowAccessManagement bool
	AllowDelete           bool
}

// HereNowAPIError preserves the stable provider error contract without leaking credentials.
type HereNowAPIError struct {
	StatusCode    int             `json:"http_code,omitempty"`
	ProviderError string          `json:"error,omitempty"`
	Code          string          `json:"code,omitempty"`
	Message       string          `json:"message"`
	Details       json.RawMessage `json:"details,omitempty"`
	RetryAfter    string          `json:"retry_after,omitempty"`
	DocsURL       string          `json:"docs_url,omitempty"`
}

// HereNowOutcomeUnknownError reports that a mutation may have reached the
// provider, but AuraGo could not determine its result. RetrySafe is true only
// for provider operations whose contract explicitly permits replay.
type HereNowOutcomeUnknownError struct {
	Operation string
	RetrySafe bool
	Cause     error
}

func (e *HereNowOutcomeUnknownError) Error() string {
	if e == nil {
		return "here.now mutation outcome is unknown"
	}
	return fmt.Sprintf("here.now %s outcome is unknown", e.Operation)
}

func (e *HereNowOutcomeUnknownError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type hereNowRequestPolicy struct {
	operation string
	retry     bool
	mutation  bool
	retrySafe bool
}

func hereNowReadPolicy(operation string) hereNowRequestPolicy {
	return hereNowRequestPolicy{operation: operation, retry: true, retrySafe: true}
}

func hereNowMutationPolicy(operation string) hereNowRequestPolicy {
	return hereNowRequestPolicy{operation: operation, mutation: true}
}

func hereNowFinalizePolicy() hereNowRequestPolicy {
	return hereNowRequestPolicy{operation: "finalize publish", retry: true, mutation: true, retrySafe: true}
}

func (e *HereNowAPIError) Error() string {
	if e == nil {
		return "here.now request failed"
	}
	if e.Code != "" {
		return fmt.Sprintf("here.now %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("here.now HTTP %d: %s", e.StatusCode, e.Message)
}

// HereNowClient is a native client for the authenticated here.now v1 API.
// Production callers always use the fixed public origin through NewHereNowClient.
type HereNowClient struct {
	baseURL        string
	apiKey         string
	defaultAccount string
	httpClient     *http.Client
	uploadClient   func(string) (*http.Client, error)
	now            func() time.Time
	sleep          func(context.Context, time.Duration) error
}

// NewHereNowClient creates a client pinned to the fixed here.now API origin.
func NewHereNowClient(apiKey, defaultAccount string) (*HereNowClient, error) {
	return newHereNowClient(hereNowBaseURL, apiKey, defaultAccount, nil)
}

func newHereNowClient(baseURL, apiKey, defaultAccount string, client *http.Client) (*HereNowClient, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("here.now API key is required")
	}
	security.RegisterSensitive(apiKey)
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("invalid here.now API origin")
	}
	if client == nil {
		client, err = security.NewStrictPublicHTTPClientForURL(baseURL, 90*time.Second)
		if err != nil {
			return nil, fmt.Errorf("create here.now HTTP client: %w", err)
		}
	}
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &HereNowClient{
		baseURL:        baseURL,
		apiKey:         apiKey,
		defaultAccount: strings.TrimSpace(defaultAccount),
		httpClient:     client,
		uploadClient: func(rawURL string) (*http.Client, error) {
			c, err := security.NewStrictPublicHTTPClientForURL(rawURL, 30*time.Minute)
			if err == nil {
				c.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
			}
			return c, err
		},
		now: time.Now,
		sleep: func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		},
	}, nil
}

func (c *HereNowClient) account(account string) string {
	if account == hereNowNoAccount {
		return ""
	}
	if strings.TrimSpace(account) != "" {
		return strings.TrimSpace(account)
	}
	return c.defaultAccount
}

func (c *HereNowClient) request(ctx context.Context, method, endpoint, account string, body interface{}, policy hereNowRequestPolicy) (json.RawMessage, error) {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode here.now request: %w", err)
		}
	}

	var lastErr error
	for attempt := 1; attempt <= hereNowMaxAttempts; attempt++ {
		var reader io.Reader
		if encoded != nil {
			reader = bytes.NewReader(encoded)
		}
		req, reqErr := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, reader)
		if reqErr != nil {
			return nil, fmt.Errorf("create here.now request: %w", reqErr)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-HereNow-Client", hereNowClientHeader)
		if encoded != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if selected := c.account(account); selected != "" {
			req.Header.Set("X-HereNow-Account", selected)
		}

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			lastErr = fmt.Errorf("here.now request failed")
			if policy.retry && attempt < hereNowMaxAttempts {
				if sleepErr := c.sleep(ctx, time.Duration(attempt)*250*time.Millisecond); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			if policy.mutation {
				return nil, &HereNowOutcomeUnknownError{Operation: policy.operation, RetrySafe: policy.retrySafe, Cause: lastErr}
			}
			return nil, lastErr
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, hereNowMaxResponseBytes+1))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read here.now response: %w", readErr)
			if policy.retry && attempt < hereNowMaxAttempts {
				if sleepErr := c.sleep(ctx, time.Duration(attempt)*250*time.Millisecond); sleepErr != nil {
					return nil, sleepErr
				}
				continue
			}
			if policy.mutation {
				return nil, &HereNowOutcomeUnknownError{Operation: policy.operation, RetrySafe: policy.retrySafe, Cause: lastErr}
			}
			return nil, lastErr
		}
		if len(data) > hereNowMaxResponseBytes {
			err := fmt.Errorf("here.now response exceeds %d bytes", hereNowMaxResponseBytes)
			if policy.mutation {
				return nil, &HereNowOutcomeUnknownError{Operation: policy.operation, RetrySafe: policy.retrySafe, Cause: err}
			}
			return nil, err
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if len(data) == 0 {
				return json.RawMessage(`{}`), nil
			}
			if !json.Valid(data) {
				err := fmt.Errorf("here.now returned invalid JSON")
				if policy.mutation {
					return nil, &HereNowOutcomeUnknownError{Operation: policy.operation, RetrySafe: policy.retrySafe, Cause: err}
				}
				return nil, err
			}
			return json.RawMessage(data), nil
		}

		apiErr := parseHereNowAPIError(resp.StatusCode, data, resp.Header.Get("Retry-After"))
		lastErr = apiErr
		if policy.retry && attempt < hereNowMaxAttempts && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) {
			if sleepErr := c.sleep(ctx, hereNowRetryDelay(apiErr, attempt, c.now())); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}
		if policy.mutation && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) {
			return nil, &HereNowOutcomeUnknownError{Operation: policy.operation, RetrySafe: policy.retrySafe, Cause: apiErr}
		}
		return nil, apiErr
	}
	return nil, lastErr
}

func parseHereNowAPIError(status int, data []byte, retryHeader string) *HereNowAPIError {
	provider := struct {
		Error      string          `json:"error"`
		Code       string          `json:"code"`
		Message    string          `json:"message"`
		Details    json.RawMessage `json:"details"`
		RetryAfter interface{}     `json:"retry_after"`
		DocsURL    string          `json:"docs_url"`
	}{}
	_ = json.Unmarshal(data, &provider)
	message := strings.TrimSpace(provider.Message)
	if message == "" {
		message = strings.TrimSpace(provider.Error)
	}
	if message == "" {
		message = http.StatusText(status)
	}
	retryAfter := strings.TrimSpace(retryHeader)
	if retryAfter == "" && provider.RetryAfter != nil {
		retryAfter = strings.TrimSpace(fmt.Sprint(provider.RetryAfter))
	}
	return &HereNowAPIError{
		StatusCode: status, ProviderError: provider.Error, Code: provider.Code, Message: message,
		Details: provider.Details, RetryAfter: retryAfter, DocsURL: provider.DocsURL,
	}
}

func hereNowRetryDelay(apiErr *HereNowAPIError, attempt int, now time.Time) time.Duration {
	if apiErr != nil && apiErr.RetryAfter != "" {
		if seconds, err := strconv.Atoi(apiErr.RetryAfter); err == nil && seconds >= 0 {
			return time.Duration(seconds) * time.Second
		}
		if retryAt, err := http.ParseTime(apiErr.RetryAfter); err == nil && retryAt.After(now) {
			return retryAt.Sub(now)
		}
	}
	return time.Duration(attempt) * 500 * time.Millisecond
}

func hereNowPath(value string) string { return url.PathEscape(strings.TrimSpace(value)) }

// Read-only API operations return the provider JSON unchanged.
func (c *HereNowClient) ListAccounts(ctx context.Context) (json.RawMessage, error) {
	return c.request(ctx, http.MethodGet, "/api/v1/accounts", hereNowNoAccount, nil, hereNowReadPolicy("list accounts"))
}

func (c *HereNowClient) ListSites(ctx context.Context, account, cursor string, limit int, all bool) (json.RawMessage, error) {
	q := url.Values{}
	if all {
		q.Set("scope", "all")
		account = hereNowNoAccount
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		if limit > 0 {
			q.Set("limit", strconv.Itoa(limit))
		}
	}
	endpoint := "/api/v1/publishes"
	if encoded := q.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	return c.request(ctx, http.MethodGet, endpoint, account, nil, hereNowReadPolicy("list Sites"))
}

func (c *HereNowClient) SearchSites(ctx context.Context, account, query, cursor string, limit int) (json.RawMessage, error) {
	q := url.Values{"q": []string{strings.TrimSpace(query)}}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return c.request(ctx, http.MethodGet, "/api/v1/publishes/search?"+q.Encode(), account, nil, hereNowReadPolicy("search Sites"))
}

func (c *HereNowClient) GetSite(ctx context.Context, account, slug string) (json.RawMessage, error) {
	return c.request(ctx, http.MethodGet, "/api/v1/publish/"+hereNowPath(slug), account, nil, hereNowReadPolicy("get Site"))
}

func (c *HereNowClient) GetAccess(ctx context.Context, account, slug string) (json.RawMessage, error) {
	return c.request(ctx, http.MethodGet, "/api/v1/publish/"+hereNowPath(slug)+"/access", account, nil, hereNowReadPolicy("get access"))
}

func (c *HereNowClient) ListVersions(ctx context.Context, account, slug string) (json.RawMessage, error) {
	return c.request(ctx, http.MethodGet, "/api/v1/publish/"+hereNowPath(slug)+"/versions", account, nil, hereNowReadPolicy("list versions"))
}

func (c *HereNowClient) DuplicateSite(ctx context.Context, account, slug string, viewer map[string]interface{}) (json.RawMessage, error) {
	selection, err := c.resolveAccountSelection(ctx, account)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{}
	if len(viewer) > 0 {
		body["viewer"] = viewer
	}
	raw, err := c.request(ctx, http.MethodPost, "/api/v1/publish/"+hereNowPath(slug)+"/duplicate", selection.RequestAccount, body, hereNowMutationPolicy("duplicate Site"))
	if err != nil {
		return nil, err
	}
	var duplicated struct {
		Slug             string  `json:"slug"`
		SiteURL          string  `json:"siteUrl"`
		Status           string  `json:"status"`
		CurrentVersionID *string `json:"currentVersionId"`
	}
	if err := json.Unmarshal(raw, &duplicated); err != nil {
		return nil, fmt.Errorf("decode here.now duplicate response: %w", err)
	}
	if duplicated.Slug == "" || duplicated.SiteURL == "" || duplicated.Status != "active" || duplicated.CurrentVersionID == nil || *duplicated.CurrentVersionID == "" {
		return nil, fmt.Errorf("here.now returned an incomplete duplicate response")
	}
	detailsRaw, err := c.GetSite(ctx, selection.RequestAccount, duplicated.Slug)
	if err != nil {
		return nil, fmt.Errorf("verify duplicated here.now Site details: %w", err)
	}
	var details struct {
		Status           string  `json:"status"`
		ExpiresAt        *string `json:"expiresAt"`
		CurrentVersionID *string `json:"currentVersionId"`
		PendingVersionID *string `json:"pendingVersionId"`
	}
	if err := json.Unmarshal(detailsRaw, &details); err != nil {
		return nil, fmt.Errorf("decode duplicated here.now Site details: %w", err)
	}
	if details.Status != "active" || details.ExpiresAt != nil || details.PendingVersionID != nil || details.CurrentVersionID == nil || *details.CurrentVersionID != *duplicated.CurrentVersionID {
		return nil, fmt.Errorf("here.now duplicate is not a permanent live Site")
	}
	verified, err := c.VerifySiteURL(ctx, duplicated.SiteURL)
	if err != nil {
		return nil, fmt.Errorf("verify duplicated here.now Site: %w", err)
	}
	var output map[string]interface{}
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, err
	}
	output["verified"] = verified
	output["verifiedUrl"] = duplicated.SiteURL
	output["account"] = selection.AccountID
	return json.Marshal(output)
}

func (c *HereNowClient) PatchMetadata(ctx context.Context, account, slug string, patch map[string]interface{}) (json.RawMessage, error) {
	return c.request(ctx, http.MethodPatch, "/api/v1/publish/"+hereNowPath(slug)+"/metadata", account, patch, hereNowMutationPolicy("update Site metadata"))
}

// SetPassword changes a Site to password mode and verifies the resulting
// access policy. The password is supplied only by trusted Vault-backed code.
func (c *HereNowClient) SetPassword(ctx context.Context, account, slug, password string) (json.RawMessage, error) {
	password = strings.TrimSpace(password)
	if password == "" {
		return nil, fmt.Errorf("here.now Site password is required")
	}
	security.RegisterSensitive(password)
	currentRaw, err := c.GetAccess(ctx, account, slug)
	if err != nil {
		return nil, err
	}
	current, err := ParseHereNowAccessPolicy(currentRaw)
	if err != nil {
		return nil, err
	}
	if current.Mode == "restricted" {
		return nil, fmt.Errorf("disable restricted access explicitly before setting a Site password")
	}
	raw, err := c.PatchMetadata(ctx, account, slug, map[string]interface{}{"password": password})
	if err != nil {
		return nil, err
	}
	verifiedRaw, err := c.GetAccess(ctx, account, slug)
	if err != nil {
		return nil, &HereNowOutcomeUnknownError{Operation: "set Site password", Cause: err}
	}
	verified, err := ParseHereNowAccessPolicy(verifiedRaw)
	if err != nil {
		return nil, &HereNowOutcomeUnknownError{Operation: "set Site password", Cause: err}
	}
	if verified.Mode != "password" {
		return nil, &HereNowOutcomeUnknownError{Operation: "set Site password", Cause: fmt.Errorf("provider did not confirm password mode")}
	}
	return raw, nil
}

// RemovePassword changes a password-protected Site back to link access and
// verifies the resulting policy before the caller removes its Vault secret.
func (c *HereNowClient) RemovePassword(ctx context.Context, account, slug string) (json.RawMessage, error) {
	currentRaw, err := c.GetAccess(ctx, account, slug)
	if err != nil {
		return nil, err
	}
	current, err := ParseHereNowAccessPolicy(currentRaw)
	if err != nil {
		return nil, err
	}
	if current.Mode != "password" {
		return nil, fmt.Errorf("the here.now Site is not in password mode; no access mode was changed")
	}
	raw, err := c.PatchMetadata(ctx, account, slug, map[string]interface{}{"password": nil})
	if err != nil {
		return nil, err
	}
	verifiedRaw, err := c.GetAccess(ctx, account, slug)
	if err != nil {
		return nil, &HereNowOutcomeUnknownError{Operation: "remove Site password", Cause: err}
	}
	verified, err := ParseHereNowAccessPolicy(verifiedRaw)
	if err != nil {
		return nil, &HereNowOutcomeUnknownError{Operation: "remove Site password", Cause: err}
	}
	if verified.Mode != "anyone_with_link" {
		return nil, &HereNowOutcomeUnknownError{Operation: "remove Site password", Cause: fmt.Errorf("provider did not confirm anyone_with_link mode")}
	}
	return raw, nil
}

// HereNowAccessPolicy is the complete provider access snapshot required before
// replacing an allowlist or changing password protection.
type HereNowAccessPolicy struct {
	Mode           string
	PolicyVersion  int
	AllowedEmails  []string
	AllowedDomains []string
}

// ParseHereNowAccessPolicy rejects incomplete provider envelopes. In
// particular, missing arrays must not be interpreted as empty replacement
// allowlists.
func ParseHereNowAccessPolicy(raw json.RawMessage) (HereNowAccessPolicy, error) {
	type wirePolicy struct {
		Mode           string    `json:"mode"`
		PolicyVersion  *int      `json:"accessPolicyVersion"`
		AllowedEmails  *[]string `json:"allowedEmails"`
		AllowedDomains *[]string `json:"allowedDomains"`
	}
	var parsed struct {
		Mode           string      `json:"mode"`
		PolicyVersion  *int        `json:"accessPolicyVersion"`
		AllowedEmails  *[]string   `json:"allowedEmails"`
		AllowedDomains *[]string   `json:"allowedDomains"`
		Access         *wirePolicy `json:"access"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return HereNowAccessPolicy{}, fmt.Errorf("decode here.now access policy: %w", err)
	}
	selected := wirePolicy{
		Mode: parsed.Mode, PolicyVersion: parsed.PolicyVersion,
		AllowedEmails: parsed.AllowedEmails, AllowedDomains: parsed.AllowedDomains,
	}
	if parsed.Access != nil {
		selected = *parsed.Access
	}
	mode := strings.TrimSpace(selected.Mode)
	switch mode {
	case "anyone_with_link", "restricted", "account_members", "password":
	default:
		return HereNowAccessPolicy{}, fmt.Errorf("here.now access policy has an invalid or missing mode")
	}
	if selected.PolicyVersion == nil || *selected.PolicyVersion <= 0 {
		return HereNowAccessPolicy{}, fmt.Errorf("here.now access policy is missing accessPolicyVersion")
	}
	if selected.AllowedEmails == nil || selected.AllowedDomains == nil {
		return HereNowAccessPolicy{}, fmt.Errorf("here.now access policy is missing its complete allowlist")
	}
	return HereNowAccessPolicy{
		Mode: mode, PolicyVersion: *selected.PolicyVersion,
		AllowedEmails:  append([]string(nil), (*selected.AllowedEmails)...),
		AllowedDomains: append([]string(nil), (*selected.AllowedDomains)...),
	}, nil
}

// UpdateAccess reads the current policy immediately before replacing its complete allowlist.
// Nil allowlist pointers preserve existing restricted entries; non-nil empty slices clear them.
func (c *HereNowClient) UpdateAccess(ctx context.Context, account, slug, mode string, allowedEmails, allowedDomains *[]string) (json.RawMessage, error) {
	mode = strings.TrimSpace(mode)
	switch mode {
	case "anyone_with_link", "restricted", "account_members":
	default:
		return nil, fmt.Errorf("unsupported here.now access mode %q", mode)
	}
	selection, err := c.resolveAccountSelection(ctx, account)
	if err != nil {
		return nil, err
	}
	if mode == "restricted" && selection.Ownership != "personal" {
		return nil, fmt.Errorf("here.now restricted access is available only for personal Sites")
	}
	if mode == "account_members" && selection.Ownership != "workspace" {
		return nil, fmt.Errorf("here.now account_members access is available only for workspace Sites")
	}
	currentRaw, err := c.request(ctx, http.MethodGet, "/api/v1/publish/"+hereNowPath(slug)+"/access", selection.RequestAccount, nil, hereNowReadPolicy("get access"))
	if err != nil {
		return nil, err
	}
	current, err := ParseHereNowAccessPolicy(currentRaw)
	if err != nil {
		return nil, err
	}
	if current.Mode == "password" && mode == "restricted" {
		return nil, fmt.Errorf("remove the Site password explicitly before enabling restricted access")
	}
	emails := []string{}
	domains := []string{}
	if mode == "restricted" {
		emails = append(emails, current.AllowedEmails...)
		domains = append(domains, current.AllowedDomains...)
		if allowedEmails != nil {
			emails = append([]string{}, (*allowedEmails)...)
		}
		if allowedDomains != nil {
			domains = append([]string{}, (*allowedDomains)...)
		}
	}
	patch := map[string]interface{}{
		"mode": mode, "allowedEmails": emails, "allowedDomains": domains, "notify": false,
	}
	raw, err := c.request(ctx, http.MethodPatch, "/api/v1/publish/"+hereNowPath(slug)+"/access", selection.RequestAccount, patch, hereNowMutationPolicy("update access"))
	if err != nil {
		return nil, err
	}
	updated, err := ParseHereNowAccessPolicy(raw)
	if err != nil {
		return nil, &HereNowOutcomeUnknownError{Operation: "update access", Cause: err}
	}
	if updated.Mode != mode || !equalHereNowAllowlist(updated.AllowedEmails, emails) || !equalHereNowAllowlist(updated.AllowedDomains, domains) {
		return nil, &HereNowOutcomeUnknownError{Operation: "update access", Cause: fmt.Errorf("provider returned a different access policy")}
	}
	return raw, nil
}

func equalHereNowAllowlist(left, right []string) bool {
	normalize := func(values []string) []string {
		result := make([]string, 0, len(values))
		for _, value := range values {
			result = append(result, strings.ToLower(strings.TrimSpace(value)))
		}
		slices.Sort(result)
		return result
	}
	return slices.Equal(normalize(left), normalize(right))
}

func (c *HereNowClient) RestoreVersion(ctx context.Context, account, slug, versionID string) (json.RawMessage, error) {
	return c.request(ctx, http.MethodPost, "/api/v1/publish/"+hereNowPath(slug)+"/versions/"+hereNowPath(versionID)+"/restore", account, nil, hereNowMutationPolicy("restore version"))
}

func (c *HereNowClient) DeleteSite(ctx context.Context, account, slug string) error {
	_, err := c.request(ctx, http.MethodDelete, "/api/v1/publish/"+hereNowPath(slug), account, nil, hereNowMutationPolicy("delete Site"))
	return err
}

func (c *HereNowClient) DeleteVersion(ctx context.Context, account, slug, versionID string) error {
	_, err := c.request(ctx, http.MethodDelete, "/api/v1/publish/"+hereNowPath(slug)+"/versions/"+hereNowPath(versionID), account, nil, hereNowMutationPolicy("delete version"))
	return err
}

type hereNowPublishFile struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType,omitempty"`
	Hash        string `json:"hash"`
	SourcePath  string `json:"-"`
}

type hereNowUploadTarget struct {
	Path    string            `json:"path"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type hereNowPublishStatus struct {
	RequestAuth string  `json:"requestAuth"`
	Ownership   string  `json:"ownership"`
	AccountID   *string `json:"accountId"`
	Persistence string  `json:"persistence"`
	State       string  `json:"state"`
}

type hereNowPublishStart struct {
	Slug             string               `json:"slug"`
	SiteURL          string               `json:"siteUrl"`
	Anonymous        bool                 `json:"anonymous"`
	RequiresFinalize bool                 `json:"requiresFinalize"`
	PublishStatus    hereNowPublishStatus `json:"publishStatus"`
	Upload           struct {
		VersionID    string                `json:"versionId"`
		Uploads      []hereNowUploadTarget `json:"uploads"`
		Skipped      []string              `json:"skipped"`
		FinalizeURL  string                `json:"finalizeUrl"`
		ExpiresInSec int                   `json:"expiresInSeconds"`
	} `json:"upload"`
}

type hereNowFinalize struct {
	Success           bool                 `json:"success"`
	Slug              string               `json:"slug"`
	SiteURL           string               `json:"siteUrl"`
	CurrentVersionID  string               `json:"currentVersionId"`
	PreviousVersionID *string              `json:"previousVersionId"`
	Unchanged         bool                 `json:"unchanged"`
	Replayed          bool                 `json:"replayed"`
	PublishStatus     hereNowPublishStatus `json:"publishStatus"`
}

type hereNowAccountSelection struct {
	RequestAccount string
	AccountID      string
	Ownership      string
}

// HereNowResolvedAccount exposes the stable account identity needed for
// account-scoped Vault keys without exposing provider credentials.
type HereNowResolvedAccount struct {
	AccountID string
	Ownership string
}

// ResolveAccount validates a personal or workspace selector and returns its
// canonical provider identity.
func (c *HereNowClient) ResolveAccount(ctx context.Context, requested string) (HereNowResolvedAccount, error) {
	selection, err := c.resolveAccountSelection(ctx, requested)
	if err != nil {
		return HereNowResolvedAccount{}, err
	}
	return HereNowResolvedAccount{AccountID: selection.AccountID, Ownership: selection.Ownership}, nil
}

func (c *HereNowClient) resolveAccountSelection(ctx context.Context, requested string) (hereNowAccountSelection, error) {
	selector := c.account(requested)
	raw, err := c.request(ctx, http.MethodGet, "/api/v1/accounts", hereNowNoAccount, nil, hereNowReadPolicy("resolve account"))
	if err != nil {
		return hereNowAccountSelection{}, fmt.Errorf("resolve here.now account: %w", err)
	}
	var response struct {
		Accounts []struct {
			AccountID          string `json:"accountId"`
			Type               string `json:"type"`
			Status             string `json:"status"`
			ProvisioningStatus string `json:"provisioningStatus"`
			Subdomain          string `json:"subdomain"`
			Selector           struct {
				ID        string `json:"id"`
				Subdomain string `json:"subdomain"`
			} `json:"selector"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return hereNowAccountSelection{}, fmt.Errorf("decode here.now accounts: %w", err)
	}
	for _, candidate := range response.Accounts {
		if candidate.Status != "" && !strings.EqualFold(candidate.Status, "active") {
			continue
		}
		if candidate.ProvisioningStatus != "" && !strings.EqualFold(candidate.ProvisioningStatus, "active") {
			continue
		}
		if candidate.Type != "personal" && candidate.Type != "org" {
			continue
		}
		matches := selector == "" && candidate.Type == "personal"
		if selector != "" {
			matches = strings.EqualFold(selector, candidate.AccountID) || strings.EqualFold(selector, candidate.Selector.ID) ||
				strings.EqualFold(selector, candidate.Subdomain) || strings.EqualFold(selector, candidate.Selector.Subdomain)
		}
		if !matches || candidate.AccountID == "" {
			continue
		}
		ownership := "workspace"
		requestAccount := candidate.AccountID
		if candidate.Type == "personal" {
			ownership = "personal"
			if selector == "" {
				requestAccount = ""
			}
		}
		return hereNowAccountSelection{RequestAccount: requestAccount, AccountID: candidate.AccountID, Ownership: ownership}, nil
	}
	if selector == "" {
		return hereNowAccountSelection{}, fmt.Errorf("here.now personal account is unavailable")
	}
	return hereNowAccountSelection{}, fmt.Errorf("here.now account selector is unavailable")
}

func verifyHereNowPublishStatus(status hereNowPublishStatus, selection hereNowAccountSelection, state string) error {
	if status.RequestAuth != "api_key" || status.Ownership != selection.Ownership || status.Persistence != "permanent" || status.State != state || status.AccountID == nil || *status.AccountID != selection.AccountID {
		return fmt.Errorf("here.now publish status does not match the selected authenticated permanent account")
	}
	return nil
}

// HereNowPublishOptions defines an authenticated permanent Site deployment.
type HereNowPublishOptions struct {
	Account            string
	Slug               string
	WorkspaceLabel     string
	DisplayName        string
	DisplayDescription string
	ViewerTitle        string
	ViewerDescription  string
	OGImagePath        string
	SPAMode            *bool
}

// HereNowDeploymentResult is shaped for both native tool output and the Homepage ledger.
type HereNowDeploymentResult struct {
	Status            string  `json:"status"`
	Slug              string  `json:"slug"`
	SiteURL           string  `json:"site_url"`
	CurrentVersionID  string  `json:"current_version_id"`
	PreviousVersionID *string `json:"previous_version_id,omitempty"`
	Account           string  `json:"account,omitempty"`
	ProjectDir        string  `json:"project_dir,omitempty"`
	BuildDir          string  `json:"build_dir,omitempty"`
	DeployPath        string  `json:"deploy_path,omitempty"`
	Unchanged         bool    `json:"unchanged,omitempty"`
	Replayed          bool    `json:"replayed,omitempty"`
	Verified          bool    `json:"verified"`
	VerifiedURL       string  `json:"verified_url,omitempty"`
}

func validateHereNowFinalizeURL(raw, slug string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	expectedPath := "/api/v1/publish/" + hereNowPath(slug) + "/finalize"
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "here.now") || parsed.Port() != "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" || parsed.EscapedPath() != expectedPath {
		return fmt.Errorf("here.now returned an invalid finalize URL")
	}
	return nil
}

type hereNowUploadError struct {
	status int
	err    error
}

func (e *hereNowUploadError) Error() string { return e.err.Error() }
func (e *hereNowUploadError) Unwrap() error { return e.err }

func validateHereNowUploadURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return fmt.Errorf("here.now upload URL must use HTTPS")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("here.now upload URL must not contain credentials or fragments")
	}
	return nil
}

func (c *HereNowClient) uploadOne(ctx context.Context, target hereNowUploadTarget, file hereNowPublishFile) error {
	if target.Method != "" && !strings.EqualFold(target.Method, http.MethodPut) {
		return fmt.Errorf("here.now returned unsupported upload method %q", target.Method)
	}
	if err := validateHereNowUploadURL(target.URL); err != nil {
		return err
	}
	client, err := c.uploadClient(target.URL)
	if err != nil {
		return fmt.Errorf("here.now upload target is not publicly reachable")
	}
	for attempt := 1; attempt <= hereNowMaxAttempts; attempt++ {
		body, openErr := os.Open(file.SourcePath)
		if openErr != nil {
			return fmt.Errorf("open upload file %s: %w", file.Path, openErr)
		}
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPut, target.URL, body)
		if reqErr != nil {
			_ = body.Close()
			return fmt.Errorf("create upload request for %s failed", file.Path)
		}
		req.ContentLength = file.Size
		for key, value := range target.Headers {
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "authorization", "proxy-authorization", "cookie", "x-herenow-client", "x-herenow-account":
				_ = body.Close()
				return fmt.Errorf("here.now upload target requested a forbidden credential header")
			}
			req.Header.Set(key, value)
		}
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", file.ContentType)
		}
		resp, doErr := client.Do(req)
		_ = body.Close()
		if doErr != nil {
			if attempt < hereNowMaxAttempts {
				if sleepErr := c.sleep(ctx, time.Duration(attempt)*250*time.Millisecond); sleepErr != nil {
					return sleepErr
				}
				continue
			}
			return fmt.Errorf("upload %s: network request failed", file.Path)
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		uploadErr := &hereNowUploadError{status: resp.StatusCode, err: fmt.Errorf("upload %s failed with HTTP %d", file.Path, resp.StatusCode)}
		if attempt < hereNowMaxAttempts && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) {
			if sleepErr := c.sleep(ctx, hereNowRetryDelay(&HereNowAPIError{RetryAfter: resp.Header.Get("Retry-After")}, attempt, c.now())); sleepErr != nil {
				return sleepErr
			}
			continue
		}
		return uploadErr
	}
	return fmt.Errorf("upload %s failed", file.Path)
}

func (c *HereNowClient) uploadAll(ctx context.Context, targets []hereNowUploadTarget, files []hereNowPublishFile) error {
	byPath := make(map[string]hereNowPublishFile, len(files))
	for _, file := range files {
		byPath[file.Path] = file
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan hereNowUploadTarget)
	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error
	workers := hereNowUploadWorkers
	if len(targets) < workers {
		workers = len(targets)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for target := range jobs {
				file, ok := byPath[target.Path]
				if !ok {
					once.Do(func() { firstErr = fmt.Errorf("here.now requested unknown upload path %q", target.Path); cancel() })
					continue
				}
				if err := c.uploadOne(ctx, target, file); err != nil {
					once.Do(func() { firstErr = err; cancel() })
				}
			}
		}()
	}
sendTargets:
	for _, target := range targets {
		select {
		case <-ctx.Done():
			break sendTargets
		case jobs <- target:
		}
	}
	close(jobs)
	wg.Wait()
	return firstErr
}

func (c *HereNowClient) publishStart(ctx context.Context, files []hereNowPublishFile, opts HereNowPublishOptions, selection hereNowAccountSelection) (hereNowPublishStart, error) {
	manifest := make([]map[string]interface{}, 0, len(files))
	for _, file := range files {
		manifest = append(manifest, map[string]interface{}{
			"path": file.Path, "size": file.Size, "contentType": file.ContentType, "hash": file.Hash,
		})
	}
	body := map[string]interface{}{"files": manifest, "ttlSeconds": nil}
	account := selection.RequestAccount
	if account != "" {
		body["account"] = account
	}
	if opts.SPAMode != nil {
		body["spaMode"] = *opts.SPAMode
	}
	if opts.DisplayName != "" {
		body["displayName"] = opts.DisplayName
	}
	if opts.DisplayDescription != "" {
		body["displayDescription"] = opts.DisplayDescription
	}
	viewer := map[string]interface{}{}
	if opts.ViewerTitle != "" {
		viewer["title"] = opts.ViewerTitle
	}
	if opts.ViewerDescription != "" {
		viewer["description"] = opts.ViewerDescription
	}
	if opts.OGImagePath != "" {
		viewer["ogImagePath"] = opts.OGImagePath
	}
	if len(viewer) > 0 {
		body["viewer"] = viewer
	}
	method := http.MethodPost
	endpoint := "/api/v1/publish"
	if strings.TrimSpace(opts.Slug) != "" {
		method = http.MethodPut
		endpoint = "/api/v1/publish/" + hereNowPath(opts.Slug)
	}
	operation := "create Site"
	if method == http.MethodPut {
		operation = "update Site"
	}
	raw, err := c.request(ctx, method, endpoint, account, body, hereNowMutationPolicy(operation))
	if err != nil {
		return hereNowPublishStart{}, err
	}
	var started hereNowPublishStart
	if err := json.Unmarshal(raw, &started); err != nil {
		return hereNowPublishStart{}, fmt.Errorf("decode here.now publish response: %w", err)
	}
	if started.Anonymous {
		return hereNowPublishStart{}, fmt.Errorf("here.now did not create an authenticated Site; anonymous publishing is disabled")
	}
	if err := verifyHereNowPublishStatus(started.PublishStatus, selection, "pending"); err != nil {
		return hereNowPublishStart{}, err
	}
	if started.Slug == "" || started.SiteURL == "" || started.Upload.VersionID == "" || !started.RequiresFinalize {
		return hereNowPublishStart{}, fmt.Errorf("here.now returned an incomplete publish response")
	}
	if err := validateHereNowFinalizeURL(started.Upload.FinalizeURL, started.Slug); err != nil {
		return hereNowPublishStart{}, err
	}
	return started, nil
}

// PublishWorkspaceDirectory publishes or updates a static directory that is
// addressed relative to the Homepage workspace. Files are uploaded only from
// an immutable private snapshot built before the first provider request.
func (c *HereNowClient) PublishWorkspaceDirectory(ctx context.Context, workspaceRoot, deployRelative string, opts HereNowPublishOptions) (HereNowDeploymentResult, error) {
	snapshot, err := buildHereNowSnapshot(workspaceRoot, deployRelative)
	if err != nil {
		return HereNowDeploymentResult{}, err
	}
	defer snapshot.Close()
	files := snapshot.files
	selection, err := c.resolveAccountSelection(ctx, opts.Account)
	if err != nil {
		return HereNowDeploymentResult{}, err
	}
	effectiveOpts := opts
	effectiveOpts.Account = selection.RequestAccount
	started, err := c.publishStart(ctx, files, effectiveOpts, selection)
	if err != nil {
		return HereNowDeploymentResult{}, err
	}
	uploadErr := c.uploadAll(ctx, started.Upload.Uploads, files)
	var expired *hereNowUploadError
	if uploadErr != nil && errors.As(uploadErr, &expired) && (expired.status == http.StatusUnauthorized || expired.status == http.StatusForbidden) {
		raw, refreshErr := c.request(ctx, http.MethodPost, "/api/v1/publish/"+hereNowPath(started.Slug)+"/uploads/refresh", selection.RequestAccount, nil, hereNowMutationPolicy("refresh uploads"))
		if refreshErr != nil {
			return HereNowDeploymentResult{}, fmt.Errorf("refresh expired here.now uploads: %w", refreshErr)
		}
		var refreshed hereNowPublishStart
		if err := json.Unmarshal(raw, &refreshed); err != nil {
			return HereNowDeploymentResult{}, fmt.Errorf("decode refreshed here.now uploads: %w", err)
		}
		if refreshed.Slug != started.Slug || refreshed.Upload.VersionID != started.Upload.VersionID || !refreshed.RequiresFinalize {
			return HereNowDeploymentResult{}, fmt.Errorf("here.now upload refresh changed the pending version")
		}
		if refreshed.Anonymous {
			return HereNowDeploymentResult{}, fmt.Errorf("here.now upload refresh returned an anonymous Site")
		}
		if err := verifyHereNowPublishStatus(refreshed.PublishStatus, selection, "pending"); err != nil {
			return HereNowDeploymentResult{}, err
		}
		if err := validateHereNowFinalizeURL(refreshed.Upload.FinalizeURL, started.Slug); err != nil {
			return HereNowDeploymentResult{}, err
		}
		uploadErr = c.uploadAll(ctx, refreshed.Upload.Uploads, files)
	}
	if uploadErr != nil {
		return HereNowDeploymentResult{}, uploadErr
	}
	finalizeBody := map[string]interface{}{"versionId": started.Upload.VersionID}
	account := selection.RequestAccount
	if account != "" {
		finalizeBody["account"] = account
	}
	if strings.TrimSpace(opts.WorkspaceLabel) != "" {
		finalizeBody["workspaceLabel"] = strings.TrimSpace(opts.WorkspaceLabel)
	}
	raw, err := c.request(ctx, http.MethodPost, "/api/v1/publish/"+hereNowPath(started.Slug)+"/finalize", account, finalizeBody, hereNowFinalizePolicy())
	if err != nil {
		return HereNowDeploymentResult{}, err
	}
	var finalized hereNowFinalize
	if err := json.Unmarshal(raw, &finalized); err != nil {
		return HereNowDeploymentResult{}, fmt.Errorf("decode here.now finalize response: %w", err)
	}
	versionMatches := finalized.CurrentVersionID != "" && (finalized.Unchanged || finalized.CurrentVersionID == started.Upload.VersionID)
	if !finalized.Success || finalized.Slug != started.Slug || finalized.SiteURL == "" || !versionMatches {
		return HereNowDeploymentResult{}, fmt.Errorf("here.now finalize did not produce an authenticated permanent live Site")
	}
	if err := verifyHereNowPublishStatus(finalized.PublishStatus, selection, "live"); err != nil {
		return HereNowDeploymentResult{}, err
	}
	verified, err := c.VerifySiteURL(ctx, finalized.SiteURL)
	if err != nil {
		return HereNowDeploymentResult{}, fmt.Errorf("verify published here.now Site: %w", err)
	}
	return HereNowDeploymentResult{
		Status: "ok", Slug: finalized.Slug, SiteURL: finalized.SiteURL,
		CurrentVersionID: finalized.CurrentVersionID, PreviousVersionID: finalized.PreviousVersionID,
		Account: selection.AccountID, Unchanged: finalized.Unchanged, Replayed: finalized.Replayed,
		Verified: verified, VerifiedURL: finalized.SiteURL,
	}, nil
}

// VerifySiteURL confirms that the finalized public or access-protected Site is reachable.
func (c *HereNowClient) VerifySiteURL(ctx context.Context, raw string) (bool, error) {
	current, err := validateHereNowSiteURL(raw, true)
	if err != nil {
		return false, err
	}
	visited := make(map[string]struct{}, 6)
	for redirects := 0; ; {
		key := current.String()
		if _, exists := visited[key]; exists {
			return false, fmt.Errorf("here.now Site verification redirect loop detected")
		}
		visited[key] = struct{}{}
		client, err := c.uploadClient(key)
		if err != nil {
			return false, fmt.Errorf("here.now Site URL is not publicly reachable")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, key, nil)
		if err != nil {
			return false, fmt.Errorf("create here.now Site verification request: %w", err)
		}
		req.Header.Set("Range", "bytes=0-0")
		resp, err := client.Do(req)
		if err != nil {
			return false, fmt.Errorf("here.now Site verification request failed")
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return true, nil
		}
		if resp.StatusCode < 300 || resp.StatusCode >= 400 {
			return false, fmt.Errorf("here.now Site verification returned HTTP %d", resp.StatusCode)
		}
		if redirects >= 5 {
			return false, fmt.Errorf("here.now Site verification exceeded five redirects")
		}
		location, parseErr := url.Parse(strings.TrimSpace(resp.Header.Get("Location")))
		if parseErr != nil || location.String() == "" {
			return false, fmt.Errorf("here.now Site verification returned an invalid redirect")
		}
		current, err = validateHereNowSiteURL(current.ResolveReference(location).String(), false)
		if err != nil {
			return false, err
		}
		redirects++
	}
}

func validateHereNowSiteURL(raw string, initial bool) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Fragment != "" || parsed.Port() != "" || (initial && parsed.RawQuery != "") {
		return nil, fmt.Errorf("invalid here.now Site URL")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host != "here.now" && !strings.HasSuffix(host, ".here.now") {
		return nil, fmt.Errorf("unexpected here.now Site host")
	}
	return parsed, nil
}

// HereNowSitePasswordVaultKey returns a stable, non-user-controlled Vault key.
func HereNowSitePasswordVaultKey(accountID, slug string) string {
	accountID = strings.ToLower(strings.TrimSpace(accountID))
	slug = strings.ToLower(strings.TrimSpace(slug))
	sum := sha256.Sum256([]byte(accountID + "\x00" + slug))
	return "HERE_NOW_SITE_PASSWORD_" + strings.ToUpper(hex.EncodeToString(sum[:12]))
}

// HereNowErrorPayload serializes provider and ambiguous-outcome errors without
// including request URLs, credentials, or transport diagnostics.
func HereNowErrorPayload(err error) map[string]interface{} {
	payload := map[string]interface{}{"status": "error", "message": err.Error()}
	var outcomeErr *HereNowOutcomeUnknownError
	if errors.As(err, &outcomeErr) {
		payload["code"] = "here_now_outcome_unknown"
		payload["outcome"] = "unknown"
		payload["retry_safe"] = outcomeErr.RetrySafe
		payload["operation"] = outcomeErr.Operation
		var apiErr *HereNowAPIError
		if errors.As(err, &apiErr) {
			payload["http_code"] = apiErr.StatusCode
			payload["provider_error"] = apiErr.ProviderError
			payload["provider_code"] = apiErr.Code
			payload["details"] = apiErr.Details
			payload["retry_after"] = apiErr.RetryAfter
			payload["docs_url"] = apiErr.DocsURL
		}
		return payload
	}
	var apiErr *HereNowAPIError
	if errors.As(err, &apiErr) {
		payload["http_code"] = apiErr.StatusCode
		payload["error"] = apiErr.ProviderError
		payload["code"] = apiErr.Code
		payload["message"] = apiErr.Message
		payload["details"] = apiErr.Details
		payload["retry_after"] = apiErr.RetryAfter
		payload["docs_url"] = apiErr.DocsURL
	}
	return payload
}

func encodeHereNowOutput(raw json.RawMessage, err error) string {
	if err == nil {
		return string(raw)
	}
	encoded, _ := json.Marshal(HereNowErrorPayload(err))
	return string(encoded)
}
