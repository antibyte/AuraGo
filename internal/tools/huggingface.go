package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	hf "aurago/internal/huggingface"
)

type HuggingFaceRequest struct {
	Operation      string                 `json:"operation"`
	Query          string                 `json:"query,omitempty"`
	Limit          int                    `json:"limit,omitempty"`
	RepoType       string                 `json:"repo_type,omitempty"`
	RepoID         string                 `json:"repo_id,omitempty"`
	Name           string                 `json:"name,omitempty"`
	Revision       string                 `json:"revision,omitempty"`
	Path           string                 `json:"path,omitempty"`
	Destination    string                 `json:"destination,omitempty"`
	Dataset        string                 `json:"dataset,omitempty"`
	Config         string                 `json:"config,omitempty"`
	Split          string                 `json:"split,omitempty"`
	Offset         int                    `json:"offset,omitempty"`
	Length         int                    `json:"length,omitempty"`
	Where          string                 `json:"where,omitempty"`
	PaperID        string                 `json:"paper_id,omitempty"`
	JobID          string                 `json:"job_id,omitempty"`
	Hardware       string                 `json:"hardware,omitempty"`
	TimeoutMinutes int                    `json:"timeout_minutes,omitempty"`
	Script         string                 `json:"script,omitempty"`
	Image          string                 `json:"image,omitempty"`
	Command        string                 `json:"command,omitempty"`
	Title          string                 `json:"title,omitempty"`
	Body           string                 `json:"body,omitempty"`
	Number         int                    `json:"number,omitempty"`
	Private        bool                   `json:"private,omitempty"`
	LocalPath      string                 `json:"local_path,omitempty"`
	Message        string                 `json:"message,omitempty"`
	Scheduled      bool                   `json:"scheduled,omitempty"`
	Schedule       string                 `json:"schedule,omitempty"`
	Env            map[string]string      `json:"env,omitempty"`
	Args           map[string]interface{} `json:"args,omitempty"`
}

func EvaluateHuggingFacePolicy(cfg config.HuggingFaceConfig, req HuggingFaceRequest, token string) error {
	op := normalizeHFOperation(req.Operation)
	if op == "" {
		return fmt.Errorf("operation is required")
	}
	if !cfg.Enabled {
		return fmt.Errorf("Hugging Face integration is not enabled. Set huggingface.enabled=true in config.yaml")
	}
	if cfg.MaxDatasetRows <= 0 {
		cfg.MaxDatasetRows = 100
	}
	if op == "dataset_rows" || op == "dataset_search" || op == "dataset_filter" {
		if req.Length > cfg.MaxDatasetRows {
			return fmt.Errorf("requested dataset rows exceed huggingface.max_dataset_rows (%d)", cfg.MaxDatasetRows)
		}
	}
	if op == "whoami" && strings.TrimSpace(token) == "" {
		return fmt.Errorf("huggingface_token is required for whoami")
	}
	if isHFJobOperation(op) {
		if cfg.ReadOnly {
			return fmt.Errorf("Hugging Face is in read-only mode; jobs are blocked")
		}
		if !cfg.AllowJobs {
			return fmt.Errorf("Hugging Face jobs are not allowed. Set huggingface.allow_jobs=true")
		}
		if req.Scheduled && !cfg.AllowScheduledJobs {
			return fmt.Errorf("scheduled Hugging Face jobs are not allowed. Set huggingface.allow_scheduled_jobs=true")
		}
		if req.Scheduled && strings.TrimSpace(req.Schedule) == "" {
			return fmt.Errorf("schedule is required for scheduled Hugging Face jobs")
		}
		if strings.TrimSpace(token) == "" {
			return fmt.Errorf("huggingface_token is required for jobs")
		}
		hardware := hfDefaultString(strings.TrimSpace(req.Hardware), "cpu-basic")
		if !hfStringInList(hardware, hfDefaultStringSlice(cfg.AllowedHardware, []string{"cpu-basic"})) {
			return fmt.Errorf("Hugging Face hardware %q is not in huggingface.allowed_hardware", hardware)
		}
		return nil
	}
	if isHFWriteOperation(op) {
		if cfg.ReadOnly {
			return fmt.Errorf("Hugging Face is in read-only mode; writes are blocked")
		}
		if !cfg.AllowWrites {
			return fmt.Errorf("Hugging Face writes are not allowed. Set huggingface.allow_writes=true")
		}
		if strings.TrimSpace(token) == "" {
			return fmt.Errorf("huggingface_token is required for writes")
		}
		if isHFDeleteOperation(op) && !cfg.AllowDelete {
			return fmt.Errorf("Hugging Face delete operations are not allowed. Set huggingface.allow_delete=true")
		}
		repoID := requestRepoID(req)
		if repoID == "" {
			return fmt.Errorf("Hugging Face repository ID is required for writes")
		}
		if err := checkHFRepoAllowlist(cfg, repoID); err != nil {
			return err
		}
	}
	return nil
}

func RunHuggingFace(ctx context.Context, cfg config.HuggingFaceConfig, token, workspaceDir, dataDir string, req HuggingFaceRequest) string {
	req.Operation = normalizeHFOperation(req.Operation)
	if err := EvaluateHuggingFacePolicy(cfg, req, token); err != nil {
		return hfToolJSON("error", nil, err)
	}
	client := hf.NewClient(hf.ClientConfig{
		HubBaseURL:            cfg.HubBaseURL,
		DatasetBaseURL:        cfg.DatasetBaseURL,
		JobsBaseURL:           cfg.JobsBaseURL,
		Token:                 token,
		MaxDatasetRows:        cfg.MaxDatasetRows,
		MaxDownloadMB:         cfg.MaxDownloadMB,
		MaxResultBytes:        cfg.MaxResultBytes,
		RequestTimeoutSeconds: cfg.RequestTimeoutSeconds,
	})
	result, err := executeHuggingFaceOperation(ctx, client, cfg, token, workspaceDir, dataDir, req)
	if err != nil {
		return hfToolJSON("error", nil, err)
	}
	return hfToolJSON("success", result, nil)
}

func executeHuggingFaceOperation(ctx context.Context, client *hf.Client, cfg config.HuggingFaceConfig, token, workspaceDir, dataDir string, req HuggingFaceRequest) (interface{}, error) {
	switch req.Operation {
	case "whoami":
		return client.WhoAmI(ctx)
	case "search_models":
		return client.SearchModels(ctx, hf.SearchOptions{Query: req.Query, Limit: req.Limit})
	case "get_model":
		return client.GetModel(ctx, requestRepoID(req))
	case "search_datasets":
		return client.SearchDatasets(ctx, hf.SearchOptions{Query: req.Query, Limit: req.Limit})
	case "get_dataset":
		return client.GetDataset(ctx, requestRepoID(req))
	case "search_spaces":
		return client.SearchSpaces(ctx, hf.SearchOptions{Query: req.Query, Limit: req.Limit})
	case "get_space":
		return client.GetSpace(ctx, requestRepoID(req))
	case "list_files":
		return client.ListFiles(ctx, hf.RepoOptions{RepoType: req.RepoType, RepoID: requestRepoID(req), Revision: req.Revision})
	case "download_file":
		dest, err := huggingFaceDownloadDestination(workspaceDir, req)
		if err != nil {
			return nil, err
		}
		return client.DownloadFile(ctx, hf.DownloadFileOptions{RepoType: req.RepoType, RepoID: requestRepoID(req), Revision: req.Revision, Path: req.Path, Destination: dest})
	case "dataset_splits":
		return client.DatasetSplits(ctx, req.Dataset)
	case "dataset_rows":
		return client.DatasetRows(ctx, hf.DatasetRowsOptions{Dataset: req.Dataset, Config: req.Config, Split: req.Split, Offset: req.Offset, Length: req.Length})
	case "dataset_search":
		return client.DatasetSearch(ctx, hf.DatasetSearchOptions{Dataset: req.Dataset, Config: req.Config, Split: req.Split, Query: req.Query, Offset: req.Offset, Length: req.Length})
	case "dataset_filter":
		return client.DatasetFilter(ctx, hf.DatasetFilterOptions{Dataset: req.Dataset, Config: req.Config, Split: req.Split, Where: req.Where, Offset: req.Offset, Length: req.Length})
	case "dataset_parquet":
		return client.DatasetParquet(ctx, req.Dataset)
	case "dataset_statistics":
		return client.DatasetStatistics(ctx, req.Dataset, req.Config, req.Split)
	case "daily_papers":
		return client.DailyPapers(ctx)
	case "search_papers":
		return client.SearchPapers(ctx, hf.SearchOptions{Query: req.Query, Limit: req.Limit})
	case "get_paper":
		return client.GetPaper(ctx, hfFirstNonEmpty(req.PaperID, requestRepoID(req)))
	case "paper_links":
		return client.PaperLinks(ctx, hfFirstNonEmpty(req.PaperID, requestRepoID(req)))
	case "jobs_list":
		return client.JobsList(ctx)
	case "job_get":
		return client.JobGet(ctx, req.JobID)
	case "job_logs":
		return client.JobLogs(ctx, req.JobID)
	case "job_cancel":
		return client.JobCancel(ctx, req.JobID)
	case "job_run_script":
		opts := hfJobRunOptions(cfg, req, token)
		result, err := client.JobRunScript(ctx, opts)
		recordHuggingFaceJob(ctx, dataDir, req.Operation, opts.Hardware, req, result, err)
		return result, err
	case "job_run_container":
		opts := hfJobRunOptions(cfg, req, token)
		result, err := client.JobRunContainer(ctx, opts)
		recordHuggingFaceJob(ctx, dataDir, req.Operation, opts.Hardware, req, result, err)
		return result, err
	case "create_repo":
		return client.CreateRepo(ctx, hf.CreateRepoOptions{RepoID: requestRepoID(req), Type: req.RepoType, Private: req.Private})
	case "upload_file":
		return client.UploadFile(ctx, hf.UploadFileOptions{RepoType: req.RepoType, RepoID: requestRepoID(req), Revision: req.Revision, Path: req.Path, LocalPath: req.LocalPath, Message: req.Message})
	case "create_discussion":
		return client.CreateDiscussion(ctx, hf.DiscussionOptions{RepoType: req.RepoType, RepoID: requestRepoID(req), Title: req.Title, Body: req.Body})
	case "comment_discussion":
		return client.CommentDiscussion(ctx, hf.DiscussionOptions{RepoType: req.RepoType, RepoID: requestRepoID(req), Number: req.Number, Body: req.Body})
	case "delete_repo":
		return client.DeleteRepo(ctx, req.RepoType, requestRepoID(req))
	default:
		return nil, fmt.Errorf("unknown huggingface operation %q", req.Operation)
	}
}

func hfJobRunOptions(cfg config.HuggingFaceConfig, req HuggingFaceRequest, token string) hf.JobRunOptions {
	timeout := req.TimeoutMinutes
	if timeout <= 0 {
		timeout = cfg.JobDefaultTimeoutMinutes
	}
	if cfg.JobMaxRuntimeMinutes > 0 && timeout > cfg.JobMaxRuntimeMinutes {
		timeout = cfg.JobMaxRuntimeMinutes
	}
	hardware := hfDefaultString(req.Hardware, "cpu-basic")
	secrets := map[string]string{}
	if strings.TrimSpace(token) != "" {
		secrets["HF_TOKEN"] = token
	}
	return hf.JobRunOptions{
		Command:        req.Command,
		Image:          req.Image,
		Script:         req.Script,
		Hardware:       hardware,
		TimeoutMinutes: timeout,
		Env:            req.Env,
		Secrets:        secrets,
		Args:           req.Args,
		Scheduled:      req.Scheduled,
		Schedule:       req.Schedule,
	}
}

func recordHuggingFaceJob(ctx context.Context, dataDir, operation, hardware string, req HuggingFaceRequest, result map[string]interface{}, runErr error) {
	store, err := hf.OpenJobStore(dataDir)
	if err != nil {
		return
	}
	defer store.Close()
	jobID := hf.ExtractJobID(result)
	if jobID == "" {
		jobID = "local-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	rec := hf.JobRecord{
		HFJobID:      jobID,
		Operation:    operation,
		Hardware:     hardware,
		Status:       hfFirstStringFromMap(result, "status", "state"),
		RequestJSON:  huggingFaceLedgerRequest(req),
		ResponseJSON: hf.EncodeLedgerPayload(result),
	}
	if runErr != nil {
		rec.Status = "error"
		rec.LastError = runErr.Error()
	}
	_ = store.RecordJob(ctx, rec)
}

func huggingFaceLedgerRequest(req HuggingFaceRequest) string {
	redacted := req
	redacted.Script = ""
	redacted.Command = ""
	redacted.Body = ""
	if len(req.Env) > 0 {
		redacted.Env = make(map[string]string, len(req.Env))
		for key := range req.Env {
			redacted.Env[key] = "[redacted]"
		}
	}
	redacted.Args = nil
	return hf.EncodeLedgerPayload(redacted)
}

func huggingFaceDownloadDestination(workspaceDir string, req HuggingFaceRequest) (string, error) {
	if strings.TrimSpace(workspaceDir) == "" {
		workspaceDir = filepath.Join("agent_workspace", "workdir")
	}
	absWorkspace, err := filepath.Abs(workspaceDir)
	if err != nil {
		return "", err
	}
	dest := strings.TrimSpace(req.Destination)
	if dest == "" {
		dest = filepath.Join("huggingface", safeHFPathSegment(req.RepoType), safeHFPathSegment(requestRepoID(req)), filepath.FromSlash(strings.TrimLeft(req.Path, "/")))
	}
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(absWorkspace, dest)
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absWorkspace, absDest)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("download destination must stay inside the AuraGo workspace")
	}
	return absDest, nil
}

func checkHFRepoAllowlist(cfg config.HuggingFaceConfig, repoID string) error {
	repoID = strings.ToLower(strings.Trim(repoID, "/"))
	if len(cfg.AllowedRepos) == 0 && len(cfg.AllowedNamespaces) == 0 {
		return fmt.Errorf("Hugging Face writes require huggingface.allowed_repos or huggingface.allowed_namespaces")
	}
	if len(cfg.AllowedRepos) > 0 && !hfStringInList(repoID, cfg.AllowedRepos) {
		return fmt.Errorf("Hugging Face repo %q is not in huggingface.allowed_repos", repoID)
	}
	if len(cfg.AllowedNamespaces) > 0 {
		ns := repoID
		if idx := strings.Index(ns, "/"); idx >= 0 {
			ns = ns[:idx]
		}
		if !hfStringInList(ns, cfg.AllowedNamespaces) {
			return fmt.Errorf("Hugging Face namespace %q is not in huggingface.allowed_namespaces", ns)
		}
	}
	return nil
}

func hfToolJSON(status string, result interface{}, err error) string {
	payload := map[string]interface{}{"status": status}
	if err != nil {
		payload["message"] = err.Error()
	}
	if result != nil {
		payload["result"] = result
	}
	raw, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return `{"status":"error","message":"failed to encode huggingface result"}`
	}
	return string(raw)
}

func normalizeHFOperation(op string) string {
	return strings.ToLower(strings.TrimSpace(op))
}

func isHFJobOperation(op string) bool {
	switch op {
	case "jobs_list", "job_get", "job_logs", "job_run_script", "job_run_container", "job_cancel":
		return true
	default:
		return false
	}
}

func isHFWriteOperation(op string) bool {
	switch op {
	case "create_repo", "upload_file", "create_discussion", "comment_discussion", "delete_repo":
		return true
	default:
		return false
	}
}

func isHFDeleteOperation(op string) bool {
	return op == "delete_repo"
}

func requestRepoID(req HuggingFaceRequest) string {
	return strings.TrimSpace(hfFirstNonEmpty(req.RepoID, req.Name))
}

func hfStringInList(value string, list []string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	for _, item := range list {
		if strings.ToLower(strings.TrimSpace(item)) == value {
			return true
		}
	}
	return false
}

func hfDefaultStringSlice(values, fallback []string) []string {
	if len(values) == 0 {
		return fallback
	}
	return values
}

func safeHFPathSegment(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "_")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "..", "_")
	if value == "" {
		return "default"
	}
	return value
}

func hfFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func hfDefaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func hfFirstStringFromMap(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func HuggingFaceLedgerJobs(ctx context.Context, dataDir string, limit int) ([]hf.JobRecord, error) {
	store, err := hf.OpenJobStore(dataDir)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return store.ListJobs(ctx, limit)
}

func ResolveHuggingFaceToken(cfg config.HuggingFaceConfig) string {
	return strings.TrimSpace(cfg.Token)
}
