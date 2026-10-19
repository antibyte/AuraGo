package tools

import (
	"strings"
	"testing"
)

func TestCronCapAllowsExistingReplacementAndRemovalWithSeconds(t *testing.T) {
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	mgr := NewCronManager(tempSystemTaskDir(t))
	defer mgr.Close()
	mgr.SetMaxJobs(1)
	expr := "*/15 * * * * *"
	result, err := mgr.ManageSchedule("add", "existing", expr, "first", "en")
	if err != nil || !strings.Contains(result, "success") {
		t.Fatalf("add: %s %v", result, err)
	}
	if _, err := mgr.ManageSchedule("add", "new", expr, "second", "en"); err == nil {
		t.Fatal("new ID exceeded cap")
	}
	result, err = mgr.ManageSchedule("add", "existing", expr, "replacement", "en")
	if err != nil || !strings.Contains(result, "success") {
		t.Fatalf("replace at cap: %s %v", result, err)
	}
	jobs := mgr.GetJobs()
	if len(jobs) != 1 || jobs[0].CronExpr != expr || jobs[0].TaskPrompt != "replacement" {
		t.Fatalf("replacement lost seconds: %+v", jobs)
	}
	result, err = mgr.ManageSchedule("remove", "existing", "", "", "en")
	if err != nil || !strings.Contains(result, "success") {
		t.Fatalf("remove: %s %v", result, err)
	}
}
