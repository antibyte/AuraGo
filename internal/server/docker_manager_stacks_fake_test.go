package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"aurago/internal/dockerstacks"
)

// dmFakeStacks stands in for *dockerstacks.Service in the handler tests. Every
// call is recorded as "Method arg arg"; errs[method] makes that method fail.
type dmFakeStacks struct {
	mu    sync.Mutex
	calls []string
	errs  map[string]error

	helpers   dockerstacks.HelperStatus
	listing   dockerstacks.Listing
	detail    dockerstacks.StackDetail
	revisions dockerstacks.FileRevisions
	operation dockerstacks.Operation
	history   []dockerstacks.Operation
	volumes   []dockerstacks.ProjectVolume
	ports     []dockerstacks.PublishedPort

	// Subscribe answers with snapshot and events (nil events = an already closed channel).
	snapshot dockerstacks.Operation
	events   chan dockerstacks.Event

	// What the last call received.
	create dockerstacks.CreateInput
	files  dockerstacks.FilesInput
	opts   dockerstacks.OpOptions
	filter dockerstacks.OperationFilter
	kind   string
}

func newDMFakeStacks() *dmFakeStacks { return &dmFakeStacks{errs: map[string]error{}} }

// record notes a call and returns the error configured for its method.
func (f *dmFakeStacks) record(call string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	method, _, _ := strings.Cut(call, " ")
	return f.errs[method]
}

func (f *dmFakeStacks) called(call string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, got := range f.calls {
		if got == call {
			return true
		}
	}
	return false
}

func (f *dmFakeStacks) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *dmFakeStacks) Helpers(_ context.Context, hostID string) (dockerstacks.HelperStatus, error) {
	return f.helpers, f.record("Helpers " + hostID)
}

func (f *dmFakeStacks) Prepare(_ context.Context, hostID string) (dockerstacks.Operation, error) {
	return f.operation, f.record("Prepare " + hostID)
}

func (f *dmFakeStacks) List(_ context.Context, hostID string, refresh bool) (dockerstacks.Listing, error) {
	call := "List " + hostID
	if refresh {
		call += " refresh"
	}
	return f.listing, f.record(call)
}

func (f *dmFakeStacks) Get(_ context.Context, hostID, name string) (dockerstacks.StackDetail, error) {
	return f.detail, f.record("Get " + hostID + " " + name)
}

func (f *dmFakeStacks) Create(_ context.Context, hostID string, in dockerstacks.CreateInput) (dockerstacks.StackDetail, error) {
	f.mu.Lock()
	f.create = in
	f.mu.Unlock()
	return f.detail, f.record("Create " + hostID + " " + in.Name)
}

func (f *dmFakeStacks) WriteFiles(_ context.Context, hostID, name string, in dockerstacks.FilesInput) (dockerstacks.FileRevisions, error) {
	f.mu.Lock()
	f.files = in
	f.mu.Unlock()
	return f.revisions, f.record("WriteFiles " + hostID + " " + name)
}

func (f *dmFakeStacks) Start(_ context.Context, hostID, name, kind string, opts dockerstacks.OpOptions) (dockerstacks.Operation, error) {
	f.mu.Lock()
	f.kind, f.opts = kind, opts
	f.mu.Unlock()
	return f.operation, f.record("Start " + hostID + " " + name + " " + kind)
}

func (f *dmFakeStacks) Operation(_ context.Context, id string) (dockerstacks.Operation, error) {
	return f.operation, f.record("Operation " + id)
}

func (f *dmFakeStacks) Operations(_ context.Context, filter dockerstacks.OperationFilter) ([]dockerstacks.Operation, error) {
	f.mu.Lock()
	f.filter = filter
	f.mu.Unlock()
	return f.history, f.record("Operations")
}

func (f *dmFakeStacks) Subscribe(id string) (dockerstacks.Operation, <-chan dockerstacks.Event, func(), error) {
	if err := f.record("Subscribe " + id); err != nil {
		return dockerstacks.Operation{}, nil, nil, err
	}
	events := f.events
	if events == nil {
		closed := make(chan dockerstacks.Event)
		close(closed)
		events = closed
	}
	return f.snapshot, events, func() { f.record("Unsubscribe " + id) }, nil
}

func (f *dmFakeStacks) ProjectAction(_ context.Context, hostID, project, action string) error {
	return f.record("ProjectAction " + hostID + " " + project + " " + action)
}

func (f *dmFakeStacks) ProjectVolumes(_ context.Context, hostID, name string) ([]dockerstacks.ProjectVolume, error) {
	return f.volumes, f.record("ProjectVolumes " + hostID + " " + name)
}

func (f *dmFakeStacks) PublishedPorts(_ context.Context, hostID, exclude string) ([]dockerstacks.PublishedPort, error) {
	return f.ports, f.record("PublishedPorts " + hostID + " " + exclude)
}

var _ dmStackService = (*dmFakeStacks)(nil)

// dmStackFixture routes requests straight into the Docker Manager router with
// the fake as the stacks service (no registry and no Engine are involved).
type dmStackFixture struct {
	t        *testing.T
	s        *Server
	fake     *dmFakeStacks
	readOnly bool
	noStacks bool // the stacks service is not open
	lastRaw  string
}

func newDMStackFixture(t *testing.T) *dmStackFixture {
	t.Helper()
	return &dmStackFixture{t: t, s: testContainerServer(true, false), fake: newDMFakeStacks()}
}

// serve routes one request; the recorder is returned for streaming checks.
func (f *dmStackFixture) serve(method, path string, body any) *httptest.ResponseRecorder {
	f.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			f.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, "/api/docker-manager"+path, reader)
	rec := httptest.NewRecorder()
	dm := &dmRequest{s: f.s, w: rec, r: req, readOnly: f.readOnly}
	if !f.noStacks {
		dm.stacks = f.fake
	}
	dm.route(dockerManagerSegments(req.URL.Path))
	f.lastRaw = rec.Body.String()
	return rec
}

// do routes a request and decodes the JSON answer.
func (f *dmStackFixture) do(method, path string, body any) (int, map[string]any) {
	f.t.Helper()
	rec := f.serve(method, path, body)
	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		f.t.Fatalf("%s %s: answer %q is not JSON: %v", method, path, f.lastRaw, err)
	}
	return rec.Code, decoded
}

func (f *dmStackFixture) expectError(code int, body map[string]any, wantStatus int, wantCode string) {
	f.t.Helper()
	if code != wantStatus || body["status"] != "error" || body["code"] != wantCode {
		f.t.Fatalf("answer = %d %s, want %d error %q", code, f.lastRaw, wantStatus, wantCode)
	}
}
