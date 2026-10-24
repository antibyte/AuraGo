package ui

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// Fixture routes of the Plan C panels: source hosts (Docker engine -> device), ignoring and restoring
// sources, deleting assets (optionally ignoring their sources) and deleting connections. They mirror the
// server contract: PUT /source-hosts, PUT /sources/ignore, DELETE /nodes/{id}?version=N&ignore_sources=1,
// DELETE /edges/{id}?version=N, and a snapshot whose sources carry `ignored` and whose source_hosts list
// every Docker instance.

func infraFixtureSourceKey(source map[string]any) string {
	return fmt.Sprint(source["kind"]) + "\x00" + fmt.Sprint(source["instance"]) + "\x00" + fmt.Sprint(source["key"])
}

// sourceRows lists the sources of the live assets plus every ignored source whose asset is gone.
func (f *infraAppBrowserFixture) sourceRows() []any {
	seen := map[string]bool{}
	rows := []any{}
	for _, node := range f.nodes {
		list, _ := node["sources"].([]any)
		for _, item := range list {
			source, ok := item.(map[string]any)
			if !ok || seen[infraFixtureSourceKey(source)] {
				continue
			}
			seen[infraFixtureSourceKey(source)] = true
			row := cloneInfraBrowserNode(source)
			row["ignored"] = f.ignoredRows[infraFixtureSourceKey(source)] != nil
			rows = append(rows, row)
		}
	}
	keys := make([]string, 0, len(f.ignoredRows))
	for key := range f.ignoredRows {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		row := cloneInfraBrowserNode(f.ignoredRows[key])
		row["ignored"] = true
		rows = append(rows, row)
	}
	return rows
}

// sourceHostRows lists every Docker instance of the live assets with its mapped device.
func (f *infraAppBrowserFixture) sourceHostRows() []any {
	rows := []any{}
	seen := map[string]bool{}
	add := func(instance string) {
		if seen[instance] {
			return
		}
		seen[instance] = true
		row := map[string]any{"kind": "docker", "instance": instance, "node_id": "", "version": 0, "inferred_node_id": ""}
		if mapped := f.sourceHosts[instance]; mapped != nil {
			row = mapped
		}
		rows = append(rows, row)
	}
	for _, node := range f.nodes {
		list, _ := node["sources"].([]any)
		for _, item := range list {
			if source, ok := item.(map[string]any); ok && source["kind"] == "docker" {
				add(fmt.Sprint(source["instance"]))
			}
		}
	}
	for instance := range f.sourceHosts {
		add(instance)
	}
	return rows
}

// projectHosts copies the mapped device onto the guests of a Docker instance (the server's host_id).
func (f *infraAppBrowserFixture) projectHosts(instance, nodeID string) {
	for _, node := range f.nodes {
		list, _ := node["sources"].([]any)
		for _, item := range list {
			if source, ok := item.(map[string]any); ok && source["kind"] == "docker" && fmt.Sprint(source["instance"]) == instance {
				if nodeID == "" {
					delete(node, "host_id")
				} else {
					node["host_id"] = nodeID
				}
			}
		}
	}
}

// serveExtras handles the Plan C routes; it answers false for everything else.
func (f *infraAppBrowserFixture) serveExtras(path string, r *http.Request, write func(int, any), decode func(any) bool) bool {
	switch {
	case path == "/source-hosts" && r.Method == http.MethodPut:
		var body map[string]any
		if !decode(&body) {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return true
		}
		f.sourceHostPuts = append(f.sourceHostPuts, body)
		instance := fmt.Sprint(body["instance"])
		if f.sourceHosts == nil {
			f.sourceHosts = map[string]map[string]any{}
		}
		saved := map[string]any{"kind": "docker", "instance": instance, "node_id": body["node_id"], "version": infraFixtureNumber(body["version"]) + 1, "inferred_node_id": ""}
		f.sourceHosts[instance] = saved
		f.projectHosts(instance, fmt.Sprint(body["node_id"]))
		write(http.StatusOK, map[string]any{"source_host": saved})
		return true
	case path == "/sources/ignore" && r.Method == http.MethodPut:
		var body struct {
			Sources []map[string]any `json:"sources"`
			Ignored bool             `json:"ignored"`
		}
		if !decode(&body) {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return true
		}
		if f.ignoredRows == nil {
			f.ignoredRows = map[string]map[string]any{}
		}
		f.ignoreCalls = append(f.ignoreCalls, map[string]any{"sources": body.Sources, "ignored": body.Ignored})
		answer := []any{}
		for _, identity := range body.Sources {
			key := infraFixtureSourceKey(identity)
			row := map[string]any{"kind": identity["kind"], "instance": identity["instance"], "key": identity["key"], "name": identity["key"], "state": "current", "observed": map[string]any{}}
			for _, node := range f.nodes {
				list, _ := node["sources"].([]any)
				for _, item := range list {
					if source, ok := item.(map[string]any); ok && infraFixtureSourceKey(source) == key {
						row = cloneInfraBrowserNode(source)
					}
				}
			}
			for _, candidate := range f.candidates {
				if source, ok := candidate["source"].(map[string]any); ok && infraFixtureSourceKey(source) == key {
					row = cloneInfraBrowserNode(source)
				}
			}
			if previous := f.ignoredRows[key]; previous != nil {
				row = previous
			}
			if body.Ignored {
				f.ignoredRows[key] = row
			} else {
				delete(f.ignoredRows, key)
			}
			out := cloneInfraBrowserNode(row)
			out["ignored"] = body.Ignored
			answer = append(answer, out)
		}
		write(http.StatusOK, map[string]any{"sources": answer})
		return true
	case strings.HasPrefix(path, "/nodes/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(path, "/nodes/")
		query, _ := url.ParseQuery(r.URL.RawQuery)
		f.nodeDeletes = append(f.nodeDeletes, id+"?"+r.URL.RawQuery)
		for index, node := range f.nodes {
			if node["id"] != id {
				continue
			}
			if fmt.Sprint(infraFixtureNumber(node["version"])) != query.Get("version") {
				write(http.StatusConflict, map[string]string{"error": "conflict"})
				return true
			}
			ignored := []any{}
			if query.Get("ignore_sources") == "1" {
				if f.ignoredRows == nil {
					f.ignoredRows = map[string]map[string]any{}
				}
				list, _ := node["sources"].([]any)
				for _, item := range list {
					if source, ok := item.(map[string]any); ok {
						f.ignoredRows[infraFixtureSourceKey(source)] = cloneInfraBrowserNode(source)
						ignored = append(ignored, map[string]any{"kind": source["kind"], "instance": source["instance"], "key": source["key"]})
					}
				}
			}
			f.nodes = append(f.nodes[:index], f.nodes[index+1:]...)
			kept := f.edges[:0]
			for _, edge := range f.edges {
				if edge["from"] != id && edge["to"] != id {
					kept = append(kept, edge)
				}
			}
			f.edges = kept
			write(http.StatusOK, map[string]any{"status": "deleted", "id": id, "ignored": ignored})
			return true
		}
		write(http.StatusNotFound, map[string]string{"error": "not_found"})
		return true
	case strings.HasPrefix(path, "/edges/") && r.Method == http.MethodDelete:
		id := strings.TrimPrefix(path, "/edges/")
		for index, edge := range f.edges {
			if edge["id"] == id {
				f.edges = append(f.edges[:index], f.edges[index+1:]...)
				write(http.StatusOK, map[string]any{"status": "deleted", "id": id})
				return true
			}
		}
		write(http.StatusNotFound, map[string]string{"error": "not_found"})
		return true
	}
	return false
}
