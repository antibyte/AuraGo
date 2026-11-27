package chatrt

import "errors"

// Errors of the chat runtime. Callers map them to HTTP answers and stream codes with errors.Is.
var (
	ErrDisabled     = errors.New("chat runtime disabled")
	ErrUnsupported  = errors.New("chat runtime unsupported on this platform")
	ErrNotInstalled = errors.New("chat runtime not installed")
	ErrInstalling   = errors.New("chat runtime is being installed")
	ErrBusy         = errors.New("chat runtime busy")
	ErrModelMissing = errors.New("model file missing")
	ErrContext      = errors.New("conversation exceeds the context size")
	// ErrStopped ends a generation or a start that StopAll or Close interrupted.
	ErrStopped = errors.New("chat runtime stopped")
	// ErrClosed answers every call after Close.
	ErrClosed = errors.New("chat runtime closed")
)

// errProcessExited marks a request that failed because the llama-server process is gone.
var errProcessExited = errors.New("llama-server process exited")

// errStreamCut marks a stream that ended without a finish reason or the [DONE] marker.
var errStreamCut = errors.New("llama-server stream ended early")

// errUpstream marks an error answer of llama-server that is not a context overflow.
var errUpstream = errors.New("llama-server refused the request")
