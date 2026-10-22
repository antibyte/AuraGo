package gamemaker

import (
	"context"
	"errors"
	"testing"
)

func TestClampRepairPasses(t *testing.T) {
	for _, tc := range []struct{ input, want int }{
		{0, 3}, {-4, 3}, {1, 1}, {3, 3}, {6, 6}, {9, 6},
	} {
		if got := clampRepairPasses(tc.input); got != tc.want {
			t.Errorf("clampRepairPasses(%d) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestRepairLimitComesFromTheStartRequest(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage != "planning" {
			return errors.New("unexpected stage")
		}
		if limit := s.repairLimit(run.Job.ID); limit != 5 {
			t.Fatalf("repair limit = %d, want 5", limit)
		}
		if remaining := s.RemainingRepairs(run.Job.ID); remaining != 5 {
			t.Fatalf("remaining repairs = %d, want 5", remaining)
		}
		return errors.New("repair limit wired")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{MaxRepairPasses: 5})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "repair limit wired" {
		t.Fatalf("job = %+v", done)
	}
}

func TestRepairLimitDefaultsAndClampsFromTheRequest(t *testing.T) {
	for _, tc := range []struct{ request, want int }{
		{0, defaultRepairPasses},
		{2, 2},
		{99, maxRepairPasses},
	} {
		s := newTestService(t)
		project := createTestProject(t, s, "2d")
		s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
			if run.Stage != "planning" {
				return errors.New("unexpected stage")
			}
			if limit := s.repairLimit(run.Job.ID); limit != tc.want {
				t.Fatalf("request %d: repair limit = %d, want %d", tc.request, limit, tc.want)
			}
			return errors.New("repair limit clamp verified")
		}))
		job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{MaxRepairPasses: tc.request})
		if err != nil {
			t.Fatal(err)
		}
		if done := waitJob(t, s, job.ID); done.Error != "repair limit clamp verified" {
			t.Fatalf("request %d: job = %+v", tc.request, done)
		}
	}
}
