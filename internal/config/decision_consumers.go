package config

// DecisionConsumerSwitch is one subsystem's opt-in. Kept comparable so that a
// consumer's identity covers only its own entry.
type DecisionConsumerSwitch struct {
	Enabled             bool `yaml:"enabled" json:"enabled"`
	AllowExternalImages bool `yaml:"allow_external_images,omitempty" json:"allow_external_images,omitempty"`
}

// DecisionConsumersConfig lives outside DecisionsConfig: toggling a consumer
// must never rotate the Decisions provider generation.
type DecisionConsumersConfig struct {
	MemoryRerank     DecisionConsumerSwitch `yaml:"memory_rerank" json:"memory_rerank"`
	Appraisal        DecisionConsumerSwitch `yaml:"appraisal" json:"appraisal"`
	SalienceGate     DecisionConsumerSwitch `yaml:"salience_gate" json:"salience_gate"`
	MailTriage       DecisionConsumerSwitch `yaml:"mail_triage" json:"mail_triage"`
	WebhookQuestions DecisionConsumerSwitch `yaml:"webhook_questions" json:"webhook_questions"`
	PersonWake       DecisionConsumerSwitch `yaml:"person_wake" json:"person_wake"`
	IndexerImages    DecisionConsumerSwitch `yaml:"indexer_images" json:"indexer_images"`
	GameMakerVisual  DecisionConsumerSwitch `yaml:"game_maker_visual" json:"game_maker_visual"`
}

// Switch returns the entry for a consumer ID; unknown IDs are off.
func (c DecisionConsumersConfig) Switch(id string) DecisionConsumerSwitch {
	switch id {
	case "memory_rerank":
		return c.MemoryRerank
	case "appraisal":
		return c.Appraisal
	case "salience_gate":
		return c.SalienceGate
	case "mail_triage":
		return c.MailTriage
	case "webhook_question":
		return c.WebhookQuestions
	case "person_wake":
		return c.PersonWake
	case "indexer_images":
		return c.IndexerImages
	case "game_maker_visual":
		return c.GameMakerVisual
	}
	return DecisionConsumerSwitch{}
}

// Normalize clears image permissions on consumers that never send images.
func (c DecisionConsumersConfig) Normalize() DecisionConsumersConfig {
	c.MemoryRerank.AllowExternalImages = false
	c.Appraisal.AllowExternalImages = false
	c.SalienceGate.AllowExternalImages = false
	c.MailTriage.AllowExternalImages = false
	c.WebhookQuestions.AllowExternalImages = false
	return c
}
