package memory

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

type Node struct {
	ID         string            `json:"id"`
	Label      string            `json:"label"`
	Properties map[string]string `json:"properties"`
}

type Edge struct {
	Source     string            `json:"source"`
	Target     string            `json:"target"`
	Relation   string            `json:"relation"`
	Properties map[string]string `json:"properties"`
}

type graphState struct {
	Nodes map[string]Node `json:"nodes"`
	Edges []Edge          `json:"edges"`
}

type KnowledgeGraph struct {
	filePath string
	mu       sync.RWMutex
	state    graphState
}

func NewKnowledgeGraph(filePath string) *KnowledgeGraph {
	kg := &KnowledgeGraph{
		filePath: filePath,
		state: graphState{
			Nodes: make(map[string]Node),
			Edges: []Edge{},
		},
	}
	kg.load()
	return kg
}

func (kg *KnowledgeGraph) Close() error {
	kg.mu.Lock()
	defer kg.mu.Unlock()
	return kg.save()
}

func (kg *KnowledgeGraph) load() {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	file, err := os.Open(kg.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return // Start fresh
		}
		fmt.Printf("Error opening graph file: %v\n", err)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		fmt.Printf("Error reading graph file: %v\n", err)
		return
	}

	if len(data) > 0 {
		var newState graphState
		if err := json.Unmarshal(data, &newState); err == nil {
			if newState.Nodes == nil {
				newState.Nodes = make(map[string]Node)
			}
			kg.state = newState
		}
	}
}

func (kg *KnowledgeGraph) save() error {
	data, err := json.MarshalIndent(kg.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(kg.filePath, data, 0644)
}

// Stats returns the number of nodes and edges in the knowledge graph.
func (kg *KnowledgeGraph) Stats() (nodes int, edges int) {
	kg.mu.RLock()
	defer kg.mu.RUnlock()
	return len(kg.state.Nodes), len(kg.state.Edges)
}

func (kg *KnowledgeGraph) AddNode(id, label string, properties map[string]string) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	if properties == nil {
		properties = make(map[string]string)
	}

	kg.state.Nodes[id] = Node{
		ID:         id,
		Label:      label,
		Properties: properties,
	}

	return kg.save()
}

func (kg *KnowledgeGraph) AddEdge(source, target, relation string, properties map[string]string) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	// Ensure nodes exist before adding an edge
	if _, ok := kg.state.Nodes[source]; !ok {
		kg.state.Nodes[source] = Node{ID: source, Label: "Unknown", Properties: make(map[string]string)}
	}
	if _, ok := kg.state.Nodes[target]; !ok {
		kg.state.Nodes[target] = Node{ID: target, Label: "Unknown", Properties: make(map[string]string)}
	}

	if properties == nil {
		properties = make(map[string]string)
	}

	// Update existing edge or append new
	edgeExists := false
	for i, e := range kg.state.Edges {
		if e.Source == source && e.Target == target && e.Relation == relation {
			kg.state.Edges[i].Properties = properties
			edgeExists = true
			break
		}
	}

	if !edgeExists {
		kg.state.Edges = append(kg.state.Edges, Edge{
			Source:     source,
			Target:     target,
			Relation:   relation,
			Properties: properties,
		})
	}

	return kg.save()
}

// Search returns a JSON string representation of nodes and edges that match the query term.
// Access counts are updated asynchronously to avoid blocking readers.
func (kg *KnowledgeGraph) Search(query string) string {
	kg.mu.RLock()

	query = strings.ToLower(query)
	var matchedNodeIDs []string
	var matchedEdgeIdxs []int
	var matchedNodes []Node
	var matchedEdges []Edge

	// Search Nodes (read-only pass)
	for _, n := range kg.state.Nodes {
		match := false
		if strings.Contains(strings.ToLower(n.ID), query) || strings.Contains(strings.ToLower(n.Label), query) {
			match = true
		} else {
			for k, v := range n.Properties {
				if strings.Contains(strings.ToLower(k), query) || strings.Contains(strings.ToLower(v), query) {
					match = true
					break
				}
			}
		}
		if match {
			matchedNodeIDs = append(matchedNodeIDs, n.ID)
			matchedNodes = append(matchedNodes, n)
		}
	}

	// Search Edges (read-only pass)
	for i, e := range kg.state.Edges {
		match := false
		if strings.Contains(strings.ToLower(e.Source), query) ||
			strings.Contains(strings.ToLower(e.Target), query) ||
			strings.Contains(strings.ToLower(e.Relation), query) {
			match = true
		} else {
			for k, v := range e.Properties {
				if strings.Contains(strings.ToLower(k), query) || strings.Contains(strings.ToLower(v), query) {
					match = true
					break
				}
			}
		}
		if match {
			matchedEdgeIdxs = append(matchedEdgeIdxs, i)
			matchedEdges = append(matchedEdges, e)
		}
	}
	kg.mu.RUnlock()

	if len(matchedNodes) == 0 && len(matchedEdges) == 0 {
		return "[]"
	}

	result := map[string]interface{}{
		"nodes": matchedNodes,
		"edges": matchedEdges,
	}
	data, _ := json.Marshal(result)

	// Update access counts asynchronously (write path)
	go func() {
		kg.mu.Lock()
		defer kg.mu.Unlock()
		for _, id := range matchedNodeIDs {
			if n, ok := kg.state.Nodes[id]; ok {
				count := 0
				if countStr := n.Properties["access_count"]; countStr != "" {
					fmt.Sscanf(countStr, "%d", &count)
				}
				n.Properties["access_count"] = fmt.Sprintf("%d", count+1)
				kg.state.Nodes[id] = n
			}
		}
		for _, idx := range matchedEdgeIdxs {
			if idx < len(kg.state.Edges) {
				e := kg.state.Edges[idx]
				count := 0
				if countStr := e.Properties["access_count"]; countStr != "" {
					fmt.Sscanf(countStr, "%d", &count)
				}
				e.Properties["access_count"] = fmt.Sprintf("%d", count+1)
				kg.state.Edges[idx] = e
			}
		}
		kg.save()
	}()

	return string(data)
}

// DeleteNode removes a node and all its connected edges from the graph.
func (kg *KnowledgeGraph) DeleteNode(id string) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	// 1. Delete Node
	delete(kg.state.Nodes, id)

	// 2. Delete all connected Edges
	var activeEdges []Edge
	for _, e := range kg.state.Edges {
		if e.Source != id && e.Target != id {
			activeEdges = append(activeEdges, e)
		}
	}
	kg.state.Edges = activeEdges

	return kg.save()
}

// DeleteEdge removes a specific edge from the graph.
func (kg *KnowledgeGraph) DeleteEdge(source, target, relation string) error {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	var activeEdges []Edge
	for _, e := range kg.state.Edges {
		if !(e.Source == source && e.Target == target && e.Relation == relation) {
			activeEdges = append(activeEdges, e)
		}
	}
	kg.state.Edges = activeEdges

	return kg.save()
}

// OptimizeGraph evaluates all nodes and archives those whose priority score falls below the threshold.
// Priority = (access_count) + (degree * 2). Nodes with properties["protected"] == "true" are skipped.
func (kg *KnowledgeGraph) OptimizeGraph(threshold int) (int, error) {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	// 1. Calculate degrees (number of connected edges) for each node
	degrees := make(map[string]int)
	for _, e := range kg.state.Edges {
		degrees[e.Source]++
		degrees[e.Target]++
	}

	var nodesToRemove []Node
	var edgesToRemove []Edge

	var activeNodes = make(map[string]Node)
	var activeEdges []Edge

	// 2. Evaluate Nodes
	for id, n := range kg.state.Nodes {
		if n.Properties["protected"] == "true" {
			activeNodes[id] = n
			continue
		}

		count := 0
		if countStr := n.Properties["access_count"]; countStr != "" {
			fmt.Sscanf(countStr, "%d", &count)
		}

		priority := count + (degrees[id] * 2)

		if priority < threshold {
			nodesToRemove = append(nodesToRemove, n)
		} else {
			activeNodes[id] = n
		}
	}

	// 3. Evaluate Edges (remove if either source or target is removed)
	for _, e := range kg.state.Edges {
		_, sourceActive := activeNodes[e.Source]
		_, targetActive := activeNodes[e.Target]

		if !sourceActive || !targetActive {
			edgesToRemove = append(edgesToRemove, e)
		} else {
			activeEdges = append(activeEdges, e)
		}
	}

	if len(nodesToRemove) == 0 {
		return 0, nil
	}

	// 4. Archive removed items
	archivePath := strings.Replace(kg.filePath, ".json", "_archive.json", 1)
	kg.appendArchive(archivePath, nodesToRemove, edgesToRemove)

	// 5. Update state
	kg.state.Nodes = activeNodes
	kg.state.Edges = activeEdges

	err := kg.save()
	return len(nodesToRemove), err
}

// maxArchiveNodes limits how many nodes the archive retains to prevent unbounded growth.
const maxArchiveNodes = 500

func (kg *KnowledgeGraph) appendArchive(archivePath string, nodes []Node, edges []Edge) {
	var archive graphState

	// Try to load existing archive
	if data, err := os.ReadFile(archivePath); err == nil {
		json.Unmarshal(data, &archive)
	}
	if archive.Nodes == nil {
		archive.Nodes = make(map[string]Node)
	}

	for _, n := range nodes {
		archive.Nodes[n.ID] = n
	}
	archive.Edges = append(archive.Edges, edges...)

	// Prune archive if it exceeds the cap: remove oldest entries (arbitrary order from map)
	if len(archive.Nodes) > maxArchiveNodes {
		excess := len(archive.Nodes) - maxArchiveNodes
		pruned := make(map[string]bool)
		for id := range archive.Nodes {
			if excess <= 0 {
				break
			}
			delete(archive.Nodes, id)
			pruned[id] = true
			excess--
		}
		// Also remove edges referencing pruned nodes
		var cleanEdges []Edge
		for _, e := range archive.Edges {
			if !pruned[e.Source] && !pruned[e.Target] {
				cleanEdges = append(cleanEdges, e)
			}
		}
		archive.Edges = cleanEdges
	}

	if data, err := json.MarshalIndent(archive, "", "  "); err == nil {
		os.WriteFile(archivePath, data, 0644)
	}
}
