package qrcode

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"net/mail"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

var bicPattern = regexp.MustCompile(`^[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?$`)
var amountPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})(\.[0-9]{1,2})?$`)
var hexPattern = regexp.MustCompile(`^[0-9A-Fa-f]+$`)

// Payload serializes one selected template. Errors never echo field contents.
func Payload(c Content) (string, error) {
	if !validStrings(reflect.ValueOf(c)) {
		return "", invalid("content", "text", "content must be valid UTF-8 without NUL and within the payload limit")
	}
	count := 0
	selected := ""
	for _, v := range []struct {
		key string
		set bool
	}{{"text", c.Text != ""}, {"url", c.URL != ""}, {"email", c.Email != nil}, {"telephone", c.Telephone != nil}, {"sms", c.SMS != nil}, {"wifi", c.WiFi != nil}, {"vcard", c.VCard != nil}, {"location", c.Location != nil}, {"calendar", c.Calendar != nil}, {"epc", c.EPC != nil}} {
		if v.set {
			count++
			selected = v.key
		}
	}
	if count != 1 || selected != c.Type {
		return "", invalid("content", "selection", "supply exactly one matching content template")
	}
	var payload string
	var err error
	switch c.Type {
	case "text":
		payload = c.Text
	case "url":
		if err = validURL(c.URL); err == nil {
			payload = c.URL
		}
	case "email":
		payload, err = emailPayload(*c.Email)
	case "telephone":
		if err = validPhone(c.Telephone.Number); err == nil {
			payload = "tel:" + compactPhone(c.Telephone.Number)
		}
	case "sms":
		if err = validPhone(c.SMS.Number); err == nil {
			payload = "sms:" + compactPhone(c.SMS.Number)
			if c.SMS.Body != "" {
				payload += "?body=" + uriEscape(c.SMS.Body)
			}
		}
	case "wifi":
		payload, err = wifiPayload(*c.WiFi)
	case "vcard":
		payload, err = vcardPayload(*c.VCard)
	case "location":
		l := c.Location
		if !finite(l.Latitude) || !finite(l.Longitude) || math.Abs(l.Latitude) > 90 || math.Abs(l.Longitude) > 180 {
			err = invalid("content.location", "range", "latitude or longitude is outside geographic bounds")
		} else {
			payload = "geo:" + strconv.FormatFloat(l.Latitude, 'f', -1, 64) + "," + strconv.FormatFloat(l.Longitude, 'f', -1, 64)
		}
	case "calendar":
		payload, err = calendarPayload(*c.Calendar)
	case "epc":
		payload, err = epcPayload(*c.EPC)
	default:
		err = invalid("content.type", "option", "unsupported content template")
	}
	if err != nil {
		return "", err
	}
	if len(payload) == 0 || len(payload) > MaxPayloadBytes {
		return "", invalid("content", "capacity", "serialized payload exceeds the supported limit")
	}
	return payload, nil
}
func validStrings(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		s := v.String()
		return utf8.ValidString(s) && len(s) <= MaxPayloadBytes && !strings.ContainsRune(s, 0)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !validStrings(v.Field(i)) {
				return false
			}
		}
	case reflect.Pointer:
		if !v.IsNil() {
			return validStrings(v.Elem())
		}
	case reflect.Slice:
		if v.Len() > 64 {
			return false
		}
		for i := 0; i < v.Len(); i++ {
			if !validStrings(v.Index(i)) {
				return false
			}
		}
	}
	return true
}
func singleLine(s string) bool { return !strings.ContainsAny(s, "\r\n\x00") }
func validURL(s string) error {
	u, err := url.Parse(s)
	if err != nil || !oneOf(strings.ToLower(u.Scheme), "http", "https") || u.Hostname() == "" || u.User != nil || !singleLine(s) {
		return invalid("content.url", "url", "use an absolute HTTP or HTTPS URL without credentials")
	}
	return nil
}
func validPhone(s string) error    { _, _, err := phoneSubscriber(s); return err }
func compactPhone(s string) string { number, _, _ := phoneSubscriber(s); return number }
func phoneDigits(s string, local bool) (string, bool) {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(" ().-", r) {
			continue
		}
		if r >= '0' && r <= '9' || local && (r >= 'A' && r <= 'F' || r >= 'a' && r <= 'f' || r == '*' || r == '#') {
			b.WriteRune(r)
		} else {
			return "", false
		}
	}
	return strings.ToUpper(b.String()), b.Len() > 0
}
func phoneDomain(s string) bool {
	s = strings.TrimSuffix(s, ".")
	if len(s) == 0 || len(s) > 253 {
		return false
	}
	labels := strings.Split(s, ".")
	for i, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for j, r := range label {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
				if i == len(labels)-1 && j == 0 && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
					return false
				}
				continue
			}
			return false
		}
	}
	return true
}
func phoneSubscriber(s string) (string, bool, error) {
	bad := func() (string, bool, error) {
		return "", false, invalid("content.telephone", "phone", "use a bounded phone number with optional ;ext= and ;phone-context= parameters")
	}
	s = strings.TrimSpace(s)
	if len(s) == 0 || len(s) > 256 || !singleLine(s) {
		return bad()
	}
	parts := strings.Split(s, ";")
	if len(parts) > 3 {
		return bad()
	}
	global := strings.HasPrefix(parts[0], "+")
	base := parts[0]
	if global {
		base = base[1:]
	}
	digits, ok := phoneDigits(base, !global)
	if !ok {
		return bad()
	}
	if global {
		digits = "+" + digits
	}
	params := map[string]string{}
	for _, p := range parts[1:] {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			return bad()
		}
		key := strings.ToLower(kv[0])
		if _, ok := params[key]; ok {
			return bad()
		}
		switch key {
		case "ext":
			value, ok := phoneDigits(kv[1], false)
			if !ok {
				return bad()
			}
			params[key] = value
		case "phone-context":
			if global {
				return bad()
			}
			value := kv[1]
			if strings.HasPrefix(value, "+") {
				v, ok := phoneDigits(value[1:], false)
				if !ok {
					return bad()
				}
				value = "+" + v
			} else {
				if !phoneDomain(value) {
					return bad()
				}
				value = strings.ToLower(strings.TrimSuffix(value, "."))
			}
			params[key] = value
		default:
			return bad()
		}
	}
	result := digits
	if params["ext"] != "" {
		result += ";ext=" + params["ext"]
	}
	if params["phone-context"] != "" {
		result += ";phone-context=" + params["phone-context"]
	}
	return result, !global && params["phone-context"] == "", nil
}
func uriEscape(s string) string { return strings.ReplaceAll(url.QueryEscape(s), "+", "%20") }
func validEmail(s string) bool {
	if !singleLine(s) {
		return false
	}
	a, e := mail.ParseAddress(s)
	return e == nil && a.Address == s && !strings.ContainsAny(s, "<>,;\" ")
}
func emailPayload(e EmailContent) (string, error) {
	if len(e.To) == 0 || !singleLine(e.Subject) {
		return "", invalid("content.email", "email", "email requires a recipient and a single-line subject")
	}
	for _, addresses := range [][]string{e.To, e.CC, e.BCC} {
		for _, a := range addresses {
			if !validEmail(a) {
				return "", invalid("content.email", "email", "email addresses must contain only valid mailbox addresses")
			}
		}
	}
	parts := make([]string, len(e.To))
	for i, s := range e.To {
		parts[i] = uriEscape(s)
	}
	q := url.Values{}
	if e.Subject != "" {
		q.Set("subject", e.Subject)
	}
	if e.Body != "" {
		body := strings.ReplaceAll(strings.ReplaceAll(e.Body, "\r\n", "\n"), "\r", "\n")
		q.Set("body", strings.ReplaceAll(body, "\n", "\r\n"))
	}
	if len(e.CC) > 0 {
		q.Set("cc", strings.Join(e.CC, ","))
	}
	if len(e.BCC) > 0 {
		q.Set("bcc", strings.Join(e.BCC, ","))
	}
	result := "mailto:" + strings.Join(parts, ",")
	if len(q) > 0 {
		result += "?" + strings.ReplaceAll(q.Encode(), "+", "%20")
	}
	return result, nil
}
func wifiEscape(s string) string {
	return strings.NewReplacer("\\", "\\\\", ";", "\\;", ",", "\\,", "\"", "\\\"", ":", "\\:").Replace(s)
}
func wifiPayload(w WiFiContent) (string, error) {
	if w.Security == "" {
		w.Security = "WPA"
	}
	if !oneOf(w.Security, "nopass", "WEP", "WPA") || len(w.SSID) == 0 || len(w.SSID) > 32 || !singleLine(w.SSID) || !singleLine(w.Password) {
		return "", invalid("content.wifi", "wifi", "WiFi requires a valid SSID of at most 32 bytes and a supported security type")
	}
	switch w.Security {
	case "nopass":
		if w.Password != "" {
			return "", invalid("content.wifi.password", "wifi_password", "an open network cannot have a password")
		}
	case "WPA":
		if !((len(w.Password) >= 8 && len(w.Password) <= 63) || (len(w.Password) == 64 && hexPattern.MatchString(w.Password))) {
			return "", invalid("content.wifi.password", "wifi_password", "WPA passwords require 8–63 bytes or 64 hexadecimal digits")
		}
	case "WEP":
		if !((len(w.Password) == 5 || len(w.Password) == 13) || (len(w.Password) == 10 || len(w.Password) == 26) && hexPattern.MatchString(w.Password)) {
			return "", invalid("content.wifi.password", "wifi_password", "WEP keys require 5 or 13 bytes, or 10 or 26 hexadecimal digits")
		}
	}
	ssid := wifiEscape(w.SSID)
	if hexPattern.MatchString(w.SSID) {
		ssid = "\"" + ssid + "\""
	}
	p := "WIFI:T:" + w.Security + ";S:" + ssid + ";"
	if w.Security != "nopass" {
		p += "P:" + wifiEscape(w.Password) + ";"
	}
	if w.Hidden {
		p += "H:true;"
	}
	return p + ";", nil
}
func textEscape(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	return strings.NewReplacer("\\", "\\\\", "\n", "\\n", ";", "\\;", ",", "\\,").Replace(s)
}
func foldLine(s string) string {
	var b strings.Builder
	column := 0
	for _, r := range s {
		n := utf8.RuneLen(r)
		if column+n > 75 {
			b.WriteString("\r\n ")
			column = 1
		}
		b.WriteRune(r)
		column += n
	}
	return b.String()
}
func vcard21Line(key, value string) string {
	const hexDigits = "0123456789ABCDEF"
	prefix := key + ";CHARSET=UTF-8;ENCODING=QUOTED-PRINTABLE:"
	var b strings.Builder
	b.WriteString(prefix)
	column := len(prefix)
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
	value = strings.ReplaceAll(value, "\n", "\r\n")
	for i := 0; i < len(value); i++ {
		c := value[i]
		token := string([]byte{c})
		if c < 33 || c > 126 || c == '=' {
			token = string([]byte{'=', hexDigits[c>>4], hexDigits[c&15]})
		}
		if column+len(token) > 75 {
			b.WriteString("=\r\n")
			column = 0
		}
		b.WriteString(token)
		column += len(token)
	}
	return b.String()
}
func vcardPayload(v VCardContent) (string, error) {
	if v.Version == "" {
		v.Version = "3.0"
	}
	if !oneOf(v.Version, "2.1", "3.0", "4.0") {
		return "", invalid("content.vcard.version", "option", "supported vCard versions are 2.1, 3.0 and 4.0")
	}
	fn := v.FormattedName
	if fn == "" {
		fn = strings.TrimSpace(v.FirstName + " " + v.LastName)
	}
	if fn == "" {
		return "", invalid("content.vcard", "name", "a contact requires a name")
	}
	for _, number := range []string{v.Phone, v.PhoneWork, v.PhonePrivate, v.PhoneMobile} {
		if number != "" {
			if err := validPhone(number); err != nil {
				return "", err
			}
		}
	}
	if v.Email != "" && !validEmail(v.Email) {
		return "", invalid("content.vcard.email", "email", "invalid contact email address")
	}
	if v.URL != "" {
		if e := validURL(v.URL); e != nil {
			return "", e
		}
	}
	if v.Birthday != "" {
		if _, e := time.Parse("2006-01-02", v.Birthday); e != nil {
			return "", invalid("content.vcard.birthday", "date", "birthday requires a valid YYYY-MM-DD date")
		}
	}
	lines := []string{"BEGIN:VCARD", "VERSION:" + v.Version}
	escapePart := textEscape
	if v.Version == "2.1" {
		escapePart = func(s string) string { return strings.ReplaceAll(s, ";", "\\;") }
	}
	addText := func(k, s string, compound bool) {
		if s == "" {
			return
		}
		if v.Version == "2.1" {
			if compound {
				s = escapePart(s)
			}
			lines = append(lines, vcard21Line(k, s))
		} else {
			lines = append(lines, foldLine(k+":"+textEscape(s)))
		}
	}
	addCompound := func(k string, parts []string) {
		for i := range parts {
			parts[i] = escapePart(parts[i])
		}
		value := strings.Join(parts, ";")
		if v.Version == "2.1" {
			lines = append(lines, vcard21Line(k, value))
		} else {
			lines = append(lines, foldLine(k+":"+value))
		}
	}
	addText("FN", fn, false)
	addCompound("N", []string{v.LastName, v.FirstName, v.MiddleName, v.Prefix, v.Suffix})
	addText("ORG", v.Organization, true)
	addText("TITLE", v.Title, false)
	addText("NOTE", v.Note, false)
	for _, p := range []struct{ number, kind string }{{v.Phone, ""}, {v.PhoneWork, "work"}, {v.PhonePrivate, "home"}, {v.PhoneMobile, "cell"}} {
		if p.number == "" {
			continue
		}
		number, bareLocal, _ := phoneSubscriber(p.number)
		key := "TEL"
		if p.kind != "" {
			if v.Version == "2.1" {
				key += ";" + strings.ToUpper(p.kind) + ";VOICE"
			} else {
				key += ";TYPE=" + p.kind + ",voice"
			}
		}
		if v.Version == "4.0" {
			if bareLocal {
				key += ";VALUE=text"
				number = textEscape(number)
			} else {
				key += ";VALUE=uri"
				number = "tel:" + number
			}
		}
		lines = append(lines, foldLine(key+":"+number))
	}
	if v.Email != "" {
		lines = append(lines, "EMAIL:"+v.Email)
	}
	if v.URL != "" {
		lines = append(lines, foldLine("URL:"+v.URL))
	}
	if v.Birthday != "" {
		lines = append(lines, "BDAY:"+v.Birthday)
	}
	if v.Address != nil {
		a := v.Address
		addCompound("ADR", []string{"", "", a.Street, a.City, a.Region, a.PostalCode, a.Country})
	}
	lines = append(lines, "END:VCARD")
	return strings.Join(lines, "\r\n"), nil
}
func parseEventTime(s, zone string, allDay bool) (time.Time, error) {
	if allDay {
		t, e := time.Parse("2006-01-02", s)
		if e != nil {
			return t, invalid("content.calendar", "date", "all-day dates require YYYY-MM-DD")
		}
		return t, nil
	}
	var loc *time.Location
	if zone != "" {
		if zone == "Local" || strings.HasPrefix(zone, "posix/") || strings.HasPrefix(zone, "right/") || !singleLine(zone) || len(zone) > 64 || strings.ContainsAny(zone, "\\:;") {
			return time.Time{}, invalid("content.calendar.timezone", "timezone", "invalid timezone")
		}
		var e error
		loc, e = time.LoadLocation(zone)
		if e != nil {
			return time.Time{}, invalid("content.calendar.timezone", "timezone", "unknown IANA timezone")
		}
	}
	if t, e := time.Parse(time.RFC3339, s); e == nil {
		if loc != nil && t.In(loc).Format("-07:00") != t.Format("-07:00") {
			return t, invalid("content.calendar", "timezone", "timestamp offset conflicts with the selected timezone")
		}
		return t, nil
	}
	if loc == nil {
		return time.Time{}, invalid("content.calendar", "datetime", "use RFC3339 timestamps or select an IANA timezone for local times")
	}
	layout := "2006-01-02T15:04"
	if len(s) == 19 {
		layout = "2006-01-02T15:04:05"
	}
	t, e := time.ParseInLocation(layout, s, loc)
	if e != nil || t.Format(layout) != s {
		return t, invalid("content.calendar", "datetime", "invalid or nonexistent local date/time")
	}
	for _, d := range []time.Duration{-2 * time.Hour, -time.Hour, -30 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour} {
		if t.Add(d).Format(layout) == s {
			return t, invalid("content.calendar", "ambiguous_time", "ambiguous local time requires an explicit RFC3339 UTC offset")
		}
	}
	return t, nil
}
func calendarPayload(c CalendarContent) (string, error) {
	createdAt, err := time.Parse(time.RFC3339, c.CreatedAt)
	if err != nil {
		return "", invalid("content.calendar.created_at", "datetime", "calendar creation time requires RFC3339; normalize a new spec before serialization")
	}
	if strings.TrimSpace(c.Title) == "" {
		return "", invalid("content.calendar.title", "required", "an event requires a title")
	}
	start, e := parseEventTime(c.Start, c.Timezone, c.AllDay)
	if e != nil {
		return "", e
	}
	end, e := parseEventTime(c.End, c.Timezone, c.AllDay)
	if e != nil {
		return "", e
	}
	if !end.After(start) {
		return "", invalid("content.calendar.end", "range", "event end must be after start; all-day end dates are exclusive")
	}
	hash := sha256.Sum256([]byte(c.Title + "\x00" + c.Start + "\x00" + c.End))
	lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//AuraGo//QR Calendar//EN", "BEGIN:VEVENT", "UID:" + hex.EncodeToString(hash[:16]) + "@aurago.local", "DTSTAMP:" + createdAt.UTC().Format("20060102T150405Z"), foldLine("SUMMARY:" + textEscape(c.Title))}
	if c.AllDay {
		lines = append(lines, "DTSTART;VALUE=DATE:"+start.Format("20060102"), "DTEND;VALUE=DATE:"+end.Format("20060102"))
	} else {
		lines = append(lines, "DTSTART:"+start.UTC().Format("20060102T150405Z"), "DTEND:"+end.UTC().Format("20060102T150405Z"))
	}
	if c.Location != "" {
		lines = append(lines, foldLine("LOCATION:"+textEscape(c.Location)))
	}
	if c.Description != "" {
		lines = append(lines, foldLine("DESCRIPTION:"+textEscape(c.Description)))
	}
	lines = append(lines, "END:VEVENT", "END:VCALENDAR")
	return strings.Join(lines, "\r\n"), nil
}

func mod97(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n = (n*10 + int(r-'0')) % 97
		} else if r >= 'A' && r <= 'Z' {
			n = (n*100 + int(r-'A') + 10) % 97
		} else {
			return -1
		}
	}
	return n
}
func validIBAN(s string) bool {
	if len(s) < 15 || len(s) > 34 || s[0] < 'A' || s[0] > 'Z' || s[1] < 'A' || s[1] > 'Z' || s[2] < '0' || s[2] > '9' || s[3] < '0' || s[3] > '9' {
		return false
	}
	return mod97(s[4:]+s[:4]) == 1
}
func epcPayload(e EPCContent) (string, error) {
	if e.Version == "" {
		e.Version = "002"
	}
	if !oneOf(e.Version, "001", "002") {
		return "", invalid("content.epc.version", "option", "EPC version must be 001 or 002")
	}
	iban := strings.ToUpper(strings.ReplaceAll(e.IBAN, " ", ""))
	bic := strings.ToUpper(e.BIC)
	if !validIBAN(iban) {
		return "", invalid("content.epc.iban", "checksum", "invalid IBAN format or checksum")
	}
	eea := strings.Contains(" AT BE BG HR CY CZ DK EE FI FR DE GR HU IS IE IT LV LI LT LU MT NL NO PL PT RO SK SI ES SE ", " "+iban[:2]+" ")
	if (e.Version == "001" || !eea) && bic == "" || bic != "" && !bicPattern.MatchString(bic) {
		return "", invalid("content.epc.bic", "bic", "EPC 001 and non-EEA beneficiary accounts require a valid BIC; supplied BICs need 8 or 11 characters")
	}
	for _, f := range []struct {
		s string
		n int
	}{{e.Name, 70}, {e.Purpose, 4}, {e.Reference, 35}, {e.Text, 140}, {e.Information, 70}} {
		if !singleLine(f.s) || utf8.RuneCountInString(f.s) > f.n {
			return "", invalid("content.epc", "field", "EPC fields must be single-line and within their character limits")
		}
	}
	if strings.TrimSpace(e.Name) == "" || e.Reference != "" && e.Text != "" {
		return "", invalid("content.epc", "remittance", "EPC requires a beneficiary name and at most one remittance field")
	}
	amount := ""
	if e.Amount != "" {
		if !amountPattern.MatchString(e.Amount) {
			return "", invalid("content.epc.amount", "amount", "use an exact EUR decimal without separators, between 0.01 and 999999999.99")
		}
		p := strings.SplitN(e.Amount, ".", 2)
		whole, _ := strconv.ParseInt(p[0], 10, 64)
		fraction := "00"
		if len(p) == 2 {
			fraction = (p[1] + "0")[:2]
		}
		cents, _ := strconv.ParseInt(fraction, 10, 64)
		if whole*100+cents < 1 {
			return "", invalid("content.epc.amount", "amount", "amount must be at least 0.01 EUR")
		}
		amount = "EUR" + p[0] + "." + fraction
	}
	if strings.HasPrefix(e.Reference, "RF") {
		ref := strings.ReplaceAll(e.Reference, " ", "")
		if len(ref) < 5 || len(ref) > 25 || ref[2] < '0' || ref[2] > '9' || ref[3] < '0' || ref[3] > '9' || mod97(ref[4:]+ref[:4]) != 1 {
			return "", invalid("content.epc.reference", "checksum", "invalid RF creditor reference checksum")
		}
	}
	payload := strings.TrimRight(strings.Join([]string{"BCD", e.Version, "1", "SCT", bic, e.Name, iban, amount, e.Purpose, e.Reference, e.Text, e.Information}, "\n"), "\n")
	if len(payload) > 331 {
		return "", invalid("content.epc", "capacity", "EPC payload exceeds 331 UTF-8 bytes")
	}
	return payload, nil
}
