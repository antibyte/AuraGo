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
		var req Request
		var p, r map[string]any
		if json.Unmarshal(j.Request, &req) != nil || json.Unmarshal(req.Parameters, &p) != nil || json.Unmarshal(j.Result, &r) != nil {
			continue
		}
		if j.Domain == "notebooks" {
			ref := stringValue(r, "ref")
			if ref == "" {
				ref = stringValue(p, "slug")
			}
			parts := strings.Split(ref, "/")
			if len(parts) != 2 {
				continue
			}
			version := fmt.Sprint(r["versionNumber"])
			if r["versionNumber"] == nil {
				continue
			}
			op, _ := Lookup("notebooks", "get_kernel_session_status")
			body, e := c.Call(ctx, op, map[string]any{"userName": parts[0], "kernelSlug": parts[1], "versionLabel": version})
			if e != nil {
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
					if json.Unmarshal(b, &response) != nil {
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
func remoteState(v string) string {
	v = strings.ToUpper(v)
	switch {
	case v == "COMPLETE" || v == "COMPLETED" || strings.HasSuffix(v, "_SUCCEEDED") || strings.HasSuffix(v, "_COMPLETED"):
		return "completed"
	case v == "ERROR" || v == "FAILED" || strings.HasSuffix(v, "_FAILED"):
		return "failed"
	case v == "CANCELLED" || v == "CANCELED" || strings.HasSuffix(v, "_CANCELLED"):
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
	json.Unmarshal(j.Result, &r)
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
		params = map[string]any{"runIds": values, "includeActive": true}
	} else {
		return errors.New("job does not support remote cancellation")
	}
	if _, err = c.Call(ctx, op, params); err != nil {
		return err
	}
	return s.Store.update(ctx, j.ID, "cancel_requested", j.Progress, j.Result, "")
}
