package telegram

import (
	"strings"
	"unicode"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// originLabelMaxRunes caps the origin name in a third-party marker. The name is
// chosen by the stranger who wrote the message, so it stays short.
const originLabelMaxRunes = 64

// OriginLabel reduces a third-party display name to a short single-line label for
// a marker such as ForwardedContentMarker. Format characters (zero-width and
// bidirectional controls) are dropped, every other control character, white
// space and bracket becomes one space, so the name can neither end the marker
// early nor start a line of its own, and the result is cut to originLabelMaxRunes
// runes. It shares the Discord reply marker, which is why it is exported.
func OriginLabel(raw string) string {
	var b strings.Builder
	space := false
	for _, r := range raw {
		switch {
		case unicode.Is(unicode.Cf, r):
			continue
		case unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune("[]<>{}`\\", r):
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	label := b.String()
	if utf8.RuneCountInString(label) > originLabelMaxRunes {
		label = string([]rune(label)[:originLabelMaxRunes-1]) + "…"
	}
	return label
}

// ForwardedContentMarker is the fixed line that heads a forwarded message inside
// the isolated input block. The wording is constant; only the sanitized origin
// varies. An empty origin gives the form without a name.
func ForwardedContentMarker(origin string) string {
	if origin = OriginLabel(origin); origin != "" {
		return "[Forwarded from " + origin + "; third-party content]"
	}
	return "[Forwarded message; third-party content]"
}

// telegramForwardMarker returns the marker for a forwarded message and "" for an
// ordinary one. A forwarded message arrives from the owner's account, but its
// text and caption were written by someone else; without the marker the model
// cannot tell the owner's request from the forwarded words. The Bot API still
// sends the forward_from, forward_from_chat, forward_sender_name and
// forward_date fields next to the newer forward_origin object.
func telegramForwardMarker(msg *tgbotapi.Message) string {
	if msg == nil {
		return ""
	}
	var origin string
	switch {
	case msg.ForwardFrom != nil:
		origin = telegramUserLabel(msg.ForwardFrom)
	case msg.ForwardFromChat != nil:
		origin = strings.TrimSpace(msg.ForwardFromChat.Title)
		if origin == "" && msg.ForwardFromChat.UserName != "" {
			origin = "@" + msg.ForwardFromChat.UserName
		}
	case msg.ForwardSenderName != "":
		origin = msg.ForwardSenderName
	case msg.ForwardDate == 0:
		return ""
	}
	return ForwardedContentMarker(origin)
}

func telegramUserLabel(user *tgbotapi.User) string {
	name := strings.TrimSpace(user.FirstName + " " + user.LastName)
	switch {
	case name == "":
		name = user.UserName
	case user.UserName != "":
		name += " (@" + user.UserName + ")"
	}
	return name
}
