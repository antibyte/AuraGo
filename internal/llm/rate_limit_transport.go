package llm

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

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
	body, _ := io.ReadAll(resp.Body)
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