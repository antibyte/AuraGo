package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/remote"
)

const maxCachedCommandResults = 1024
const maxCachedResultBytes = 32 << 20
const maxConcurrentCommands = 4

func (e *Executor) Execute(cmd remote.CommandPayload, readOnly bool, paths []string) remote.ResultPayload {
	return e.ExecuteContext(context.Background(), cmd, readOnly, paths)
}

func (e *Executor) ExecuteContext(ctx context.Context, cmd remote.CommandPayload, readOnly bool, paths []string) remote.ResultPayload {
	record, owner, refusal := e.admit(cmd.CommandID)
	if refusal != nil {
		return *refusal
	}
	if !owner {
		select {
		case <-record.done:
			return e.cached(record)
		case <-ctx.Done():
			return commandError(cmd.CommandID, "CANCELLED", ctx.Err())
		}
	}
	return e.run(ctx, cmd, readOnly, paths, record)
}

// Start admits synchronously so an overloaded peer cannot create execution goroutines.
func (e *Executor) Start(ctx context.Context, cmd remote.CommandPayload, readOnly bool, paths []string, deliver func(remote.ResultPayload)) {
	record, owner, refusal := e.admit(cmd.CommandID)
	if refusal != nil {
		deliver(*refusal)
		return
	}
	if !owner {
		select {
		case <-record.done:
			deliver(e.cached(record))
		default:
			deliver(commandError(cmd.CommandID, "COMMAND_IN_PROGRESS", fmt.Errorf("command is already running")))
		}
		return
	}
	go func() { deliver(e.run(ctx, cmd, readOnly, paths, record)) }()
}

func commandError(id, code string, err error) remote.ResultPayload {
	return remote.ResultPayload{CommandID: id, Status: "error", ErrorCode: code, Error: err.Error()}
}

func (e *Executor) admit(id string) (*commandExecutionRecord, bool, *remote.ResultPayload) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if id != "" {
		if record := e.commandResults[id]; record != nil {
			return record, false, nil
		}
	}
	if e.active >= maxConcurrentCommands {
		r := commandError(id, "CLIENT_BUSY", fmt.Errorf("remote client is busy (maximum four commands)"))
		return nil, false, &r
	}
	e.active++
	e.running.Add(1)
	record := &commandExecutionRecord{done: make(chan struct{})}
	if id != "" {
		if e.commandResults == nil {
			e.commandResults = make(map[string]*commandExecutionRecord)
		}
		e.commandResults[id] = record
	}
	return record, true, nil
}

func (e *Executor) cached(record *commandExecutionRecord) remote.ResultPayload {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return record.result
}

func (e *Executor) run(ctx context.Context, cmd remote.CommandPayload, ro bool, paths []string, record *commandExecutionRecord) (result remote.ResultPayload) {
	defer e.running.Done()
	started := time.Now()
	defer func() {
		if p := recover(); p != nil {
			result = commandError(cmd.CommandID, "EXECUTION_FAILED", fmt.Errorf("command failed: %v", p))
		}
		result.DurationMs = time.Since(started).Milliseconds()
		if cmd.Operation != remote.OpFileRead && len(result.Output) > remote.MaxTextOutputBytes {
			cut := remote.MaxTextOutputBytes - len(truncationMarker)
			for cut > 0 && !utf8.RuneStart(result.Output[cut]) {
				cut--
			}
			result.Output = result.Output[:cut] + truncationMarker
			result.Truncated = true
		}
		if strings.HasSuffix(result.Output, truncationMarker) {
			result.Truncated = true
		}
		e.mu.Lock()
		e.active--
		record.result = result
		record.closeDone()
		if cmd.CommandID != "" {
			e.commandOrder = append(e.commandOrder, cmd.CommandID)
		}
		e.trimCommandResultCacheLocked(cmd.CommandID)
		e.mu.Unlock()
	}()
	seconds := cmd.TimeoutSec
	if seconds <= 0 {
		seconds = 60
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return commandError(cmd.CommandID, "CANCELLED", err)
	}
	limit := e.maxFileSizeBytesSnapshot()
	if cmd.Operation != remote.OpFileWrite {
		data, err := json.Marshal(cmd.Args)
		if err != nil || int64(len(data)) > limit {
			return commandError(cmd.CommandID, "FILE_TOO_LARGE", fmt.Errorf("command input exceeds configured file limit"))
		}
	}
	op := &fileOperation{Executor: e, ctx: ctx, limit: limit}
	if !remote.IsShellOperation(cmd.Operation) && cmd.Operation != remote.OpSysinfo {
		path, _ := cmd.Args["path"].(string)
		if err := op.authorize(path, paths); err != nil {
			result := commandError(cmd.CommandID, "FILE_ACCESS_DENIED", err)
			result.Status = "denied"
			return result
		}
		defer op.root.Close()
		unlock := e.lockFile(op.root.Name() + "/" + op.path)
		defer unlock()
	}
	result = op.executeOnce(cmd, ro, paths)
	if err := ctx.Err(); err != nil {
		return commandError(cmd.CommandID, "CANCELLED", err)
	}
	return result
}

// Completed IDs outlive their cached payload; in-flight IDs are never evicted.
func (e *Executor) trimCommandResultCacheLocked(_ string) {
	kept := e.commandOrder[:0]
	for _, id := range e.commandOrder {
		r := e.commandResults[id]
		if r == nil {
			continue
		}
		select {
		case <-r.done:
			kept = append(kept, id)
		default:
		}
	}
	e.commandOrder = kept
	for len(e.commandOrder) > maxCachedCommandResults {
		delete(e.commandResults, e.commandOrder[0])
		e.commandOrder = e.commandOrder[1:]
	}
	var size int
	for _, id := range e.commandOrder {
		r := e.commandResults[id]
		size += len(r.result.Output) + len(r.result.Error)
	}
	for _, id := range e.commandOrder {
		if size <= maxCachedResultBytes {
			break
		}
		r := e.commandResults[id]
		size -= len(r.result.Output) + len(r.result.Error)
		r.result = commandError(id, "RESULT_EVICTED", fmt.Errorf("command already completed; cached result has expired"))
		size += len(r.result.Error)
	}
}
