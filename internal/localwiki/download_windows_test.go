//go:build windows

package localwiki

import (
	"context"
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

// On Windows another open handle on the part file (an antivirus scanner, a
// backup tool) blocks the rename that replaces it with the restart file. The
// job ends with a write error, keeps the part file and removes the restart
// file.
func TestDownloadJobRestartRenameBlockedOnWindows(t *testing.T) {
	const kept = 100_000
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	e.setMode("m1", "ignore-range")
	e.setMode("m2", "fail")
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	if err := os.WriteFile(job.partPath, e.data[:kept], 0o644); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(job.partPath)
	if err != nil {
		t.Fatal(err)
	}
	// Read and write sharing lets the job open the part file; without delete
	// sharing nothing can replace it while this handle is open.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	err = job.run(context.Background())
	_ = windows.CloseHandle(handle)
	var write *writeError
	if !errors.As(err, &write) {
		t.Fatalf("run = %v, want a writeError", err)
	}
	assertPartEquals(t, job, e.data[:kept])
	assertNoRestartFile(t, job)
}
