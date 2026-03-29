package server

import (
	"log/slog"
	"net/http"
)

func jsonLoggedError(w http.ResponseWriter, logger *slog.Logger, status int, clientMessage string, logMessage string, err error, attrs ...any) {
	if logger != nil && err != nil {
		logAttrs := append([]any{}, attrs...)
		logAttrs = append(logAttrs, "error", err)
		if status >= 500 {
			logger.Error(logMessage, logAttrs...)
		} else {
			logger.Warn(logMessage, logAttrs...)
		}
	}
	jsonError(w, clientMessage, status)
}
