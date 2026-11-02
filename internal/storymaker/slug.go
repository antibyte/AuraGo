package storymaker

import (
	"regexp"
	"strings"
)

var projectIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// ValidProjectID reports a lowercase UUID v4, the only form project folders are named by.
func ValidProjectID(id string) bool { return projectIDPattern.MatchString(id) }

// maxSlugLen is the longest slug Slugify returns.
const maxSlugLen = 80

// translit maps letters outside ASCII to ASCII spellings (German umlauts the usual way).
var translit = buildTranslit()

func buildTranslit() map[rune]string {
	m := map[rune]string{
		'ä': "ae", 'ö': "oe", 'ü': "ue", 'Ä': "Ae", 'Ö': "Oe", 'Ü': "Ue", 'ß': "ss", 'ẞ': "SS",
		'æ': "ae", 'Æ': "AE", 'œ': "oe", 'Œ': "OE", 'þ': "th", 'Þ': "Th",
	}
	groups := map[string]string{
		"àáâãåāăąǎ": "a", "ÀÁÂÃÅĀĂĄǍ": "A",
		"çćĉċč": "c", "ÇĆĈĊČ": "C",
		"ďđð": "d", "ĎĐÐ": "D",
		"èéêëēĕėęě": "e", "ÈÉÊËĒĔĖĘĚ": "E",
		"ĝğġģ": "g", "ĜĞĠĢ": "G",
		"ĥ": "h", "Ĥ": "H",
		"ìíîïĩīĭįı": "i", "ÌÍÎÏĨĪĬĮİ": "I",
		"ĵ": "j", "Ĵ": "J",
		"ķ": "k", "Ķ": "K",
		"ĺļľł": "l", "ĹĻĽŁ": "L",
		"ñńņň": "n", "ÑŃŅŇ": "N",
		"òóôõōŏőøǒ": "o", "ÒÓÔÕŌŎŐØǑ": "O",
		"ŕŗř": "r", "ŔŖŘ": "R",
		"śŝşš": "s", "ŚŜŞŠ": "S",
		"ţťŧ": "t", "ŢŤŦ": "T",
		"ùúûũūŭůűųǔ": "u", "ÙÚÛŨŪŬŮŰŲǓ": "U",
		"ŵ": "w", "Ŵ": "W",
		"ýÿŷ": "y", "ÝŸŶ": "Y",
		"źżž": "z", "ŹŻŽ": "Z",
	}
	for letters, ascii := range groups {
		for _, r := range letters {
			m[r] = ascii
		}
	}
	return m
}

// windowsReserved are device names Windows refuses as file names (with or without extension).
func windowsReserved(name string) bool {
	n := strings.ToUpper(name)
	switch n {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(n) == 4 && (strings.HasPrefix(n, "COM") || strings.HasPrefix(n, "LPT")) && n[3] >= '1' && n[3] <= '9' {
		return true
	}
	return false
}

// Slugify turns a title into a safe file-name stem: ASCII letters, digits and single dashes,
// at most 80 characters, case kept. Umlauts and other Latin letters are transliterated
// (ä becomes ae, ß becomes ss). When nothing usable remains, or the result is a Windows
// reserved device name (CON, PRN, AUX, NUL, COM1-9, LPT1-9), it returns fallback (the project
// id), or "storymaker" when that is empty too.
//
//	Slugify("Was ist eine Firewall?", id) == "Was-ist-eine-Firewall"
func Slugify(title, fallback string) string {
	var b strings.Builder
	dash := true
	for _, r := range title {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
			continue
		}
		if t, ok := translit[r]; ok {
			b.WriteString(t)
			dash = false
			continue
		}
		if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > maxSlugLen {
		out = strings.Trim(out[:maxSlugLen], "-")
	}
	if out == "" || windowsReserved(out) {
		if fallback == "" {
			return "storymaker"
		}
		return fallback
	}
	return out
}
