package ecalc

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// The date_code kind: the date of manufacture that makers print on a component. Two forms are read. Four digits are YYWW (IEC 60062
// 8.3.2, in the 2004 edition 6.2.1): the last two digits of the year and the ISO 8601 week; two characters are the IEC 60062 year
// and month code (8.2.2, 2004 edition 6.1.1): a year letter of the 20-year cycle A B C D E F H J K L M N P R S T U V W X and a month
// character 1 to 9, O, N, D. The kind has no number: the result carries text fields only. All date arithmetic is on calendar dates
// in UTC; the decoder knows no "today", so a letter code lists its candidate years. Makers count the week in slightly different
// ways (the notes say so); the ISO week is what the standard defines.

type codeDateMonth struct {
	Char  string `yaml:"char" json:"char"`
	Month int    `yaml:"month" json:"month"`
}

type codeDateData struct {
	FirstYear   int             `yaml:"first_year"`
	LastYear    int             `yaml:"last_year"`
	YearLetters []string        `yaml:"year_letters"`
	Months      []codeDateMonth `yaml:"months"`
	Notes       []Text          `yaml:"notes"`

	letter map[string]int // year letter -> its index in the cycle
	month  map[string]int // month character -> month number
	char   [13]string     // month number -> month character
}

func codeLoadDate(raw *yaml.Node, fail func(string, ...any)) any {
	var d codeDateData
	if err := decodeNode(raw, &d); err != nil {
		fail("data: %v", err)
		return nil
	}
	d.letter, d.month = map[string]int{}, map[string]int{}
	if d.FirstYear < 1000 || d.LastYear < d.FirstYear {
		fail("data: first_year and last_year: a range of years from 1000 on is required")
	}
	for i, l := range d.YearLetters {
		if len(l) != 1 || l < "A" || l > "Z" {
			fail("data: year_letters[%d]: a capital letter is required", i)
			continue
		}
		if _, dup := d.letter[l]; dup {
			fail("data: year_letters[%d]: the letter %s is listed twice", i, l)
			continue
		}
		d.letter[l] = i
	}
	if len(d.letter) != 20 {
		fail("data: year_letters: the cycle has 20 letters, found %d", len(d.letter))
	}
	for i, m := range d.Months {
		if len(m.Char) != 1 || m.Month < 1 || m.Month > 12 || d.char[m.Month] != "" {
			fail("data: months[%d]: a character and a month 1 to 12 that is not listed yet are required", i)
			continue
		}
		if _, dup := d.month[m.Char]; dup {
			fail("data: months[%d]: the character %s is listed twice", i, m.Char)
			continue
		}
		if _, clash := d.letter[m.Char]; clash && m.Char != "N" && m.Char != "D" {
			fail("data: months[%d]: the character %s is also a year letter", i, m.Char)
		}
		d.month[m.Char] = m.Month
		d.char[m.Month] = m.Char
	}
	if len(d.month) != 12 {
		fail("data: months: 12 month characters are required, found %d", len(d.month))
	}
	codeCheckNotes(fail, d.Notes)
	return &d
}

type codeDateDoc struct {
	FirstYear   int             `json:"first_year"`
	LastYear    int             `json:"last_year"`
	YearLetters []string        `json:"year_letters"`
	Months      []codeDateMonth `json:"months"`
	Notes       []string        `json:"notes"`
}

func codeDateJSON(data any, lang string) any {
	d := data.(*codeDateData)
	return codeDateDoc{FirstYear: d.FirstYear, LastYear: d.LastYear, YearLetters: orEmpty(d.YearLetters), Months: orEmpty(d.Months), Notes: codeLocalizedNotes(d.Notes, lang)}
}

var (
	codeDateYYWWRe   = regexp.MustCompile(`^\d{4}$`)
	codeDateLetterRe = regexp.MustCompile(`^[A-Z][0-9A-Z]$`)
	codeDateISORe    = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
)

const codeDateUnusedLetters = "G, I, O, Q, Y and Z"

// codeDateWeeks is the number of ISO weeks of a year: 52 or 53 (28 December always lies in the last week).
func codeDateWeeks(year int) int {
	_, w := time.Date(year, time.December, 28, 0, 0, 0, 0, time.UTC).ISOWeek()
	return w
}

// codeDateMonday is the Monday of ISO week w of a year (week 01 holds 4 January).
func codeDateMonday(year, week int) time.Time {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	back := (int(jan4.Weekday()) + 6) % 7 // days since Monday
	return jan4.AddDate(0, 0, -back+7*(week-1))
}

func codeDateText(t time.Time) string { return t.Format("2006-01-02") }

func (r *CodeResult) weekFields(year, week int) {
	mon := codeDateMonday(year, week)
	r.field("year", "Year", fmt.Sprintf("%d", year), math.NaN(), "")
	r.field("week", "Week (ISO 8601)", fmt.Sprintf("%02d", week), math.NaN(), "")
	r.field("week_start", "Week starts (Monday)", codeDateText(mon), math.NaN(), "")
	r.field("week_end", "Week ends (Sunday)", codeDateText(mon.AddDate(0, 0, 6)), math.NaN(), "")
}

func (d *codeDateData) decode(u *Units, code string) (CodeResult, error) {
	s := strings.ToUpper(strings.Join(strings.Fields(code), ""))
	if s == "" {
		return CodeResult{}, codeErr(CodeInvalidCode, "the code is empty")
	}
	switch {
	case codeDateYYWWRe.MatchString(s):
		return d.decodeWeek(s)
	case codeDateLetterRe.MatchString(s):
		return d.decodeLetter(s)
	}
	return CodeResult{}, codeErr(CodeInvalidCode, "cannot read %q as a date code (four digits YYWW such as 2448, or a year letter and a month character such as J5)", s)
}

func (d *codeDateData) decodeWeek(s string) (CodeResult, error) {
	year := 2000 + int(s[0]-'0')*10 + int(s[1]-'0')
	week := int(s[2]-'0')*10 + int(s[3]-'0')
	switch weeks := codeDateWeeks(year); {
	case week == 0:
		return CodeResult{}, codeErr(CodeInvalidCode, "week 00 does not exist (the weeks run from 01 to 52 or 53)")
	case week > 53:
		return CodeResult{}, codeErr(CodeInvalidCode, "week %02d does not exist (the weeks run from 01 to 52 or 53)", week)
	case week > weeks:
		return CodeResult{}, codeErr(CodeInvalidCode, "%d has only %d weeks, so week %02d does not exist", year, weeks, week)
	}
	res := codeNewResult("date_code")
	res.Code = s
	res.weekFields(year, week)
	return res, nil
}

func (d *codeDateData) decodeLetter(s string) (CodeResult, error) {
	idx, ok := d.letter[s[:1]]
	if !ok {
		return CodeResult{}, codeErr(CodeInvalidCode, "%s is not a year letter (the letters %s are not used)", s[:1], codeDateUnusedLetters)
	}
	month, ok := d.month[s[1:]]
	if !ok {
		return CodeResult{}, codeErr(CodeInvalidCode, "%s is not a month character (1 to 9, O for October, N for November, D for December)", s[1:])
	}
	var years []string
	for y := d.FirstYear + idx; y <= d.LastYear; y += len(d.YearLetters) {
		years = append(years, fmt.Sprintf("%d", y))
	}
	res := codeNewResult("date_code")
	res.Code = s
	res.field("years", "Years", strings.Join(years, ", "), math.NaN(), "")
	res.field("month", "Month", time.Month(month).String(), math.NaN(), "")
	return res, nil
}

func (d *codeDateData) encode(u *Units, req EncodeRequest) (CodeResult, error) {
	text := strings.TrimSpace(req.Value)
	if text == "" {
		return CodeResult{}, codeErr(CodeInvalidValue, "give a date as YYYY-MM-DD")
	}
	m := codeDateISORe.FindStringSubmatch(text)
	if m == nil {
		return CodeResult{}, codeErr(CodeInvalidValue, "value: cannot read %q as a date (YYYY-MM-DD)", text)
	}
	var y, mo, da int
	fmt.Sscanf(m[1]+" "+m[2]+" "+m[3], "%d %d %d", &y, &mo, &da)
	t := time.Date(y, time.Month(mo), da, 0, 0, 0, 0, time.UTC)
	if y < 1583 || mo < 1 || mo > 12 || t.Year() != y || int(t.Month()) != mo || t.Day() != da {
		return CodeResult{}, codeErr(CodeInvalidValue, "value: %s is not a calendar date from the year 1583 on", text)
	}
	letter := d.YearLetters[((y-d.FirstYear)%20+20)%20] + d.char[mo]
	wy, ww := t.ISOWeek()
	res := codeNewResult("date_code")
	if wy >= 2000 && wy <= 2099 {
		res.Code = fmt.Sprintf("%02d%02d", wy%100, ww)
	} else {
		res.Code = letter
		res.warn("the ISO week-year %d is outside 2000 to 2099, so there is no YYWW code; the letter code is given", wy)
	}
	res.weekFields(wy, ww)
	res.field("letter_code", "IEC 60062 letter code", letter, math.NaN(), "")
	return res, nil
}
