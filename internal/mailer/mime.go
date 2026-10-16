package mailer

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	message "github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	mailmime "github.com/emersion/go-message/mail"
)

const maxOutgoingAttachmentBytes = 25 << 20
const maxOutgoingMessageBytes = 32 << 20

// BuildMIME serializes a draft without accepting arbitrary headers. The only
// threading headers are validated Message-ID values carried by Draft.
func BuildMIME(from string, draft Draft, now time.Time) ([]byte, error) {
	return buildMIME(from, draft, now, false)
}

// buildGmailMIME includes Bcc so Gmail can address those recipients from the
// raw message. Gmail removes the Bcc header before delivering the message.
func buildGmailMIME(from string, draft Draft, now time.Time) ([]byte, error) {
	return buildMIME(from, draft, now, true)
}

func buildMIME(from string, draft Draft, now time.Time, includeBCC bool) ([]byte, error) {
	fromAddr, err := parseMailbox(from)
	if err != nil {
		return nil, fmt.Errorf("invalid from address")
	}
	to, err := parseMailboxes(draft.To)
	if err != nil {
		return nil, fmt.Errorf("invalid to address")
	}
	cc, err := parseMailboxes(draft.CC)
	if err != nil {
		return nil, fmt.Errorf("invalid cc address")
	}
	for _, raw := range draft.BCC {
		if _, err := parseMailbox(raw); err != nil {
			return nil, fmt.Errorf("invalid bcc address")
		}
	}
	if len(to)+len(cc)+len(draft.BCC) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}

	var header mailmime.Header
	header.SetAddressList("From", []*mailmime.Address{fromAddr})
	header.SetAddressList("To", to)
	header.SetAddressList("Cc", cc)
	if includeBCC {
		bcc, err := parseMailboxes(draft.BCC)
		if err != nil {
			return nil, fmt.Errorf("invalid bcc address")
		}
		header.SetAddressList("Bcc", bcc)
	}
	header.SetSubject(cleanHeaderText(draft.Subject))
	messageID, err := generatedMessageID(fromAddr.Address, draft.ID)
	if err != nil {
		return nil, err
	}
	header.Set("Message-ID", messageID)
	if now.IsZero() {
		now = time.Now()
	}
	header.SetDate(now)
	if draft.InReplyTo != "" {
		value, err := safeMessageID(draft.InReplyTo)
		if err != nil {
			return nil, fmt.Errorf("invalid in-reply-to")
		}
		header.Set("In-Reply-To", value)
	}
	if len(draft.References) > 0 {
		if len(draft.References) > 100 {
			return nil, fmt.Errorf("too many reference identifiers")
		}
		refs := make([]string, len(draft.References))
		for i, raw := range draft.References {
			refs[i], err = safeMessageID(raw)
			if err != nil {
				return nil, fmt.Errorf("invalid reference identifier")
			}
		}
		header.Set("References", strings.Join(refs, " "))
	}
	var out bytes.Buffer
	if draft.HTML == "" && len(draft.Attachments) == 0 {
		part, err := mailmime.CreateSingleInlineWriter(&out, header)
		if err != nil {
			return nil, fmt.Errorf("create MIME body: %w", err)
		}
		if _, err = io.WriteString(part, draft.Text); err != nil {
			return nil, err
		}
		if err = part.Close(); err != nil {
			return nil, err
		}
		if out.Len() > maxOutgoingMessageBytes {
			return nil, fmt.Errorf("message exceeds size limit")
		}
		return out.Bytes(), nil
	}
	w, err := mailmime.CreateWriter(&out, header)
	if err != nil {
		return nil, fmt.Errorf("create MIME message: %w", err)
	}
	inline, err := w.CreateInline()
	if err != nil {
		return nil, fmt.Errorf("create alternative body: %w", err)
	}
	var textHeader message.Header
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	textPart, err := inline.CreatePart(mailmime.InlineHeader{Header: textHeader})
	if err != nil {
		return nil, fmt.Errorf("create text part: %w", err)
	}
	if _, err = io.WriteString(textPart, draft.Text); err != nil {
		return nil, err
	}
	if err = textPart.Close(); err != nil {
		return nil, err
	}
	if draft.HTML != "" {
		safeHTML, sanitizeErr := SanitizeHTML(draft.HTML)
		if sanitizeErr != nil {
			return nil, fmt.Errorf("sanitize HTML body: %w", sanitizeErr)
		}
		var h message.Header
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Content-Transfer-Encoding", "quoted-printable")
		p, e := inline.CreatePart(mailmime.InlineHeader{Header: h})
		if e != nil {
			return nil, e
		}
		if _, e = io.WriteString(p, safeHTML); e != nil {
			return nil, e
		}
		if e = p.Close(); e != nil {
			return nil, e
		}
	}
	if err = inline.Close(); err != nil {
		return nil, err
	}
	total := int64(0)
	for _, a := range draft.Attachments {
		if len(a.Data) == 0 && a.Size != 0 {
			return nil, fmt.Errorf("attachment data is missing")
		}
		total += int64(len(a.Data))
		if total > maxOutgoingAttachmentBytes {
			return nil, fmt.Errorf("attachments exceed size limit")
		}
		name := safeFilename(a.Filename)
		var h message.Header
		ct := strings.TrimSpace(a.ContentType)
		if ct == "" || strings.ContainsAny(ct, "\r\n") {
			ct = "application/octet-stream"
		}
		h.Set("Content-Type", ct)
		disp := "attachment"
		if a.Inline {
			disp = "inline"
		}
		h.Set("Content-Disposition", disp+`; filename="`+strings.ReplaceAll(strings.ReplaceAll(name, `\`, `\\`), `"`, `\"`)+`"`)
		if a.ContentID != "" {
			id, e := safeMessageID(a.ContentID)
			if e != nil {
				return nil, fmt.Errorf("invalid attachment content id")
			}
			h.Set("Content-ID", id)
		}
		p, e := w.CreateAttachment(mailmime.AttachmentHeader{Header: h})
		if e != nil {
			return nil, e
		}
		if _, e = p.Write(a.Data); e != nil {
			return nil, e
		}
		if e = p.Close(); e != nil {
			return nil, e
		}
	}
	if err = w.Close(); err != nil {
		return nil, err
	}
	if out.Len() > maxOutgoingMessageBytes {
		return nil, fmt.Errorf("message exceeds size limit")
	}
	return out.Bytes(), nil
}

func ParseMIME(raw []byte, maxBytes int64) (subject, from string, to, cc []string, text, html string, attachments []Attachment, htmlSkipped bool, err error) {
	if maxBytes <= 0 || maxBytes > 32<<20 {
		maxBytes = 32 << 20
	}
	if int64(len(raw)) > maxBytes {
		err = fmt.Errorf("message exceeds size limit")
		return
	}
	r, e := mailmime.CreateReader(bytes.NewReader(raw))
	if e != nil {
		err = e
		return
	}
	defer r.Close()
	subject, _ = r.Header.Subject()
	if a, e := r.Header.AddressList("From"); e == nil && len(a) > 0 {
		from = a[0].Address
	}
	if a, e := r.Header.AddressList("To"); e == nil {
		for _, x := range a {
			to = append(to, x.Address)
		}
	}
	if a, e := r.Header.AddressList("Cc"); e == nil {
		for _, x := range a {
			cc = append(cc, x.Address)
		}
	}
	var decodedTotal int64
	partCount := 0
	for {
		p, e := r.NextPart()
		if e == io.EOF {
			break
		}
		if e != nil {
			err = e
			return
		}
		partCount++
		if partCount > 512 {
			err = fmt.Errorf("MIME contains too many parts")
			return
		}
		remaining := maxBytes - decodedTotal
		if remaining <= 0 {
			err = fmt.Errorf("decoded MIME exceeds size limit")
			return
		}
		data, e := io.ReadAll(io.LimitReader(p.Body, remaining+1))
		if e != nil {
			err = e
			return
		}
		if int64(len(data)) > remaining {
			err = fmt.Errorf("decoded MIME exceeds size limit")
			return
		}
		decodedTotal += int64(len(data))
		switch h := p.Header.(type) {
		case *mailmime.InlineHeader:
			ct, _, _ := h.ContentType()
			if strings.EqualFold(ct, "text/html") {
				sanitized, sanitizeErr := SanitizeHTML(string(data))
				if sanitizeErr != nil {
					if html == "" {
						htmlSkipped = true
					}
					continue
				}
				html = sanitized
				htmlSkipped = false
			} else if strings.HasPrefix(strings.ToLower(ct), "text/") && text == "" {
				text = string(data)
			}
		case *mailmime.AttachmentHeader:
			filename, _ := h.Filename()
			ct, _, _ := h.ContentType()
			disp, _, _ := h.ContentDisposition()
			if filename == "" {
				filename = "attachment"
			}
			attachments = append(attachments, Attachment{ID: fmt.Sprintf("part-%d", len(attachments)+1), Filename: safeFilename(filename), ContentType: ct, Size: int64(len(data)), ContentID: strings.Trim(h.Get("Content-ID"), "<>"), Inline: strings.EqualFold(disp, "inline"), Data: data})
		}
	}
	return
}

func generatedMessageID(from, draftID string) (string, error) {
	address, err := mailboxAddress(from)
	if err != nil {
		return "", fmt.Errorf("invalid sender address")
	}
	_, domain, ok := strings.Cut(address, "@")
	if !ok || domain == "" || strings.ContainsAny(domain, " <>\r\n\t") {
		domain = "localhost"
	}
	var opaque string
	if strings.TrimSpace(draftID) != "" {
		sum := sha256.Sum256([]byte(draftID + "\x00" + address))
		opaque = hex.EncodeToString(sum[:16])
	} else {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "", fmt.Errorf("generate message id")
		}
		opaque = hex.EncodeToString(b)
	}
	return "<" + opaque + "@" + strings.ToLower(domain) + ">", nil
}

func ParseThreadHeaders(raw []byte) (messageID, inReplyTo string, references []string, err error) {
	r, e := mailmime.CreateReader(bytes.NewReader(raw))
	if e != nil {
		return "", "", nil, e
	}
	defer r.Close()
	messageID, _ = r.Header.MessageID()
	if ids, e := r.Header.MsgIDList("In-Reply-To"); e == nil && len(ids) > 0 {
		inReplyTo = ids[len(ids)-1]
	}
	references, _ = r.Header.MsgIDList("References")
	return messageID, inReplyTo, references, nil
}

// ParseReplyTo returns the mailbox addresses from the trusted RFC 5322
// Reply-To header. It deliberately excludes display names, matching To/CC.
func ParseReplyTo(raw []byte) []string {
	r, err := mailmime.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	defer r.Close()
	addresses, err := r.Header.AddressList("Reply-To")
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address != nil && address.Address != "" {
			out = append(out, address.Address)
		}
	}
	return out
}

func parseMailbox(raw string) (*mailmime.Address, error) {
	if strings.TrimSpace(raw) == "" || strings.ContainsAny(raw, "\r\n\x00") {
		return nil, fmt.Errorf("invalid mailbox")
	}
	a, e := mailmime.ParseAddress(raw)
	if e != nil || a.Address == "" {
		return nil, fmt.Errorf("invalid mailbox")
	}
	return a, nil
}
func parseMailboxes(raw []string) ([]*mailmime.Address, error) {
	out := make([]*mailmime.Address, 0, len(raw))
	for _, v := range raw {
		a, e := parseMailbox(v)
		if e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, nil
}
func safeMessageID(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if strings.ContainsAny(s, "\r\n\x00") {
		return "", fmt.Errorf("invalid id")
	}
	if !strings.HasPrefix(s, "<") {
		s = "<" + s
	}
	if !strings.HasSuffix(s, ">") {
		s += ">"
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(s, "<"), ">")
	if inner == "" || strings.ContainsAny(inner, " <>\t") || !strings.Contains(inner, "@") {
		return "", fmt.Errorf("invalid id")
	}
	return s, nil
}
func cleanHeaderText(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == 0 {
			return ' '
		}
		return r
	}, s)
}
func safeFilename(s string) string {
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = cleanHeaderText(s)
	if len(s) > 255 {
		s = s[:255]
	}
	if strings.TrimSpace(s) == "" {
		return "attachment"
	}
	return s
}
