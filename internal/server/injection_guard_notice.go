package server

import (
	"fmt"

	"aurago/internal/injectionguard"
	"aurago/internal/memory"
)

const horizonOptionalNoticeSourceID = "security:horizon_optional"

// queueOptionalInjectionGuardNotice uses the existing durable notification receipt.
// Reading status is passive: this hint never prepares or starts the classifier.
func queueOptionalInjectionGuardNotice(s *Server) {
	if s == nil || s.ShortTermMem == nil || s.InjectionGuard == nil {
		return
	}
	status := s.InjectionGuard.Status()
	if !optionalInjectionGuardNoticeAvailable(status) {
		return
	}
	_, _, err := s.ShortTermMem.AddSystemNotification(memory.SystemNotification{
		Type:     "security_advisory",
		SourceID: horizonOptionalNoticeSourceID,
		Title:    "Optional local prompt-injection protection",
		Message:  fmt.Sprintf("The optional Horizon Labs classifier provides additional checks for web and email content. It is off by default. It needs about %.0f MB of model downloads plus its CPU runtime, additional CPU capacity and roughly 1–2 GiB of available RAM. You decide whether your system has enough resources. Download and activation are separate actions in Config → PromptSec Guardian → Local protection for web and email.", float64(status.SizeBytes)/1e6),
		Data: map[string]interface{}{
			"code":           "horizon_optional",
			"download_bytes": status.SizeBytes,
			"runtime_bytes":  status.RuntimeSizeBytes,
		},
	})
	if err != nil && s.Logger != nil {
		s.Logger.Warn("Could not store optional local protection notice", "error", err)
	}
}

func optionalInjectionGuardNoticeAvailable(status injectionguard.Status) bool {
	return status.Available && !status.Enabled
}
