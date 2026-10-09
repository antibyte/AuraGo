package localwiki

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDiskCheckInterval int64 = 1 << 30
	defaultStallTimeout            = 2 * time.Minute
	copyBufferSize                 = 1 << 20
	rehashChunk                    = 4 << 20
	userAgent                      = "AuraGo-LocalWikipedia/1"
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

// downloadJob fetches one edition into partPath: HTTP Range resume, mirror
// fallback, SHA-256 while downloading (an existing part is re-hashed first),
// a free-space re-check every checkEvery bytes and a stall watchdog.
//
// Mirrors are tried in order. A mirror that fails moves on to the next one; a
// round over all mirrors that moved the download forward is followed by
// another round, a round without progress ends the job with errDownloadFailed.
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
	onPhase      func(phase string) // StateVerifying while re-hashing, then StateDownloading
	onProgress   func(done, total int64)
	onURL        func(finalURL string) // mirror that is answering (after redirects)

	file   *os.File
	hash   hash.Hash
	offset int64
}

// run returns nil once partPath holds exactly size bytes with the expected
// SHA-256. A context error means cancel or shutdown (the part file is kept), a
// *spaceError pauses the download, errChecksumMismatch removes the part file,
// errDownloadFailed means the mirrors stopped making progress (the part file
// is kept for a later resume).
func (j *downloadJob) run(ctx context.Context) error {
	defer j.closeFile()
	if err := j.validate(); err != nil {
		return err
	}
	if err := j.prepare(ctx); err != nil {
		return err
	}
	var lastErr error
	for j.offset < j.size {
		roundStart := j.offset
		for _, rawURL := range j.urls {
			if j.offset == j.size {
				break
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			attemptErr := j.fetch(ctx, rawURL)
			if attemptErr == nil {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			var space *spaceError
			var write *writeError
			if errors.As(attemptErr, &space) || errors.As(attemptErr, &write) {
				return attemptErr
			}
			j.logger.Warn("[LocalWikipedia] Mirror failed, trying the next one", "host", hostOf(rawURL), "offset", j.offset, "error", attemptErr)
			lastErr = attemptErr
		}
		if j.offset <= roundStart {
			break
		}
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

func (j *downloadJob) prepare(ctx context.Context) error {
	if j.checkEvery <= 0 {
		j.checkEvery = defaultDiskCheckInterval
	}
	if j.stallTimeout <= 0 {
		j.stallTimeout = defaultStallTimeout
	}
	if j.logger == nil {
		j.logger = slog.Default()
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

// fetch runs one attempt against one mirror. The stall watchdog covers the
// wait for the response headers and every read of the body.
func (j *downloadJob) fetch(ctx context.Context, rawURL string) error {
	attemptCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	watchdog := time.AfterFunc(j.stallTimeout, func() { cancel(errMirrorStalled) })
	defer watchdog.Stop()
	resp, err := j.open(attemptCtx, rawURL)
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

func (j *downloadJob) get(ctx context.Context, rawURL string) (*http.Response, error) {
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
	if j.offset > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(j.offset, 10)+"-")
	}
	return j.client.Do(req)
}

// open requests the remaining bytes. 206 continues at the offset; 200 (Range
// ignored) and 416 (Range not satisfiable) restart the part file from byte 0.
// A 200 answer restarts only when its length is the edition's size, so an
// error page served with 200 never discards the bytes already downloaded.
func (j *downloadJob) open(ctx context.Context, rawURL string) (*http.Response, error) {
	resp, err := j.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusPartialContent && j.offset > 0:
		start, total, ok := parseContentRange(resp.Header.Get("Content-Range"))
		if !ok || start != j.offset || (total >= 0 && total != j.size) {
			resp.Body.Close()
			return nil, fmt.Errorf("mirror answered range %q for offset %d of %d bytes", resp.Header.Get("Content-Range"), j.offset, j.size)
		}
		return resp, nil
	case resp.StatusCode == http.StatusOK:
		if resp.ContentLength >= 0 && resp.ContentLength != j.size {
			resp.Body.Close()
			return nil, fmt.Errorf("mirror serves %d bytes, the edition has %d", resp.ContentLength, j.size)
		}
		if j.offset > 0 {
			if resp.ContentLength < 0 {
				resp.Body.Close()
				return nil, errors.New("mirror ignored the range and sent no length; keeping the downloaded bytes")
			}
			if err := j.restart(); err != nil {
				resp.Body.Close()
				return nil, err
			}
		}
		return resp, nil
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && j.offset > 0:
		resp.Body.Close()
		if err := j.restart(); err != nil {
			return nil, err
		}
		return j.open(ctx, rawURL)
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}
}

func (j *downloadJob) restart() error {
	if err := j.file.Truncate(0); err != nil {
		return &writeError{err}
	}
	if _, err := j.file.Seek(0, io.SeekStart); err != nil {
		return &writeError{err}
	}
	j.hash.Reset()
	j.offset = 0
	j.progress(0)
	return nil
}

func (j *downloadJob) copy(body io.Reader, watchdog *time.Timer) error {
	buf := make([]byte, copyBufferSize)
	var sinceCheck int64
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
			sinceCheck += int64(written)
			if sinceCheck >= j.checkEvery {
				sinceCheck = 0
				if err := j.checkSpace(); err != nil {
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

// writeFailure reports a full disk as a pause, anything else as a write error.
func (j *downloadJob) writeFailure(err error) error {
	if spaceErr := j.checkSpace(); spaceErr != nil {
		return spaceErr
	}
	return &writeError{err}
}

func (j *downloadJob) checkSpace() error {
	if j.freeDisk == nil {
		return nil
	}
	free, err := j.freeDisk(j.dir)
	if err != nil {
		// Unknown free space was confirmed by the administrator before the start.
		return nil
	}
	if required := requiredBytes(j.size, j.offset); free < required {
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
