package tools

import (
	"fmt"

	"aurago/internal/memory"
)

// ManageCoreMemory handles all CRUD operations on the SQLite-backed core memory.
//
// Supported operations:
//
//	add     – insert a new fact (returns assigned id).
//	update  – overwrite an existing entry by id.
//	delete  – remove an entry by id.
//	remove  – backward-compat: find entry by exact text and delete it.
//	list    – return all entries as a JSON array string.
//
// maxEntries / capMode:
//
//	capMode = "hard"  → reject add when COUNT(*) >= maxEntries.
//	capMode = "soft"  → allow add but include a warning in the response.
func ManageCoreMemory(operation, fact string, id int64, stm *memory.SQLiteMemory, maxEntries int, capMode string) (string, error) {
	switch operation {

	case "add", "save", "store", "set":
		if stm.CoreMemoryFactExists(fact) {
			return `{"status":"success","message":"Fact already exists in core memory."}`, nil
		}

		count, err := stm.GetCoreMemoryCount()
		if err != nil {
			return "", fmt.Errorf("core memory count check: %w", err)
		}
		if maxEntries > 0 && count >= maxEntries {
			if capMode == "hard" {
				return fmt.Sprintf(`{"status":"error","message":"Core memory is full (%d/%d entries). Delete outdated entries first."}`, count, maxEntries), nil
			}
			// soft cap: proceed but warn
			newID, err := stm.AddCoreMemoryFact(fact)
			if err != nil {
				return "", fmt.Errorf("core memory add: %w", err)
			}
			return fmt.Sprintf(`{"status":"success","id":%d,"message":"Fact added (soft-cap warning: %d/%d entries used). Consider removing outdated entries."}`, newID, count+1, maxEntries), nil
		}

		newID, err := stm.AddCoreMemoryFact(fact)
		if err != nil {
			return "", fmt.Errorf("core memory add: %w", err)
		}
		return fmt.Sprintf(`{"status":"success","id":%d,"message":"Fact permanently added to core memory."}`, newID), nil

	case "update":
		if id <= 0 {
			return `{"status":"error","message":"'id' is required for operation 'update'. Use the numeric id shown in [brackets] before each memory entry."}`, nil
		}
		if fact == "" {
			return `{"status":"error","message":"'fact' is required for operation 'update'."}`, nil
		}
		if err := stm.UpdateCoreMemoryFact(id, fact); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%v"}`, err), nil
		}
		return fmt.Sprintf(`{"status":"success","id":%d,"message":"Core memory entry updated."}`, id), nil

	case "delete":
		if id <= 0 {
			return `{"status":"error","message":"'id' is required for operation 'delete'. Use the numeric id shown in [brackets] before each memory entry."}`, nil
		}
		if err := stm.DeleteCoreMemoryFact(id); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%v"}`, err), nil
		}
		return fmt.Sprintf(`{"status":"success","id":%d,"message":"Core memory entry deleted."}`, id), nil

	case "remove":
		// Backward-compatible text-based deletion.
		if fact == "" {
			return `{"status":"error","message":"'fact' text is required for operation 'remove'."}`, nil
		}
		foundID, err := stm.FindCoreMemoryIDByFact(fact)
		if err != nil {
			return `{"status":"warning","message":"Fact not found in core memory."}`, nil
		}
		if delErr := stm.DeleteCoreMemoryFact(foundID); delErr != nil {
			return fmt.Sprintf(`{"status":"error","message":"%v"}`, delErr), nil
		}
		return fmt.Sprintf(`{"status":"success","id":%d,"message":"Fact permanently removed from core memory."}`, foundID), nil

	case "list":
		text := stm.ReadCoreMemory()
		if text == "" {
			return `{"status":"success","entries":[]}`, nil
		}
		return fmt.Sprintf(`{"status":"success","entries":%q}`, text), nil

	default:
		return "", fmt.Errorf("unsupported operation '%s'. Use 'add', 'update', 'delete', 'remove', or 'list'", operation)
	}
}
