package server

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"

	"aurago/internal/planner"
)

type todoCompleteRequest struct {
	CompleteItemsToo bool `json:"complete_items_too"`
}

type todoItemReorderRequest struct {
	ItemIDs []string `json:"item_ids"`
}

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
			if err := planner.AutoExpireAppointments(s.PlannerDB); err != nil {
				s.Logger.Warn("Failed to auto-expire appointments", "error", err)
			}
			list, err := planner.ListAppointments(s.PlannerDB, query, status)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list appointments", "Failed to list appointments", err)
				return
			}
			if s.ContactsDB != nil {
				_ = planner.EnrichAppointmentsWithContacts(s.PlannerDB, s.ContactsDB, list)
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
			contactIDs := a.ContactIDs
			a.ContactIDs = nil
			a.Participants = nil
			id, err := planner.CreateAppointment(s.PlannerDB, a)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to create appointment", "Failed to create appointment", err)
				return
			}
			if len(contactIDs) > 0 {
				if err := planner.SetAppointmentContacts(s.PlannerDB, id, contactIDs); err != nil {
					s.Logger.Warn("Failed to set appointment contacts", "appointment_id", id, "error", err)
				}
			}
			syncAppointmentToKGAsync(s.PlannerDB, id, s.KG, s.Logger)
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
			if s.ContactsDB != nil {
				_ = planner.EnrichAppointmentWithContacts(s.PlannerDB, s.ContactsDB, a)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(a)

		case http.MethodPut:
			existing, err := planner.GetAppointment(s.PlannerDB, id)
			if err != nil {
				jsonError(w, `{"error":"appointment not found"}`, http.StatusNotFound)
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				jsonError(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}
			// Parse into raw map for key-presence detection (BUG-2 / ISSUE-6)
			var rawMap map[string]interface{}
			if err := json.Unmarshal(body, &rawMap); err != nil {
				jsonError(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			var patch planner.Appointment
			json.Unmarshal(body, &patch) //nolint:errcheck — error already caught above
			// Only update fields that were explicitly provided in the JSON body.
			// Empty string or null explicitly clears optional fields (ISSUE-6).
			if _, ok := rawMap["title"]; ok && patch.Title != "" {
				existing.Title = patch.Title
			}
			if _, ok := rawMap["description"]; ok {
				existing.Description = patch.Description
			}
			if _, ok := rawMap["date_time"]; ok {
				existing.DateTime = patch.DateTime
			}
			if _, ok := rawMap["notification_at"]; ok {
				existing.NotificationAt = patch.NotificationAt
			}
			if _, ok := rawMap["status"]; ok && patch.Status != "" {
				existing.Status = patch.Status
			}
			if _, ok := rawMap["agent_instruction"]; ok {
				existing.AgentInstruction = patch.AgentInstruction
			}
			// BUG-2: Only overwrite WakeAgent when the key was explicitly present in JSON.
			if _, ok := rawMap["wake_agent"]; ok {
				existing.WakeAgent = patch.WakeAgent
			}
			if err := planner.UpdateAppointment(s.PlannerDB, *existing); err != nil {
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
			// Handle contact_ids update: if the key is present, replace all associations.
			if _, ok := rawMap["contact_ids"]; ok {
				if err := planner.SetAppointmentContacts(s.PlannerDB, id, patch.ContactIDs); err != nil {
					s.Logger.Warn("Failed to update appointment contacts", "appointment_id", id, "error", err)
				}
			}
			syncAppointmentToKGAsync(s.PlannerDB, id, s.KG, s.Logger)
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
			syncTodoToKGAsync(s.PlannerDB, id, s.KG, s.Logger)
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
		path := strings.TrimPrefix(r.URL.Path, "/api/todos/")
		path = strings.Trim(path, "/")
		if path == "" {
			jsonError(w, `{"error":"missing todo id"}`, http.StatusBadRequest)
			return
		}
		parts := strings.Split(path, "/")
		id := parts[0]

		if len(parts) > 1 {
			switch parts[1] {
			case "items":
				handleTodoItemsByID(s, id, parts[2:])(w, r)
				return
			case "complete":
				handleTodoComplete(s, id)(w, r)
				return
			default:
				jsonError(w, `{"error":"todo not found"}`, http.StatusNotFound)
				return
			}
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
			existing, err := planner.GetTodo(s.PlannerDB, id)
			if err != nil {
				jsonError(w, `{"error":"todo not found"}`, http.StatusNotFound)
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				jsonError(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}
			// Parse into raw map for key-presence detection (ISSUE-6)
			var rawTodoMap map[string]interface{}
			if err := json.Unmarshal(body, &rawTodoMap); err != nil {
				jsonError(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			var patch planner.Todo
			json.Unmarshal(body, &patch) //nolint:errcheck — error already caught above
			// Only update fields that were explicitly provided in the JSON body.
			// Empty string or null explicitly clears optional fields (ISSUE-6).
			if _, ok := rawTodoMap["title"]; ok && patch.Title != "" {
				existing.Title = patch.Title
			}
			if _, ok := rawTodoMap["description"]; ok {
				existing.Description = patch.Description
			}
			if _, ok := rawTodoMap["priority"]; ok && patch.Priority != "" {
				existing.Priority = patch.Priority
			}
			if _, ok := rawTodoMap["status"]; ok && patch.Status != "" {
				existing.Status = patch.Status
			}
			if _, ok := rawTodoMap["due_date"]; ok {
				existing.DueDate = patch.DueDate
			}
			if _, ok := rawTodoMap["remind_daily"]; ok {
				existing.RemindDaily = patch.RemindDaily
			}
			if _, ok := rawTodoMap["items"]; ok {
				if patch.Items == nil {
					existing.Items = []planner.TodoItem{}
				} else {
					existing.Items = patch.Items
				}
			}
			if err := planner.UpdateTodo(s.PlannerDB, *existing); err != nil {
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
			syncTodoToKGAsync(s.PlannerDB, id, s.KG, s.Logger)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

		case http.MethodDelete:
			// Remove from KG before deleting from DB to avoid orphaned nodes
			t, _ := planner.GetTodo(s.PlannerDB, id)
			kgNodeID := ""
			if t != nil {
				kgNodeID = t.KGNodeID
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
			deleteTodoFromKGAsync(s.KG, kgNodeID, s.Logger)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

		default:
			jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

func handleTodoItemsByID(s *Server, todoID string, parts []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case len(parts) == 0:
			if r.Method != http.MethodPost {
				jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
				return
			}
			var item planner.TodoItem
			if !decodePlannerJSON(w, r, &item) {
				return
			}
			itemID, err := planner.AddTodoItem(s.PlannerDB, todoID, item)
			if err != nil {
				status := http.StatusBadRequest
				if strings.Contains(err.Error(), "not found") {
					status = http.StatusNotFound
				}
				jsonLoggedError(w, s.Logger, status, "Failed to add todo item", "Failed to add todo item", err, "todo_id", todoID)
				return
			}
			syncTodoToKGAsync(s.PlannerDB, todoID, s.KG, s.Logger)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": itemID})

		case len(parts) == 1 && parts[0] == "reorder":
			if r.Method != http.MethodPost {
				jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
				return
			}
			var payload todoItemReorderRequest
			if !decodePlannerJSON(w, r, &payload) {
				return
			}
			if len(payload.ItemIDs) == 0 {
				jsonError(w, `{"error":"item_ids is required"}`, http.StatusBadRequest)
				return
			}
			if err := planner.ReorderTodoItems(s.PlannerDB, todoID, payload.ItemIDs); err != nil {
				status := http.StatusBadRequest
				if strings.Contains(err.Error(), "not found") {
					status = http.StatusNotFound
				}
				jsonLoggedError(w, s.Logger, status, "Failed to reorder todo items", "Failed to reorder todo items", err, "todo_id", todoID)
				return
			}
			syncTodoToKGAsync(s.PlannerDB, todoID, s.KG, s.Logger)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

		case len(parts) == 1:
			itemID := parts[0]
			switch r.Method {
			case http.MethodPut:
				todo, err := planner.GetTodo(s.PlannerDB, todoID)
				if err != nil {
					jsonError(w, `{"error":"todo not found"}`, http.StatusNotFound)
					return
				}
				item, found := plannerTodoItemByID(todo.Items, itemID)
				if !found {
					jsonError(w, `{"error":"todo item not found"}`, http.StatusNotFound)
					return
				}
				var raw map[string]interface{}
				body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
				if err != nil {
					jsonError(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
					return
				}
				if err := json.Unmarshal(body, &raw); err != nil {
					jsonError(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
					return
				}
				var patch planner.TodoItem
				json.Unmarshal(body, &patch) //nolint:errcheck
				if _, ok := raw["title"]; ok && patch.Title != "" {
					item.Title = patch.Title
				}
				if _, ok := raw["description"]; ok {
					item.Description = patch.Description
				}
				if _, ok := raw["position"]; ok {
					item.Position = patch.Position
				}
				if _, ok := raw["is_done"]; ok {
					item.IsDone = patch.IsDone
				}
				if err := planner.UpdateTodoItem(s.PlannerDB, item); err != nil {
					status := http.StatusBadRequest
					if strings.Contains(err.Error(), "not found") {
						status = http.StatusNotFound
					}
					jsonLoggedError(w, s.Logger, status, "Failed to update todo item", "Failed to update todo item", err, "todo_id", todoID, "item_id", itemID)
					return
				}
				syncTodoToKGAsync(s.PlannerDB, todoID, s.KG, s.Logger)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

			case http.MethodDelete:
				if err := planner.DeleteTodoItem(s.PlannerDB, todoID, itemID); err != nil {
					status := http.StatusBadRequest
					if strings.Contains(err.Error(), "not found") {
						status = http.StatusNotFound
					}
					jsonLoggedError(w, s.Logger, status, "Failed to delete todo item", "Failed to delete todo item", err, "todo_id", todoID, "item_id", itemID)
					return
				}
				syncTodoToKGAsync(s.PlannerDB, todoID, s.KG, s.Logger)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

			default:
				jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			}

		default:
			jsonError(w, `{"error":"todo item not found"}`, http.StatusNotFound)
		}
	}
}

func handleTodoComplete(s *Server, todoID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var payload todoCompleteRequest
		if r.Body != nil && r.ContentLength != 0 {
			if !decodePlannerJSON(w, r, &payload) {
				return
			}
		}

		if err := planner.CompleteTodo(s.PlannerDB, todoID, payload.CompleteItemsToo); err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "not found") {
				status = http.StatusNotFound
			}
			jsonLoggedError(w, s.Logger, status, "Failed to complete todo", "Failed to complete todo", err, "id", todoID)
			return
		}
		syncTodoToKGAsync(s.PlannerDB, todoID, s.KG, s.Logger)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	}
}

func decodePlannerJSON(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		jsonError(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
		return false
	}
	if err := json.Unmarshal(body, target); err != nil {
		jsonError(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return false
	}
	return true
}

func plannerTodoItemByID(items []planner.TodoItem, itemID string) (planner.TodoItem, bool) {
	for _, item := range items {
		if item.ID == itemID {
			return item, true
		}
	}
	return planner.TodoItem{}, false
}

// ── KG sync helpers ──

// syncAppointmentToKG syncs an appointment to the knowledge graph.
func syncAppointmentToKGAsync(db *sql.DB, id string, kg planner.KnowledgeGraph, logger interface{ Error(string, ...any) }) {
	runPlannerKGSyncAsync(logger, func() error {
		return planner.SyncAppointmentToKG(kg, db, id)
	}, "Failed to sync appointment to KG", "id", id)
}

func syncTodoToKGAsync(db *sql.DB, id string, kg planner.KnowledgeGraph, logger interface{ Error(string, ...any) }) {
	runPlannerKGSyncAsync(logger, func() error {
		return planner.SyncTodoToKG(kg, db, id)
	}, "Failed to sync todo to KG", "id", id)
}

func runPlannerKGSyncAsync(logger interface{ Error(string, ...any) }, syncFn func() error, message string, args ...any) {
	go func() {
		if err := syncFn(); err != nil {
			logger.Error(message, append([]any{"error", err}, args...)...)
		}
	}()
}

func deleteTodoFromKGAsync(kg planner.KnowledgeGraph, nodeID string, logger interface{ Error(string, ...any) }) {
	if strings.TrimSpace(nodeID) == "" || isNilPlannerKnowledgeGraph(kg) {
		return
	}
	go func() {
		if err := kg.DeleteNode(nodeID); err != nil {
			logger.Error("Failed to delete todo from KG", "error", err, "node_id", nodeID)
		}
	}()
}

func isNilPlannerKnowledgeGraph(kg planner.KnowledgeGraph) bool {
	if kg == nil {
		return true
	}
	value := reflect.ValueOf(kg)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
