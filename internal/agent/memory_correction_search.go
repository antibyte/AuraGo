package agent

import (
	"fmt"
	"sort"
	"strings"

	"aurago/internal/memory"
)

func appendCorrectionProfileResults(stm *memory.SQLiteMemory, query string, limit int, bundle *memorySearchBundle) {
	if stm == nil {
		return
	}
	entries, err := stm.GetProfileEntries("")
	if err != nil {
		bundle.Errors = append(bundle.Errors, fmt.Sprintf("user_profile: %v", err))
		return
	}
	var data []memory.ProfileEntry
	var refs []memory.MemoryReference
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Category+" "+entry.Key+" "+entry.Value), strings.ToLower(query)) {
			data = append(data, entry)
			refs = append(refs, memory.MemoryReference{Store: "profile", Category: entry.Category, Key: entry.Key})
			if len(data) >= limit {
				break
			}
		}
	}
	if len(data) > 0 {
		bundle.Results = append(bundle.Results, memorySourceResult{Source: "user_profile", Count: len(data), Data: data, References: refs})
	}
}

func correctionVectorReferences(ids []string, stm *memory.SQLiteMemory, ltm memory.VectorDB) []memory.MemoryReference {
	meta, err := loadMemoryMetaMap(stm, ids)
	if err != nil {
		return nil
	}
	var refs []memory.MemoryReference
	for _, id := range ids {
		if memory.IsMemoryArchived(meta[id]) {
			continue
		}
		// SearchSimilar also includes documents that must never be edited as memories.
		if _, err := ltm.GetByIDFromCollection(id, "aurago_memories"); err == nil {
			refs = append(refs, memory.MemoryReference{Store: "ltm", ID: id})
		}
	}
	return refs
}

func correctionGraphReferences(result memory.KnowledgeGraphContextResult) []memory.MemoryReference {
	var refs []memory.MemoryReference
	seen := map[string]bool{}
	for _, node := range result.Nodes {
		refs = append(refs, memory.MemoryReference{Store: "kg_node", ID: node.ID, Field: "label"})
		keys := make([]string, 0, len(node.Properties))
		for key := range node.Properties {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			refs = append(refs, memory.MemoryReference{Store: "kg_node", ID: node.ID, Field: key})
		}
		for _, edge := range result.EdgesByNodeID[node.ID] {
			key := edge.Source + "\x00" + edge.Target + "\x00" + edge.Relation
			if !seen[key] {
				refs = append(refs, memory.MemoryReference{Store: "kg_edge", ID: edge.Source, Target: edge.Target, Relation: edge.Relation})
				seen[key] = true
			}
		}
	}
	return refs
}
