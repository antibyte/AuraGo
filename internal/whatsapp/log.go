package whatsapp

import (
	"context"
	"fmt"
	"log/slog"

	"aurago/internal/security"

	waBinary "go.mau.fi/whatsmeow/binary"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// slogLogger forwards whatsmeow logging to slog. Debug output is dropped
// because it carries QR payloads and raw protocol nodes. Every line is
// scrubbed, and phone numbers keep only their last four digits.
type slogLogger struct {
	logger *slog.Logger
	module string
}

func newWALogger(logger *slog.Logger, module string) waLog.Logger {
	if logger == nil {
		return waLog.Noop
	}
	return slogLogger{logger: logger, module: module}
}

func (l slogLogger) Errorf(msg string, args ...any) { l.log(slog.LevelError, msg, args...) }
func (l slogLogger) Warnf(msg string, args ...any)  { l.log(slog.LevelWarn, msg, args...) }
func (l slogLogger) Infof(msg string, args ...any)  { l.log(slog.LevelInfo, msg, args...) }
func (l slogLogger) Debugf(string, ...any)          {}

func (l slogLogger) Sub(module string) waLog.Logger {
	return slogLogger{logger: l.logger, module: l.module + "/" + module}
}

func (l slogLogger) log(level slog.Level, msg string, args ...any) {
	text := maskDigits(security.Scrub(fmt.Sprintf(msg, redactNodes(args)...)))
	l.logger.Log(context.Background(), level, "whatsmeow: "+text, "module", l.module)
}

// redactNodes replaces protocol nodes with their tag. A node prints its whole
// content, which can hold pairing refs and other secrets.
func redactNodes(args []any) []any {
	out := make([]any, len(args))
	for i, arg := range args {
		switch node := arg.(type) {
		case *waBinary.Node:
			if node == nil {
				out[i] = "<node>"
			} else {
				out[i] = "<node " + node.Tag + ">"
			}
		case waBinary.Node:
			out[i] = "<node " + node.Tag + ">"
		default:
			out[i] = arg
		}
	}
	return out
}
