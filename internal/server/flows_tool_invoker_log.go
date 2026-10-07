package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"aurago/internal/security"
)

// flowLogHandler cleans what the dispatcher logs while it runs a flow's tool call:
//   - every URL in any string value, in the message, and in the text of an error or any
//     other value loses its user info, query and fragment, under any key (api_request logs
//     its URL at Info, a camera tool logs a source URL, an error repeats the URL it failed
//     on, and a flow's URL can hold a key in its query);
//   - values under argument-like keys (body, args, arguments, params, payload,
//     authorization) and under any key that contains "header" are left out;
//   - registered secrets are scrubbed (security.Scrub), and every value is cut to
//     flowLogRunes runes.
type flowLogHandler struct{ inner slog.Handler }

// flowDispatchLogger wraps base (slog.Default when nil) in a flowLogHandler.
func flowDispatchLogger(base *slog.Logger) *slog.Logger {
	if base == nil {
		base = slog.Default()
	}
	return slog.New(flowLogHandler{inner: base.Handler()})
}

func (h flowLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h flowLogHandler) Handle(ctx context.Context, r slog.Record) error {
	out := slog.NewRecord(r.Time, r.Level, flowLogText(r.Message, 2*flowLogRunes), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(flowLogAttr(a))
		return true
	})
	return h.inner.Handle(ctx, out)
}

func (h flowLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cleaned := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		cleaned[i] = flowLogAttr(a)
	}
	return flowLogHandler{inner: h.inner.WithAttrs(cleaned)}
}

func (h flowLogHandler) WithGroup(name string) slog.Handler {
	return flowLogHandler{inner: h.inner.WithGroup(name)}
}

// flowLogOmittedKeys are attribute keys whose values are tool arguments as a whole. Any key
// that contains "header" is left out as well.
var flowLogOmittedKeys = map[string]bool{"body": true, "args": true, "arguments": true, "params": true,
	"payload": true, "authorization": true}

func flowLogAttr(a slog.Attr) slog.Attr {
	key := strings.ToLower(a.Key)
	if flowLogOmittedKeys[key] || strings.Contains(key, "header") {
		return slog.String(a.Key, "[omitted]")
	}
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, flowLogText(v.String(), flowLogRunes))
	case slog.KindGroup:
		group := v.Group()
		cleaned := make([]any, len(group))
		for i, g := range group {
			cleaned[i] = flowLogAttr(g)
		}
		return slog.Group(a.Key, cleaned...)
	case slog.KindAny:
		return slog.String(a.Key, flowLogText(fmt.Sprint(v.Any()), flowLogRunes))
	}
	return slog.Attr{Key: a.Key, Value: v}
}

// flowURLPattern finds URLs inside text: a scheme followed by "://" up to the next blank,
// quote or angle bracket.
var flowURLPattern = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s"'<>]+`)

// flowLogText redacts every URL in s, scrubs registered secrets and cuts the result.
func flowLogText(s string, maxRunes int) string {
	s = flowURLPattern.ReplaceAllStringFunc(s, flowRedactURL)
	return flowBoundRunes(security.Scrub(s), maxRunes)
}

// flowRedactURL keeps scheme, host and path of a URL for the log.
func flowRedactURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "[url]"
	}
	query := u.RawQuery != "" || u.ForceQuery
	u.User, u.RawQuery, u.ForceQuery, u.Fragment, u.RawFragment = nil, "", false, "", ""
	if query {
		return u.String() + "?[redacted]"
	}
	return u.String()
}
