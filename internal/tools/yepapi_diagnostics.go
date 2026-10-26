package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
)

type yepAPILoggerContextKey struct{}

// WithYepAPILogger attaches a request-scoped logger for YepAPI diagnostics.
func WithYepAPILogger(ctx context.Context, logger *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, yepAPILoggerContextKey{}, logger)
}

func yepAPILoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return nil
	}
	if logger, ok := ctx.Value(yepAPILoggerContextKey{}).(*slog.Logger); ok {
		return logger
	}
	return nil
}

type yepAPIRequestContextKey struct{}

// yepAPIRequestContext names the tool and operation behind a request.
type yepAPIRequestContext struct {
	Tool      string
	Operation string
}

// WithYepAPIRequestContext attaches the dispatching tool and operation so the
// request diagnostics name them for every YepAPI tool, not just one family.
func WithYepAPIRequestContext(ctx context.Context, tool, operation string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, yepAPIRequestContextKey{}, yepAPIRequestContext{Tool: tool, Operation: operation})
}

func yepAPIRequestContextFromContext(ctx context.Context) yepAPIRequestContext {
	if ctx == nil {
		return yepAPIRequestContext{}
	}
	if info, ok := ctx.Value(yepAPIRequestContextKey{}).(yepAPIRequestContext); ok {
		return info
	}
	return yepAPIRequestContext{}
}

// logYepAPIRequestPayload records the request shape. Only key names are
// logged, never payload values.
func logYepAPIRequestPayload(ctx context.Context, endpoint string, payloadKeys []string) {
	logger := yepAPILoggerFromContext(ctx)
	if logger == nil {
		return
	}
	info := yepAPIRequestContextFromContext(ctx)
	logger.Debug("[YepAPI] Prepared request payload",
		"tool", info.Tool,
		"operation", info.Operation,
		"endpoint", endpoint,
		"payload_keys", payloadKeys,
	)
}

func logYepAPIMarshaledRequestBody(ctx context.Context, endpoint string, body []byte) {
	logger := yepAPILoggerFromContext(ctx)
	if logger == nil {
		return
	}
	logger.Debug("[YepAPI] Marshaled request body",
		"endpoint", endpoint,
		"body_keys", safeYepAPIJSONBodyKeys(body),
	)
}

func safeYepAPIPayloadKeys(payload map[string]interface{}) []string {
	if len(payload) == 0 {
		return nil
	}
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func safeYepAPIJSONBodyKeys(body []byte) []string {
	if len(body) == 0 {
		return nil
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil
	}
	return safeYepAPIPayloadKeys(obj)
}
