package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
)

func terminalPlanTaskStatus(status string) bool {
	return status == PlanTaskCompleted || status == PlanTaskFailed || status == PlanTaskSkipped
}

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
	return invalidPlanDependenciesTxWithStatus(tx, planID, "", "")
}

func invalidPlanDependenciesTxWithStatus(tx *sql.Tx, planID, overrideTaskID, overrideStatus string) (string, error) {
	rows, err := tx.Query(`SELECT id,status,depends_on_json FROM plan_tasks WHERE plan_id=? ORDER BY task_order`, planID)
	if err != nil {
		return "", fmt.Errorf("read plan dependencies: %w", err)
	}
	defer rows.Close()
	dependencies := map[string][]string{}
	for rows.Next() {
		var id, status, raw string
		if err := rows.Scan(&id, &status, &raw); err != nil {
			return "", fmt.Errorf("scan plan dependencies: %w", err)
		}
		if id == overrideTaskID {
			status = overrideStatus
		}
		// Completed and skipped tasks are terminal leaves. Their historical
		// dependency data must not prevent the remaining plan from progressing.
		if status == PlanTaskCompleted || status == PlanTaskSkipped {
			dependencies[id] = nil
			continue
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

func planTaskDependenciesReadyTx(tx *sql.Tx, planID, taskID string) (bool, error) {
	var raw string
	if err := tx.QueryRow(`SELECT depends_on_json FROM plan_tasks WHERE plan_id=? AND id=?`, planID, taskID).Scan(&raw); err != nil {
		return false, fmt.Errorf("read task dependencies: %w", err)
	}
	var dependencies []string
	if err := json.Unmarshal([]byte(raw), &dependencies); err != nil || dependencies == nil {
		return false, nil
	}
	for _, dependencyID := range dependencies {
		var status string
		if err := tx.QueryRow(`SELECT status FROM plan_tasks WHERE plan_id=? AND id=?`, planID, dependencyID).Scan(&status); err != nil {
			return false, nil
		}
		if status != PlanTaskCompleted && status != PlanTaskSkipped {
			return false, nil
		}
	}
	return true, nil
}
