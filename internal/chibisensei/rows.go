package chibisensei

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	rowKindChat = "chat"
	rowKindText = "text"
	maxRowTags  = 16
	maxTagRunes = 40
)

func issue(field, key, msg string) Issue {
	return Issue{Field: field, MessageKey: "chibi_sensei." + key, Message: msg}
}

func validSplit(s string) bool {
	switch s {
	case "", "auto", "train", "eval", "exclude":
		return true
	}
	return false
}

// validTag reports whether a tag follows the tag rules (the value is trimmed by the caller).
func validTag(t string) bool {
	n := utf8.RuneCountInString(t)
	if n < 1 || n > maxTagRunes || strings.ContainsRune(t, ',') {
		return false
	}
	for _, r := range t {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// ValidateRowInput applies the hard limits; violations refuse the write (422).
func ValidateRowInput(in RowInput) []Issue {
	var out []Issue
	hasMsgs, hasText := len(in.Messages) > 0, in.Text != nil
	switch {
	case hasMsgs == hasText:
		out = append(out, issue("messages", "row.shape", "A row needs either messages or text."))
	case hasText:
		if strings.TrimSpace(*in.Text) == "" {
			out = append(out, issue("text", "row.empty_text", "The text is empty."))
		} else if utf8.RuneCountInString(*in.Text) > MaxMessageChars || strings.ContainsRune(*in.Text, 0) {
			out = append(out, issue("text", "row.too_long", "The text is too long."))
		}
	default:
		if len(in.Messages) > MaxMessages {
			out = append(out, issue("messages", "row.too_many_messages", "A row has too many messages."))
			break
		}
		for i, m := range in.Messages {
			f := fmt.Sprintf("messages[%d]", i)
			if m.Role != "system" && m.Role != "user" && m.Role != "assistant" {
				out = append(out, issue(f+".role", "row.bad_role", "Unknown message role."))
			}
			if utf8.RuneCountInString(m.Content) > MaxMessageChars || strings.ContainsRune(m.Content, 0) {
				out = append(out, issue(f+".content", "row.too_long", "The message is too long."))
			}
		}
	}
	if !validSplit(in.Split) {
		out = append(out, issue("split", "row.bad_split", "Unknown split."))
	}
	if len(in.Tags) > maxRowTags {
		out = append(out, issue("tags", "row.bad_tag", "Too many tags."))
	} else {
		for i, t := range in.Tags {
			if !validTag(strings.TrimSpace(t)) {
				out = append(out, issue(fmt.Sprintf("tags[%d]", i), "row.bad_tag", "Invalid tag."))
			}
		}
	}
	return out
}

// StructureProblems returns the reasons a stored row is structurally invalid (FlagStructure); empty means valid.
func StructureProblems(kind string, msgs []Message, recipe string) []string {
	if kind == rowKindText {
		if recipe != RecipeFree {
			return []string{"text_outside_free"}
		}
		return nil
	}
	var out []string
	start := 0
	if len(msgs) > 0 && msgs[0].Role == "system" {
		start = 1
	}
	orderOK := true
	for i, m := range msgs[start:] {
		want := "user"
		if i%2 == 1 {
			want = "assistant"
		}
		if m.Role != want {
			orderOK = false
			break
		}
	}
	if !orderOK {
		out = append(out, "role_order")
	} else if len(msgs) == start || msgs[len(msgs)-1].Role != "assistant" {
		out = append(out, "missing_assistant")
	}
	for _, m := range msgs {
		if m.Role == "assistant" && strings.TrimSpace(m.Content) == "" {
			out = append(out, "empty_assistant")
			break
		}
	}
	return out
}

func normalizeForHash(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(norm.NFC.String(s))), " ")
}

func hashPairs(pairs [][2]string) string {
	b, _ := json.Marshal(pairs)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ContentHash is the hex sha256 of the normalized (role, content) pairs; a text row hashes one "text" pair.
func ContentHash(kind string, msgs []Message, text string) string {
	if kind == rowKindText {
		return hashPairs([][2]string{{"text", normalizeForHash(text)}})
	}
	pairs := make([][2]string, 0, len(msgs))
	for _, m := range msgs {
		pairs = append(pairs, [2]string{m.Role, normalizeForHash(m.Content)})
	}
	return hashPairs(pairs)
}

// PromptHash hashes every message before the last assistant message; "" for text rows and rows without an assistant.
func PromptHash(kind string, msgs []Message) string {
	if kind == rowKindText {
		return ""
	}
	last := -1
	for i, m := range msgs {
		if m.Role == "assistant" {
			last = i
		}
	}
	if last < 0 {
		return ""
	}
	pairs := make([][2]string, 0, last)
	for _, m := range msgs[:last] {
		pairs = append(pairs, [2]string{m.Role, normalizeForHash(m.Content)})
	}
	return hashPairs(pairs)
}

// LastAssistant returns the content of the last assistant message ("" when none).
func LastAssistant(msgs []Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			return msgs[i].Content
		}
	}
	return ""
}

// DeriveLabel is the trimmed answer cut to 64 runes for classify projects, otherwise "".
func DeriveLabel(taskKind string, msgs []Message) string {
	if taskKind != "classify" {
		return ""
	}
	r := []rune(strings.TrimSpace(LastAssistant(msgs)))
	if len(r) > 64 {
		r = r[:64]
	}
	return string(r)
}

// LocalFlags computes the flags that depend on the row alone.
func LocalFlags(kind string, msgs []Message, recipe, taskKind string) Flag {
	var f Flag
	if len(StructureProblems(kind, msgs, recipe)) > 0 {
		f |= FlagStructure
	}
	if kind == rowKindChat {
		last := strings.TrimSpace(LastAssistant(msgs))
		n := utf8.RuneCountInString(last)
		if (recipe == RecipeSpecialist && n < 2) || (recipe == RecipeAssistant && n < 10) {
			f |= FlagShortAnswer
		}
		if taskKind == "to_json" && !json.Valid([]byte(last)) {
			f |= FlagInvalidJSON
		}
	}
	return f
}

// encodeJSON marshals without HTML escaping and without a trailing newline.
func encodeJSON(v any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "null"
	}
	return strings.TrimSuffix(buf.String(), "\n")
}
