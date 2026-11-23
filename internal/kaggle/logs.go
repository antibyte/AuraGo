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
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// kernelLogReadCap bounds one notebook log read; ReadKernelLog never buffers more.
const kernelLogReadCap int64 = 16 << 20

// kernelLogWaitSeconds lets Kaggle wait briefly for the log URL of a running session.
const kernelLogWaitSeconds = 10

// kernelRefPattern accepts a Kaggle user name or a bare kernel slug (no owner prefix).
var kernelRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`)

// ReadKernelLog reads the session log of one notebook version without a durable job and without a
// project file. It needs the download grant, filesystem write access and network access, and owner
// must be the authenticated account. slug is the bare kernel slug. It returns at most the last
// maxBytes bytes (maxBytes <= 0 or above 16 MiB means 16 MiB; the read itself stops at 16 MiB) of
// the scrubbed log text. An empty result with a nil error means "no log yet" (HTTP 404, 204 or an
// empty body). Another HTTP status returns *APIError. A JSON object body is refused, because only
// a JSON array of log entries or plain text is a log.
func (s *Service) ReadKernelLog(ctx context.Context, access Access, owner, slug string, version int, maxBytes int64) ([]byte, error) {
	if !kernelRefPattern.MatchString(owner) || !kernelRefPattern.MatchString(slug) {
		return nil, errors.New("invalid Kaggle notebook reference")
	}
	if version <= 0 {
		return nil, errors.New("Kaggle notebook version must be positive")
	}
	if maxBytes <= 0 || maxBytes > kernelLogReadCap {
		maxBytes = kernelLogReadCap
	}
	op, err := Lookup("notebooks", "get_kernel_session_logs_stream")
	if err != nil {
		return nil, err
	}
	params := map[string]any{"userName": owner, "kernelSlug": slug, "versionLabel": strconv.Itoa(version), "waitForLogsUrlSeconds": kernelLogWaitSeconds}
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, kernelLogReadCap))
	if err != nil {
		return nil, fmt.Errorf("read Kaggle notebook log: %w", err)
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
