package qrcode

import (
	"io"
	"mime/quotedprintable"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Classify identifies supported payload syntax without following or executing it.
// Text is always the exact decoded payload; Content contains only parsed fields.
func Classify(payload string) ScanResult {
	r := ScanResult{Text: payload, Type: "text"}
	if len(payload) > MaxPayloadBytes || !utf8.ValidString(payload) {
		return ScanResult{Type: "text", Diagnostics: []Diagnostic{warning("payload_limit", "Decoded content exceeds supported text bounds.")}}
	}
	lower, upper := strings.ToLower(payload), strings.ToUpper(payload)
	var content *Content
	switch {
	case strings.HasPrefix(upper, "WIFI:"):
		r.Type = "wifi"
		content = parseWiFi(payload)
	case strings.HasPrefix(upper, "BEGIN:VCARD"):
		r.Type = "vcard"
		content = parseVCard(payload)
	case strings.HasPrefix(upper, "BEGIN:VCALENDAR") || strings.HasPrefix(upper, "BEGIN:VEVENT"):
		r.Type = "calendar"
		content = parseCalendar(payload)
	case strings.HasPrefix(upper, "BCD\n") || strings.HasPrefix(upper, "BCD\r\n"):
		r.Type = "epc"
		content = parseEPC(payload)
	case strings.HasPrefix(lower, "mailto:"):
		r.Type = "email"
		content = parseEmail(payload)
	case strings.HasPrefix(lower, "tel:"):
		r.Type = "telephone"
		if n, e := url.PathUnescape(payload[4:]); e == nil {
			content = &Content{Type: "telephone", Telephone: &TelephoneContent{Number: n}}
		}
	case strings.HasPrefix(lower, "sms:") || strings.HasPrefix(upper, "SMSTO:"):
		r.Type = "sms"
		content = parseSMS(payload)
	case strings.HasPrefix(lower, "geo:"):
		r.Type = "location"
		values := strings.Split(strings.SplitN(payload[4:], ";", 2)[0], ",")
		if len(values) == 2 {
			lat, e1 := strconv.ParseFloat(values[0], 64)
			lon, e2 := strconv.ParseFloat(values[1], 64)
			if e1 == nil && e2 == nil {
				content = &Content{Type: "location", Location: &LocationContent{Latitude: lat, Longitude: lon}}
			}
		}
	case validURL(payload) == nil:
		r.Type = "url"
		content = &Content{Type: "url", URL: payload}
	default:
		content = &Content{Type: "text", Text: payload}
	}
	if content != nil {
		if _, e := Payload(*content); e != nil {
			content = nil
		}
	}
	r.Content = content
	if content == nil {
		r.Diagnostics = append(r.Diagnostics, warning("payload_fields", "The payload type was identified, but some fields are unsupported or invalid."))
	}
	return r
}

func splitEscaped(s string, separator byte) []string {
	parts := []string{}
	start := 0
	escaped := false
	for i := 0; i < len(s); i++ {
		if escaped {
			escaped = false
			continue
		}
		if s[i] == '\\' {
			escaped = true
			continue
		}
		if s[i] == separator {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}
func unescapeText(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			if s[i] == 'n' || s[i] == 'N' {
				b.WriteByte('\n')
			} else {
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
func parseWiFi(s string) *Content {
	fields := map[string]string{}
	for _, part := range splitEscaped(s[5:], ';') {
		if part == "" {
			continue
		}
		p := strings.SplitN(part, ":", 2)
		if len(p) != 2 || fields[p[0]] != "" || !oneOf(p[0], "S", "P", "T", "H") {
			return nil
		}
		fields[p[0]] = unescapeText(p[1])
	}
	ssid := fields["S"]
	if len(ssid) >= 2 && ssid[0] == '"' && ssid[len(ssid)-1] == '"' {
		ssid = ssid[1 : len(ssid)-1]
	}
	security := fields["T"]
	if security == "" {
		security = "nopass"
	}
	return &Content{Type: "wifi", WiFi: &WiFiContent{SSID: ssid, Password: fields["P"], Security: security, Hidden: strings.EqualFold(fields["H"], "true")}}
}
func parseEmail(s string) *Content {
	u, e := url.Parse(s)
	if e != nil {
		return nil
	}
	to, e := url.PathUnescape(u.Opaque)
	if e != nil {
		return nil
	}
	q, e := url.ParseQuery(u.RawQuery)
	if e != nil {
		return nil
	}
	v := &EmailContent{To: strings.Split(to, ","), Subject: q.Get("subject"), Body: q.Get("body")}
	if q.Get("cc") != "" {
		v.CC = strings.Split(q.Get("cc"), ",")
	}
	if q.Get("bcc") != "" {
		v.BCC = strings.Split(q.Get("bcc"), ",")
	}
	return &Content{Type: "email", Email: v}
}
func parseSMS(s string) *Content {
	if strings.HasPrefix(strings.ToUpper(s), "SMSTO:") {
		v := strings.SplitN(s[6:], ":", 2)
		sms := &SMSContent{Number: v[0]}
		if len(v) == 2 {
			sms.Body = v[1]
		}
		return &Content{Type: "sms", SMS: sms}
	}
	parts := strings.SplitN(s[4:], "?", 2)
	number, e := url.PathUnescape(parts[0])
	if e != nil {
		return nil
	}
	body := ""
	if len(parts) == 2 {
		q, e := url.ParseQuery(parts[1])
		if e != nil {
			return nil
		}
		body = q.Get("body")
	}
	return &Content{Type: "sms", SMS: &SMSContent{Number: number, Body: body}}
}
func contentLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n ", "")
	s = strings.ReplaceAll(s, "\n\t", "")
	raw := strings.Split(s, "\n")
	lines := make([]string, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		line := raw[i]
		key := strings.SplitN(line, ":", 2)[0]
		if strings.Contains(strings.ToUpper(key), "ENCODING=QUOTED-PRINTABLE") {
			for strings.HasSuffix(line, "=") && i+1 < len(raw) {
				i++
				line = strings.TrimSuffix(line, "=") + raw[i]
			}
		}
		lines = append(lines, line)
	}
	return lines
}
func parsedTELKind(key string) string {
	types := map[string]bool{}
	for _, p := range strings.Split(strings.ToUpper(key), ";")[1:] {
		if strings.HasPrefix(p, "TYPE=") {
			for _, v := range strings.Split(strings.TrimPrefix(p, "TYPE="), ",") {
				types[v] = true
			}
		} else {
			types[p] = true
		}
	}
	if types["CELL"] {
		return "cell"
	}
	if types["HOME"] {
		return "home"
	}
	if types["WORK"] {
		return "work"
	}
	return ""
}
func parseVCard(s string) *Content {
	v := &VCardContent{}
	for _, line := range contentLines(s) {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToUpper(parts[0])
		name := strings.SplitN(key, ";", 2)[0]
		value := parts[1]
		qp := strings.Contains(key, "ENCODING=QUOTED-PRINTABLE")
		if qp {
			raw, e := io.ReadAll(io.LimitReader(quotedprintable.NewReader(strings.NewReader(value)), MaxPayloadBytes+1))
			if e != nil || len(raw) > MaxPayloadBytes || !utf8.Valid(raw) {
				return nil
			}
			value = string(raw)
		}
		text := unescapeText(value)
		if qp {
			text = value
		}
		switch name {
		case "VERSION":
			v.Version = value
		case "FN":
			v.FormattedName = text
		case "N":
			fields := splitEscaped(value, ';')
			for len(fields) < 5 {
				fields = append(fields, "")
			}
			v.LastName, v.FirstName, v.MiddleName, v.Prefix, v.Suffix = unescapeText(fields[0]), unescapeText(fields[1]), unescapeText(fields[2]), unescapeText(fields[3]), unescapeText(fields[4])
		case "ORG":
			v.Organization = text
		case "TITLE":
			v.Title = text
		case "TEL":
			number := unescapeText(value)
			if strings.HasPrefix(strings.ToLower(number), "tel:") {
				number = number[4:]
			}
			target := &v.Phone
			switch parsedTELKind(key) {
			case "work":
				target = &v.PhoneWork
			case "home":
				target = &v.PhonePrivate
			case "cell":
				target = &v.PhoneMobile
			}
			if *target != "" && *target != number {
				return nil
			}
			*target = number
		case "EMAIL":
			v.Email = value
		case "URL":
			v.URL = value
		case "NOTE":
			v.Note = text
		case "BDAY":
			v.Birthday = value
		case "ADR":
			fields := splitEscaped(value, ';')
			if len(fields) >= 7 {
				v.Address = &AddressContent{Street: unescapeText(fields[2]), City: unescapeText(fields[3]), Region: unescapeText(fields[4]), PostalCode: unescapeText(fields[5]), Country: unescapeText(fields[6])}
			}
		}
	}
	return &Content{Type: "vcard", VCard: v}
}
func parseCalendar(s string) *Content {
	c := &CalendarContent{}
	for _, line := range contentLines(s) {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToUpper(parts[0])
		name := strings.SplitN(key, ";", 2)[0]
		value := parts[1]
		switch name {
		case "DTSTAMP":
			stamp, err := time.Parse("20060102T150405Z", value)
			if err != nil {
				return nil
			}
			c.CreatedAt = stamp.UTC().Format(time.RFC3339)
		case "SUMMARY":
			c.Title = unescapeText(value)
		case "LOCATION":
			c.Location = unescapeText(value)
		case "DESCRIPTION":
			c.Description = unescapeText(value)
		case "DTSTART", "DTEND":
			date := strings.Contains(key, "VALUE=DATE") && !strings.Contains(key, "VALUE=DATE-TIME")
			layout := "20060102T150405"
			if date {
				layout = "20060102"
				c.AllDay = true
			}
			if strings.HasSuffix(value, "Z") {
				layout = "20060102T150405Z"
			}
			t, e := time.Parse(layout, value)
			if e != nil {
				return nil
			}
			formatted := t.Format("2006-01-02T15:04:05")
			if strings.HasSuffix(value, "Z") {
				formatted = t.Format(time.RFC3339)
			}
			if date {
				formatted = t.Format("2006-01-02")
			}
			for _, parameter := range strings.Split(parts[0], ";")[1:] {
				kv := strings.SplitN(parameter, "=", 2)
				if len(kv) == 2 && strings.EqualFold(kv[0], "TZID") {
					c.Timezone = kv[1]
				}
			}
			if name == "DTSTART" {
				c.Start = formatted
			} else {
				c.End = formatted
			}
		}
	}
	return &Content{Type: "calendar", Calendar: c}
}
func parseEPC(s string) *Content {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if len(lines) < 7 || len(lines) > 12 || lines[2] != "1" || lines[3] != "SCT" {
		return nil
	}
	for len(lines) < 12 {
		lines = append(lines, "")
	}
	amount := lines[7]
	if amount != "" && !strings.HasPrefix(amount, "EUR") {
		return nil
	}
	return &Content{Type: "epc", EPC: &EPCContent{Version: lines[1], BIC: lines[4], Name: lines[5], IBAN: lines[6], Amount: strings.TrimPrefix(amount, "EUR"), Purpose: lines[8], Reference: lines[9], Text: lines[10], Information: lines[11]}}
}
