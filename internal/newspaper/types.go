package newspaper

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	ErrNotFound = errors.New("newspaper item not found")
	ErrConflict = errors.New("newspaper version conflict")
	ErrDisabled = errors.New("newspaper is disabled or read-only")
	ErrBusy     = errors.New("newspaper research is already running")
)

// SafeDeliveryError means the adapter knows no edition message was accepted.
// Other delivery errors remain uncertain and must not be replayed automatically.
type SafeDeliveryError struct{ Err error }

func (e *SafeDeliveryError) Error() string { return e.Err.Error() }
func (e *SafeDeliveryError) Unwrap() error { return e.Err }
func SafeDelivery(err error) error         { return &SafeDeliveryError{Err: err} }

var Sections = []string{"regional", "national", "international", "politics", "economy", "culture", "technology", "science", "environment", "health", "sport"}

var sectionSet = func() map[string]bool {
	m := make(map[string]bool, len(Sections))
	for _, section := range Sections {
		m[section] = true
	}
	return m
}()

var languageRE = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z]{2,8})?$`)
var countryRE = regexp.MustCompile(`^[A-Z]{2}$`)
var clockRE = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// Profile is the single installation-owned editorial and delivery preference.
// EmailVerified is server-owned and cannot be set through a profile update.
type Profile struct {
	Version        int64     `json:"version"`
	Name           string    `json:"name"`
	Sections       []string  `json:"sections"`
	Interests      []string  `json:"interests"`
	Exclusions     []string  `json:"exclusions"`
	RSSFeeds       []RSSFeed `json:"rss_feeds"`
	Language       string    `json:"language"`
	Country        string    `json:"country"`
	Region         string    `json:"region"`
	City           string    `json:"city"`
	TimeZone       string    `json:"time_zone"`
	ReadyTime      string    `json:"ready_time"`
	Length         string    `json:"length"`
	Daily          bool      `json:"daily"`
	EmailDaily     bool      `json:"email_daily"`
	EmailAccountID string    `json:"email_account_id"`
	EmailTo        string    `json:"email_to"`
	EmailVerified  bool      `json:"email_verified"`
	TelegramDaily  bool      `json:"telegram_daily"`
}

type RSSFeed struct {
	URL     string `json:"url"`
	Section string `json:"section"`
}

func DefaultProfile() Profile {
	return Profile{Name: "Newspaper", Sections: []string{"national", "international", "politics", "culture", "technology", "science"}, Language: "de", Country: "DE", TimeZone: "Europe/Berlin", ReadyTime: "07:00", Length: "standard"}
}

func (p *Profile) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = "Newspaper"
	}
	if len([]rune(p.Name)) > 70 || strings.ContainsAny(p.Name, "\r\n\x00") {
		return errors.New("publication name must be single-line text up to 70 characters")
	}
	if len(p.Sections) > len(Sections) || len(p.Interests) > 20 || len(p.Exclusions) > 20 {
		return errors.New("too many sections, interests or exclusions")
	}
	seen := map[string]bool{}
	for _, s := range p.Sections {
		if !sectionSet[s] || seen[s] {
			return fmt.Errorf("invalid or duplicate section %q", s)
		}
		seen[s] = true
	}
	for _, list := range [][]string{p.Interests, p.Exclusions} {
		seen = map[string]bool{}
		for i, value := range list {
			value = strings.TrimSpace(value)
			if value == "" || len([]rune(value)) > 120 || strings.ContainsAny(value, "\r\n\x00") || seen[strings.ToLower(value)] {
				return errors.New("interests and exclusions must be unique, single-line text up to 120 characters")
			}
			list[i] = value
			seen[strings.ToLower(value)] = true
		}
	}
	if len(p.Sections)+len(p.Interests) == 0 {
		return errors.New("choose a section or interest")
	}
	if len(p.RSSFeeds) > 10 {
		return errors.New("choose at most 10 RSS feeds")
	}
	feedURLs := map[string]bool{}
	for i := range p.RSSFeeds {
		feed := &p.RSSFeeds[i]
		feed.URL = strings.TrimSpace(feed.URL)
		u, err := url.Parse(feed.URL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.Fragment != "" || len(feed.URL) > 2048 || feedURLs[strings.ToLower(feed.URL)] {
			return errors.New("RSS feeds need unique public HTTP(S) URLs")
		}
		if !contains(p.Sections, feed.Section) && !(feed.Section == "interests" && len(p.Interests) > 0) {
			return errors.New("RSS feed section must be selected")
		}
		feedURLs[strings.ToLower(feed.URL)] = true
	}
	if contains(p.Sections, "regional") && strings.TrimSpace(p.Region+p.City) == "" {
		return errors.New("regional news needs a city or region")
	}
	for _, field := range []*string{&p.Country, &p.Region, &p.City} {
		*field = strings.TrimSpace(*field)
		if len([]rune(*field)) > 100 || strings.ContainsAny(*field, "\r\n\x00") {
			return errors.New("place must be single-line text up to 100 characters")
		}
	}
	p.Country = strings.ToUpper(p.Country)
	if p.Country != "" && !countryRE.MatchString(p.Country) {
		return errors.New("country must be a two-letter code")
	}
	if (contains(p.Sections, "regional") || contains(p.Sections, "national")) && p.Country == "" {
		return errors.New("regional and national news need a country code")
	}
	if !languageRE.MatchString(p.Language) {
		return errors.New("invalid publication language")
	}
	if _, err := time.LoadLocation(p.TimeZone); err != nil {
		return errors.New("invalid IANA time zone")
	}
	if !clockRE.MatchString(p.ReadyTime) {
		return errors.New("ready time must be HH:MM")
	}
	if p.Length != "brief" && p.Length != "standard" && p.Length != "in_depth" {
		return errors.New("invalid reading length")
	}
	if p.EmailTo != "" {
		a, err := mail.ParseAddress(p.EmailTo)
		if err != nil || a.Address != p.EmailTo || len(p.EmailTo) > 254 || strings.ContainsAny(p.EmailTo, "\r\n") {
			return errors.New("invalid email destination")
		}
	}
	return nil
}

func contains(values []string, wanted string) bool {
	for _, v := range values {
		if v == wanted {
			return true
		}
	}
	return false
}

type Source struct {
	ID          string     `json:"id"`
	URL         string     `json:"url"`
	Publisher   string     `json:"publisher"`
	Title       string     `json:"title"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	RetrievedAt time.Time  `json:"retrieved_at"`
	Excerpt     string     `json:"excerpt"`
}

type Paragraph struct {
	Text          string   `json:"text"`
	SourceIDs     []string `json:"source_ids"`
	EvidenceQuote string   `json:"evidence_quote"`
}

type Story struct {
	ID           string      `json:"id"`
	Section      string      `json:"section"`
	Headline     string      `json:"headline"`
	Deck         string      `json:"deck"`
	Paragraphs   []Paragraph `json:"paragraphs"`
	SourceIDs    []string    `json:"source_ids"`
	SingleSource bool        `json:"single_source"`
}

type Draft struct {
	Stories []Story  `json:"stories"`
	Sources []Source `json:"sources"`
	Partial bool     `json:"partial"`
}

type Edition struct {
	ID          string    `json:"id"`
	LocalDate   string    `json:"local_date"`
	Revision    int       `json:"revision"`
	Title       string    `json:"title"`
	Language    string    `json:"language"`
	Place       string    `json:"place"`
	CreatedAt   time.Time `json:"created_at"`
	CutoffAt    time.Time `json:"cutoff_at"`
	Hash        string    `json:"hash"`
	Partial     bool      `json:"partial"`
	Corrections []string  `json:"corrections,omitempty"`
	Stories     []Story   `json:"stories"`
	Sources     []Source  `json:"sources"`
}

type Run struct {
	ID             string    `json:"id"`
	LocalDate      string    `json:"local_date"`
	Revision       int       `json:"revision"`
	Status         string    `json:"status"`
	Phase          string    `json:"phase"`
	Sources        int       `json:"sources"`
	Stories        int       `json:"stories"`
	Reason         string    `json:"reason,omitempty"`
	CorrectionNote string    `json:"correction_note,omitempty"`
	StartedAt      time.Time `json:"started_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Event struct {
	ID    int64     `json:"id"`
	RunID string    `json:"run_id"`
	At    time.Time `json:"at"`
	Phase string    `json:"phase"`
	Text  string    `json:"text"`
}

type Delivery struct {
	ID         int64     `json:"id"`
	EditionID  string    `json:"edition_id"`
	Hash       string    `json:"hash"`
	Channel    string    `json:"channel"`
	Kind       string    `json:"kind"`
	TargetHash string    `json:"target_hash"`
	Key        string    `json:"key"`
	Status     string    `json:"status"`
	Reason     string    `json:"reason,omitempty"`
	ProviderID string    `json:"provider_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func ValidateDraft(d Draft, p Profile, cutoff time.Time) error {
	if len(d.Stories) == 0 || len(d.Stories) > 16 || len(d.Sources) == 0 || len(d.Sources) > 60 {
		return errors.New("edition requires 1-16 sourced stories and at most 60 sources")
	}
	sources := map[string]Source{}
	for _, s := range d.Sources {
		u, err := url.Parse(s.URL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || len(s.URL) > 2048 || s.ID == "" || len([]rune(s.Excerpt)) < 80 || len([]rune(s.Excerpt)) > 6000 || s.RetrievedAt.IsZero() || s.RetrievedAt.After(cutoff.Add(time.Second)) {
			return errors.New("source is incomplete, unsafe or outside the research cutoff")
		}
		if s.PublishedAt != nil && s.PublishedAt.After(cutoff.Add(time.Hour)) {
			return errors.New("source publication is after the research cutoff")
		}
		if _, exists := sources[s.ID]; exists {
			return errors.New("duplicate source ID")
		}
		sources[s.ID] = s
	}
	seen := map[string]bool{}
	for _, story := range d.Stories {
		if story.ID == "" || seen[story.ID] || (!sectionSet[story.Section] && story.Section != "interests") || (story.Section != "interests" && !contains(p.Sections, story.Section)) || story.Section == "interests" && len(p.Interests) == 0 || strings.TrimSpace(story.Headline) == "" || len([]rune(story.Headline)) > 180 || len([]rune(story.Deck)) > 350 || len(story.Paragraphs) == 0 || len(story.Paragraphs) > 12 {
			return errors.New("invalid story structure or section")
		}
		seen[story.ID] = true
		used := map[string]bool{}
		for _, para := range story.Paragraphs {
			if strings.TrimSpace(para.Text) == "" || len([]rune(para.Text)) > 1400 || len(para.SourceIDs) == 0 || len([]rune(para.EvidenceQuote)) < 20 || len([]rune(para.EvidenceQuote)) > 500 {
				return errors.New("every paragraph needs bounded text and a source")
			}
			matched := false
			for _, id := range para.SourceIDs {
				source, ok := sources[id]
				if !ok {
					return errors.New("paragraph cites an unread source")
				}
				if strings.Contains(source.Excerpt, para.EvidenceQuote) {
					matched = true
				}
				used[id] = true
			}
			if !matched {
				return errors.New("paragraph evidence quote was not found in a read source")
			}
		}
		if len(used) == 0 || (len(used) == 1 && !story.SingleSource) {
			return errors.New("single-source story must be labeled")
		}
		listed := map[string]bool{}
		for _, id := range story.SourceIDs {
			if !used[id] {
				return errors.New("story source list differs from paragraph references")
			}
			if listed[id] {
				return errors.New("duplicate story source ID")
			}
			listed[id] = true
		}
		if len(listed) != len(used) {
			return errors.New("story source list omits a paragraph reference")
		}
	}
	return nil
}

func (e *Edition) Seal() error {
	e.Hash = ""
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	h := sha256.Sum256(b)
	e.Hash = hex.EncodeToString(h[:])
	return nil
}
