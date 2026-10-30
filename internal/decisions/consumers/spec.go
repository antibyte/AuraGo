// Package consumers is the shared contract between AuraGo subsystems and the
// optional Decisions service. It never starts a runtime and imports no server.
package consumers

import "time"

type ID string

const (
	Router          ID = "router"
	MemoryRerank    ID = "memory_rerank"
	Appraisal       ID = "appraisal"
	SalienceGate    ID = "salience_gate"
	MailTriage      ID = "mail_triage"
	WebhookQuestion ID = "webhook_question"
	PersonWake      ID = "person_wake"
	IndexerImages   ID = "indexer_images"
	GameMakerVisual ID = "game_maker_visual"
)

type Class uint8

const (
	Interactive Class = iota // chat turn; preempts local background work
	Background               // gives up at once when anything else runs
)

type Modality uint8

const (
	TextOnly      Modality = iota
	ImageRequired          // stays off when images cannot be sent
)

type Spec struct {
	ID             ID
	Class          Class
	Modality       Modality
	BudgetCategory string
	Timeout        time.Duration
	MinInterval    time.Duration
}

var Registry = map[ID]Spec{
	Router:          {ID: Router, Class: Interactive, BudgetCategory: "routing", Timeout: 1500 * time.Millisecond},
	MemoryRerank:    {ID: MemoryRerank, Class: Interactive, BudgetCategory: "decisions_memory_rerank", Timeout: 1200 * time.Millisecond},
	Appraisal:       {ID: Appraisal, Class: Background, BudgetCategory: "decisions_appraisal", Timeout: 10 * time.Second},
	SalienceGate:    {ID: SalienceGate, Class: Background, BudgetCategory: "decisions_salience_gate", Timeout: 10 * time.Second},
	MailTriage:      {ID: MailTriage, Class: Background, BudgetCategory: "decisions_mail_triage", Timeout: 20 * time.Second},
	WebhookQuestion: {ID: WebhookQuestion, Class: Background, BudgetCategory: "decisions_webhook_question", Timeout: 20 * time.Second},
	PersonWake:      {ID: PersonWake, Class: Background, Modality: ImageRequired, BudgetCategory: "decisions_person_wake", Timeout: 20 * time.Second, MinInterval: 2 * time.Second},
	IndexerImages:   {ID: IndexerImages, Class: Background, Modality: ImageRequired, BudgetCategory: "decisions_indexer_images", Timeout: 30 * time.Second},
	GameMakerVisual: {ID: GameMakerVisual, Class: Background, Modality: ImageRequired, BudgetCategory: "decisions_game_maker_visual", Timeout: 30 * time.Second},
}
