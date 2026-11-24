package homedash

import "context"

// SetAuditForTest replaces the audit callback of a hub.
func SetAuditForTest(h *Hub, fn func(context.Context, Command, string)) {
	h.mu.Lock()
	h.opts.Audit = fn
	h.mu.Unlock()
}

// SetQueuedHookForTest registers a callback that runs after a command was parked in a latest-wins slot.
func SetQueuedHookForTest(h *Hub, fn func()) {
	h.cmds.mu.Lock()
	h.cmds.queuedHook = fn
	h.cmds.mu.Unlock()
}

// SetSemWaitHookForTest registers a callback that runs when a command starts waiting for one of the
// per-source in-flight slots.
func SetSemWaitHookForTest(h *Hub, fn func()) {
	h.cmds.mu.Lock()
	h.cmds.semWaitHook = fn
	h.cmds.mu.Unlock()
}

// SetDedupeWaitHookForTest registers a callback that runs when a duplicate request id starts waiting.
func SetDedupeWaitHookForTest(h *Hub, fn func()) {
	h.cmds.mu.Lock()
	h.cmds.dedupeWaitHook = fn
	h.cmds.mu.Unlock()
}
