package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

func handleBackgroundTasks(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.BackgroundTasks == nil {
			http.Error(w, "Background tasks unavailable", http.StatusServiceUnavailable)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"summary": s.BackgroundTasks.Summary(),
			"tasks":   s.BackgroundTasks.ListTasks(50),
		})
	}
}

func handleBackgroundTaskByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.BackgroundTasks == nil {
			http.Error(w, "Background tasks unavailable", http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/background-tasks/")
		id = strings.TrimSpace(id)
		if id == "" {
			http.Error(w, "task id required", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			task, ok := s.BackgroundTasks.GetTask(id)
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(task)
		case http.MethodPost:
			var body struct {
				Action string `json:"action"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid JSON body", http.StatusBadRequest)
				return
			}
			switch strings.ToLower(strings.TrimSpace(body.Action)) {
			case "cancel":
				if !s.BackgroundTasks.CancelTask(id) {
					http.Error(w, "task cannot be canceled", http.StatusConflict)
					return
				}
			case "retry":
				if !s.BackgroundTasks.RetryTask(id) {
					http.Error(w, "task cannot be retried", http.StatusConflict)
					return
				}
			default:
				http.Error(w, "unsupported action", http.StatusBadRequest)
				return
			}
			task, _ := s.BackgroundTasks.GetTask(id)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(task)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
