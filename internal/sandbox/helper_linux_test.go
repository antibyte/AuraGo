//go:build linux

package sandbox

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestRlimitPlanSkipsProcessCount(t *testing.T) {
	t.Setenv("AURAGO_SBX_PROCS", "50")
	t.Setenv("AURAGO_SBX_MEM", "1024")
	resources := plannedRlimits()
	for _, resource := range resources {
		if resource == unix.RLIMIT_NPROC {
			t.Fatal("RLIMIT_NPROC must not be applied")
		}
	}
	if !containsResource(resources, unix.RLIMIT_AS) {
		t.Fatal("expected RLIMIT_AS")
	}
}

func containsResource(resources []int, want int) bool {
	for _, resource := range resources {
		if resource == want {
			return true
		}
	}
	return false
}
