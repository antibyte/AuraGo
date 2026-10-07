package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"aurago/internal/security"
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
	return buildLogger(os.Stdout, debug)
}

// SetupWithFile creates a logger that writes to both stdout and the given file.
// The file is private to the owner (0600, see openLogFile); a log shipper
// running as another user needs group access granted with chmod.
// The returned LogFile must be closed on shutdown to release the file handle.
func SetupWithFile(debug bool, logPath string, appendMode bool) (*LogFile, error) {
	file, err := openLogFile(logPath, appendMode)
	if err != nil {
		return nil, err
	}

	return &LogFile{
		Logger: buildLogger(io.MultiWriter(os.Stdout, file), debug),
		file:   file,
	}, nil
}

// SetupFileOnly creates a logger that writes exclusively to the given file,
// with the same owner-only file mode as SetupWithFile.
// The returned LogFile must be closed on shutdown to release the file handle.
func SetupFileOnly(debug bool, logPath string, appendMode bool) (*LogFile, error) {
	file, err := openLogFile(logPath, appendMode)
	if err != nil {
		return nil, err
	}

	return &LogFile{
		Logger: buildLogger(file, debug),
		file:   file,
	}, nil
}

// openLogFile opens a log file readable by its owner only (0600): log lines
// carry prompts, paths and tool output. An existing regular file that is
// readable by everyone (created by an older release) is tightened to 0600 on
// open; a mode without world access is left alone, so a log shipper running
// as another user can be given group access with chmod (for example 0640 plus
// a shared group) and keeps it across restarts. A non-regular target (a
// symlink to /dev/null, a pipe) is never chmod'ed. Windows has no POSIX modes
// to adjust.
func openLogFile(logPath string, appendMode bool) (*os.File, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, err
	}

	mode := os.O_TRUNC
	if appendMode {
		mode = os.O_APPEND
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|mode|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" {
		// Only a regular file is ours to tighten: a log path symlinked to
		// /dev/null or a collector pipe keeps the mode its owner chose.
		if info, statErr := file.Stat(); statErr == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o007 != 0 {
			_ = file.Chmod(0o600)
		}
	}
	return file, nil
}

func buildLogger(writer io.Writer, debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			a.Value = a.Value.Resolve()
			switch a.Value.Kind() {
			case slog.KindString:
				a.Value = slog.StringValue(security.Scrub(a.Value.String()))
			case slog.KindAny:
				a.Value = slog.StringValue(security.Scrub(fmt.Sprint(a.Value.Any())))
			}
			return a
		},
	}

	if writer == nil {
		writer = io.Discard
	}

	handler := slog.NewTextHandler(writer, opts)
	return slog.New(handler)
}
