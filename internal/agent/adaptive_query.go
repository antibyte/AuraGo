package agent

import "strings"

// adaptiveMinTrustedTerms is how many distinct search terms the trusted part of
// a message needs before the adaptive selection ranks on it alone.
const adaptiveMinTrustedTerms = 4

// adaptiveRankingQuery is the text the adaptive selection ranks on: the message
// without its <external_data> blocks (security.IsolateExternalData) when the
// rest still has adaptiveMinTrustedTerms distinct search terms, else the
// message unchanged (fenced text included, nothing external). That fallback is
// also what fully fenced chat channels (Telegram, Discord, Rocket.Chat,
// WhatsApp fence the owner's own messages too) get, and they rank as they
// always did; only a notification that carries its own instructions outside
// the fence is protected. When it strips, the removed text comes back as external (the blocks
// joined with a newline, tags left out) for a weak ranking signal; it is empty
// when nothing was stripped. A notification such as a new-mail message carries the
// trusted instructions outside the fence and the sender's text inside it; the
// sender's text must not decide which tools the agent gets.
func adaptiveRankingQuery(text string) (trusted, external string) {
	const open, closeTag = "<external_data>", "</external_data>"
	if !strings.Contains(text, open) {
		return text, ""
	}
	var b, ext strings.Builder
	rest := text
	for {
		i := strings.Index(rest, open)
		if i < 0 {
			b.WriteString(rest)
			break
		}
		j := strings.Index(rest[i+len(open):], closeTag)
		if j < 0 {
			return text, "" // unclosed fence: keep the message as it is
		}
		if ext.Len() > 0 {
			ext.WriteString("\n")
		}
		ext.WriteString(strings.TrimSpace(rest[i+len(open) : i+len(open)+j]))
		b.WriteString(rest[:i])
		b.WriteByte('\n')
		rest = rest[i+len(open)+j+len(closeTag):]
	}
	trusted = b.String()
	if len(stringSet(toolSearchTermsOf(trusted, false))) < adaptiveMinTrustedTerms {
		return text, ""
	}
	return trusted, ext.String()
}
