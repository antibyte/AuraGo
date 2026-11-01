package server

import (
	"log"
	"log/slog"
	"strings"
)

func newHTTPServerErrorLog(logger Logger) *log.Logger {
	if logger == nil {
		logger = slog.Default()
	}
	return log.New(httpServerErrorWriter{logger}, "", 0)
}

type httpServerErrorWriter struct{ logger Logger }

func (w httpServerErrorWriter) Write(p []byte) (int, error) {
	message := strings.TrimSpace(string(p))
	// Clients may open TCP and close before completing TLS. Keep only that
	// exact EOF at Debug; certificate, protocol and other server errors stay visible.
	if strings.HasPrefix(message, "http: TLS handshake error from ") && strings.HasSuffix(message, ": EOF") {
		if debug, ok := w.logger.(interface{ Debug(string, ...any) }); ok {
			debug.Debug(message)
		}
	} else {
		w.logger.Error(message)
	}
	return len(p), nil
}
