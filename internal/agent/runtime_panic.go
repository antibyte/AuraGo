package agent

import (
	"context"
	"errors"
	"log/slog"
)

// ErrRuntimePanic marks an uncertain execution boundary. It is never retryable.
var ErrRuntimePanic = errors.New("agent runtime interrupted unexpectedly; action outcome may be unknown")

// recoverToolDispatch must be deferred directly by the dispatch boundary.
// Never expose panic values: dependencies can panic with secrets or request data.
func recoverToolDispatch(dc *DispatchContext, result *ToolDispatchResult) {
	if recover() == nil {
		return
	}
	if dc != nil && dc.Logger != nil {
		dc.Logger.Error("Tool dispatch panicked; execution outcome requires verification")
	}
	*result = ToolDispatchResult{Status: ToolResultUnknown, IsError: true,
		Output: `{"status":"unknown","code":"tool_panic","message":"Execution stopped unexpectedly. The action may have taken effect. Verify its state before any retry."}`}
}

func recoverAgentRuntime(ctx context.Context, logger *slog.Logger, retErr *error) {
	if recover() == nil {
		return
	}
	if logger != nil {
		logger.Error("Agent runtime panicked; run stopped without replay")
	}
	*retErr = ErrRuntimePanic
	if ctx.Err() != nil {
		*retErr = ctx.Err()
	}
}
