package kaggle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func TestKaggleNotFound(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"404", &APIError{Status: 404}, true},
		{"403", &APIError{Status: 403}, true},
		{"wrapped 403", fmt.Errorf("read notebook: %w", &APIError{Status: 403}), true},
		{"401", &APIError{Status: 401}, false},
		{"429", &APIError{Status: 429}, false},
		{"500", &APIError{Status: 500}, false},
		{"uncertain 404", &APIError{Status: 404, Uncertain: true}, false},
		{"uncertain 403", &APIError{Status: 403, Uncertain: true}, false},
		{"nil", nil, false},
		{"plain error", errors.New("boom"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NotFound(tc.err); got != tc.want {
				t.Fatalf("NotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestKaggleSaveKernelCreatesWhenKaggleDeniesTheMissingNotebook(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		slug        string
		allowCreate bool
		allowOwners []string
		runAll      bool
		busy        bool
		wantErr     bool
	}{
		{name: "own account 403 creates", status: 403, slug: "owner/new-book", allowCreate: true},
		{name: "own account 403 owner case differs", status: 403, slug: "Owner/new-book", allowCreate: true},
		{name: "own account 404 creates", status: 404, slug: "owner/new-book", allowCreate: true},
		{name: "own account 403 without create", status: 403, slug: "owner/new-book", wantErr: true},
		{name: "own account 404 without create", status: 404, slug: "owner/new-book", wantErr: true},
		{name: "other allowed owner 403 stays an error", status: 403, slug: "friend/new-book", allowCreate: true, allowOwners: []string{"friend"}, wantErr: true},
		{name: "own account 500 stays an error", status: 500, slug: "owner/new-book", allowCreate: true, wantErr: true},
		{name: "own account 401 stays an error", status: 401, slug: "owner/new-book", allowCreate: true, wantErr: true},
		{name: "run after create still checks compute slots", status: 403, slug: "owner/new-book", allowCreate: true, runAll: true, busy: true, wantErr: true},
		{name: "run after create with a free slot", status: 403, slug: "owner/new-book", allowCreate: true, runAll: true},
	}
	op, err := Lookup("notebooks", "save_kernel")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestService(t)
			c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				io.WriteString(w, `{"error":{"code":403,"message":"Permission 'kernels.get' was denied","status":"PERMISSION_DENIED"}}`)
			})
			cfg := s.current().Config
			cfg.AllowCreate = tc.allowCreate
			cfg.AllowedOwners = tc.allowOwners
			cfg.MaxRemoteJobs = 1
			params := map[string]any{"slug": tc.slug}
			if tc.runAll {
				params["kernelExecutionType"] = "SAVE_AND_RUN_ALL"
			}
			if tc.busy {
				request, _ := json.Marshal(Request{Domain: "notebooks", Operation: "save_kernel", Parameters: json.RawMessage(`{"kernelExecutionType":"SAVE_AND_RUN_ALL"}`)})
				for i := 0; i < 2; i++ {
					job := Job{ID: newID(), Account: "owner", Domain: "notebooks", Operation: "save_kernel", Hash: fmt.Sprintf("hash-%d", i), Request: request}
					if _, _, err := s.Store.admit(context.Background(), job); err != nil {
						t.Fatal(err)
					}
				}
			}
			err := s.checkResource(context.Background(), c, cfg, op, params, "owner")
			if tc.wantErr && err == nil {
				t.Fatal("expected an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
