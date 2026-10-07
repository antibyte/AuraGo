package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aurago/internal/fritzbox"
	"aurago/internal/security"
)

func (s *Server) configureFritzPoller() {
	s.fritzPollerMu.Lock()
	defer s.fritzPollerMu.Unlock()
	if previous := s.fritzPoller.Swap(nil); previous != nil {
		previous.Stop()
	}
	if s.fritzPollerClosed || s.MissionManagerV2 == nil {
		return
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	cfg := s.ConfigSnapshot()
	if cfg == nil || cfg.EggMode.Enabled || !cfg.FritzBox.Enabled || !cfg.FritzBox.Telephony.Enabled || !cfg.FritzBox.Telephony.Polling.Enabled {
		return
	}
	parent := s.integrationCtx
	if parent == nil {
		parent = context.Background()
	}
	var poller *fritzbox.Poller
	poller = fritzbox.NewPoller(*cfg, func(kind, summary string) {
		ctx := poller.Context()
		if ctx.Err() != nil {
			return
		}
		s.MissionManagerV2.NotifyFritzBoxEvent(kind, summary)
		if s.fritzLoopbackSem != nil {
			if err := acquireLoopbackSem(ctx, s.fritzLoopbackSem); err != nil {
				return
			}
			defer releaseLoopbackSem(s.fritzLoopbackSem)
		}
		prompt := fmt.Sprintf("[FRITZ!BOX EVENT: %s] %s", kind, security.IsolateExternalData(summary))
		body, _ := json.Marshal(map[string]any{"model": "aurago", "stream": false, "messages": []map[string]string{{"role": "user", "content": prompt}}})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, InternalAPIURL(s.ConfigSnapshot())+"/v1/chat/completions", strings.NewReader(string(body)))
		if err != nil {
			s.Logger.Warn("[FritzBox Poller] Request construction failed", "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-FollowUp", "true")
		req.Header.Set("X-Internal-Token", s.internalToken)
		client := NewInternalHTTPClient(10 * time.Minute)
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() == nil {
				s.Logger.Warn("[FritzBox Poller] Loopback request failed", "error", err)
			}
			return
		}
		_ = resp.Body.Close()
	}, s.Logger)
	s.fritzPoller.Store(poller)
	poller.StartContext(parent)
}

func (s *Server) stopFritzPoller() {
	s.fritzPollerMu.Lock()
	defer s.fritzPollerMu.Unlock()
	s.fritzPollerClosed = true
	if previous := s.fritzPoller.Swap(nil); previous != nil {
		previous.Stop()
	}
}
