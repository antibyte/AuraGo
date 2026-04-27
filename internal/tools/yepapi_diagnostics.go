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

func logYepAPIRequestPayload(ctx context.Context, toolName, operation, endpoint string, payload map[string]interface{}) {
	logger := yepAPILoggerFromContext(ctx)
	if logger == nil {
		return
	}
	logger.Warn("[YepAPI] Prepared request payload",
		"tool", toolName,
		"operation", operation,
		"endpoint", endpoint,
		"payload_keys", safeYepAPIPayloadKeys(payload),
	)
}

func logYepAPIMarshaledRequestBody(ctx context.Context, endpoint string, body []byte) {
	logger := yepAPILoggerFromContext(ctx)
	if logger == nil {
		return
	}
	logger.Warn("[YepAPI] Marshaled request body",
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
