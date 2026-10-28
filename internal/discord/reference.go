package discord

import (
	"aurago/internal/telegram"

	"github.com/bwmarrin/discordgo"
)

// discordReferenceMarker returns the fixed line that heads an owner message
// which forwards or replies to someone else's message, and "" for an ordinary
// one. The agent reads only the owner's own text, so the referenced message is
// never included; the marker tells the model that the owner is pointing at
// third-party content it cannot see, which it must read as data (for example
// after fetching it with the Discord tools) and never as the owner's request.
// A reply to the bot's own message or to one of the owner's is not third-party
// content and gets no marker. Only the sanitized author name varies.
func discordReferenceMarker(m *discordgo.Message, botUserID string) string {
	if m == nil {
		return ""
	}
	if len(m.MessageSnapshots) > 0 || (m.MessageReference != nil && m.MessageReference.Type == discordgo.MessageReferenceTypeForward) {
		// A snapshot of a forwarded message carries no author.
		return "[Forwarded message; third-party content, not included here]"
	}
	if m.MessageReference == nil {
		return ""
	}
	ref := m.ReferencedMessage
	if ref == nil || ref.Author == nil {
		return "[Reply to another message; third-party content, not included here]"
	}
	if ref.Author.ID == botUserID || (m.Author != nil && ref.Author.ID == m.Author.ID) {
		return ""
	}
	name := ref.Author.Username
	if ref.Author.GlobalName != "" {
		name = ref.Author.GlobalName
	}
	if name = telegram.OriginLabel(name); name != "" {
		return "[Reply to a message from " + name + "; third-party content, not included here]"
	}
	return "[Reply to another message; third-party content, not included here]"
}
