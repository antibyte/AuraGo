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
