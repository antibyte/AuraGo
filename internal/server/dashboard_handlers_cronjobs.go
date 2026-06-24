package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/tools"

	"github.com/robfig/cron/v3"
)

type dashboardCronjob struct {
	ID         string `json:"id"`
	CronExpr   string `json:"cron_expr"`
	TaskPrompt string `json:"task_prompt"`
	Disabled   bool   `json:"disabled"`
	Status     string `json:"status"`
	Source     string `json:"source"`
	NextRun    string `json:"next_run,omitempty"`
	Registered bool   `json:"registered"`
	LastError  string `json:"last_error,omitempty"`
}

type dashboardCronjobUpdateRequest struct {
	ID         string `json:"id"`
	CronExpr   string `json:"cron_expr"`
	TaskPrompt string `json:"task_prompt"`
	Disabled   *bool  `json:"disabled,omitempty"`
}

type cronManagerStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	ID      string `json:"id,omitempty"`
}

// handleDashboardCronjobs lists and updates internal CronManager jobs for the dashboard.
func handleDashboardCronjobs(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.CronManager == nil {
			jsonError(w, "Cron scheduler not available", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			handleDashboardCronjobsList(s, w, r)
		case http.MethodPut:
			handleDashboardCronjobsUpdate(s, w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleDashboardCronjobByID deletes one cron job by ID.
func handleDashboardCronjobByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.CronManager == nil {
			jsonError(w, "Cron scheduler not available", http.StatusServiceUnavailable)
			return
		}
		id, err := cronjobIDFromPath(r.URL.Path)
		if err != nil || id == "" {
			jsonError(w, "Invalid cron job id", http.StatusBadRequest)
			return
		}
		if !dashboardCronjobExists(s.CronManager.GetJobs(), id) {
			jsonError(w, "Cron job not found", http.StatusNotFound)
			return
		}
		result, err := s.CronManager.ManageSchedule("remove", id, "", "", dashboardLanguage(s))
		if err != nil {
			if s.Logger != nil {
				s.Logger.Error("Failed to remove cron job", "id", id, "error", err)
			}
			jsonError(w, "Failed to remove cron job", http.StatusInternalServerError)
			return
		}
		if !writeCronManagerStatus(w, result) {
			return
		}
	}
}

func handleDashboardCronjobsList(s *Server, w http.ResponseWriter, r *http.Request) {
	jobs := dashboardCronjobsFromTools(s.CronManager.GetJobsWithRuntimeStatus())
	filtered := filterDashboardCronjobs(jobs, r.URL.Query())

	enabled := 0
	disabled := 0
	errors := 0
	sources := map[string]int{}
	for _, job := range filtered {
		switch job.Status {
		case "disabled":
			disabled++
		case "error":
			errors++
		default:
			enabled++
		}
		if job.Source != "" {
			sources[job.Source]++
		}
	}

	writeJSON(w, map[string]interface{}{
		"jobs":     filtered,
		"total":    len(filtered),
		"enabled":  enabled,
		"disabled": disabled,
		"errors":   errors,
		"sources":  sources,
	})
}

func handleDashboardCronjobsUpdate(s *Server, w http.ResponseWriter, r *http.Request) {
	var body dashboardCronjobUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid cron job update request", http.StatusBadRequest)
		return
	}
	body.ID = strings.TrimSpace(body.ID)
	body.CronExpr = strings.TrimSpace(body.CronExpr)
	body.TaskPrompt = strings.TrimSpace(body.TaskPrompt)
	if body.ID == "" || body.CronExpr == "" || body.TaskPrompt == "" {
		jsonError(w, "id, cron_expr, and task_prompt required", http.StatusBadRequest)
		return
	}
	if _, err := dashboardCronParser().Parse(body.CronExpr); err != nil {
		jsonError(w, "Invalid cron expression", http.StatusBadRequest)
		return
	}

	existing, ok := findDashboardCronjob(s.CronManager.GetJobs(), body.ID)
	if !ok {
		jsonError(w, "Cron job not found", http.StatusNotFound)
		return
	}
	disabled := existing.Disabled
	if body.Disabled != nil {
		disabled = *body.Disabled
	}

	removeResult, err := s.CronManager.ManageSchedule("remove", body.ID, "", "", dashboardLanguage(s))
	if err != nil {
		jsonError(w, "Failed to remove existing cron job", http.StatusInternalServerError)
		return
	}
	if !cronManagerSucceeded(removeResult) {
		writeCronManagerStatus(w, removeResult)
		return
	}
	result, err := s.CronManager.ManageScheduleWithSource("add", body.ID, body.CronExpr, body.TaskPrompt, dashboardLanguage(s), existing.Source)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Error("Failed to update cron job", "id", body.ID, "error", err)
		}
		jsonError(w, "Failed to update cron job", http.StatusInternalServerError)
		return
	}
	if !cronManagerSucceeded(result) {
		writeCronManagerStatus(w, result)
		return
	}
	if disabled {
		if result, err := s.CronManager.ManageSchedule("disable", body.ID, "", "", dashboardLanguage(s)); err != nil {
			jsonError(w, "Failed to disable updated cron job", http.StatusInternalServerError)
			return
		} else if !cronManagerSucceeded(result) {
			writeCronManagerStatus(w, result)
			return
		}
	}
	writeCronManagerStatus(w, result)
}

func dashboardCronjobsFromTools(jobs []tools.CronJobRuntimeStatus) []dashboardCronjob {
	out := make([]dashboardCronjob, 0, len(jobs))
	for _, job := range jobs {
		item := dashboardCronjob{
			ID:         job.ID,
			CronExpr:   job.CronExpr,
			TaskPrompt: job.TaskPrompt,
			Disabled:   job.Disabled,
			Status:     "enabled",
			Source:     job.Source,
			Registered: job.Registered,
			LastError:  job.LastError,
		}
		if item.Source == "" {
			item.Source = "agent"
		}
		if item.Disabled {
			item.Status = "disabled"
		} else if item.LastError != "" || !item.Registered {
			item.Status = "error"
			if item.LastError == "" {
				item.LastError = "cron job is not registered"
			}
		} else if next := nextCronRun(job.CronExpr); !next.IsZero() {
			item.NextRun = next.Format(time.RFC3339)
		}
		out = append(out, item)
	}
	return out
}

func filterDashboardCronjobs(jobs []dashboardCronjob, query url.Values) []dashboardCronjob {
	q := strings.ToLower(strings.TrimSpace(query.Get("q")))
	source := strings.TrimSpace(query.Get("source"))
	status := strings.TrimSpace(query.Get("status"))
	filtered := make([]dashboardCronjob, 0, len(jobs))
	for _, job := range jobs {
		if source != "" && job.Source != source {
			continue
		}
		if status != "" && job.Status != status {
			continue
		}
		if q != "" {
			haystack := strings.ToLower(strings.Join([]string{
				job.ID, job.CronExpr, job.TaskPrompt, job.Source, job.Status, job.LastError,
			}, " "))
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		filtered = append(filtered, job)
	}
	return filtered
}

func dashboardCronParser() cron.Parser {
	return cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
}

func nextCronRun(expr string) time.Time {
	schedule, err := dashboardCronParser().Parse(expr)
	if err != nil {
		return time.Time{}
	}
	return schedule.Next(time.Now())
}

func findDashboardCronjob(jobs []tools.CronJob, id string) (tools.CronJob, bool) {
	for _, job := range jobs {
		if job.ID == id {
			return job, true
		}
	}
	return tools.CronJob{}, false
}

func dashboardCronjobExists(jobs []tools.CronJob, id string) bool {
	_, ok := findDashboardCronjob(jobs, id)
	return ok
}

func cronjobIDFromPath(path string) (string, error) {
	raw := strings.Trim(strings.TrimPrefix(path, "/api/dashboard/cronjobs/"), "/")
	if raw == "" {
		return "", nil
	}
	return url.PathUnescape(raw)
}

func cronManagerSucceeded(raw string) bool {
	var result cronManagerStatus
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return false
	}
	return result.Status == "success"
}

func writeCronManagerStatus(w http.ResponseWriter, raw string) bool {
	var result cronManagerStatus
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		jsonError(w, "Invalid cron manager response", http.StatusInternalServerError)
		return false
	}
	if result.Status != "success" {
		status := http.StatusBadRequest
		if result.Status == "warning" {
			status = http.StatusNotFound
		}
		jsonError(w, result.Message, status)
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(raw))
	return true
}

func dashboardLanguage(s *Server) string {
	if s == nil || s.Cfg == nil {
		return ""
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return s.Cfg.Server.UILanguage
}
