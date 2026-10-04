package telnyx

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Rejecting an unauthorized inbound call is enforcement, including in read-only
// mode. It never authorizes answering, sending messages or making another call.
func (h *WebhookHandler) rejectIncomingCall(id, cause string) {
	if id == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _, err := h.client.do(ctx, http.MethodPost, "/calls/"+url.PathEscape(id)+"/actions/reject", map[string]string{"cause": cause})
	if err != nil {
		h.logger.Warn("Telnyx: rejection result unconfirmed", "error", err)
	}
}
