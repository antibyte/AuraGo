package kaggle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Service) pollRemote() {
	ctx, cancel := context.WithTimeout(s.ctx, 45*time.Second)
	defer cancel()
	jobs, err := s.Store.List(ctx)
	if err != nil {
		return
	}
	active := false
	for _, j := range jobs {
		if j.Status == "remote_running" || j.Status == "cancel_requested" {
			active = true
			break
		}
	}
	if !active {
		return
	}
	snap, c, err := s.client()
	if err != nil || !snap.Access.Network {
		return
	}
	account, err := c.Identity(ctx)
	if err != nil {
		return
	}
	for _, j := range jobs {
		if j.Account != account || j.Status != "remote_running" && j.Status != "cancel_requested" {
			continue
		}
		if j.Domain == "benchmarks" && j.Status == "remote_running" {
			created, e := time.Parse(time.RFC3339Nano, j.Created)
			if e == nil && time.Since(created) >= time.Duration(snap.Config.RemoteTimeoutMinutes)*time.Minute {
				_ = s.cancelRemote(ctx, j)
				if refreshed, e := s.Store.Get(ctx, j.ID); e == nil {
					j = refreshed
				}
			}
		}
		var req Request
		var p, r map[string]any
		if json.Unmarshal(j.Request, &req) != nil || decodeResponse(req.Parameters, &p) != nil || decodeResponse(j.Result, &r) != nil {
			continue
		}
		if j.Domain == "notebooks" {
			owner, slug, ok := notebookRef(stringValue(r, "ref"))
			if !ok {
				owner, slug, ok = notebookRef(stringValue(p, "slug"))
			}
			if !ok || r["versionNumber"] == nil {
				continue
			}
			op, _ := Lookup("notebooks", "get_kernel_session_status")
			// No versionLabel: a version number is not a label, and Kaggle answers 404 for it.
			body, e := c.Call(ctx, op, map[string]any{"userName": owner, "kernelSlug": slug})
			if e != nil {
				if NotFound(e) {
					s.endMissingNotebook(ctx, j, r, owner, account, e, snap.Config.RemoteTimeoutMinutes)
				}
				continue
			}
			var response map[string]any
			if json.Unmarshal(body, &response) != nil {
				continue
			}
			state := strings.ToUpper(stringValue(response, "status"))
			status := remoteState(state)
			r["remote_status"] = state
			b, _ := json.Marshal(r)
			if status != "" {
				s.Store.update(ctx, j.ID, status, j.Progress, cleanOutput(b), "")
			} else if string(b) != string(j.Result) {
				s.Store.update(ctx, j.ID, j.Status, j.Progress, cleanOutput(b), "")
			}
		} else if j.Domain == "benchmarks" {
			op, _ := Lookup("benchmarks", "list_benchmark_task_runs")
			tasks, _ := p["taskSlugs"].([]any)
			expected := benchmarkRunIDs(r)
			if len(tasks) == 0 || len(expected) == 0 {
				continue
			}
			states := map[string]string{}
			for _, task := range tasks {
				query := map[string]any{"taskSlug": task, "pageSize": 100}
				for page := 0; page < 20; page++ {
					b, e := c.Call(ctx, op, query)
					if e != nil {
						break
					}
					var response struct {
						Runs []map[string]any `json:"runs"`
						Next string           `json:"nextPageToken"`
					}
					if decodeResponse(b, &response) != nil {
						break
					}
					for _, run := range response.Runs {
						id := fmt.Sprint(run["id"])
						if expected[id] {
							states[id] = remoteState(stringValue(run, "state"))
						}
					}
					if response.Next == "" {
						break
					}
					query["pageToken"] = response.Next
				}
			}
			if len(states) != len(expected) {
				continue
			}
			status := "completed"
			for _, state := range states {
				if state == "" {
					status = ""
					break
				}
				if state == "failed" {
					status = "failed"
				}
				if state == "cancelled" && status == "completed" {
					status = "cancelled"
				}
			}
			if status != "" {
				s.Store.update(ctx, j.ID, status, j.Progress, j.Result, "")
			}
		}
	}
}

// notebookGoneGrace is how old a notebook job must be before a 403 for the own notebook means it was deleted.
var notebookGoneGrace = 2 * time.Minute

// notebookRef splits a Kaggle notebook reference into owner and slug. save_kernel answers "/code/<owner>/<slug>",
// get_kernel and the request slug use "<owner>/<slug>".
func notebookRef(ref string) (owner, slug string, ok bool) {
	ref = strings.TrimPrefix(ref, "/")
	ref = strings.TrimPrefix(ref, "code/")
	parts := strings.Split(ref, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// endMissingNotebook ends a notebook job whose status read answered 403 or 404 for good: the own notebook is gone
// (403 past the grace), or Kaggle shows no run for it past the remote timeout. Otherwise the job keeps waiting.
func (s *Service) endMissingNotebook(ctx context.Context, j Job, r map[string]any, owner, account string, readErr error, timeoutMinutes int) {
	created, err := time.Parse(time.RFC3339Nano, j.Created)
	if err != nil {
		return
	}
	age := time.Since(created)
	var api *APIError
	errors.As(readErr, &api)
	message := ""
	switch {
	case api != nil && api.Status == 403 && strings.EqualFold(owner, account) && age >= notebookGoneGrace:
		message = "The Kaggle notebook no longer exists"
	case timeoutMinutes > 0 && age >= time.Duration(timeoutMinutes)*time.Minute:
		message = "Kaggle reports no run for this notebook"
	default:
		return
	}
	status := "failed"
	if j.Status == "cancel_requested" {
		status = "cancelled"
	}
	r["remote_status"] = "NOT_FOUND"
	b, _ := json.Marshal(r)
	s.Store.update(ctx, j.ID, status, j.Progress, cleanOutput(b), message)
}

func remoteState(v string) string {
	v = strings.ToUpper(v)
	switch {
	case v == "COMPLETE" || v == "COMPLETED" || strings.HasSuffix(v, "_SUCCEEDED") || strings.HasSuffix(v, "_COMPLETED"):
		return "completed"
	case v == "ERROR" || v == "FAILED" || strings.HasSuffix(v, "_FAILED") || strings.HasSuffix(v, "_ERRORED"):
		return "failed"
	case v == "CANCELLED" || v == "CANCELED" || v == "CANCEL_ACKNOWLEDGED" || strings.HasSuffix(v, "_CANCELLED"):
		return "cancelled"
	}
	return ""
}
func benchmarkRunIDs(r map[string]any) map[string]bool {
	ids := map[string]bool{}
	items, _ := r["results"].([]any)
	for _, item := range items {
		m, _ := item.(map[string]any)
		if m["runId"] != nil {
			ids[fmt.Sprint(m["runId"])] = true
		}
	}
	return ids
}

func benchmarkAcceptance(raw json.RawMessage) (string, string) {
	var response map[string]any
	if decodeResponse(raw, &response) != nil {
		return "unknown", "Kaggle returned no verifiable benchmark receipt; do not replay"
	}
	results, _ := response["results"].([]any)
	allSkipped := len(results) > 0
	missingID := false
	for _, item := range results {
		r, _ := item.(map[string]any)
		scheduled, known := r["runScheduled"].(bool)
		if !known || scheduled {
			allSkipped = false
			if r["runId"] == nil || fmt.Sprint(r["runId"]) == "0" {
				missingID = true
			}
		}
	}
	if allSkipped {
		return "failed", "Kaggle skipped the requested benchmark run; inspect the receipt"
	}
	if missingID || len(benchmarkRunIDs(response)) == 0 {
		return "unknown", "Kaggle returned no verifiable benchmark run ID; do not replay"
	}
	return "remote_running", ""
}

func (s *Service) cancelRemote(ctx context.Context, j Job) error {
	snap, c, err := s.client()
	if err != nil {
		return err
	}
	if !snap.Access.Admin || !snap.Access.Network {
		return errors.New("administrative cancellation is unavailable")
	}
	account, err := c.Identity(ctx)
	if err != nil {
		return err
	}
	if account != j.Account {
		return errors.New("job belongs to another Kaggle account")
	}
	var r map[string]any
	decodeResponse(j.Result, &r)
	var op Operation
	var params map[string]any
	if j.Domain == "notebooks" {
		if r["kernelSessionId"] == nil {
			return errors.New("Kaggle did not return a verifiable session ID; cancel this run on Kaggle")
		}
		op, _ = Lookup("notebooks", "cancel_kernel_session")
		params = map[string]any{"kernelSessionId": r["kernelSessionId"]}
	} else if j.Domain == "benchmarks" {
		ids := benchmarkRunIDs(r)
		if len(ids) == 0 {
			return errors.New("Kaggle did not return run IDs")
		}
		values := []any{}
		for id := range ids {
			values = append(values, json.Number(id))
		}
		op, _ = Lookup("benchmarks", "api_cancel_benchmark_task_runs")
		params = map[string]any{"runIds": map[string]any{"ids": values}, "includeActive": true}
	} else {
		return errors.New("job does not support remote cancellation")
	}
	raw, _ := json.Marshal(params)
	_, typed, err := Decode(op, raw)
	if err != nil {
		return err
	}
	claimed, err := s.Store.claimCancellation(ctx, j.ID)
	if err != nil || !claimed {
		return err
	}
	if _, err = c.Call(ctx, op, typed); err != nil {
		final, done := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer done()
		_ = s.Store.update(final, j.ID, "cancel_requested", j.Progress, j.Result, PublicError(err))
		return err
	}
	return s.Store.update(ctx, j.ID, "cancel_requested", j.Progress, j.Result, "")
}
