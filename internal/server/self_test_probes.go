package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/selftest"
)

// These existing probes have reviewed, read-only saved-configuration contracts.
// A new endpoint is never admitted merely because its route contains "test".
func (s *Server) selfTestProbes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"integration:telegram": handleTelegramConnectionTest(s), "integration:discord": handleDiscordTest(s),
		"integration:rocketchat": handleRocketChatTest(s), "integration:home-assistant": handleHomeAssistantTest(s),
		"integration:proxmox": handleProxmoxTest(s), "integration:s3": handleS3Test(s),
		"integration:ansible": handleAnsibleTest(s), "integration:frigate": handleFrigateTest(s),
	}
}

func (s *Server) bindSelfTestProbes(inventory []selftest.Capability) {
	probes := s.selfTestProbes()
	for i := range inventory {
		if _, ok := probes[inventory[i].Name]; ok && inventory[i].Enabled {
			inventory[i].Callable = true
			inventory[i].Status = "active"
		}
	}
}

type selfTestProbeResponse struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (w *selfTestProbeResponse) Header() http.Header { return w.header }
func (w *selfTestProbeResponse) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
}
func (w *selfTestProbeResponse) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = 200
	}
	if w.body.Len()+len(b) <= 16384 {
		_, _ = w.body.Write(b)
	}
	return len(b), nil
}

func selfTestDiagnosis(body string) string {
	b := strings.ToLower(body)
	for _, pair := range [][2]string{{"401", "auth"}, {"unauthorized", "auth"}, {"403", "permission"}, {"forbidden", "permission"}, {"429", "quota"}, {"rate limit", "quota"}, {"timeout", "timeout"}, {"deadline", "timeout"}, {"certificate", "network_tls"}, {"no such host", "network_dns"}, {"connection refused", "network"}, {"not configured", "setup"}, {"disabled", "permission"}} {
		if strings.Contains(b, pair[0]) {
			return pair[1]
		}
	}
	return "connection_probe_failed"
}

func (s *Server) executeSelfTestProbe(ctx context.Context, c selftest.Case, emit func(selftest.Evidence)) selftest.Execution {
	probe, ok := s.selfTestProbes()[c.Tool]
	if !ok {
		return selftest.Execution{Verdict: "BLOCKED_ACCESS", Code: "diagnostic_adapter_required", Cleanup: "not_needed"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://selftest.invalid/", nil)
	w := &selfTestProbeResponse{header: http.Header{}}
	start := time.Now()
	probe(w, req)
	sum := sha256.Sum256(w.body.Bytes())
	e := selftest.Evidence{ID: c.TemplateID + "-probe", Tool: c.Tool, Status: "failed", OutputHash: hex.EncodeToString(sum[:]), DurationMS: time.Since(start).Milliseconds()}
	out := selftest.Execution{Verdict: "FAIL", Code: selfTestDiagnosis(w.body.String()), Cleanup: "not_needed"}
	var probeResult struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(w.body.Bytes(), &probeResult) == nil && probeResult.Code == "read_verification_unavailable" {
		out.Verdict, out.Code = "BLOCKED_ACCESS", probeResult.Code
	}
	e.Executed = w.code == http.StatusOK || w.code == http.StatusBadGateway
	if w.code >= 200 && w.code < 300 {
		e.Status = "success"
		out.Verdict, out.Code = "PASS", ""
		out.Assertions = []string{"saved_configuration_read_only_connection_probe"}
	}
	if ctx.Err() != nil {
		e.Status = "cancelled"
		out.Verdict, out.Code = "CANCELLED", "deadline_or_stop"
	}
	if emit != nil {
		emit(e)
	}
	out.Evidence = []selftest.Evidence{e}
	return out
}
