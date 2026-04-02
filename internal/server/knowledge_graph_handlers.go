package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/memory"
)

func handleKnowledgeGraphNodes(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.KG == nil {
			writeJSON(w, []interface{}{})
			return
		}

		limit := parseKnowledgeGraphLimit(r, 200)
		nodes, err := s.KG.GetAllNodes(limit)
		if err != nil {
			http.Error(w, "Failed to load knowledge graph nodes", http.StatusInternalServerError)
			return
		}
		writeJSON(w, nodes)
	}
}

func handleKnowledgeGraphEdges(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.KG == nil {
			writeJSON(w, []interface{}{})
			return
		}

		limit := parseKnowledgeGraphLimit(r, 200)
		edges, err := s.KG.GetAllEdges(limit)
		if err != nil {
			http.Error(w, "Failed to load knowledge graph edges", http.StatusInternalServerError)
			return
		}
		writeJSON(w, edges)
	}
}

func handleKnowledgeGraphSearch(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.KG == nil {
			writeJSON(w, map[string]interface{}{"query": "", "nodes": []interface{}{}, "edges": []interface{}{}})
			return
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			writeJSON(w, map[string]interface{}{"query": "", "nodes": []interface{}{}, "edges": []interface{}{}})
			return
		}

		raw := s.KG.Search(query)
		if strings.TrimSpace(raw) == "" || raw == "[]" {
			writeJSON(w, map[string]interface{}{"query": query, "nodes": []interface{}{}, "edges": []interface{}{}})
			return
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			http.Error(w, "Failed to decode knowledge graph search result", http.StatusInternalServerError)
			return
		}
		payload["query"] = query
		writeJSON(w, payload)
	}
}

func handleKnowledgeGraphQuality(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.KG == nil {
			writeJSON(w, &memory.KnowledgeGraphQualityReport{
				IsolatedSample:      []memory.Node{},
				UntypedSample:       []memory.Node{},
				DuplicateCandidates: []memory.KnowledgeGraphDuplicateCandidate{},
			})
			return
		}

		report, err := s.KG.QualityReport(parseKnowledgeGraphLimit(r, 5))
		if err != nil {
			http.Error(w, "Failed to build knowledge graph quality report", http.StatusInternalServerError)
			return
		}
		writeJSON(w, report)
	}
}

func handleKnowledgeGraphNodeDetail(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if s.KG == nil {
				writeJSON(w, map[string]interface{}{"node": nil, "neighbors": []interface{}{}, "edges": []interface{}{}})
				return
			}

			nodeID := strings.TrimSpace(r.URL.Query().Get("id"))
			if nodeID == "" {
				http.Error(w, "Missing node id", http.StatusBadRequest)
				return
			}

			node, err := s.KG.GetNode(nodeID)
			if err != nil {
				http.Error(w, "Failed to load knowledge graph node", http.StatusInternalServerError)
				return
			}
			if node == nil {
				http.Error(w, "Knowledge graph node not found", http.StatusNotFound)
				return
			}

			limit := parseKnowledgeGraphLimit(r, 25)
			neighbors, edges := s.KG.GetNeighbors(nodeID, limit)
			writeJSON(w, map[string]interface{}{
				"node":      node,
				"neighbors": neighbors,
				"edges":     edges,
			})

		case http.MethodPut:
			if s.KG == nil {
				jsonError(w, "Knowledge graph is unavailable", http.StatusServiceUnavailable)
				return
			}

			var req struct {
				ID         string            `json:"id"`
				Label      string            `json:"label"`
				Properties map[string]string `json:"properties"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.ID) == "" {
				jsonError(w, "id is required", http.StatusBadRequest)
				return
			}

			node, err := s.KG.UpdateNode(req.ID, req.Label, req.Properties)
			if err != nil {
				jsonError(w, "Failed to update knowledge graph node", http.StatusInternalServerError)
				return
			}
			if node == nil {
				jsonError(w, "Knowledge graph node not found", http.StatusNotFound)
				return
			}

			writeJSON(w, map[string]interface{}{"status": "ok", "node": node})

		case http.MethodDelete:
			if s.KG == nil {
				jsonError(w, "Knowledge graph is unavailable", http.StatusServiceUnavailable)
				return
			}

			nodeID := strings.TrimSpace(r.URL.Query().Get("id"))
			if nodeID == "" {
				jsonError(w, "Missing node id", http.StatusBadRequest)
				return
			}

			if err := s.KG.DeleteNode(nodeID); err != nil {
				if errors.Is(err, memory.ErrKnowledgeGraphProtectedNode) {
					jsonError(w, "Protected nodes cannot be deleted", http.StatusConflict)
					return
				}
				jsonError(w, "Failed to delete knowledge graph node", http.StatusInternalServerError)
				return
			}

			writeJSON(w, map[string]interface{}{"status": "ok", "id": nodeID})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleKnowledgeGraphNodeProtect(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.KG == nil {
			jsonError(w, "Knowledge graph is unavailable", http.StatusServiceUnavailable)
			return
		}

		var req struct {
			ID        string `json:"id"`
			Protected bool   `json:"protected"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.ID) == "" {
			jsonError(w, "id is required", http.StatusBadRequest)
			return
		}

		node, err := s.KG.SetNodeProtected(req.ID, req.Protected)
		if err != nil {
			jsonError(w, "Failed to update node protection", http.StatusInternalServerError)
			return
		}
		if node == nil {
			jsonError(w, "Knowledge graph node not found", http.StatusNotFound)
			return
		}

		writeJSON(w, map[string]interface{}{"status": "ok", "node": node})
	}
}

func handleKnowledgeGraphEdgeMutate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.KG == nil {
			jsonError(w, "Knowledge graph is unavailable", http.StatusServiceUnavailable)
			return
		}

		switch r.Method {
		case http.MethodPut:
			var req struct {
				Source      string            `json:"source"`
				Target      string            `json:"target"`
				Relation    string            `json:"relation"`
				NewRelation string            `json:"new_relation"`
				Properties  map[string]string `json:"properties"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Source) == "" || strings.TrimSpace(req.Target) == "" || strings.TrimSpace(req.Relation) == "" {
				jsonError(w, "source, target, and relation are required", http.StatusBadRequest)
				return
			}

			edge, err := s.KG.UpdateEdge(req.Source, req.Target, req.Relation, req.NewRelation, req.Properties)
			if err != nil {
				jsonError(w, "Failed to update knowledge graph edge", http.StatusInternalServerError)
				return
			}
			if edge == nil {
				jsonError(w, "Knowledge graph edge not found", http.StatusNotFound)
				return
			}

			writeJSON(w, map[string]interface{}{"status": "ok", "edge": edge})

		case http.MethodDelete:
			source := strings.TrimSpace(r.URL.Query().Get("source"))
			target := strings.TrimSpace(r.URL.Query().Get("target"))
			relation := strings.TrimSpace(r.URL.Query().Get("relation"))
			if source == "" || target == "" || relation == "" {
				jsonError(w, "source, target, and relation are required", http.StatusBadRequest)
				return
			}

			if err := s.KG.DeleteEdge(source, target, relation); err != nil {
				jsonError(w, "Failed to delete knowledge graph edge", http.StatusInternalServerError)
				return
			}

			writeJSON(w, map[string]interface{}{
				"status":   "ok",
				"source":   source,
				"target":   target,
				"relation": relation,
			})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func parseKnowledgeGraphLimit(r *http.Request, defaultLimit int) int {
	limit := defaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit <= 0 {
		return defaultLimit
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}
