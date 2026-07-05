package memory

import "fmt"

type KGLifecycleCounts struct {
	AcceptedEdges   int `json:"accepted_edges"`
	SupersededEdges int `json:"superseded_edges"`
	RetractedEdges  int `json:"retracted_edges"`
	OpenConflicts   int `json:"open_conflicts"`
}

func (kg *KnowledgeGraph) GetLifecycleCounts() (KGLifecycleCounts, error) {
	var counts KGLifecycleCounts
	if kg == nil || kg.db == nil {
		return counts, fmt.Errorf("knowledge graph not initialized")
	}

	rows, err := kg.db.Query(`
		SELECT status, COUNT(*)
		FROM kg_edges
		GROUP BY status
	`)
	if err != nil {
		return counts, fmt.Errorf("query kg edge lifecycle counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return counts, fmt.Errorf("scan kg edge lifecycle counts: %w", err)
		}
		switch KGClaimStatus(status) {
		case KGClaimAccepted:
			counts.AcceptedEdges = count
		case KGClaimSuperseded:
			counts.SupersededEdges = count
		case KGClaimRetracted:
			counts.RetractedEdges = count
		}
	}
	if err := rows.Err(); err != nil {
		return counts, fmt.Errorf("iterate kg edge lifecycle counts: %w", err)
	}

	if err := kg.db.QueryRow(`
		SELECT COUNT(*)
		FROM kg_conflicts
		WHERE status = 'open'
	`).Scan(&counts.OpenConflicts); err != nil {
		return counts, fmt.Errorf("query open kg conflict count: %w", err)
	}

	return counts, nil
}
