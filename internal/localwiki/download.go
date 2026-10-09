package localwiki

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/fileutil"
)

const (
	defaultDiskCheckInterval int64 = 1 << 30
	defaultStallTimeout            = 2 * time.Minute
	copyBufferSize                 = 1 << 20
	rehashChunk                    = 4 << 20
	userAgent                      = "AuraGo-LocalWikipedia/1"

	// defaultRoundBackoff is the pause before the mirrors are asked again
	// after a round in which only mirrors that would start over answered.
	defaultRoundBackoff = 30 * time.Second
	// restartAfterRounds consecutive rounds without progress are required
	// before the download starts over from byte 0.
	restartAfterRounds = 2
	// restartSuffix names the sibling file a restart downloads into
	// (<edition>.zim.part.restart) until it holds more than the part file.
	restartSuffix = ".restart"
)

var errMirrorStalled = errors.New("mirror stalled")

// spaceError pauses a download that would leave less than the margin free.
type spaceError struct{ required, available int64 }

func (e *spaceError) Error() string {
	return fmt.Sprintf("%s: %d bytes required, %d available", CodeInsufficientDiskSpace, e.required, e.available)
}

// writeError is a local disk failure: it ends the job instead of trying the next mirror.
type writeError struct{ err error }

func (e *writeError) Error() string { return "write partial edition: " + e.err.Error() }
func (e *writeError) Unwrap() error { return e.err }

// errRangeRefused marks a mirror that cannot continue at the current offset:
// it answered 416 or ignored the Range header and offered the whole file. The
// part file is untouched; the mirror may still serve a restart from byte 0.
var errRangeRefused = errors.New("mirror cannot continue at the current offset")

// downloadJob fetches one edition into partPath: HTTP Range resume, mirror
// fallback, SHA-256 while downloading (an existing part is re-hashed first),
// a free-space re-check (with an fsync) after every checkEvery written bytes
// across all attempts, and a stall watchdog.
//
// Mirrors are tried in order. A mirror that fails moves on to the next one; a
// round over all mirrors that moved the download forward is followed by
// another round, and a round without progress ends the job with
// errDownloadFailed. The exception are mirrors that could only serve the
// whole file (416, or 200 to a Range request): after such a round the job
// waits roundBackoff and asks every mirror again, and only after
// restartAfterRounds consecutive rounds without progress does it start over.
// The restart downloads into a sibling file (restartPath) that replaces the
// part file only once it holds more bytes; a restart that ends earlier is
// discarded, so the part file never shrinks.
type downloadJob struct {
	client       *http.Client
	logger       *slog.Logger
	partPath     string
	dir          string
	size         int64
	sha256       string
	urls         []string
	freeDisk     func(string) (int64, error)
	checkEvery   int64
	stallTimeout time.Duration
	roundBackoff time.Duration      // pause between rounds before a restart; 0 → defaultRoundBackoff
	onPhase      func(phase string) // StateVerifying while re-hashing, then StateDownloading
	onProgress   func(done, total int64)
	onURL        func(finalURL string) // mirror that is answering (after redirects)

	// file, hash and offset describe the file being written: the part file,
	// or the restart file while kept holds the part file.
	file      *os.File
	hash      hash.Hash
	offset    int64
	kept      *keptPart
	unchecked int64 // bytes written since the last free-space check, across attempts
}

// keptPart is the part file a running restart may replace.
type keptPart struct {
	file   *os.File
	hash   hash.Hash
	offset int64
}

func (j *downloadJob) restartPath() string { return j.partPath + restartSuffix }

// removePartialDownload removes the partial file of an edition and the file of
// an unfinished restart (see downloadJob).
func removePartialDownload(dir, fileName string) error {
	part := filepath.Join(dir, fileName+".part")
	return errors.Join(removeIfExists(part), removeIfExists(part+restartSuffix))
}

// removeRestartFiles deletes every restart file (<edition>.zim.part.restart)
// in dir. A restart file is never resumed; one that exists while no download
// runs was left by a killed process. Other files are never touched.
func removeRestartFiles(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range entries {
		name, ok := strings.CutSuffix(entry.Name(), ".part"+restartSuffix)
		if !ok || !entry.Type().IsRegular() || !zimFileNamePattern.MatchString(name) {
			continue
		}
		errs = append(errs, removeIfExists(filepath.Join(dir, entry.Name())))
	}
	return errors.Join(errs...)
}

// run returns nil once partPath holds exactly size bytes with the expected
// SHA-256. A context error means cancel or shutdown (the part file is kept), a
// *spaceError pauses the download, errChecksumMismatch removes the part file,
// errDownloadFailed means the mirrors stopped making progress (the part file
// is kept for a later resume).
func (j *downloadJob) run(ctx context.Context) error {
	defer j.cleanup()
	if err := j.validate(); err != nil {
		return err
	}
	if err := j.prepare(ctx); err != nil {
		return err
	}
	var lastErr error
	stalled := 0
	for j.offset < j.size {
		roundStart := j.offset
		refused, err := j.round(ctx, &lastErr)
		if err != nil {
			return err
		}
		if j.offset == j.size || j.offset > roundStart {
			stalled = 0
			continue
		}
		if len(refused) == 0 {
			break
		}
		// Only mirrors that would start over answered. One round of
		// transient failures of the others must not cost the kept bytes:
		// wait and ask every mirror again first.
		if stalled++; stalled < restartAfterRounds {
			if err := sleepContext(ctx, j.roundBackoff); err != nil {
				return err
			}
			continue
		}
		if err := j.restartFrom(ctx, refused, &lastErr); err != nil {
			return err
		}
		if j.offset <= roundStart {
			break
		}
		stalled = 0
	}
	if j.offset != j.size {
		if lastErr == nil {
			lastErr = fmt.Errorf("mirrors delivered %d of %d bytes", j.offset, j.size)
		}
		return fmt.Errorf("%w: %v", errDownloadFailed, lastErr)
	}
	if err := j.file.Sync(); err != nil {
		return &writeError{err}
	}
	if err := j.closeFile(); err != nil {
		return &writeError{err}
	}
	if sum := hex.EncodeToString(j.hash.Sum(nil)); !strings.EqualFold(sum, j.sha256) {
		if err := os.Remove(j.partPath); err != nil {
			j.logger.Warn("[LocalWikipedia] The damaged download could not be removed", "file", j.partPath, "error", err)
		}
		return errChecksumMismatch
	}
	return nil
}

// validate refuses a target the manager should never have planned, before the
// part file is touched or a mirror is contacted.
func (j *downloadJob) validate() error {
	if j.client == nil || strings.TrimSpace(j.partPath) == "" {
		return errors.New("localwiki: download job without client or part file")
	}
	if j.size <= 0 || j.size > maxEditionBytes {
		return fmt.Errorf("localwiki: implausible edition size %d", j.size)
	}
	if !sha256HexPattern.MatchString(strings.ToLower(j.sha256)) {
		return errors.New("localwiki: download job without a valid SHA-256")
	}
	if len(j.urls) == 0 {
		return errors.New("localwiki: download job without mirrors")
	}
	for _, rawURL := range j.urls {
		if _, err := requireHTTPS(rawURL); err != nil {
			return err
		}
	}
	return nil
}

func (j *downloadJob) closeFile() error {
	if j.file == nil {
		return nil
	}
	err := j.file.Close()
	j.file = nil
	return err
}

// cleanup discards an unfinished restart and closes the part file.
func (j *downloadJob) cleanup() {
	j.abandonRestart()
	_ = j.closeFile()
}

// sleepContext waits d or until ctx ends.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (j *downloadJob) prepare(ctx context.Context) error {
	if j.checkEvery <= 0 {
		j.checkEvery = defaultDiskCheckInterval
	}
	if j.stallTimeout <= 0 {
		j.stallTimeout = defaultStallTimeout
	}
	if j.roundBackoff <= 0 {
		j.roundBackoff = defaultRoundBackoff
	}
	if j.logger == nil {
		j.logger = slog.Default()
	}
	// A restart file is never resumed: the part file is the only progress.
	if err := removeIfExists(j.restartPath()); err != nil {
		return &writeError{err}
	}
	file, err := os.OpenFile(j.partPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return &writeError{err}
	}
	j.file = file
	j.hash = sha256.New()
	info, err := file.Stat()
	if err != nil {
		return &writeError{err}
	}
	existing := info.Size()
	if existing > j.size {
		if err := file.Truncate(0); err != nil {
			return &writeError{err}
		}
		existing = 0
	}
	if existing > 0 {
		j.phase(StateVerifying)
		if err := j.rehash(ctx, existing); err != nil {
			return err
		}
	}
	if _, err := file.Seek(existing, io.SeekStart); err != nil {
		return &writeError{err}
	}
	j.offset = existing
	j.phase(StateDownloading)
	j.progress(j.offset)
	return nil
}

// rehash feeds the bytes of an existing part file into the hash, so a resumed
// download still verifies the whole file.
func (j *downloadJob) rehash(ctx context.Context, length int64) error {
	buf := make([]byte, rehashChunk)
	var done int64
	for done < length {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := j.file.ReadAt(buf[:min(int64(len(buf)), length-done)], done)
		if n > 0 {
			j.hash.Write(buf[:n])
			done += int64(n)
			j.progress(done)
		}
		if err != nil && !(errors.Is(err, io.EOF) && done == length) {
			return &writeError{fmt.Errorf("read partial edition: %w", err)}
		}
	}
	return nil
}

// round tries every mirror once at the current offset. It returns the mirrors
// that could only restart the part file (errRangeRefused) and the error that
// ends the job (see stopError); other failures move on to the next mirror.
func (j *downloadJob) round(ctx context.Context, lastErr *error) ([]string, error) {
	var refused []string
	for _, rawURL := range j.urls {
		if j.offset == j.size {
			break
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		err := j.fetch(ctx, rawURL, false)
		if err == nil {
			continue
		}
		if stop := stopError(ctx, err); stop != nil {
			return nil, stop
		}
		if errors.Is(err, errRangeRefused) {
			j.logger.Info("[LocalWikipedia] Mirror cannot continue the partial download, trying the next one", "host", hostOf(rawURL), "offset", j.offset)
			refused = append(refused, rawURL)
			continue
		}
		j.logger.Warn("[LocalWikipedia] Mirror failed, trying the next one", "host", hostOf(rawURL), "offset", j.offset, "error", err)
		*lastErr = err
	}
	return refused, nil
}

// restartFrom downloads the whole file again from the first of the mirrors
// that delivers more of it than the part file holds. Each attempt writes into
// the restart file, which replaces the part file once it holds more bytes (see
// promoteRestart); an attempt that ends earlier is discarded and the next
// mirror is tried, so the part file never shrinks.
func (j *downloadJob) restartFrom(ctx context.Context, mirrors []string, lastErr *error) error {
	kept := j.offset
	for _, rawURL := range mirrors {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := j.fetch(ctx, rawURL, true)
		if err != nil {
			if stop := stopError(ctx, err); stop != nil {
				return stop
			}
			j.logger.Warn("[LocalWikipedia] Mirror failed to restart the download", "host", hostOf(rawURL), "kept", kept, "error", err)
			*lastErr = err
		}
		if j.offset > kept {
			return nil
		}
	}
	return nil
}

// stopError returns the error that ends the job instead of moving on to the
// next mirror: cancellation or shutdown, a full disk or a local write failure.
func stopError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var space *spaceError
	var write *writeError
	if errors.As(err, &space) || errors.As(err, &write) {
		return err
	}
	return nil
}

// fetch runs one attempt against one mirror: the remaining bytes, or with
// full the whole file from byte 0 into the restart file. The stall watchdog
// covers the wait for the response headers and every read of the body. A
// restart that has not replaced the part file when the attempt ends is
// discarded.
func (j *downloadJob) fetch(ctx context.Context, rawURL string, full bool) error {
	defer j.abandonRestart()
	attemptCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	watchdog := time.AfterFunc(j.stallTimeout, func() { cancel(errMirrorStalled) })
	defer watchdog.Stop()
	resp, err := j.open(attemptCtx, rawURL, full)
	if err != nil {
		return attemptError(attemptCtx, err)
	}
	defer resp.Body.Close()
	if j.onURL != nil && resp.Request != nil && resp.Request.URL != nil {
		j.onURL(resp.Request.URL.String())
	}
	if err := j.copy(resp.Body, watchdog); err != nil {
		return attemptError(attemptCtx, err)
	}
	return nil
}

// attemptError reports a stalled mirror as such instead of a bare cancellation.
func attemptError(attemptCtx context.Context, err error) error {
	if errors.Is(context.Cause(attemptCtx), errMirrorStalled) {
		return errMirrorStalled
	}
	return err
}

func (j *downloadJob) get(ctx context.Context, rawURL string, from int64) (*http.Response, error) {
	if _, err := requireHTTPS(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	// The SHA-256 covers the raw file; a transparently decompressed body would
	// also hide its length.
	req.Header.Set("Accept-Encoding", "identity")
	if from > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(from, 10)+"-")
	}
	return j.client.Do(req)
}

// open requests the remaining bytes (from byte 0 with full) and accepts only
// an answer that is exactly the edition: no content encoding, a 206 whose
// Content-Range starts at the offset and names the edition's size, or a 200
// whose Content-Length is the edition's size. Anything else (an error page, a
// chunked answer of unknown length) fails before a byte is written.
//
// A mirror that cannot continue at the offset (416, or 200 to a Range request)
// returns errRangeRefused and leaves the part file alone. With full, a 200
// with the whole file starts a restart (beginRestart); the part file stays
// until the restart file holds more bytes.
func (j *downloadJob) open(ctx context.Context, rawURL string, full bool) (*http.Response, error) {
	from := j.offset
	if full {
		from = 0
	}
	resp, err := j.get(ctx, rawURL, from)
	if err != nil {
		return nil, err
	}
	for _, value := range resp.Header.Values("Content-Encoding") {
		for _, encoding := range strings.Split(value, ",") {
			if encoding = strings.TrimSpace(encoding); encoding != "" && !strings.EqualFold(encoding, "identity") {
				resp.Body.Close()
				return nil, fmt.Errorf("mirror sent the file with content encoding %q", encoding)
			}
		}
	}
	switch {
	case resp.StatusCode == http.StatusPartialContent && from > 0:
		start, total, ok := parseContentRange(resp.Header.Get("Content-Range"))
		if !ok || start != from || (total >= 0 && total != j.size) {
			resp.Body.Close()
			return nil, fmt.Errorf("mirror answered range %q for offset %d of %d bytes", resp.Header.Get("Content-Range"), from, j.size)
		}
		return resp, nil
	case resp.StatusCode == http.StatusOK:
		if resp.ContentLength != j.size {
			resp.Body.Close()
			if resp.ContentLength < 0 {
				return nil, fmt.Errorf("mirror sent no length, the edition has %d bytes", j.size)
			}
			return nil, fmt.Errorf("mirror serves %d bytes, the edition has %d", resp.ContentLength, j.size)
		}
		if j.offset > 0 {
			if !full {
				resp.Body.Close()
				return nil, errRangeRefused
			}
			if err := j.beginRestart(); err != nil {
				resp.Body.Close()
				return nil, err
			}
		}
		return resp, nil
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && from > 0:
		resp.Body.Close()
		return nil, errRangeRefused
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}
}

// beginRestart sets the part file aside (file, hash and offset) and directs
// the writes into an empty restart file with a fresh hash.
//
// The free space is checked first (see restartRequiredBytes), so a restart
// that cannot fit pauses before it writes anything.
func (j *downloadJob) beginRestart() error {
	if err := j.checkSpaceFor(restartRequiredBytes(j.size, j.offset, 0)); err != nil {
		return err
	}
	file, err := os.OpenFile(j.restartPath(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return &writeError{err}
	}
	j.kept = &keptPart{file: j.file, hash: j.hash, offset: j.offset}
	j.file, j.hash, j.offset = file, sha256.New(), 0
	j.logger.Info("[LocalWikipedia] Starting the download over in a separate file", "kept", j.kept.offset)
	j.progress(0)
	return nil
}

// promoteRestart replaces the part file with the restart file, which now
// holds more bytes, and keeps writing to it. The restart's hash and offset
// become the job's.
func (j *downloadJob) promoteRestart() error {
	kept := j.kept
	j.kept = nil
	syncErr := j.file.Sync()
	closeErr := j.file.Close()
	j.file = nil
	_ = kept.file.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		_ = os.Remove(j.restartPath())
		return &writeError{fmt.Errorf("finish restarted download: %w", err)}
	}
	if err := fileutil.RenameContext(context.Background(), j.restartPath(), j.partPath); err != nil {
		_ = os.Remove(j.restartPath())
		return &writeError{fmt.Errorf("replace partial edition: %w", err)}
	}
	file, err := os.OpenFile(j.partPath, os.O_RDWR, 0o644)
	if err != nil {
		return &writeError{err}
	}
	if _, err := file.Seek(j.offset, io.SeekStart); err != nil {
		_ = file.Close()
		return &writeError{err}
	}
	j.file = file
	j.logger.Info("[LocalWikipedia] The restarted download replaced the partial file", "bytes", j.offset, "kept", kept.offset)
	return nil
}

// abandonRestart discards a restart that did not replace the part file and
// continues with the part file's own file, hash and offset. The phase is
// reported again before the progress jumps back to the part file's length,
// so the rate meter starts over instead of showing that jump as throughput.
func (j *downloadJob) abandonRestart() {
	if j.kept == nil {
		return
	}
	_ = j.file.Close()
	if err := removeIfExists(j.restartPath()); err != nil {
		j.logger.Warn("[LocalWikipedia] An unfinished restart file could not be removed", "file", j.restartPath(), "error", err)
	}
	j.file, j.hash, j.offset = j.kept.file, j.kept.hash, j.kept.offset
	j.kept = nil
	j.phase(StateDownloading)
	j.progress(j.offset)
}

func (j *downloadJob) copy(body io.Reader, watchdog *time.Timer) error {
	buf := make([]byte, copyBufferSize)
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			watchdog.Reset(j.stallTimeout)
			if j.offset+int64(n) > j.size {
				return fmt.Errorf("mirror sent more than %d bytes", j.size)
			}
			written, err := j.file.Write(buf[:n])
			j.hash.Write(buf[:written])
			j.offset += int64(written)
			if err != nil {
				return j.writeFailure(err)
			}
			j.progress(j.offset)
			if j.kept != nil && j.offset > j.kept.offset {
				if err := j.promoteRestart(); err != nil {
					return err
				}
			}
			j.unchecked += int64(written)
			if j.unchecked >= j.checkEvery {
				j.unchecked = 0
				if err := j.checkpoint(); err != nil {
					return err
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			if j.offset < j.size {
				return io.ErrUnexpectedEOF
			}
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// checkpoint flushes the part file to disk and re-checks the free space.
func (j *downloadJob) checkpoint() error {
	if err := j.file.Sync(); err != nil {
		return j.writeFailure(err)
	}
	return j.checkSpace()
}

// writeFailure reports a full disk as a pause, anything else as a write error.
func (j *downloadJob) writeFailure(err error) error {
	if spaceErr := j.checkSpace(); spaceErr != nil {
		return spaceErr
	}
	return &writeError{err}
}

// checkSpace checks the free space the rest of the download needs: missing
// bytes plus margin (requiredBytes), during a restart the exact need of the
// restart file next to the kept part file (restartRequiredBytes).
func (j *downloadJob) checkSpace() error {
	if j.kept != nil {
		return j.checkSpaceFor(restartRequiredBytes(j.size, j.kept.offset, j.offset))
	}
	return j.checkSpaceFor(requiredBytes(j.size, j.offset))
}

func (j *downloadJob) checkSpaceFor(required int64) error {
	if j.freeDisk == nil {
		return nil
	}
	free, err := j.freeDisk(j.dir)
	if err != nil {
		// Unknown free space was confirmed by the administrator before the start.
		return nil
	}
	if free < required {
		return &spaceError{required: required, available: free}
	}
	return nil
}

func (j *downloadJob) phase(phase string) {
	if j.onPhase != nil {
		j.onPhase(phase)
	}
}

func (j *downloadJob) progress(done int64) {
	if j.onProgress != nil {
		j.onProgress(done, j.size)
	}
}

// parseContentRange reads "bytes <start>-<end>/<total>"; total is -1 for "*".
func parseContentRange(header string) (start, total int64, ok bool) {
	rest, found := strings.CutPrefix(strings.TrimSpace(header), "bytes ")
	if !found {
		return 0, 0, false
	}
	span, totalText, found := strings.Cut(rest, "/")
	if !found {
		return 0, 0, false
	}
	startText, endText, found := strings.Cut(span, "-")
	if !found {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(strings.TrimSpace(startText), 10, 64)
	if err != nil || start < 0 {
		return 0, 0, false
	}
	end, err := strconv.ParseInt(strings.TrimSpace(endText), 10, 64)
	if err != nil || end < start {
		return 0, 0, false
	}
	total = -1
	if totalText = strings.TrimSpace(totalText); totalText != "*" {
		total, err = strconv.ParseInt(totalText, 10, 64)
		if err != nil || total <= end {
			return 0, 0, false
		}
	}
	return start, total, true
}
