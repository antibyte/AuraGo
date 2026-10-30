package server

import (
	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/decisions/consumers"
)

type consumerStatus struct {
	ID            consumers.ID    `json:"id"`
	Enabled       bool            `json:"enabled"`
	Available     bool            `json:"available"`
	Code          string          `json:"code"`
	ImagesAllowed bool            `json:"images_allowed"`
	Stats         consumers.Stats `json:"stats"`
}

type decisionsStatusResponse struct {
	decisions.Status
	Capabilities consumers.Capabilities `json:"capabilities"`
	Consumers    []consumerStatus       `json:"consumers"`
}

// decisionsFullStatus adds the passive consumer view to the service status. It
// reads snapshots and counters only; nothing is evaluated, installed or started.
func (s *Server) decisionsFullStatus() decisionsStatusResponse {
	out := decisionsStatusResponse{Status: s.decisionsSetupStatus(), Consumers: []consumerStatus{}}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return out
	}
	hub := s.decisionHub()
	settings := cfg.Decisions.Normalize()
	var generationID, modelEpoch uint64
	if router, ok := s.Decisions.(*routedDecisions); ok {
		router.mu.Lock()
		if router.generation != nil {
			generationID, modelEpoch = router.generation.id, router.generation.modelEpoch
		}
		router.mu.Unlock()
	}
	out.Capabilities = hub.capabilities(settings, out.Status, generationID, modelEpoch)
	stats := hub.Stats()
	for _, id := range consumers.Order {
		snap := hub.Snapshot(cfg, id)
		out.Consumers = append(out.Consumers, consumerStatus{
			ID: id, Enabled: policyFor(cfg, id).on(id), Available: snap.Available, Code: snap.Code,
			ImagesAllowed: snap.ImagesAllowed, Stats: stats[id],
		})
	}
	return out
}

// decisionConsumersView lists every switch explicitly, including false image
// permissions, so the settings page starts from a complete saved value.
func decisionConsumersView(c config.DecisionConsumersConfig) map[string]map[string]bool {
	c = c.Normalize()
	plain := func(sw config.DecisionConsumerSwitch) map[string]bool { return map[string]bool{"enabled": sw.Enabled} }
	images := func(sw config.DecisionConsumerSwitch) map[string]bool {
		return map[string]bool{"enabled": sw.Enabled, "allow_external_images": sw.AllowExternalImages}
	}
	return map[string]map[string]bool{
		"memory_rerank": plain(c.MemoryRerank), "appraisal": plain(c.Appraisal), "salience_gate": plain(c.SalienceGate),
		"mail_triage": plain(c.MailTriage), "webhook_questions": plain(c.WebhookQuestions),
		"person_wake": images(c.PersonWake), "indexer_images": images(c.IndexerImages), "game_maker_visual": images(c.GameMakerVisual),
	}
}
