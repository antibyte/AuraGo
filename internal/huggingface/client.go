package huggingface

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultUserAgent = "AuraGo HuggingFace Integration"

type ClientConfig struct {
	HubBaseURL            string
	DatasetBaseURL        string
	JobsBaseURL           string
	JobNamespace          string
	Token                 string
	MaxDatasetRows        int
	MaxDownloadMB         int
	MaxResultBytes        int
	RequestTimeoutSeconds int
	RequestUserAgent      string
	HTTPClient            *http.Client
}

type Client struct {
	cfg        ClientConfig
	httpClient *http.Client
}

type APIError struct {
	StatusCode int
	Message    string
	RetryAfter string
}

func (e *APIError) Error() string {
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	if e.RetryAfter != "" {
		return fmt.Sprintf("huggingface api error: status %d: %s (retry_after=%s)", e.StatusCode, msg, e.RetryAfter)
	}
	return fmt.Sprintf("huggingface api error: status %d: %s", e.StatusCode, msg)
}

type SearchOptions struct {
	Query  string
	Limit  int
	Cursor string
}

type RepoOptions struct {
	RepoType string
	RepoID   string
	Revision string
}

type DatasetRowsOptions struct {
	Dataset string
	Config  string
	Split   string
	Offset  int
	Length  int
}

type DatasetSearchOptions struct {
	Dataset string
	Config  string
	Split   string
	Query   string
	Offset  int
	Length  int
}

type DatasetFilterOptions struct {
	Dataset string
	Config  string
	Split   string
	Where   string
	Offset  int
	Length  int
}

type DownloadFileOptions struct {
	RepoType    string
	RepoID      string
	Revision    string
	Path        string
	Destination string
}

type DownloadResult struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

type JobOptions struct {
	ID string
}

type JobRunOptions struct {
	Command        string                 `json:"command,omitempty"`
	Image          string                 `json:"image,omitempty"`
	Script         string                 `json:"script,omitempty"`
	Hardware       string                 `json:"hardware,omitempty"`
	TimeoutMinutes int                    `json:"timeout_minutes,omitempty"`
	Env            map[string]string      `json:"env,omitempty"`
	Secrets        map[string]string      `json:"secrets,omitempty"`
	Args           map[string]interface{} `json:"args,omitempty"`
	Scheduled      bool                   `json:"scheduled,omitempty"`
	Schedule       string                 `json:"schedule,omitempty"`
}

type CreateRepoOptions struct {
	RepoID  string `json:"repo_id"`
	Type    string `json:"type,omitempty"`
	Private bool   `json:"private,omitempty"`
}

type UploadFileOptions struct {
	RepoType  string
	RepoID    string
	Revision  string
	Path      string
	LocalPath string
	Message   string
}

type DiscussionOptions struct {
	RepoType string
	RepoID   string
	Title    string
	Body     string
	Number   int
}

func NewClient(cfg ClientConfig) *Client {
	cfg.HubBaseURL = trimBaseURL(defaultString(cfg.HubBaseURL, "https://huggingface.co"))
	cfg.DatasetBaseURL = trimBaseURL(defaultString(cfg.DatasetBaseURL, "https://datasets-server.huggingface.co"))
	cfg.JobsBaseURL = trimBaseURL(defaultString(cfg.JobsBaseURL, "https://huggingface.co/api/jobs"))
	if cfg.MaxDatasetRows <= 0 {
		cfg.MaxDatasetRows = 100
	}
	if cfg.MaxDownloadMB <= 0 {
		cfg.MaxDownloadMB = 512
	}
	if cfg.MaxResultBytes <= 0 {
		cfg.MaxResultBytes = 524288
	}
	timeout := time.Duration(cfg.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{cfg: cfg, httpClient: httpClient}
}

func (c *Client) WhoAmI(ctx context.Context) (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.doJSON(ctx, http.MethodGet, c.cfg.HubBaseURL, "/api/whoami-v2", nil, nil, &out)
	return out, err
}

func (c *Client) SearchModels(ctx context.Context, opts SearchOptions) ([]map[string]interface{}, error) {
	return c.searchHub(ctx, "/api/models", opts)
}

func (c *Client) GetModel(ctx context.Context, repoID string) (map[string]interface{}, error) {
	return c.getHubObject(ctx, "/api/models/"+strings.Trim(repoID, "/"))
}

func (c *Client) SearchDatasets(ctx context.Context, opts SearchOptions) ([]map[string]interface{}, error) {
	return c.searchHub(ctx, "/api/datasets", opts)
}

func (c *Client) GetDataset(ctx context.Context, repoID string) (map[string]interface{}, error) {
	return c.getHubObject(ctx, "/api/datasets/"+strings.Trim(repoID, "/"))
}

func (c *Client) SearchSpaces(ctx context.Context, opts SearchOptions) ([]map[string]interface{}, error) {
	return c.searchHub(ctx, "/api/spaces", opts)
}

func (c *Client) GetSpace(ctx context.Context, repoID string) (map[string]interface{}, error) {
	return c.getHubObject(ctx, "/api/spaces/"+strings.Trim(repoID, "/"))
}

func (c *Client) ListFiles(ctx context.Context, opts RepoOptions) ([]map[string]interface{}, error) {
	revision := defaultString(opts.Revision, "main")
	p := "/api/" + repoPlural(opts.RepoType) + "/" + strings.Trim(opts.RepoID, "/") + "/tree/" + revision
	q := neturl.Values{"recursive": []string{"1"}}
	var out []map[string]interface{}
	err := c.doJSON(ctx, http.MethodGet, c.cfg.HubBaseURL, p, q, nil, &out)
	return out, err
}

func (c *Client) DownloadFile(ctx context.Context, opts DownloadFileOptions) (DownloadResult, error) {
	revision := defaultString(opts.Revision, "main")
	prefix := repoResolvePrefix(opts.RepoType)
	p := prefix + strings.Trim(opts.RepoID, "/") + "/resolve/" + revision + "/" + strings.TrimLeft(opts.Path, "/")
	u, err := buildURL(c.cfg.HubBaseURL, p, nil)
	if err != nil {
		return DownloadResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return DownloadResult{}, err
	}
	c.decorate(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DownloadResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DownloadResult{}, c.apiError(resp)
	}
	maxBytes := int64(c.cfg.MaxDownloadMB) * 1024 * 1024
	if resp.ContentLength > maxBytes {
		return DownloadResult{}, fmt.Errorf("download exceeds max_download_mb (%d MB)", c.cfg.MaxDownloadMB)
	}
	limited := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return DownloadResult{}, err
	}
	if int64(len(data)) > maxBytes {
		return DownloadResult{}, fmt.Errorf("download exceeds max_download_mb (%d MB)", c.cfg.MaxDownloadMB)
	}
	if err := os.MkdirAll(filepath.Dir(opts.Destination), 0o755); err != nil {
		return DownloadResult{}, err
	}
	if err := os.WriteFile(opts.Destination, data, 0o600); err != nil {
		return DownloadResult{}, err
	}
	return DownloadResult{Path: opts.Destination, Bytes: int64(len(data))}, nil
}

func (c *Client) DatasetSplits(ctx context.Context, dataset string) (map[string]interface{}, error) {
	return c.datasetMap(ctx, "/splits", neturl.Values{"dataset": []string{dataset}})
}

func (c *Client) DatasetRows(ctx context.Context, opts DatasetRowsOptions) (map[string]interface{}, error) {
	q := c.datasetRowsQuery(opts.Dataset, opts.Config, opts.Split, opts.Offset, opts.Length)
	return c.datasetMap(ctx, "/rows", q)
}

func (c *Client) DatasetSearch(ctx context.Context, opts DatasetSearchOptions) (map[string]interface{}, error) {
	q := c.datasetRowsQuery(opts.Dataset, opts.Config, opts.Split, opts.Offset, opts.Length)
	if opts.Query != "" {
		q.Set("query", opts.Query)
	}
	return c.datasetMap(ctx, "/search", q)
}

func (c *Client) DatasetFilter(ctx context.Context, opts DatasetFilterOptions) (map[string]interface{}, error) {
	q := c.datasetRowsQuery(opts.Dataset, opts.Config, opts.Split, opts.Offset, opts.Length)
	if opts.Where != "" {
		q.Set("where", opts.Where)
	}
	return c.datasetMap(ctx, "/filter", q)
}

func (c *Client) DatasetParquet(ctx context.Context, dataset string) (map[string]interface{}, error) {
	return c.datasetMap(ctx, "/parquet", neturl.Values{"dataset": []string{dataset}})
}

func (c *Client) DatasetStatistics(ctx context.Context, dataset, configName, split string) (map[string]interface{}, error) {
	q := neturl.Values{"dataset": []string{dataset}}
	if configName != "" {
		q.Set("config", configName)
	}
	if split != "" {
		q.Set("split", split)
	}
	return c.datasetMap(ctx, "/statistics", q)
}

func (c *Client) DailyPapers(ctx context.Context) ([]map[string]interface{}, error) {
	var out []map[string]interface{}
	err := c.doJSON(ctx, http.MethodGet, c.cfg.HubBaseURL, "/api/daily_papers", nil, nil, &out)
	return out, err
}

func (c *Client) SearchPapers(ctx context.Context, opts SearchOptions) ([]map[string]interface{}, error) {
	q := neturl.Values{}
	if opts.Query != "" {
		q.Set("q", opts.Query)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Cursor != "" {
		q.Set("cursor", opts.Cursor)
	}
	var out []map[string]interface{}
	err := c.doJSON(ctx, http.MethodGet, c.cfg.HubBaseURL, "/api/papers/search", q, nil, &out)
	return out, err
}

func (c *Client) GetPaper(ctx context.Context, id string) (map[string]interface{}, error) {
	return c.getHubObject(ctx, "/api/papers/"+strings.Trim(id, "/"))
}

func (c *Client) PaperLinks(ctx context.Context, id string) (map[string]interface{}, error) {
	return c.getHubObject(ctx, "/api/papers/"+strings.Trim(id, "/"))
}

func (c *Client) JobsList(ctx context.Context) ([]map[string]interface{}, error) {
	var out []map[string]interface{}
	err := c.jobsJSON(ctx, "", nil, nil, &out)
	return out, err
}

func (c *Client) JobGet(ctx context.Context, id string) (map[string]interface{}, error) {
	return c.jobsMap(ctx, "/"+strings.Trim(id, "/"), nil, nil)
}

func (c *Client) JobLogs(ctx context.Context, id string) (map[string]interface{}, error) {
	return c.jobsMap(ctx, "/"+strings.Trim(id, "/")+"/logs", nil, nil)
}

func (c *Client) JobCancel(ctx context.Context, id string) (map[string]interface{}, error) {
	return c.jobsMap(ctx, "/"+strings.Trim(id, "/")+"/cancel", nil, map[string]interface{}{})
}

func (c *Client) JobRunScript(ctx context.Context, opts JobRunOptions) (map[string]interface{}, error) {
	if strings.TrimSpace(opts.Script) == "" {
		return nil, fmt.Errorf("huggingface script is required")
	}
	payload := map[string]interface{}{
		"dockerImage": "python:3.12",
		"command":     []string{"python", "-c", opts.Script},
	}
	if opts.Scheduled {
		return c.createScheduledJob(ctx, payload, opts)
	}
	mergeJobPayload(payload, opts)
	return c.jobsMap(ctx, "", nil, payload)
}

func (c *Client) JobRunContainer(ctx context.Context, opts JobRunOptions) (map[string]interface{}, error) {
	command := strings.Fields(opts.Command)
	if len(command) == 0 {
		return nil, fmt.Errorf("huggingface container command is required")
	}
	payload := map[string]interface{}{"dockerImage": opts.Image, "command": command}
	if opts.Scheduled {
		return c.createScheduledJob(ctx, payload, opts)
	}
	mergeJobPayload(payload, opts)
	return c.jobsMap(ctx, "", nil, payload)
}

func (c *Client) createScheduledJob(ctx context.Context, jobSpec map[string]interface{}, opts JobRunOptions) (map[string]interface{}, error) {
	if strings.TrimSpace(opts.Schedule) == "" {
		return nil, fmt.Errorf("huggingface schedule is required for scheduled jobs")
	}
	namespace, err := c.jobNamespace(ctx)
	if err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"jobSpec":  jobSpec,
		"schedule": strings.TrimSpace(opts.Schedule),
		"suspend":  false,
	}
	var out map[string]interface{}
	path := "/" + neturl.PathEscape(namespace)
	err = c.doJSON(ctx, http.MethodPost, c.scheduledJobsBaseURL(), path, nil, payload, &out)
	return out, err
}

func (c *Client) CreateRepo(ctx context.Context, opts CreateRepoOptions) (map[string]interface{}, error) {
	payload := map[string]interface{}{"name": opts.RepoID, "type": defaultString(opts.Type, "model"), "private": opts.Private}
	return c.postHubMap(ctx, "/api/repos/create", payload)
}

func (c *Client) UploadFile(ctx context.Context, opts UploadFileOptions) (map[string]interface{}, error) {
	data, err := os.ReadFile(opts.LocalPath)
	if err != nil {
		return nil, err
	}
	revision := defaultString(opts.Revision, "main")
	payload := map[string]interface{}{
		"summary": defaultString(opts.Message, "Upload file via AuraGo"),
		"operations": []map[string]interface{}{{
			"operation":    "addOrUpdate",
			"path_in_repo": strings.TrimLeft(opts.Path, "/"),
			"content":      base64.StdEncoding.EncodeToString(data),
			"encoding":     "base64",
		}},
	}
	p := "/api/" + repoPlural(opts.RepoType) + "/" + strings.Trim(opts.RepoID, "/") + "/commit/" + revision
	return c.postHubMap(ctx, p, payload)
}

func (c *Client) CreateDiscussion(ctx context.Context, opts DiscussionOptions) (map[string]interface{}, error) {
	p := "/api/" + repoPlural(opts.RepoType) + "/" + strings.Trim(opts.RepoID, "/") + "/discussions"
	return c.postHubMap(ctx, p, map[string]interface{}{"title": opts.Title, "description": opts.Body})
}

func (c *Client) CommentDiscussion(ctx context.Context, opts DiscussionOptions) (map[string]interface{}, error) {
	p := "/api/" + repoPlural(opts.RepoType) + "/" + strings.Trim(opts.RepoID, "/") + "/discussions/" + strconv.Itoa(opts.Number) + "/comment"
	return c.postHubMap(ctx, p, map[string]interface{}{"comment": opts.Body})
}

func (c *Client) DeleteRepo(ctx context.Context, repoType, repoID string) (map[string]interface{}, error) {
	p := "/api/repos/delete"
	return c.postHubMap(ctx, p, map[string]interface{}{"name": repoID, "type": defaultString(repoType, "model")})
}

func (c *Client) searchHub(ctx context.Context, endpoint string, opts SearchOptions) ([]map[string]interface{}, error) {
	q := neturl.Values{}
	if opts.Query != "" {
		q.Set("search", opts.Query)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Cursor != "" {
		q.Set("cursor", opts.Cursor)
	}
	var out []map[string]interface{}
	err := c.doJSON(ctx, http.MethodGet, c.cfg.HubBaseURL, endpoint, q, nil, &out)
	return out, err
}

func (c *Client) getHubObject(ctx context.Context, endpoint string) (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.doJSON(ctx, http.MethodGet, c.cfg.HubBaseURL, endpoint, nil, nil, &out)
	return out, err
}

func (c *Client) datasetMap(ctx context.Context, endpoint string, q neturl.Values) (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.doJSON(ctx, http.MethodGet, c.cfg.DatasetBaseURL, endpoint, q, nil, &out)
	return out, err
}

func (c *Client) jobsMap(ctx context.Context, endpoint string, q neturl.Values, payload interface{}) (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.jobsJSON(ctx, endpoint, q, payload, &out)
	return out, err
}

func (c *Client) jobsJSON(ctx context.Context, endpoint string, q neturl.Values, payload interface{}, out interface{}) error {
	namespace, err := c.jobNamespace(ctx)
	if err != nil {
		return err
	}
	path := "/" + neturl.PathEscape(namespace)
	if trimmed := strings.TrimPrefix(endpoint, "/"); trimmed != "" {
		path += "/" + trimmed
	}
	method := http.MethodGet
	if payload != nil {
		method = http.MethodPost
	}
	return c.doJSON(ctx, method, c.cfg.JobsBaseURL, path, q, payload, out)
}

func (c *Client) jobNamespace(ctx context.Context) (string, error) {
	if namespace := strings.TrimSpace(c.cfg.JobNamespace); namespace != "" {
		return namespace, nil
	}
	identity, err := c.WhoAmI(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve huggingface job namespace: %w", err)
	}
	namespace := firstString(identity["name"], identity["username"], identity["user"])
	if namespace == "" {
		return "", fmt.Errorf("huggingface whoami response does not contain a job namespace")
	}
	return namespace, nil
}

func (c *Client) postHubMap(ctx context.Context, endpoint string, payload interface{}) (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.doJSON(ctx, http.MethodPost, c.cfg.HubBaseURL, endpoint, nil, payload, &out)
	return out, err
}

func (c *Client) datasetRowsQuery(dataset, configName, split string, offset, length int) neturl.Values {
	if length <= 0 || length > c.cfg.MaxDatasetRows {
		length = c.cfg.MaxDatasetRows
	}
	q := neturl.Values{
		"dataset": []string{dataset},
		"offset":  []string{strconv.Itoa(maxInt(offset, 0))},
		"length":  []string{strconv.Itoa(length)},
	}
	if configName != "" {
		q.Set("config", configName)
	}
	if split != "" {
		q.Set("split", split)
	}
	return q
}

func (c *Client) doJSON(ctx context.Context, method, baseURL, endpoint string, q neturl.Values, payload interface{}, out interface{}) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	u, err := buildURL(baseURL, endpoint, q)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	c.decorate(req)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.apiError(resp)
	}
	limited := io.LimitReader(resp.Body, int64(c.cfg.MaxResultBytes)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if len(data) > c.cfg.MaxResultBytes {
		return fmt.Errorf("huggingface response exceeds max_result_bytes (%d)", c.cfg.MaxResultBytes)
	}
	if len(bytes.TrimSpace(data)) == 0 || out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode huggingface response: %w", err)
	}
	return nil
}

func (c *Client) decorate(req *http.Request) {
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	ua := strings.TrimSpace(c.cfg.RequestUserAgent)
	if ua == "" {
		ua = defaultUserAgent
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json")
}

func (c *Client) apiError(resp *http.Response) error {
	limited := io.LimitReader(resp.Body, int64(c.cfg.MaxResultBytes))
	data, _ := io.ReadAll(limited)
	msg := strings.TrimSpace(string(data))
	var parsed map[string]interface{}
	if json.Unmarshal(data, &parsed) == nil {
		msg = firstString(parsed["error"], parsed["message"])
	}
	return &APIError{StatusCode: resp.StatusCode, Message: msg, RetryAfter: resp.Header.Get("Retry-After")}
}

func buildURL(baseURL, endpoint string, q neturl.Values) (string, error) {
	baseURL = trimBaseURL(baseURL)
	if strings.TrimSpace(endpoint) != "" {
		parts := strings.Split(strings.Trim(endpoint, "/"), "/")
		joined, err := neturl.JoinPath(baseURL, parts...)
		if err != nil {
			return "", err
		}
		baseURL = joined
	}
	u, err := neturl.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if len(q) > 0 {
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

func mergeJobPayload(payload map[string]interface{}, opts JobRunOptions) {
	if opts.Hardware != "" {
		payload["flavor"] = opts.Hardware
	}
	if opts.TimeoutMinutes > 0 {
		payload["timeoutSeconds"] = opts.TimeoutMinutes * 60
	}
	if len(opts.Env) > 0 {
		payload["environment"] = opts.Env
	}
	if len(opts.Secrets) > 0 {
		payload["secrets"] = opts.Secrets
	}
	if len(opts.Args) > 0 {
		payload["args"] = opts.Args
	}
}

func (c *Client) scheduledJobsBaseURL() string {
	base := strings.TrimSuffix(trimBaseURL(c.cfg.JobsBaseURL), "/api/jobs")
	return base + "/api/scheduled-jobs"
}

func repoPlural(repoType string) string {
	switch strings.ToLower(strings.TrimSpace(repoType)) {
	case "dataset", "datasets":
		return "datasets"
	case "space", "spaces":
		return "spaces"
	default:
		return "models"
	}
}

func repoResolvePrefix(repoType string) string {
	switch strings.ToLower(strings.TrimSpace(repoType)) {
	case "dataset", "datasets":
		return "/datasets/"
	case "space", "spaces":
		return "/spaces/"
	default:
		return "/"
	}
}

func trimBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func firstString(values ...interface{}) string {
	for _, value := range values {
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
