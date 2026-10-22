package ocr

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

type fakeRuntime struct {
	convert func(context.Context, func(Page) error) error
	calls   atomic.Int32
}

func (*fakeRuntime) Start(context.Context, bool) error { return nil }
func (*fakeRuntime) Stop(context.Context) error        { return nil }
func (f *fakeRuntime) Convert(ctx context.Context, _ string, _ string, _ string, emit func(Page) error) error {
	f.calls.Add(1)
	if f.convert != nil {
		return f.convert(ctx, emit)
	}
	return emit(Page{Index: 0, Text: "hello", Markdown: "hello", Source: "ocr"})
}
func fixture(t *testing.T, runtime *fakeRuntime) *Manager {
	t.Helper()
	m, err := New(filepath.Join(t.TempDir(), "jobs"), Policy{Enabled: true, DockerEnabled: true}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := m.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	if err := m.Start(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	return m
}
func stage(t *testing.T, m *Manager, owner string) File {
	t.Helper()
	f, err := m.Stage(owner, "page.pdf", bytes.NewBufferString("%PDF-1.7\nfixture"))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestOCRStricterIngressAndManualStop(t *testing.T) {
	m := fixture(t, &fakeRuntime{})
	if _, err := m.StageBounded("a", "test.pdf", bytes.NewBufferString("%PDF-1.7 extra"), 8); !errors.Is(err, ErrLimit) {
		t.Fatalf("ingress limit: %v", err)
	}
	if m.storage != 0 || len(m.uploads) != 0 {
		t.Fatal("rejected input retained")
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	policy := m.Policy()
	policy.DockerHost = "changed"
	m.Configure(policy)
	time.Sleep(20 * time.Millisecond)
	if m.Status().Ready {
		t.Fatal("config reload undid manual stop")
	}
	if err := m.Start(context.Background(), false); err != nil || !m.Status().Ready {
		t.Fatalf("explicit start: %v", err)
	}
}

func TestOCRModelLicenseVerbatim(t *testing.T) {
	data, err := os.ReadFile("../../licences/texts/Indra-OCR-models.txt")
	if err != nil || !bytes.Equal(data, ModelLicense) {
		t.Fatal("model license copies differ", err)
	}
}

func TestOCRCancelWaitsForPartialResultPublication(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	m := fixture(t, &fakeRuntime{convert: func(ctx context.Context, emit func(Page) error) error {
		if err := emit(Page{Index: 0, Text: "finished page", Source: "ocr"}); err != nil {
			return err
		}
		close(started)
		<-ctx.Done()
		<-release
		return ctx.Err()
	}})
	f := stage(t, m, "owner")
	job, err := m.Submit("owner", []string{f.ID}, "auto")
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if err = m.Cancel("owner", job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Result("owner", job.ID, f.ID); !errors.Is(err, ErrBusy) {
		t.Fatal("unfinished result appeared final", err)
	}
	current, _ := m.Get("owner", job.ID)
	if current.FinishedAt != nil {
		t.Fatal("cancelled job ended before publication")
	}
	close(release)
	finish(t, m, "owner", job.ID)
	result, err := m.Result("owner", job.ID, f.ID)
	if err != nil || len(result.Pages) != 1 || result.Pages[0].Text != "finished page" {
		t.Fatal("completed page lost", err, result)
	}
}

func TestOCRTimeoutAndPageFailureDoNotStopBatch(t *testing.T) {
	var calls int
	m := fixture(t, &fakeRuntime{convert: func(ctx context.Context, emit func(Page) error) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > FileTimeout {
			t.Error("file timeout missing")
		}
		calls++
		if calls == 1 {
			return context.DeadlineExceeded
		}
		return emit(Page{Index: 0, Source: "ocr", Text: "next file"})
	}})
	first, second := stage(t, m, "a"), stage(t, m, "a")
	job, err := m.Submit("a", []string{first.ID, second.ID}, "auto")
	if err != nil {
		t.Fatal(err)
	}
	job = finish(t, m, "a", job.ID)
	if job.State != "partial" || job.Files[0].Error != "ocr_timeout" || job.Files[1].State != "completed" {
		t.Fatal(job)
	}
}

func TestOCRStorageFailureLeavesReadableError(t *testing.T) {
	release := make(chan struct{})
	m := fixture(t, &fakeRuntime{convert: func(_ context.Context, emit func(Page) error) error {
		<-release
		return emit(Page{Index: 0, Text: "recognized", Source: "ocr"})
	}})
	f := stage(t, m, "a")
	if err := os.Mkdir(filepath.Join(m.root, f.ID+".json"), 0700); err != nil {
		t.Fatal(err)
	}
	job, err := m.Submit("a", []string{f.ID}, "auto")
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	finish(t, m, "a", job.ID)
	result, err := m.Result("a", job.ID, f.ID)
	if err != nil || result.Error != "ocr_storage_failed" || result.State != "error" {
		t.Fatal("storage error could not be read", result, err)
	}
}
func finish(t *testing.T, m *Manager, owner, key string) Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := m.Get(owner, key)
		if err != nil {
			t.Fatal(err)
		}
		if job.FinishedAt != nil {
			return job
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("job did not finish")
	return Job{}
}
func TestOCRJobOwnershipRetentionAndRestart(t *testing.T) {
	m := fixture(t, &fakeRuntime{})
	f := stage(t, m, "a")
	if _, err := m.Submit("b", []string{f.ID}, "auto"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign upload: %v", err)
	}
	job, err := m.Submit("a", []string{f.ID}, "auto")
	if err != nil {
		t.Fatal(err)
	}
	job = finish(t, m, "a", job.ID)
	if job.State != "completed" || job.Files[0].PageCount != 1 {
		t.Fatalf("job=%+v", job)
	}
	if _, err := m.Get("b", job.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign job visible")
	}
	if _, err := m.Result("b", job.ID, f.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign result visible")
	}
	if err := m.Cancel("b", job.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign cancellation accepted")
	}
	if _, _, err := m.Source("b", job.ID, f.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign source visible")
	}
	result, err := m.Result("a", job.ID, f.ID)
	if err != nil || result.Pages[0].Text != "hello" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	m.mu.Lock()
	m.now = func() time.Time { return job.FinishedAt.Add(Retention + time.Second) }
	m.mu.Unlock()
	if _, err := m.Get("a", job.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("expired job visible")
	}
	if _, err := os.Stat(f.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expired input survived")
	}
}
func TestOCRPartialAndCancellation(t *testing.T) {
	rt := &fakeRuntime{convert: func(ctx context.Context, emit func(Page) error) error {
		if err := emit(Page{Index: 0, Text: "first", Source: "ocr"}); err != nil {
			return err
		}
		return errors.New("second page failed")
	}}
	m := fixture(t, rt)
	f := stage(t, m, "a")
	job, _ := m.Submit("a", []string{f.ID}, "auto")
	job = finish(t, m, "a", job.ID)
	if job.State == "completed" || job.Files[0].State != "partial" {
		t.Fatalf("partial marked complete: %+v", job)
	}
	result, err := m.Result("a", job.ID, f.ID)
	if err != nil || len(result.Pages) != 1 {
		t.Fatal("partial output lost")
	}
}
func TestOCRStopCancelsWorkAndRejectsNewJobs(t *testing.T) {
	started := make(chan struct{})
	m := fixture(t, &fakeRuntime{convert: func(ctx context.Context, _ func(Page) error) error { close(started); <-ctx.Done(); return ctx.Err() }})
	f := stage(t, m, "a")
	job, _ := m.Submit("a", []string{f.ID}, "auto")
	<-started
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	job = finish(t, m, "a", job.ID)
	if job.State != "cancelled" {
		t.Fatal(job.State)
	}
	if _, err := m.Stage("a", "x.pdf", bytes.NewBufferString("%PDF-1.7")); !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
	if err := m.Start(context.Background(), false); err != nil {
		t.Fatal(err)
	}
}
func TestOCRInputValidationAndDeduplication(t *testing.T) {
	rt := &fakeRuntime{}
	m := fixture(t, rt)
	for _, name := range []string{"x.png", "x.exe", "x.svg"} {
		if _, err := m.Stage("a", name, bytes.NewBufferString("%PDF-1.7")); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	f := stage(t, m, "a")
	if _, err := m.Submit("a", []string{f.ID, f.ID}, "auto"); !errors.Is(err, ErrNotFound) {
		t.Fatal("duplicate accepted")
	}
	if _, err := m.Submit("a", []string{f.ID}, "text"); !errors.Is(err, ErrInvalid) {
		t.Fatal("unknown mode accepted")
	}
	job, err := m.SubmitAutomatic("a", f.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = finish(t, m, "a", job.ID)
	second := stage(t, m, "a")
	same, err := m.SubmitAutomatic("a", second.ID)
	if err != nil || same.ID != job.ID || rt.calls.Load() != 1 {
		t.Fatal("automatic input processed twice")
	}
	m.mu.Lock()
	m.storage = MaxStorageBytes - 1024
	m.mu.Unlock()
	reader := io.MultiReader(bytes.NewBufferString("%PDF-1.7"), bytes.NewReader(make([]byte, 2048)))
	if _, err := m.Stage("a", "big.pdf", reader); !errors.Is(err, ErrLimit) {
		t.Fatal("storage limit ignored", err)
	}
}
func TestOCRRestartCleanupPreservesUnownedFiles(t *testing.T) {
	dir := t.TempDir()
	owned := filepath.Join(dir, id()+".pdf")
	for _, path := range []string{owned, filepath.Join(dir, "user.pdf")} {
		if err := os.WriteFile(path, []byte("input"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m, err := New(dir, Policy{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close(context.Background())
	if _, err := os.Stat(owned); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("old job survived restart")
	}
	if _, err := os.Stat(filepath.Join(dir, "user.pdf")); err != nil {
		t.Fatal("unowned file removed")
	}
}

func TestOCRRetainedResultsDoNotConsumeQueueCapacity(t *testing.T) {
	m := fixture(t, &fakeRuntime{})
	for i := 0; i <= cap(m.queue); i++ {
		owner := "indexer:document-" + strconv.Itoa(i)
		file := stage(t, m, owner)
		job, err := m.SubmitAutomatic(owner, file.ID)
		if err != nil {
			t.Fatalf("file %d rejected with %d retained jobs and %d queued: %v", i+1, len(m.jobs), m.Status().Queued, err)
		}
		if job = finish(t, m, owner, job.ID); job.State != "completed" {
			t.Fatal(job.State)
		}
	}
	if len(m.List("indexer:document-0")) != 1 {
		t.Fatal("new work evicted a retained result")
	}
}

func TestOCRAutomaticRetriesPartialAfterServiceRestart(t *testing.T) {
	rt := &fakeRuntime{convert: func(_ context.Context, emit func(Page) error) error {
		if err := emit(Page{Index: 0, Source: "ocr", Text: "first page"}); err != nil {
			return err
		}
		return errors.New("temporary page failure")
	}}
	m := fixture(t, rt)
	file := stage(t, m, "indexer:file")
	job, err := m.SubmitAutomatic("indexer:file", file.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job = finish(t, m, "indexer:file", job.ID); job.State != "partial" {
		t.Fatal(job.State)
	}
	if err = m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	rt.convert = nil
	if err = m.Start(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	file = stage(t, m, "indexer:file")
	retry, err := m.SubmitAutomatic("indexer:file", file.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retry = finish(t, m, "indexer:file", retry.ID); retry.State != "completed" || retry.ID == job.ID || rt.calls.Load() != 2 {
		t.Fatal("partial result prevented retry", retry.State, rt.calls.Load())
	}
	previous, err := m.Result("indexer:file", job.ID, job.Files[0].ID)
	if err != nil || previous.State != "partial" || len(previous.Pages) != 1 {
		t.Fatal("retry removed earlier partial output", previous, err)
	}
}

func TestOCRFullQueueStillReusesCompletedAutomaticResult(t *testing.T) {
	started := make(chan struct{})
	rt := &fakeRuntime{}
	m := fixture(t, rt)
	file := stage(t, m, "cached")
	cached, err := m.SubmitAutomatic("cached", file.ID)
	if err != nil {
		t.Fatal(err)
	}
	finish(t, m, "cached", cached.ID)
	rt.convert = func(ctx context.Context, _ func(Page) error) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	file = stage(t, m, "running")
	if _, err = m.Submit("running", []string{file.ID}, "auto"); err != nil {
		t.Fatal(err)
	}
	<-started
	for i := 0; i < cap(m.queue); i++ {
		file = stage(t, m, "queued")
		if _, err = m.Submit("queued", []string{file.ID}, "auto"); err != nil {
			t.Fatal(err)
		}
	}
	file = stage(t, m, "queued")
	if _, err = m.Submit("queued", []string{file.ID}, "auto"); !errors.Is(err, ErrBusy) {
		t.Fatal("queue limit ignored", err)
	}
	file = stage(t, m, "cached")
	same, err := m.SubmitAutomatic("cached", file.ID)
	if err != nil || same.ID != cached.ID {
		t.Fatal("full queue blocked cached result", err)
	}
}
