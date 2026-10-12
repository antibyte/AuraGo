package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/gog"
)

func TestGogStorageKeepsDuplicateNamesDistinct(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gog.Accounts = []config.GogAccount{{ID: "first", Services: map[string]config.GogGrants{"drive": {Read: true, Create: true}}}}
	g := &GogStorage{Service: &gog.Service{Config: func() *config.Config { return cfg }}}
	g.run = func(_ context.Context, _ string, r gog.Request, _ *gog.Files) gog.Result {
		if r.AccountID != "first" || r.Args["parent"] != "root" {
			t.Fatal("lost account/parent")
		}
		return gog.Result{Status: "ok", Data: json.RawMessage(`{"files":[{"id":"one","name":"same.txt","mimeType":"text/plain","size":"4","version":"2"},{"id":"two","name":"same.txt","mimeType":"text/plain","size":"4","version":"8"}]}`)}
	}
	loc := StorageLocation{Provider: "gog", AccountID: "first", ItemID: "root"}
	entries, e := g.List(context.Background(), loc)
	if e != nil || len(entries) != 2 || entries[0].Path == entries[1].Path || entries[0].Version != "gog-2" {
		t.Fatalf("bad listing: %+v %v", entries, e)
	}
	loc.ItemID = ""
	loc.Path = "same.txt"
	if _, e = g.resolve(context.Background(), loc); !errors.Is(e, ErrStorageConflict) {
		t.Fatalf("ambiguous name resolved: %v", e)
	}
	loc.ItemID = "two"
	loc.Name = "same.txt"
	resolved, e := g.resolve(context.Background(), loc)
	if e != nil || resolved.ItemID != "two" {
		t.Fatal("ID lost to name")
	}
}

func TestGogStorageRequiresObservedVersionAndRetainsSource(t *testing.T) {
	cfg := &config.Config{}
	g := &GogStorage{Service: &gog.Service{Config: func() *config.Config { return cfg }}}
	writes := 0
	g.run = func(_ context.Context, _ string, r gog.Request, _ *gog.Files) gog.Result {
		if r.Operation != "get" {
			writes++
		}
		return gog.Result{Status: "ok", Data: json.RawMessage(`{"file":{"id":"file","name":"file.txt","mimeType":"text/plain","size":"4","version":"3"}}`)}
	}
	loc := StorageLocation{Provider: "gog", AccountID: "first", ItemID: "file", Path: "file"}
	if _, e := g.Put(context.Background(), loc, bytes.NewBufferString("data"), "gog-2"); !errors.Is(e, ErrStorageConflict) || writes != 0 {
		t.Fatal("stale replacement attempted")
	}
	c := CloudStorage{Gog: g}
	if e := c.Remove(context.Background(), loc, "gog-3", false); !errors.Is(e, ErrStorageSourceRetained) || writes != 0 {
		t.Fatal("source deleted without conditional delete")
	}
	for _, bad := range []StorageLocation{{Provider: "gog", AccountID: "../x"}, {Provider: "gog", AccountID: "first", ItemID: "../x"}, {Provider: "local", AccountID: "first", ItemID: "file"}} {
		if ValidateStorageLocation(bad) == nil {
			t.Fatal("invalid identity accepted")
		}
	}
}

func TestGogPublishIsCreateOnlyAndCancellationSafe(t *testing.T) {
	ctx := WithRuntimePermissions(context.Background(), RuntimePermissions{AllowFilesystemWrite: true})
	target := filepath.Join(t.TempDir(), "copy.docx")
	if e := publishGogFile(ctx, target, []byte("first")); e != nil {
		t.Fatal(e)
	}
	if e := publishGogFile(ctx, target, []byte("second")); e == nil {
		t.Fatal("overwrote destination")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "first" {
		t.Fatal("destination changed")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if e := publishGogFile(cancelled, target+"-new", nil); e == nil {
		t.Fatal("published after cancellation")
	}
	files, _ := os.ReadDir(filepath.Dir(target))
	if len(files) != 1 {
		t.Fatal("staging leaked")
	}
}

func TestGogStorageStopsCyclicPages(t *testing.T) {
	g := &GogStorage{Service: &gog.Service{}}
	calls := 0
	g.run = func(_ context.Context, _ string, r gog.Request, _ *gog.Files) gog.Result {
		calls++
		if calls > 3 {
			t.Fatal("cyclic empty pages kept requesting Google")
		}
		next := "a"
		if r.Args["page"] == "a" {
			next = "b"
		}
		return gog.Result{Status: "ok", Data: json.RawMessage(`{"files":[],"nextPageToken":"` + next + `"}`)}
	}
	_, err := g.List(context.Background(), StorageLocation{AccountID: "first", ItemID: "root"})
	if err == nil || !strings.Contains(err.Error(), "repeated Drive page") {
		t.Fatalf("missing page cycle error: %v", err)
	}
}

func TestGogStorageReportsMutationBeforeFailedFollowup(t *testing.T) {
	for _, operation := range []string{"put", "mkdir", "copy", "move"} {
		t.Run(operation, func(t *testing.T) {
			cfg := &config.Config{}
			g := &GogStorage{Service: &gog.Service{Config: func() *config.Config { return cfg }}}
			writes := 0
			g.run = func(_ context.Context, _ string, r gog.Request, _ *gog.Files) gog.Result {
				switch r.Operation {
				case "get":
					if r.Args["file_id"] == "new" {
						return gog.Failure("provider_unavailable")
					}
					return gog.Result{Status: "ok", Data: json.RawMessage(`{"file":{"id":"source","name":"original.txt","mimeType":"text/plain","size":"4","version":"3","parents":["old-parent"]}}`)}
				case "list":
					return gog.Result{Status: "ok", Data: json.RawMessage(`{"files":[]}`)}
				case "rename":
					return gog.Failure("permission_denied")
				default:
					writes++
					if r.Operation == "mkdir" {
						return gog.Result{Status: "ok", Data: json.RawMessage(`{}`)}
					}
					return gog.Result{Status: "ok", Data: json.RawMessage(`{"file":{"id":"new"}}`)}
				}
			}
			from := StorageLocation{Provider: "gog", AccountID: "first", ItemID: "source", Path: "source"}
			to := StorageLocation{Provider: "gog", AccountID: "first", ParentID: "new-parent", Path: "destination.txt"}
			var err error
			switch operation {
			case "put":
				_, err = g.Put(context.Background(), to, strings.NewReader("data"), "")
			case "mkdir":
				err = g.Mkdir(context.Background(), to)
			case "copy":
				err = g.Copy(context.Background(), from, to, "gog-3")
			case "move":
				err = g.Move(context.Background(), from, to, "gog-3")
			}
			if writes != 1 || err == nil || !strings.Contains(err.Error(), "storage_outcome_unknown") {
				t.Fatalf("mutation was hidden by follow-up failure: writes=%d err=%v", writes, err)
			}
		})
	}
}
