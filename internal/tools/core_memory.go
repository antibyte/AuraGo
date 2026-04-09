package tools

import (
	"encoding/json"
	"fmt"

	"aurago/internal/i18n"
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
//
// The lang parameter is used for i18n of user-facing messages. If empty, English is used.
func ManageCoreMemory(operation, fact string, id int64, stm *memory.SQLiteMemory, maxEntries int, capMode string, lang string) (string, error) {
	switch operation {

	case "add", "save", "store", "set":
		if stm.CoreMemoryFactExists(fact) {
			return `{"status":"success","message":"` + i18n.T(lang, "tools.core_memory_fact_exists") + `"}`, nil
		}

		count, err := stm.GetCoreMemoryCount()
		if err != nil {
			return "", fmt.Errorf("core memory count check: %w", err)
		}
		if maxEntries > 0 && count >= maxEntries {
			if capMode == "hard" {
				return fmt.Sprintf(`{"status":"error","message":"%s"}`, i18n.T(lang, "tools.core_memory_full", count, maxEntries)), nil
			}
			// soft cap: proceed but warn
			newID, err := stm.AddCoreMemoryFact(fact)
			if err != nil {
				return "", fmt.Errorf("core memory add: %w", err)
			}
			return fmt.Sprintf(`{"status":"success","id":%d,"message":"%s"}`, newID, i18n.T(lang, "tools.core_memory_soft_cap_warning", count+1, maxEntries)), nil
		}

		newID, err := stm.AddCoreMemoryFact(fact)
		if err != nil {
			return "", fmt.Errorf("core memory add: %w", err)
		}
		return fmt.Sprintf(`{"status":"success","id":%d,"message":"%s"}`, newID, i18n.T(lang, "tools.core_memory_fact_added")), nil

	case "update":
		if id <= 0 {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, i18n.T(lang, "tools.core_memory_update_id_required")), nil
		}
		if fact == "" {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, i18n.T(lang, "tools.core_memory_update_fact_required")), nil
		}
		if err := stm.UpdateCoreMemoryFact(id, fact); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%v"}`, err), nil
		}
		return fmt.Sprintf(`{"status":"success","id":%d,"message":"%s"}`, id, i18n.T(lang, "tools.core_memory_entry_updated")), nil

	case "delete":
		if id <= 0 {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, i18n.T(lang, "tools.core_memory_delete_id_required")), nil
		}
		if err := stm.DeleteCoreMemoryFact(id); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%v"}`, err), nil
		}
		return fmt.Sprintf(`{"status":"success","id":%d,"message":"%s"}`, id, i18n.T(lang, "tools.core_memory_entry_deleted")), nil

	case "remove":
		// Backward-compatible text-based deletion.
		if fact == "" {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, i18n.T(lang, "tools.core_memory_remove_fact_required")), nil
		}
		foundID, err := stm.FindCoreMemoryIDByFact(fact)
		if err != nil {
			return fmt.Sprintf(`{"status":"warning","message":"%s"}`, i18n.T(lang, "tools.core_memory_fact_not_found")), nil
		}
		if delErr := stm.DeleteCoreMemoryFact(foundID); delErr != nil {
			return fmt.Sprintf(`{"status":"error","message":"%v"}`, delErr), nil
		}
		return fmt.Sprintf(`{"status":"success","id":%d,"message":"%s"}`, foundID, i18n.T(lang, "tools.core_memory_fact_removed")), nil

	case "list":
		text := stm.ReadCoreMemory()
		entriesJSON, err := json.Marshal(text)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, i18n.T(lang, "tools.core_memory_serialize_failed")), nil
		}
		return fmt.Sprintf(`{"status":"success","entries":%s}`, string(entriesJSON)), nil

	default:
		return "", fmt.Errorf("unsupported operation '%s'. Supported: 'add' (or 'store'), 'update', 'delete', 'remove', 'list'", operation)
	}
}
