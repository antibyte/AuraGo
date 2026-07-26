package huggingface

import (
	"context"
	"testing"
)

func TestJobStoreInitializesUpsertsAndLimits(t *testing.T) {
	store, err := OpenJobStore(t.TempDir())
	if err != nil {
		t.Fatalf("OpenJobStore() error = %v", err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.RecordJob(ctx, JobRecord{HFJobID: "job-1", Operation: "job_run_python", Status: "pending"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordJob(ctx, JobRecord{HFJobID: "job-2", Operation: "job_run_container", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordJob(ctx, JobRecord{HFJobID: "job-1", Operation: "job_run_python", Status: "complete"}); err != nil {
		t.Fatal(err)
	}

	jobs, err := store.ListJobs(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 {
		t.Fatalf("ListJobs() = %#v", jobs)
	}
	var foundUpdated bool
	for _, job := range jobs {
		if job.HFJobID == "job-1" && job.Status == "complete" {
			foundUpdated = true
		}
	}
	if !foundUpdated {
		t.Fatalf("upserted job missing from ListJobs(): %#v", jobs)
	}
	limited, err := store.ListJobs(ctx, 1)
	if err != nil || len(limited) != 1 {
		t.Fatalf("limited ListJobs() = %#v, %v", limited, err)
	}
}
