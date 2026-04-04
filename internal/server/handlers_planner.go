package server

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"aurago/internal/memory"
	"aurago/internal/planner"
)

// ── Appointments ──

// handleAppointments handles GET (list) and POST (create) on /api/appointments.
func handleAppointments(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.PlannerDB == nil {
			jsonError(w, `{"error":"planner database not initialized"}`, http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			query := r.URL.Query().Get("q")
			status := r.URL.Query().Get("status")
			list, err := planner.ListAppointments(s.PlannerDB, query, status)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list appointments", "Failed to list appointments", err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(list)

		case http.MethodPost:
			var a planner.Appointment
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				jsonError(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}
			if err := json.Unmarshal(body, &a); err != nil {
				jsonError(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			id, err := planner.CreateAppointment(s.PlannerDB, a)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to create appointment", "Failed to create appointment", err)
				return
			}
			syncAppointmentToKG(s.KG, s.PlannerDB, id, s.Logger)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": id})

		default:
			jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// handleAppointmentByID handles GET, PUT, DELETE on /api/appointments/{id}.
func handleAppointmentByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.PlannerDB == nil {
			jsonError(w, `{"error":"planner database not initialized"}`, http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/appointments/")
		if id == "" {
			jsonError(w, `{"error":"missing appointment id"}`, http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			a, err := planner.GetAppointment(s.PlannerDB, id)
			if err != nil {
				jsonError(w, `{"error":"appointment not found"}`, http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(a)

		case http.MethodPut:
			var a planner.Appointment
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				jsonError(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}
			if err := json.Unmarshal(body, &a); err != nil {
				jsonError(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			a.ID = id
			if err := planner.UpdateAppointment(s.PlannerDB, a); err != nil {
				status := http.StatusInternalServerError
				if strings.Contains(err.Error(), "not found") {
					status = http.StatusNotFound
				}
				if status == http.StatusNotFound {
					jsonError(w, `{"error":"appointment not found"}`, status)
				} else {
					jsonLoggedError(w, s.Logger, status, "Failed to update appointment", "Failed to update appointment", err, "id", id)
				}
				return
			}
			syncAppointmentToKG(s.KG, s.PlannerDB, id, s.Logger)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

		case http.MethodDelete:
			// Remove from KG before deleting from DB to avoid orphaned nodes
			a, _ := planner.GetAppointment(s.PlannerDB, id)
			if a != nil && a.KGNodeID != "" && s.KG != nil {
				_ = s.KG.DeleteNode(a.KGNodeID)
			}
			if err := planner.DeleteAppointment(s.PlannerDB, id); err != nil {
				status := http.StatusInternalServerError
				if strings.Contains(err.Error(), "not found") {
					status = http.StatusNotFound
				}
				if status == http.StatusNotFound {
					jsonError(w, `{"error":"appointment not found"}`, status)
				} else {
					jsonLoggedError(w, s.Logger, status, "Failed to delete appointment", "Failed to delete appointment", err, "id", id)
				}
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

		default:
			jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// ── Todos ──

// handleTodos handles GET (list) and POST (create) on /api/todos.
func handleTodos(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.PlannerDB == nil {
			jsonError(w, `{"error":"planner database not initialized"}`, http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			query := r.URL.Query().Get("q")
			status := r.URL.Query().Get("status")
			list, err := planner.ListTodos(s.PlannerDB, query, status)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list todos", "Failed to list todos", err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(list)

		case http.MethodPost:
			var t planner.Todo
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				jsonError(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}
			if err := json.Unmarshal(body, &t); err != nil {
				jsonError(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			id, err := planner.CreateTodo(s.PlannerDB, t)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to create todo", "Failed to create todo", err)
				return
			}
			syncTodoToKG(s.KG, s.PlannerDB, id, s.Logger)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": id})

		default:
			jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// handleTodoByID handles GET, PUT, DELETE on /api/todos/{id}.
func handleTodoByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.PlannerDB == nil {
			jsonError(w, `{"error":"planner database not initialized"}`, http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/todos/")
		if id == "" {
			jsonError(w, `{"error":"missing todo id"}`, http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			t, err := planner.GetTodo(s.PlannerDB, id)
			if err != nil {
				jsonError(w, `{"error":"todo not found"}`, http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(t)

		case http.MethodPut:
			var t planner.Todo
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				jsonError(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}
			if err := json.Unmarshal(body, &t); err != nil {
				jsonError(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			t.ID = id
			if err := planner.UpdateTodo(s.PlannerDB, t); err != nil {
				status := http.StatusInternalServerError
				if strings.Contains(err.Error(), "not found") {
					status = http.StatusNotFound
				}
				if status == http.StatusNotFound {
					jsonError(w, `{"error":"todo not found"}`, status)
				} else {
					jsonLoggedError(w, s.Logger, status, "Failed to update todo", "Failed to update todo", err, "id", id)
				}
				return
			}
			syncTodoToKG(s.KG, s.PlannerDB, id, s.Logger)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

		case http.MethodDelete:
			// Remove from KG before deleting from DB to avoid orphaned nodes
			t, _ := planner.GetTodo(s.PlannerDB, id)
			if t != nil && t.KGNodeID != "" && s.KG != nil {
				_ = s.KG.DeleteNode(t.KGNodeID)
			}
			if err := planner.DeleteTodo(s.PlannerDB, id); err != nil {
				status := http.StatusInternalServerError
				if strings.Contains(err.Error(), "not found") {
					status = http.StatusNotFound
				}
				if status == http.StatusNotFound {
					jsonError(w, `{"error":"todo not found"}`, status)
				} else {
					jsonLoggedError(w, s.Logger, status, "Failed to delete todo", "Failed to delete todo", err, "id", id)
				}
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

		default:
			jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// ── KG sync helpers ──

func syncAppointmentToKG(kg *memory.KnowledgeGraph, db *sql.DB, id string, logger interface{ Error(string, ...any) }) {
	if kg == nil || db == nil {
		return
	}
	a, err := planner.GetAppointment(db, id)
	if err != nil {
		return
	}
	props := map[string]string{
		"type":   "event",
		"source": "planner",
		"date":   a.DateTime,
		"status": a.Status,
	}
	if a.Description != "" {
		props["description"] = a.Description
	}
	_ = kg.AddNode(a.KGNodeID, a.Title, props)
}

func syncTodoToKG(kg *memory.KnowledgeGraph, db *sql.DB, id string, logger interface{ Error(string, ...any) }) {
	if kg == nil || db == nil {
		return
	}
	t, err := planner.GetTodo(db, id)
	if err != nil {
		return
	}
	props := map[string]string{
		"type":     "task",
		"source":   "planner",
		"priority": t.Priority,
		"status":   t.Status,
	}
	if t.DueDate != "" {
		props["due_date"] = t.DueDate
	}
	if t.Description != "" {
		props["description"] = t.Description
	}
	_ = kg.AddNode(t.KGNodeID, t.Title, props)
}
