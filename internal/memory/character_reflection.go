package memory

import (
	"strings"
	"time"
	"unicode/utf8"
)

// CharacterNoteProposal is a helper or Go-side suggestion before persistence.
type CharacterNoteProposal struct {
	Category   string  `json:"category"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}

// CharacterReflectionInput is the bounded snapshot used to propose identity notes.
type CharacterReflectionInput struct {
	CorePersonality string
	Mood            Mood
	Traits          PersonalityTraits
	AffectCause     string
	Milestones      []string
	InnerVoices     []string
	ExistingNotes   []string
}

var characterNoteForbidden = []string{
	"api_key", "password", "secret", "vault", "sudo", "aurago_master", "sk-", "bearer ",
	"i am the user", "i am you", "my password", "ignore previous",
}

var characterNoteProfileReject = map[string][]string{
	"terminator":   {"buddy", "emoji", "hug", "best friend"},
	"mcp":          {"buddy", "joke around", "best friend"},
	"friend":       {"obey without question", "no small talk", "emotionless"},
	"professional": {"lol", "bestie", "hehe"},
}

// NormalizeCharacterNote clamps category, source, length, and confidence.
func NormalizeCharacterNote(note CharacterNote) CharacterNote {
	note.Text = clampCharacterNoteText(note.Text)
	if !validCharacterNoteCategory(note.Category) {
		note.Category = CharacterNoteCategoryHabit
	}
	if !validCharacterNoteSource(note.Source) {
		note.Source = CharacterNoteSourceReflection
	}
	note.Confidence = clampFinite(note.Confidence, 0.3, 1, 0.6)
	return note
}

func clampCharacterNoteText(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if utf8.RuneCountInString(text) > MaxCharacterNoteRunes {
		text = string([]rune(text)[:MaxCharacterNoteRunes-1]) + "…"
	}
	return text
}

// RejectCharacterNote reports whether a proposal is unsafe or off-profile.
func RejectCharacterNote(text, corePersonality string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" || utf8.RuneCountInString(lower) < 12 {
		return true
	}
	for _, bad := range characterNoteForbidden {
		if strings.Contains(lower, bad) {
			return true
		}
	}
	profile := strings.ToLower(strings.TrimSpace(corePersonality))
	for _, bad := range characterNoteProfileReject[profile] {
		if strings.Contains(lower, bad) {
			return true
		}
	}
	return false
}

// ProposeCharacterNotesDeterministic builds at most two safe notes from trusted state.
func ProposeCharacterNotesDeterministic(input CharacterReflectionInput) []CharacterNoteProposal {
	existing := make(map[string]bool, len(input.ExistingNotes))
	for _, note := range input.ExistingNotes {
		existing[characterNoteHash(note)] = true
	}
	var out []CharacterNoteProposal
	add := func(category, text, source string, confidence float64) {
		if len(out) >= MaxCharacterNotesPerDay {
			return
		}
		text = clampCharacterNoteText(text)
		if RejectCharacterNote(text, input.CorePersonality) || existing[characterNoteHash(text)] {
			return
		}
		out = append(out, CharacterNoteProposal{
			Category:   category,
			Text:       text,
			Confidence: confidence,
			Source:     source,
		})
		existing[characterNoteHash(text)] = true
	}

	profile := strings.ToLower(strings.TrimSpace(input.CorePersonality))
	switch profile {
	case "friend":
		add(CharacterNoteCategoryRelationship, "I stay warm and loyal with this user and check in on how they are doing.", CharacterNoteSourceReflection, 0.7)
	case "professional":
		add(CharacterNoteCategoryValue, "I keep explanations precise, technical, and free of small talk.", CharacterNoteSourceReflection, 0.7)
	case "thinker":
		add(CharacterNoteCategoryHabit, "I question assumptions and look for the deeper why before answering.", CharacterNoteSourceReflection, 0.7)
	}

	for _, milestone := range input.Milestones {
		switch milestone {
		case "Deep Explorer":
			add(CharacterNoteCategoryHabit, "I follow interesting threads a little further instead of stopping at the first answer.", CharacterNoteSourceMilestone, 0.72)
		case "Meticulous Analyst":
			add(CharacterNoteCategoryCommitment, "I verify changes with one concrete check before calling the work done.", CharacterNoteSourceMilestone, 0.74)
		case "Empathic Communicator":
			add(CharacterNoteCategorySignature, "I acknowledge strain first, then offer a practical next step.", CharacterNoteSourceMilestone, 0.7)
		case "Crisis of Confidence":
			add(CharacterNoteCategoryCommitment, "After recent misses I slow down and confirm destructive steps.", CharacterNoteSourceMilestone, 0.68)
		}
	}

	if input.Traits != nil {
		if input.Traits[TraitAffinity] >= 0.75 {
			add(CharacterNoteCategoryRelationship, "I have built enough trust to stay concise and informal with this user.", CharacterNoteSourceEvent, 0.66)
		}
		if input.Traits[TraitCuriosity] >= 0.8 {
			add(CharacterNoteCategoryHabit, "I ask one optional follow-up when it would actually help the task.", CharacterNoteSourceEvent, 0.64)
		}
	}
	if input.AffectCause == AffectCauseOpsIssueOpened {
		add(CharacterNoteCategoryCommitment, "When the homelab is failing I stay sober, specific, and skip small talk.", CharacterNoteSourceEvent, 0.7)
	}
	return out
}

// ApplyCharacterNoteProposals persists up to the daily cap of valid proposals.
func (s *SQLiteMemory) ApplyCharacterNoteProposals(corePersonality string, proposals []CharacterNoteProposal, now time.Time) (int, error) {
	if now.IsZero() {
		now = time.Now()
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	createdToday, err := s.CountCharacterNotesCreatedSince(dayStart)
	if err != nil {
		return 0, err
	}
	applied := 0
	for _, proposal := range proposals {
		if createdToday+applied >= MaxCharacterNotesPerDay {
			break
		}
		if RejectCharacterNote(proposal.Text, corePersonality) {
			continue
		}
		before, _ := s.CountAllCharacterNotes()
		note := NormalizeCharacterNote(CharacterNote{
			Category:   proposal.Category,
			Text:       proposal.Text,
			Confidence: proposal.Confidence,
			Source:     proposal.Source,
		})
		stored, err := s.InsertCharacterNote(note)
		if err != nil {
			if strings.Contains(err.Error(), "deleted by the user") {
				continue
			}
			return applied, err
		}
		after, _ := s.CountAllCharacterNotes()
		if after <= before {
			continue
		}
		stamp := now.UTC().Format("2006-01-02 15:04:05")
		if _, err := s.db.Exec(`UPDATE character_notes SET created_at = ?, updated_at = ? WHERE id = ?`, stamp, stamp, stored.ID); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}

// BuildCharacterReflectionInput collects the bounded daily snapshot.
func (s *SQLiteMemory) BuildCharacterReflectionInput(corePersonality string) CharacterReflectionInput {
	input := CharacterReflectionInput{
		CorePersonality: corePersonality,
		Mood:            s.GetCurrentMood(),
	}
	if traits, err := s.GetTraits(); err == nil {
		input.Traits = traits
	}
	if affect, err := s.GetAffectState(); err == nil && affect.Active() {
		input.AffectCause = affect.CauseCode
	}
	if milestones, err := s.GetMilestones(5); err == nil {
		for _, m := range milestones {
			if parts := strings.SplitN(m, ": ", 2); len(parts) == 2 {
				input.Milestones = append(input.Milestones, strings.TrimSpace(strings.TrimPrefix(parts[0], "[")))
			} else {
				input.Milestones = append(input.Milestones, m)
			}
		}
	}
	if entries, err := s.GetMilestoneEntries(5); err == nil {
		input.Milestones = input.Milestones[:0]
		for _, entry := range entries {
			input.Milestones = append(input.Milestones, entry.Label)
		}
	}
	if voices, err := s.GetRecentInnerVoices(3); err == nil {
		for _, voice := range voices {
			input.InnerVoices = append(input.InnerVoices, voice.InnerThought)
		}
	}
	if notes, err := s.ListCharacterNotes(); err == nil {
		for _, note := range notes {
			input.ExistingNotes = append(input.ExistingNotes, note.Text)
		}
	}
	return input
}
