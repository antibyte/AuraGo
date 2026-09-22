package memory

import "testing"

func TestMemoryMaintenanceStateRejectsUnavailableStore(t *testing.T) {
	var stm *SQLiteMemory
	if _, err := stm.GetMemoryMaintenanceState("cursor"); err == nil {
		t.Fatal("GetMemoryMaintenanceState returned nil error for unavailable store")
	}
	if err := stm.SetMemoryMaintenanceState("cursor", "value"); err == nil {
		t.Fatal("SetMemoryMaintenanceState returned nil error for unavailable store")
	}
	if err := stm.ClearMemoryMaintenanceState("cursor"); err == nil {
		t.Fatal("ClearMemoryMaintenanceState returned nil error for unavailable store")
	}
}
