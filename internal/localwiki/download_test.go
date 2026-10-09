package localwiki

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testPayloadSize = 300_000

func newTestJob(t *testing.T, f *fakeKiwix, e *fakeEdition, urls []string) *downloadJob {
	t.Helper()
	dir := t.TempDir()
	return &downloadJob{
		client:       newHTTPClient(f.server.Client()),
		partPath:     filepath.Join(dir, e.name+".zim.part"),
		dir:          dir,
		size:         int64(len(e.data)),
		sha256:       e.sha256,
		urls:         urls,
		freeDisk:     func(string) (int64, error) { return 1 << 40, nil },
		checkEvery:   1 << 30,
		stallTimeout: 5 * time.Second,
		roundBackoff: 10 * time.Millisecond,
	}
}

func assertNoRestartFile(t *testing.T, job *downloadJob) {
	t.Helper()
	if _, err := os.Stat(job.restartPath()); !os.IsNotExist(err) {
		t.Fatalf("restart file left behind: %v", err)
	}
}

func assertPartEquals(t *testing.T, job *downloadJob, want []byte) {
	t.Helper()
	got, err := os.ReadFile(job.partPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("part file has %d bytes, want the %d expected bytes", len(got), len(want))
	}
}

func rangesFor(f *fakeKiwix, prefix string) []string {
	var out []string
	for _, request := range f.requestLog() {
		if strings.HasPrefix(request.Path, prefix) {
			out = append(out, request.Range)
		}
	}
	return out
}

func TestDownloadJobFetchesAndVerifies(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	// A restart file left by a crash is never resumed.
	if err := os.WriteFile(job.restartPath(), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	var lastDone int64
	var urls []string
	job.onProgress = func(done, total int64) { lastDone = done }
	job.onURL = func(u string) { urls = append(urls, u) }
	if err := job.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertPartEquals(t, job, e.data)
	assertNoRestartFile(t, job)
	if lastDone != int64(len(e.data)) || len(urls) != 1 || !strings.Contains(urls[0], "/m1/") {
		t.Fatalf("progress %d, urls %v", lastDone, urls)
	}
}

func TestDownloadJobResumesWithRangeAfterRehash(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	if err := os.WriteFile(job.partPath, e.data[:100_000], 0o644); err != nil {
		t.Fatal(err)
	}
	var phases []string
	job.onPhase = func(phase string) { phases = append(phases, phase) }
	if err := job.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertPartEquals(t, job, e.data)
	if got := strings.Join(phases, ","); got != StateVerifying+","+StateDownloading {
		t.Fatalf("phases = %s", got)
	}
	if got := rangesFor(f, "/m1/"); len(got) != 1 || got[0] != "bytes=100000-" {
		t.Fatalf("m1 ranges = %v", got)
	}
}

func TestDownloadJobRestartsWhenNoMirrorHonoursRange(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	e.setMode("m1", "ignore-range")
	e.setMode("m2", "fail")
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	// A wrong prefix proves the restart: the 200 answer replaces it from byte 0.
	if err := os.WriteFile(job.partPath, make([]byte, 100_000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := job.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertPartEquals(t, job, e.data)
	assertNoRestartFile(t, job)
	want := []string{"bytes=100000-", "bytes=100000-", ""}
	if got := rangesFor(f, "/m1/"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("m1 ranges = %v, want two rounds of Range requests, then a full request", got)
	}
}

// One round in which the mirrors that honour Range fail (a single 503) is not
// enough to start over: the job waits and asks every mirror again.
func TestDownloadJobRestartNeedsTwoStalledRounds(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	e.cutChunk = 10_000
	e.setMode("m1", "ignore-range-cut")
	e.setMode("m2", "fail-once")
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	job.roundBackoff = 50 * time.Millisecond
	if err := os.WriteFile(job.partPath, e.data[:200_000], 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := job.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if elapsed := time.Since(start); elapsed < job.roundBackoff {
		t.Fatalf("the second round did not wait for the backoff (%v)", elapsed)
	}
	assertPartEquals(t, job, e.data)
	if got := rangesFor(f, "/m1/"); strings.Join(got, ",") != "bytes=200000-,bytes=200000-" {
		t.Fatalf("m1 ranges = %v, want no full request", got)
	}
	if got := rangesFor(f, "/m2/"); strings.Join(got, ",") != "bytes=200000-,bytes=200000-" {
		t.Fatalf("m2 ranges = %v", got)
	}
}

// A restart downloads into a separate file; one that ends before it holds
// more than the part file is discarded and the part file stays.
func TestDownloadJobRestartNeverShrinksThePart(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	e.cutChunk = 10_000
	e.setMode("m1", "ignore-range-cut")
	e.setMode("m2", "fail")
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	if err := os.WriteFile(job.partPath, e.data[:200_000], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := job.run(context.Background()); !errors.Is(err, errDownloadFailed) {
		t.Fatalf("run = %v, want errDownloadFailed", err)
	}
	assertPartEquals(t, job, e.data[:200_000])
	assertNoRestartFile(t, job)
	if got := rangesFor(f, "/m1/"); strings.Join(got, ",") != "bytes=200000-,bytes=200000-," {
		t.Fatalf("m1 ranges = %v, want two rounds of Range requests and one restart", got)
	}

	// The kept part resumes once a mirror honours Range again.
	e.setMode("m1", "")
	resumed := newTestJob(t, f, e, f.mirrorURLs(e))
	resumed.partPath = job.partPath
	if err := resumed.run(context.Background()); err != nil {
		t.Fatalf("resume: %v", err)
	}
	assertPartEquals(t, resumed, e.data)
}

// A restart that gets past the part file's length replaces it, and its hash
// carries on: a later round resumes the replaced file with Range.
func TestDownloadJobRestartReplacesThePartPastItsLength(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	e.cutChunk = 150_000
	e.setMode("m1", "ignore-range-cut")
	e.setMode("m2", "fail")
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	// Zeros: only a replacement by the restart turns them into edition bytes.
	if err := os.WriteFile(job.partPath, make([]byte, 100_000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := job.run(context.Background()); !errors.Is(err, errDownloadFailed) {
		t.Fatalf("run = %v, want errDownloadFailed", err)
	}
	assertPartEquals(t, job, e.data[:150_000])
	assertNoRestartFile(t, job)

	e.setMode("m2", "")
	resumed := newTestJob(t, f, e, f.mirrorURLs(e))
	resumed.partPath = job.partPath
	if err := resumed.run(context.Background()); err != nil {
		t.Fatalf("resume: %v", err)
	}
	assertPartEquals(t, resumed, e.data)
	if got := rangesFor(f, "/m2/"); got[len(got)-1] != "bytes=150000-" {
		t.Fatalf("m2 ranges = %v, want the resume after the replaced part", got)
	}
}

// Within one run the restart's hash continues after the replacement: the
// restart drops after 150,000 bytes, m2 recovers and completes with Range,
// and the SHA-256 of the whole file still matches.
func TestDownloadJobHashCarriesOverARestart(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	e.cutChunk = 150_000
	e.setMode("m1", "ignore-range-cut")
	e.setMode("m2", "fail")
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	if err := os.WriteFile(job.partPath, make([]byte, 100_000), 0o644); err != nil {
		t.Fatal(err)
	}
	job.onURL = func(u string) {
		// The restart has started on m1: m2 serves the next round.
		if strings.Contains(u, "/m1/") {
			e.setMode("m2", "")
		}
	}
	if err := job.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertPartEquals(t, job, e.data)
	assertNoRestartFile(t, job)
	if got := rangesFor(f, "/m2/"); got[len(got)-1] != "bytes=150000-" {
		t.Fatalf("m2 ranges = %v, want the resume after the replaced part", got)
	}
}

// A mirror that ignores Range does not restart the download while another
// mirror continues at the offset.
func TestDownloadJobPrefersMirrorsThatHonourRange(t *testing.T) {
	for _, mode := range []string{"ignore-range", "range-416"} {
		t.Run(mode, func(t *testing.T) {
			f := newFakeKiwix(t)
			e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
			e.setMode("m1", mode)
			job := newTestJob(t, f, e, f.mirrorURLs(e))
			if err := os.WriteFile(job.partPath, e.data[:100_000], 0o644); err != nil {
				t.Fatal(err)
			}
			if err := job.run(context.Background()); err != nil {
				t.Fatalf("run: %v", err)
			}
			assertPartEquals(t, job, e.data)
			if got := rangesFor(f, "/m1/"); len(got) != 1 || got[0] != "bytes=100000-" {
				t.Fatalf("m1 ranges = %v", got)
			}
			if got := rangesFor(f, "/m2/"); len(got) != 1 || got[0] != "bytes=100000-" {
				t.Fatalf("m2 ranges = %v, want the resume at the kept offset", got)
			}
		})
	}
}

func TestDownloadJobRestartsAfter416(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	e.setMode("m1", "range-416")
	e.setMode("m2", "range-416")
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	if err := os.WriteFile(job.partPath, e.data[:50_000], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := job.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertPartEquals(t, job, e.data)
	assertNoRestartFile(t, job)
	if got := rangesFor(f, "/m1/"); strings.Join(got, ",") != "bytes=50000-,bytes=50000-," {
		t.Fatalf("m1 ranges = %v, want two rounds of Range requests, then a full request", got)
	}
	if got := rangesFor(f, "/m2/"); strings.Join(got, ",") != "bytes=50000-,bytes=50000-" {
		t.Fatalf("m2 ranges = %v, want two Range requests", got)
	}
}

// A 416 never discards the part file by itself: it is truncated only once a
// full request has delivered the whole file.
func TestDownloadJobKeepsPartWhen416RestartFails(t *testing.T) {
	t.Run("next mirror continues", func(t *testing.T) {
		f := newFakeKiwix(t)
		e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
		e.setMode("m1", "range-416-then-fail")
		job := newTestJob(t, f, e, f.mirrorURLs(e))
		if err := os.WriteFile(job.partPath, e.data[:50_000], 0o644); err != nil {
			t.Fatal(err)
		}
		if err := job.run(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
		assertPartEquals(t, job, e.data)
		if got := rangesFor(f, "/m2/"); len(got) != 1 || got[0] != "bytes=50000-" {
			t.Fatalf("m2 ranges = %v, want the resume at the kept offset", got)
		}
	})
	t.Run("no mirror delivers", func(t *testing.T) {
		f := newFakeKiwix(t)
		e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
		e.setMode("m1", "range-416-then-fail")
		e.setMode("m2", "range-416-then-fail")
		job := newTestJob(t, f, e, f.mirrorURLs(e))
		if err := os.WriteFile(job.partPath, e.data[:50_000], 0o644); err != nil {
			t.Fatal(err)
		}
		if err := job.run(context.Background()); !errors.Is(err, errDownloadFailed) {
			t.Fatalf("run = %v, want errDownloadFailed", err)
		}
		assertPartEquals(t, job, e.data[:50_000])
		assertNoRestartFile(t, job)
		for _, mirror := range []string{"/m1/", "/m2/"} {
			if got := rangesFor(f, mirror); strings.Join(got, ",") != "bytes=50000-,bytes=50000-," {
				t.Fatalf("%s ranges = %v, want two rounds of Range requests, then a full request", mirror, got)
			}
		}
	})
}

func TestDownloadJobFallsBackToNextMirror(t *testing.T) {
	for _, mode := range []string{"fail", "http-redirect", "html-200", "chunked-html", "gzip", "multi-encoding"} {
		t.Run(mode, func(t *testing.T) {
			f := newFakeKiwix(t)
			e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
			e.setMode("m1", mode)
			job := newTestJob(t, f, e, f.mirrorURLs(e))
			if err := job.run(context.Background()); err != nil {
				t.Fatalf("run: %v", err)
			}
			assertPartEquals(t, job, e.data)
			// m2 starts at byte 0: m1's answer left nothing in the part file.
			if got := rangesFor(f, "/m2/"); len(got) != 1 || got[0] != "" {
				t.Fatalf("m2 was not used from byte 0: %v", f.requestLog())
			}
		})
	}
}

// A 200 answer whose length is not the edition's (an error page) must not
// discard the bytes already downloaded.
func TestDownloadJobKeepsPartWhenMirrorAnswersWithAPage(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	e.setMode("m1", "html-200")
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	if err := os.WriteFile(job.partPath, e.data[:100_000], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := job.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertPartEquals(t, job, e.data)
	if got := rangesFor(f, "/m2/"); len(got) != 1 || got[0] != "bytes=100000-" {
		t.Fatalf("m2 ranges = %v, want the resume at the kept offset", got)
	}
}

// Mirrors that keep dropping the connection are tried again as long as a
// round over all mirrors moves the download forward.
func TestDownloadJobRetriesMirrorsThatMakeProgress(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	e.setMode("m1", "cut")
	e.setMode("m2", "cut")
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	if err := job.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertPartEquals(t, job, e.data)
	if got := rangesFor(f, "/m1/"); len(got) < 2 || got[0] != "" || got[1] != "bytes=131072-" {
		t.Fatalf("m1 ranges = %v", got)
	}
	if got := rangesFor(f, "/m2/"); len(got) < 2 || got[0] != "bytes=65536-" {
		t.Fatalf("m2 ranges = %v", got)
	}
}

func TestDownloadJobAllMirrorsFailed(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	e.setMode("m1", "fail")
	e.setMode("m2", "fail")
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	err := job.run(context.Background())
	if !errors.Is(err, errDownloadFailed) {
		t.Fatalf("run = %v, want errDownloadFailed", err)
	}
	if _, statErr := os.Stat(job.partPath); statErr != nil {
		t.Fatalf("the part file must stay for a later resume: %v", statErr)
	}
	if got := len(f.requestLog()); got != 2 {
		t.Fatalf("%d requests, want one per mirror: a round without progress ends the job", got)
	}
}

func TestDownloadJobChecksumMismatchRemovesPart(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	job.sha256 = strings.Repeat("0", 64)
	if err := job.run(context.Background()); !errors.Is(err, errChecksumMismatch) {
		t.Fatalf("run = %v, want errChecksumMismatch", err)
	}
	if _, err := os.Stat(job.partPath); !os.IsNotExist(err) {
		t.Fatalf("damaged part file still exists: %v", err)
	}
}

// A complete part file is only re-hashed: no mirror is contacted.
func TestDownloadJobVerifiesCompletePartWithoutDownloading(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	if err := os.WriteFile(job.partPath, e.data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := job.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertPartEquals(t, job, e.data)

	damaged := newTestJob(t, f, e, f.mirrorURLs(e))
	corrupt := bytes.Clone(e.data)
	corrupt[len(corrupt)/2] ^= 0xff
	if err := os.WriteFile(damaged.partPath, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := damaged.run(context.Background()); !errors.Is(err, errChecksumMismatch) {
		t.Fatalf("run on a damaged complete part = %v, want errChecksumMismatch", err)
	}
	if _, err := os.Stat(damaged.partPath); !os.IsNotExist(err) {
		t.Fatalf("damaged part file still exists: %v", err)
	}
	if got := f.requestLog(); len(got) != 0 {
		t.Fatalf("complete part files must not be downloaded again: %v", got)
	}
}

func TestDownloadJobRefusesInvalidTargets(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	for name, mutate := range map[string]func(*downloadJob){
		"zero size":     func(j *downloadJob) { j.size = 0 },
		"huge size":     func(j *downloadJob) { j.size = maxEditionBytes + 1 },
		"no sha256":     func(j *downloadJob) { j.sha256 = "" },
		"short sha256":  func(j *downloadJob) { j.sha256 = "abc" },
		"no part path":  func(j *downloadJob) { j.partPath = "" },
		"http mirror":   func(j *downloadJob) { j.urls = []string{"http://127.0.0.1:9/x.zim"} },
		"empty mirrors": func(j *downloadJob) { j.urls = nil },
	} {
		t.Run(name, func(t *testing.T) {
			job := newTestJob(t, f, e, f.mirrorURLs(e))
			mutate(job)
			if err := job.run(context.Background()); err == nil {
				t.Fatal("run accepted an invalid target")
			}
		})
	}
	if got := f.requestLog(); len(got) != 0 {
		t.Fatalf("invalid targets must not reach a mirror: %v", got)
	}
}

// The free-space check counts the bytes written across attempts and mirrors,
// so mirrors that each deliver less than checkEvery still trigger it.
func TestDownloadJobChecksDiskAcrossAttempts(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	e.setMode("m1", "cut")
	e.setMode("m2", "cut")
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	job.checkEvery = 100_000
	checks := 0
	job.freeDisk = func(string) (int64, error) { checks++; return 10, nil }
	err := job.run(context.Background())
	var space *spaceError
	if !errors.As(err, &space) || checks != 1 {
		t.Fatalf("run = %v after %d disk checks, want a spaceError after one", err, checks)
	}
	info, statErr := os.Stat(job.partPath)
	if statErr != nil || info.Size() < 100_000 || info.Size() > 2*int64(e.cutChunk) {
		t.Fatalf("part file after pause: %v, %v", info, statErr)
	}

	plenty := newTestJob(t, f, e, f.mirrorURLs(e))
	plenty.checkEvery = 100_000
	checks = 0
	plenty.freeDisk = func(string) (int64, error) { checks++; return 1 << 40, nil }
	if err := plenty.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertPartEquals(t, plenty, e.data)
	if checks < 2 {
		t.Fatalf("%d disk checks for %d bytes in %d-byte attempts, want one per 100000 bytes", checks, len(e.data), e.cutChunk)
	}
}

func TestDownloadJobPausesWhenDiskRunsLow(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	job.checkEvery = 64 << 10
	job.freeDisk = func(string) (int64, error) { return 10, nil }
	err := job.run(context.Background())
	var space *spaceError
	if !errors.As(err, &space) || space.available != 10 || space.required <= 1<<30 {
		t.Fatalf("run = %v, want a spaceError", err)
	}
	if !strings.Contains(err.Error(), CodeInsufficientDiskSpace) {
		t.Fatalf("space error %q does not carry the stable code", err)
	}
	info, statErr := os.Stat(job.partPath)
	if statErr != nil || info.Size() < 64<<10 || info.Size() >= int64(len(e.data)) {
		t.Fatalf("part file after pause: %v, %v", info, statErr)
	}
	if len(rangesFor(f, "/m2/")) != 0 {
		t.Fatal("a full disk must pause, not try the next mirror")
	}
}

func TestDownloadJobStalledMirrorFallsBack(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	e.setMode("m1", "slow")
	defer e.unblock()
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	job.stallTimeout = 300 * time.Millisecond
	if err := job.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	assertPartEquals(t, job, e.data)
	if got := rangesFor(f, "/m2/"); len(got) != 1 || got[0] != "bytes=1024-" {
		t.Fatalf("m2 ranges = %v, want the resume after the stalled mirror's bytes", got)
	}
}

func TestDownloadJobCancelKeepsPart(t *testing.T) {
	f := newFakeKiwix(t)
	e := f.addEdition("wikipedia_de_all_nopic_2026-10", testPayload(testPayloadSize))
	e.setMode("m1", "slow")
	defer e.unblock()
	job := newTestJob(t, f, e, f.mirrorURLs(e))
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	job.onProgress = func(done, total int64) {
		if done >= 1024 {
			once.Do(cancel)
		}
	}
	if err := job.run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("run = %v, want context.Canceled", err)
	}
	info, err := os.Stat(job.partPath)
	if err != nil || info.Size() < 1024 {
		t.Fatalf("part file after cancel: %v, %v", info, err)
	}
}

func TestParseContentRange(t *testing.T) {
	for header, want := range map[string]struct {
		start, total int64
		ok           bool
	}{
		"bytes 100-299/300":      {100, 300, true},
		"bytes 0-79/18585337585": {0, 18585337585, true},
		"bytes 100-299/*":        {100, -1, true},
		"bytes */300":            {0, 0, false},
		"items 1-2/3":            {0, 0, false},
		"bytes x-2/3":            {0, 0, false},
		"bytes -5-2/3":           {0, 0, false},
		"bytes 5-2/x":            {0, 0, false},
		"":                       {0, 0, false},
	} {
		start, total, ok := parseContentRange(header)
		if ok != want.ok || (ok && (start != want.start || total != want.total)) {
			t.Fatalf("parseContentRange(%q) = %d, %d, %v; want %+v", header, start, total, ok, want)
		}
	}
}

// Partial downloads include an unfinished restart file; nothing else is touched.
func TestRemovePartialDownloadsIncludeRestartFiles(t *testing.T) {
	dir := t.TempDir()
	const name = "wikipedia_de_all_nopic_2026-10.zim"
	files := []string{name, name + ".part", name + ".part" + restartSuffix, "notes.part" + restartSuffix, "wikipedia_de_all_maxi_2026-10.zim.part" + restartSuffix}
	write := func() {
		for _, file := range files {
			if err := os.WriteFile(filepath.Join(dir, file), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	exists := func(file string) bool {
		_, err := os.Stat(filepath.Join(dir, file))
		return err == nil
	}
	write()
	if err := removePartialDownload(dir, name); err != nil {
		t.Fatal(err)
	}
	if exists(name+".part") || exists(name+".part"+restartSuffix) || !exists(name) || !exists("wikipedia_de_all_maxi_2026-10.zim.part"+restartSuffix) {
		t.Fatal("removePartialDownload removed the wrong files")
	}
	write()
	if err := removePartFiles(dir); err != nil {
		t.Fatal(err)
	}
	if exists(name+".part") || exists(name+".part"+restartSuffix) || exists("wikipedia_de_all_maxi_2026-10.zim.part"+restartSuffix) || !exists(name) || !exists("notes.part"+restartSuffix) {
		t.Fatal("removePartFiles removed the wrong files")
	}
}
