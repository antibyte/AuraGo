package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
)

func invalidPlanDependencies(dependencies map[string][]string) (string, error) {
	state := make(map[string]uint8, len(dependencies))
	var visit func(string) error
	visit = func(id string) error {
		deps, exists := dependencies[id]
		if !exists {
			return fmt.Errorf("dependency %q does not exist", id)
		}
		if state[id] == 1 {
			return fmt.Errorf("dependency cycle at %q", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, dep := range deps {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	ids := make([]string, 0, len(dependencies))
	for id := range dependencies {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := visit(id); err != nil {
			return id, err
		}
	}
	return "", nil
}

func invalidPlanDependenciesTx(tx *sql.Tx, planID string) (string, error) {
	rows, err := tx.Query(`SELECT id,depends_on_json FROM plan_tasks WHERE plan_id=? ORDER BY task_order`, planID)
	if err != nil {
		return "", fmt.Errorf("read plan dependencies: %w", err)
	}
	defer rows.Close()
	dependencies := map[string][]string{}
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return "", fmt.Errorf("scan plan dependencies: %w", err)
		}
		var deps []string
		if err := json.Unmarshal([]byte(raw), &deps); err != nil || deps == nil {
			return id, nil
		}
		dependencies[id] = deps
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate plan dependencies: %w", err)
	}
	id, _ := invalidPlanDependencies(dependencies)
	return id, nil
}
