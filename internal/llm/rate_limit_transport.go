package llm

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// maxRateLimitBodyBytes bounds how much of a 429 response body is read into
// the resulting RateLimitError message.
const maxRateLimitBodyBytes = 4 << 10

type rateLimitAwareTransport struct {
	base http.RoundTripper
}

func (t *rateLimitAwareTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil || resp == nil || resp.StatusCode != http.StatusTooManyRequests {
		return resp, err
	}

	retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
	// The body only feeds the error message; cap it so a hostile or broken
	// endpoint cannot buffer megabytes into memory, errors and logs.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxRateLimitBodyBytes))
	resp.Body.Close()

	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = "rate limit exceeded"
	}
	apiErr := &openai.APIError{
		HTTPStatusCode: http.StatusTooManyRequests,
		Message:        msg,
	}
	rlErr := &RateLimitError{
		LLMError:          WrapError(ErrCategoryRateLimit, apiErr, "rate limited"),
		RetryAfterSeconds: int(retryAfter.Seconds()),
	}
	if rlErr.RetryAfterSeconds <= 0 {
		rlErr.RetryAfterSeconds = 0
	}
	return nil, rlErr
}

func parseRetryAfterHeader(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		wait := time.Until(when)
		if wait > 0 {
			return wait
		}
	}
	return 0
}
