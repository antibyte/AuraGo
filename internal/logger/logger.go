package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// LogFile wraps a logger and its optional file handle for clean shutdown.
type LogFile struct {
	Logger *slog.Logger
	file   *os.File
}

// Close closes the underlying log file, if any.
func (lf *LogFile) Close() error {
	if lf.file != nil {
		return lf.file.Close()
	}
	return nil
}

func Setup(debug bool) *slog.Logger {
	return buildLogger(os.Stdout, nil, debug)
}

// SetupWithFile creates a logger that writes to both stdout and the given file.
// The returned LogFile must be closed on shutdown to release the file handle.
func SetupWithFile(debug bool, logPath string, appendMode bool) (*LogFile, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, err
	}

	mode := os.O_TRUNC
	if appendMode {
		mode = os.O_APPEND
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|mode|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &LogFile{
		Logger: buildLogger(os.Stdout, file, debug),
		file:   file,
	}, nil
}

func buildLogger(stdout *os.File, file *os.File, debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var writer io.Writer = stdout
	if file != nil {
		writer = io.MultiWriter(stdout, file)
	}

	handler := slog.NewTextHandler(writer, opts)
	return slog.New(handler)
}
