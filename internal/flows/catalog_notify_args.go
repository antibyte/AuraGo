package flows

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Parameter readers, limits and result cleaning of the notification nodes
// (catalog_actions_notify.go). Every reader returns the value the tool gets or a
// FLOW_PARAM_INVALID whose message never echoes the value, and the node validators
// run the same readers on literal parameters, so what Validate rejects Execute
// rejects too.

// Limits of the notification nodes. Each one is checked before the tool is called and
// fails with FLOW_PARAM_INVALID; nothing is cut silently. They follow what the real
// tools and channels do.
const (
	// maxEmailBodyBytes bounds a mail body. The SMTP tool sends the body as one
	// 8bit text part and a mail this size is already large for most mail servers.
	maxEmailBodyBytes = 64 << 10
	// maxEmailSubjectBytes: send_email writes the subject as ONE base64 encoded word
	// ("Subject: =?UTF-8?B?…?="), and a header line may hold 998 octets (RFC 5322),
	// which leaves room for about 730 bytes of subject.
	maxEmailSubjectBytes = 700
	// maxEmailToBytes and maxEmailRecipients bound the recipient list; every
	// address costs the tool one RCPT command. maxEmailAddressBytes is the length of
	// an address (RFC 5321 gives a path 256 octets, the angle brackets included).
	maxEmailToBytes      = 4096
	maxEmailRecipients   = 20
	maxEmailAddressBytes = 254
	// maxAccountBytes bounds an email account id, maxChannelBytes the name of a
	// notification channel.
	maxAccountBytes = 100
	maxChannelBytes = 32
	// maxNotifyTitleRunes: Pushover accepts titles of 250 characters, and the title
	// of a push notification travels in an HTTP header (ntfy).
	maxNotifyTitleRunes = 250
	// telegramTextLimit is what Telegram accepts for the text of one message,
	// counted after formatting is parsed. The tool sends "<title>\n<message>" as one
	// text and has no splitting, so title, newline and message share the limit.
	// Telegram counts UTF-16 code units, and so does the check.
	telegramTextLimit   = 4096
	telegramDefaultName = "AuraGo" // the title send_telegram uses when there is none
	// maxPushMessageBytes is the most any push channel takes (ntfy: 4096 bytes, Web
	// Push: a payload of about 4 KB). Channels with a smaller limit (Pushover 1024
	// characters, Discord 2000) refuse a longer text themselves, which shows up as an
	// error entry in the results.
	maxPushMessageBytes = 4096
	// maxDiscordMessageRunes: Discord takes 2000 characters per message and the tool
	// splits a longer text into messages of 1990, so there is no hard limit. The
	// bound keeps a flow from flooding a channel with more than about four messages.
	maxDiscordMessageRunes = 8000
	// maxDiscordIDDigits: a Discord id (a snowflake) is a number of up to 20 digits.
	maxDiscordIDDigits = 20

	// Bounds of the per-channel results a notification tool reports. There are at
	// most seven channels; the limits keep a hostile answer small.
	maxNotifyResults    = 20
	maxNotifyNameRunes  = 64
	maxNotifyDetailText = 200
	// maxNotifyScanRunes bounds the text the redaction patterns run over.
	maxNotifyScanRunes = 1 << 16
)

// notifyPriorities are the priorities of send_notification; normal is the default.
var notifyPriorities = []string{"low", "normal", "high", "critical"}

var (
	// notifyURLRe finds the request URL Go puts into a failed HTTP call ("Post
	// \"https://…\": dial tcp …"), closed or cut off. The Telegram bot token is part of
	// that URL and so is a ntfy topic, which on a public server is as good as a password.
	notifyURLRe = regexp.MustCompile(`\b(Get|Post|Put|Patch|Delete|Head|Options) "[^"]*("|$)`)
	// notifyBotTokenRe finds a Telegram bot token in the form it has in a URL.
	notifyBotTokenRe = regexp.MustCompile(`bot[0-9]{5,}:[A-Za-z0-9_-]{20,}`)
)

// notifyText makes text a tool or a channel gave us fit for an error message or an
// output: valid UTF-8, a URL that an HTTP error carries and a Telegram bot token
// removed, control characters and line breaks turned into single blanks, at most
// limit runes. It is not quoted: the message is the tool's own words, and the
// control characters are what could forge extra lines.
func notifyText(s string, limit int) string {
	s = truncateRunes(validUTF8(s), maxNotifyScanRunes)
	s = notifyURLRe.ReplaceAllString(s, `$1 "[url]"`)
	s = notifyBotTokenRe.ReplaceAllString(s, "bot[redacted]")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return truncateRunes(strings.Join(strings.Fields(s), " "), limit)
}

// notifyResult is one channel's entry of a send_notification-style answer.
type notifyResult struct{ channel, status, detail string }

func (r notifyResult) sent() bool { return r.status == "sent" }

// notifyResults reads the per-channel results of a tool answer, at most
// maxNotifyResults of them. Every text is cleaned (see notifyText). An entry that is
// not an object has no status and so counts as not sent. isList is false when the
// answer has no results list (absent, null or another type).
func notifyResults(out map[string]any) (results []notifyResult, isList bool) {
	list, isList := out["results"].([]any)
	if !isList {
		return nil, false
	}
	for _, item := range list {
		if len(results) == maxNotifyResults {
			break
		}
		m, _ := item.(map[string]any)
		results = append(results, notifyResult{
			channel: notifyText(outString(m, "channel"), maxNotifyNameRunes),
			status:  strings.ToLower(notifyText(outString(m, "status"), maxNotifyNameRunes)),
			detail:  notifyText(outString(m, "detail"), maxNotifyDetailText),
		})
	}
	return results, true
}

// literalIssueIfSet is literalIssue for a parameter that may be left out: an absent
// value is the required-parameter check's business, a template is judged at run time.
func literalIssueIfSet(n *Node, param string, check func(any) (string, error)) []Issue {
	if isEmptyValue(n.Params[param]) {
		return nil
	}
	return literalIssue(n, param, check)
}

// utf16Len is the length of s in UTF-16 code units, the way Telegram counts.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if l := utf16.RuneLen(r); l > 0 {
			n += l
		} else {
			n++
		}
	}
	return n
}

// telegramUnits is the size of the text send_telegram builds from a title and a
// message: the title (AuraGo when there is none), a newline and the message.
func telegramUnits(title, message string) int {
	if title == "" {
		title = telegramDefaultName
	}
	return utf16Len(title) + 1 + utf16Len(message)
}

func tooLong(what string, n, limit int, unit string) error {
	return NewNodeError("FLOW_PARAM_INVALID", "the %s is %d %s; the limit is %d", what, n, unit, limit)
}

// notifyTitle reads the optional title of a notification: text of one line, trimmed,
// at most maxNotifyTitleRunes characters.
func notifyTitle(v any) (string, error) {
	s, err := scalarTextParam(v, "the title")
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n > maxNotifyTitleRunes {
		return "", tooLong("title", n, maxNotifyTitleRunes, "characters")
	}
	if !validHeaderValue(s) || strings.ContainsRune(s, '\t') {
		return "", NewNodeError("FLOW_PARAM_INVALID", "the title contains characters that are not allowed")
	}
	return s, nil
}

// notifyMessage reads the required message of a notification: text, or a list or
// object as compact JSON (a value that cannot be encoded fails instead of being sent
// as a placeholder), trimmed and valid UTF-8. An empty list or object is no message
// (an empty search result must not go out as "[]"). A node adds its own size limit.
func notifyMessage(v any) (string, error) {
	if isEmptyValue(v) {
		return "", NewNodeError("FLOW_PARAM_INVALID", "enter a message")
	}
	s, err := documentText("the message", v)
	if err != nil {
		return "", err
	}
	if s = strings.TrimSpace(s); s == "" {
		return "", NewNodeError("FLOW_PARAM_INVALID", "enter a message")
	}
	return validUTF8(s), nil
}

// telegramMessage is the message alone; telegramFits adds the title. Validate only
// has the message, so it assumes the shortest title there can be (one character and
// the newline); Execute checks the real total.
func telegramMessage(v any) (string, error) {
	s, err := notifyMessage(v)
	if err != nil {
		return "", err
	}
	if n := utf16Len(s); n+2 > telegramTextLimit {
		return "", tooLong("message", n, telegramTextLimit-2, "characters for Telegram")
	}
	return s, nil
}

func telegramFits(title, message string) error {
	if n := telegramUnits(title, message); n > telegramTextLimit {
		return tooLong("message with its title", n, telegramTextLimit, "characters for Telegram")
	}
	return nil
}

func pushMessage(v any) (string, error) {
	s, err := notifyMessage(v)
	if err != nil {
		return "", err
	}
	if len(s) > maxPushMessageBytes {
		return "", tooLong("message", len(s), maxPushMessageBytes, "bytes")
	}
	return s, nil
}

func discordMessage(v any) (string, error) {
	s, err := notifyMessage(v)
	if err != nil {
		return "", err
	}
	if n := utf8.RuneCountInString(s); n > maxDiscordMessageRunes {
		return "", tooLong("message", n, maxDiscordMessageRunes, "characters")
	}
	return s, nil
}

// attachmentParam reads the optional file of a notification (a path or a file
// object): nothing, blank text and empty values mean no attachment; anything else
// must hold a bounded path, so a number or an object without a path is an error
// instead of a mail that silently goes out without its file.
func attachmentParam(v any) (string, error) {
	if isEmptyValue(v) {
		return "", nil
	}
	return filePathParam(v, "the attachment is not a file")
}

// emailRecipients reads the recipients of a mail: plain addresses (name@host, no
// display name, no angle brackets) separated by commas, at most maxEmailRecipients.
// send_email writes the text into the To header as it is and splits it at the commas
// for the RCPT commands, so a line break would add headers (a Bcc), and an address
// with a display name or in angle brackets would break the RCPT command. The result
// is the addresses joined with ", ".
func emailRecipients(v any) (string, error) {
	s, err := scalarTextParam(v, "the recipient")
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", NewNodeError("FLOW_PARAM_INVALID", "enter a recipient")
	case len(s) > maxEmailToBytes:
		return "", tooLong("recipient list", len(s), maxEmailToBytes, "bytes")
	case !validHeaderValue(s):
		return "", NewNodeError("FLOW_PARAM_INVALID", "the recipient contains characters that are not allowed")
	}
	parts := strings.Split(s, ",")
	if len(parts) > maxEmailRecipients {
		return "", NewNodeError("FLOW_PARAM_INVALID", "there are %d recipients; the limit is %d", len(parts), maxEmailRecipients)
	}
	for i, part := range parts {
		part = strings.TrimSpace(part)
		parts[i] = part
		if part == "" {
			return "", NewNodeError("FLOW_PARAM_INVALID", "recipient %d is empty", i+1)
		}
		if a, err := mail.ParseAddress(part); err != nil || a.Name != "" || a.Address != part || len(part) > maxEmailAddressBytes {
			return "", NewNodeError("FLOW_PARAM_INVALID", "recipient %d is not a plain email address (write name@example.com, separated by commas)", i+1)
		}
	}
	return strings.Join(parts, ", "), nil
}

// emailSubject reads the optional subject: one line of text, trimmed, at most
// maxEmailSubjectBytes. A line break inside it is rejected (surrounding whitespace is
// trimmed away, a trailing newline of a template result is harmless).
func emailSubject(v any) (string, error) {
	s, err := scalarTextParam(v, "the subject")
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	switch {
	case len(s) > maxEmailSubjectBytes:
		return "", tooLong("subject", len(s), maxEmailSubjectBytes, "bytes")
	case !validHeaderValue(s):
		return "", NewNodeError("FLOW_PARAM_INVALID", "the subject contains characters that are not allowed")
	}
	return s, nil
}

// emailAccount reads the optional account id. The id is looked up by the tool, which
// also puts it into its own "account not found" answer as JSON text without
// escaping, so a quote or a backslash would let the id write fields of that answer
// (a "status":"success" among them). Control characters, quotes and backslashes are
// rejected; any other text is a possible id (the settings only require it not to be
// empty).
func emailAccount(v any) (string, error) {
	s, err := scalarTextParam(v, "the account")
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	switch {
	case len(s) > maxAccountBytes:
		return "", tooLong("account", len(s), maxAccountBytes, "bytes")
	case !validHeaderValue(s) || strings.ContainsAny(s, "\t\"\\"):
		return "", NewNodeError("FLOW_PARAM_INVALID", "the account contains characters that are not allowed")
	}
	return s, nil
}

// emailBody reads the body: text, or a list or object as compact JSON, at most
// maxEmailBodyBytes. It is not trimmed. An empty body is allowed.
func emailBody(v any) (string, error) {
	s, err := documentText("the body", v)
	if err != nil {
		return "", err
	}
	if len(s) > maxEmailBodyBytes {
		return "", tooLong("body", len(s), maxEmailBodyBytes, "bytes")
	}
	return validUTF8(s), nil
}

// pushChannel reads the channel of send_notification: absent or blank is "all", else
// a lower case name of letters, digits, "_" and "-". The names come from the
// configuration (the editor lists the ones that are set up), so the list is not
// fixed here; the tool reports a name it does not know in the results.
func pushChannel(v any) (string, error) {
	var s string
	switch x := v.(type) {
	case nil:
	case string:
		s = strings.ToLower(strings.TrimSpace(x))
	default:
		return "", NewNodeError("FLOW_PARAM_INVALID", "the channel must be text")
	}
	if s == "" {
		return "all", nil
	}
	if len(s) > maxChannelBytes {
		return "", tooLong("channel name", len(s), maxChannelBytes, "bytes")
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return "", NewNodeError("FLOW_PARAM_INVALID", "the channel name contains characters that are not allowed")
		}
	}
	return s, nil
}

// discordChannelID reads the optional Discord channel id: text of 1 to 20 digits.
// A number is rejected, because flow numbers are floating point and lose the lower
// digits of an id, which would send the message to another channel. The tool puts
// the id into its answer without escaping, so only digits are let through.
func discordChannelID(v any) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return "", nil
		}
		if len(s) > maxDiscordIDDigits {
			return "", tooLong("channel id", len(s), maxDiscordIDDigits, "characters")
		}
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				return "", NewNodeError("FLOW_PARAM_INVALID", "the channel id must be the number of a Discord channel")
			}
		}
		return s, nil
	}
	return "", NewNodeError("FLOW_PARAM_INVALID", "the channel id must be given as text, a number loses digits")
}
