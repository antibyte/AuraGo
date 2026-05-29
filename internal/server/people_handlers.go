package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/contacts"
)

func handlePeopleLookup(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		if s.KG == nil {
			jsonError(w, `{"error":"knowledge graph not initialized"}`, http.StatusServiceUnavailable)
			return
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"nodes": []interface{}{}, "edges": []interface{}{}})
			return
		}
		mode := strings.ToLower(r.URL.Query().Get("mode"))

		var resultMap map[string]interface{}
		if mode == "fts" {
			searchJSON := s.KG.Search(query)
			if strings.TrimSpace(searchJSON) == "[]" {
				resultMap = map[string]interface{}{"nodes": []interface{}{}, "edges": []interface{}{}}
			} else {
				json.Unmarshal([]byte(searchJSON), &resultMap)
			}
		} else {
			exploreJSON := s.KG.Explore(query)
			json.Unmarshal([]byte(exploreJSON), &resultMap)
		}

		if resultMap == nil {
			resultMap = map[string]interface{}{"nodes": []interface{}{}, "edges": []interface{}{}}
		}

		filtered := filterPersonNodes(resultMap)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(filtered)
	}
}

func filterPersonNodes(data map[string]interface{}) map[string]interface{} {
	nodesRaw, _ := data["nodes"].([]interface{})
	edgesRaw, _ := data["edges"].([]interface{})

	var personIDs []string
	var personNodes []interface{}
	for _, n := range nodesRaw {
		node, ok := n.(map[string]interface{})
		if !ok {
			continue
		}
		props, _ := node["properties"].(map[string]interface{})
		nodeType, _ := props["type"].(string)
		nodeID, _ := node["id"].(string)
		if strings.EqualFold(nodeType, "person") || strings.HasPrefix(nodeID, "contact_") {
			personIDs = append(personIDs, nodeID)
			personNodes = append(personNodes, node)
		}
	}

	personSet := make(map[string]bool, len(personIDs))
	for _, id := range personIDs {
		personSet[id] = true
	}

	var personEdges []interface{}
	for _, e := range edgesRaw {
		edge, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		src, _ := edge["source"].(string)
		tgt, _ := edge["target"].(string)
		if personSet[src] || personSet[tgt] {
			personEdges = append(personEdges, edge)
		}
	}

	if personNodes == nil {
		personNodes = []interface{}{}
	}
	if personEdges == nil {
		personEdges = []interface{}{}
	}
	return map[string]interface{}{
		"nodes": personNodes,
		"edges": personEdges,
	}
}

func handlePeopleKGPersons(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		if s.KG == nil {
			jsonError(w, `{"error":"knowledge graph not initialized"}`, http.StatusServiceUnavailable)
			return
		}
		limit := 100
		if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 500 {
			limit = n
		}
		nodes, err := s.KG.GetNodesByType("person", limit)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to get person nodes", "Failed to get KG person nodes", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nodes)
	}
}

func handlePeopleUpcoming(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		if s.ContactsDB == nil {
			jsonError(w, `{"error":"contacts database not initialized"}`, http.StatusServiceUnavailable)
			return
		}
		days := 30
		if n, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && n > 0 && n <= 365 {
			days = n
		}
		result, err := contacts.UpcomingBirthdays(s.ContactsDB, days)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to get upcoming birthdays", "Failed to get upcoming birthdays", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}
