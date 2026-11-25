package kaggle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// kernelLogReadCap bounds one notebook log read; ReadKernelLog never buffers more.
const kernelLogReadCap int64 = 16 << 20

// kernelLogStreamMax bounds the request and the read of one log read; a running session keeps its event stream open.
// kernelLogIdle ends the read when no byte arrived for that long after the response headers. Variables so tests can shorten them.
var (
	kernelLogStreamMax = 45 * time.Second
	kernelLogIdle      = 3 * time.Second
)

// kernelLogWaitSeconds lets Kaggle wait briefly for the log URL of a running session.
const kernelLogWaitSeconds = 10

// kernelRefPattern accepts a Kaggle user name or a bare kernel slug (no owner prefix).
var kernelRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`)

// ReadKernelLog reads the log of the latest session of one notebook (no version label: a version number is not a
// label and Kaggle answers 404 for it) without a durable job and without a project file. It needs the download
// grant, filesystem write access and network access, and owner must be the authenticated account. slug is the bare
// kernel slug. It returns at most the last maxBytes bytes (maxBytes <= 0 or above 16 MiB means 16 MiB; the read
// itself stops at 16 MiB) of the scrubbed log text. A running session answers a text/event-stream that replays the
// log and then stays open: the read ends at EOF, after kernelLogIdle without a byte or after kernelLogStreamMax,
// and for an event stream returns what arrived. A finished session answers a JSON array, where a failed or stalled
// read is an error. An empty result with a nil error means "no log yet" (HTTP 404, 204 or an empty body). Another
// HTTP status returns *APIError. A JSON object body is refused, because only a JSON array of log entries, event
// stream data or plain text is a log.
func (s *Service) ReadKernelLog(ctx context.Context, access Access, owner, slug string, maxBytes int64) ([]byte, error) {
	if !kernelRefPattern.MatchString(owner) || !kernelRefPattern.MatchString(slug) {
		return nil, errors.New("invalid Kaggle notebook reference")
	}
	if maxBytes <= 0 || maxBytes > kernelLogReadCap {
		maxBytes = kernelLogReadCap
	}
	op, err := Lookup("notebooks", "get_kernel_session_logs_stream")
	if err != nil {
		return nil, err
	}
	params := map[string]any{"userName": owner, "kernelSlug": slug, "waitForLogsUrlSeconds": kernelLogWaitSeconds}
	snap, c, err := s.client()
	if err != nil {
		return nil, err
	}
	if run, ok := ctx.Value(grantSnapshotKey{}).(config.KaggleConfig); ok {
		snap.Config = intersectConfig(snap.Config, run)
	}
	access = intersect(access, snap.Access)
	if err := CheckGrants(snap.Config, access, op, params); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("encode Kaggle log request: %w", err)
	}
	_, typed, err := Decode(op, raw)
	if err != nil {
		return nil, err
	}
	account, err := c.Identity(ctx)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(account, owner) {
		return nil, errors.New("Kaggle notebook logs are readable only for the authenticated account")
	}
	callerCtx := ctx
	ctx, cancel := context.WithTimeout(ctx, kernelLogStreamMax)
	defer cancel()
	resp, err := c.request(ctx, op, typed)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	resp, err = s.followStorageRedirect(ctx, resp, time.Duration(snap.Config.RequestTimeoutSeconds)*time.Second)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNoContent, http.StatusNotFound:
		return nil, nil
	default:
		return nil, &APIError{Status: resp.StatusCode}
	}
	body, err := readLogBody(callerCtx, cancel, resp.Body, strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream"))
	if err != nil {
		return nil, err
	}
	if trimmed := bytes.TrimLeft(body, " \t\r\n"); len(trimmed) > 0 && trimmed[0] == '{' {
		return nil, errors.New("Kaggle returned an unexpected notebook log response")
	}
	text := security.Scrub(string(body))
	if int64(len(text)) > maxBytes {
		text = text[len(text)-int(maxBytes):]
	}
	if text == "" {
		return nil, nil
	}
	return []byte(text), nil
}

// readLogBody reads up to kernelLogReadCap bytes and cancels the request when no byte arrives for kernelLogIdle.
// For an event stream a read that ends by that timer, the overall bound or a transport error is a normal end; any
// other read error, and the end of the caller's own context, is returned.
func readLogBody(caller context.Context, cancel context.CancelFunc, body io.Reader, stream bool) ([]byte, error) {
	idle := time.AfterFunc(kernelLogIdle, cancel)
	defer idle.Stop()
	var buf bytes.Buffer
	chunk := make([]byte, 32<<10)
	for int64(buf.Len()) < kernelLogReadCap {
		n, err := body.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:min(int64(n), kernelLogReadCap-int64(buf.Len()))])
			idle.Reset(kernelLogIdle)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			if caller.Err() != nil || !stream {
				return nil, fmt.Errorf("read Kaggle notebook log: %w", err)
			}
			break
		}
	}
	return buf.Bytes(), nil
}
