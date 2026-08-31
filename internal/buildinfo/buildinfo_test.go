package buildinfo

import "testing"

func TestCurrentPrefersReleaseBuilderMetadata(t *testing.T) {
	originalID := BuildID
	originalRevision := BuildVCSRevision
	originalTime := BuildVCSTime
	originalModified := BuildVCSModified
	t.Cleanup(func() {
		BuildID = originalID
		BuildVCSRevision = originalRevision
		BuildVCSTime = originalTime
		BuildVCSModified = originalModified
	})

	BuildID = "abc123"
	BuildVCSRevision = "abc123"
	BuildVCSTime = "2026-08-29T08:00:00Z"
	BuildVCSModified = "false"

	got := Current()
	if got.BuildID != BuildID || got.VCSRevision != BuildVCSRevision || got.VCSTime != BuildVCSTime {
		t.Fatalf("Current() = %+v, want release builder metadata", got)
	}
	if got.VCSModified {
		t.Fatalf("Current().VCSModified = true, want false")
	}
}

func TestCurrentMarksExplicitDirtyBuild(t *testing.T) {
	originalID := BuildID
	originalRevision := BuildVCSRevision
	originalTime := BuildVCSTime
	originalModified := BuildVCSModified
	t.Cleanup(func() {
		BuildID = originalID
		BuildVCSRevision = originalRevision
		BuildVCSTime = originalTime
		BuildVCSModified = originalModified
	})

	BuildID = "abc123"
	BuildVCSRevision = "abc123"
	BuildVCSTime = "2026-08-29T08:00:00Z"
	BuildVCSModified = "true"

	got := Current()
	if !got.VCSModified || got.BuildID != "abc123-dirty" {
		t.Fatalf("Current() = %+v, want explicit dirty build", got)
	}
}
