package personabooster

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// GenericPersona names the fallback catalog used by custom personas.
const GenericPersona = "_generic"

// Catalog limits from the design.
const (
	MinPhases         = 2
	MaxPhases         = 6
	MinVariants       = 3
	MaxVariantChars   = 700
	MaxAmplifierChars = 600
	maxNameChars      = 60
	maxPhaseMinutes   = 1440
	maxPhaseTurns     = 200
	maxTriggerWeight  = 10
)

// Catalog is one persona's phase file (prompts/persona_phases/<persona>.yaml).
type Catalog struct {
	Persona            string  `yaml:"persona"`
	Amplifier          string  `yaml:"amplifier"`
	SpontaneousPerHour float64 `yaml:"spontaneous_per_hour"`
	Phases             []Phase `yaml:"phases"`
}

// Phase is a temporary mood of the persona with interchangeable variants.
type Phase struct {
	ID       string             `yaml:"id"`
	Name     string             `yaml:"name"`
	Hostile  bool               `yaml:"hostile"`
	Minutes  []int              `yaml:"minutes"`
	Turns    []int              `yaml:"turns"`
	Triggers map[string]float64 `yaml:"triggers"`
	EndsOn   []string           `yaml:"ends_on"`
	Requires *Requirement       `yaml:"requires"`
	Variants []Variant          `yaml:"variants"`
}

// Variant is one direction a phase can take; its text goes into the prompt.
type Variant struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
	Text string `yaml:"text"`
}

// Requirement limits a generic phase to fitting profiles. Configured
// conditions are alternatives: one met condition is enough.
type Requirement struct {
	MinVolatility    float64 `yaml:"min_volatility"`
	MinEmpathyBias   float64 `yaml:"min_empathy_bias"`
	ConflictResponse string  `yaml:"conflict_response"`
}

// ProfileMeta is the part of a persona profile's meta block that the generic
// catalog reads.
type ProfileMeta struct {
	Volatility       float64
	EmpathyBias      float64
	ConflictResponse string
}

// ParseCatalog decodes and validates one catalog file. Unknown keys fail.
func ParseCatalog(data []byte) (*Catalog, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var cat Catalog
	if err := decoder.Decode(&cat); err != nil {
		return nil, fmt.Errorf("parse persona phase catalog: %w", err)
	}
	if err := cat.Validate(); err != nil {
		return nil, err
	}
	return &cat, nil
}

// Validate checks names, triggers, counts, lengths and duration ranges.
func (c *Catalog) Validate() error {
	if c == nil {
		return errors.New("persona phase catalog is nil")
	}
	if c.Persona != GenericPersona && !validPersonaID(c.Persona) {
		return fmt.Errorf("invalid persona %q", c.Persona)
	}
	if err := checkProse("amplifier", c.Amplifier, MaxAmplifierChars); err != nil {
		return err
	}
	if math.IsNaN(c.SpontaneousPerHour) || c.SpontaneousPerHour < 0 || c.SpontaneousPerHour > 1 {
		return fmt.Errorf("spontaneous_per_hour %v outside 0..1", c.SpontaneousPerHour)
	}
	if n := len(c.Phases); n < MinPhases || n > MaxPhases {
		return fmt.Errorf("catalog has %d phases, want %d-%d", n, MinPhases, MaxPhases)
	}
	seen := make(map[string]bool, len(c.Phases))
	for _, phase := range c.Phases {
		if err := phase.validate(c.Persona == GenericPersona); err != nil {
			return fmt.Errorf("phase %q: %w", phase.ID, err)
		}
		if seen[phase.ID] {
			return fmt.Errorf("duplicate phase %q", phase.ID)
		}
		seen[phase.ID] = true
	}
	return nil
}

func (p Phase) validate(generic bool) error {
	if !validSlug(p.ID) {
		return errors.New("id must use a-z, 0-9 and _")
	}
	if err := checkName(p.Name); err != nil {
		return err
	}
	if err := checkRange("minutes", p.Minutes, maxPhaseMinutes); err != nil {
		return err
	}
	if err := checkRange("turns", p.Turns, maxPhaseTurns); err != nil {
		return err
	}
	if len(p.Triggers) == 0 {
		return errors.New("no triggers")
	}
	for _, name := range sortedTriggerNames(p.Triggers) {
		if !KnownTrigger(name) {
			return fmt.Errorf("unknown trigger %q", name)
		}
		if weight := p.Triggers[name]; math.IsNaN(weight) || weight <= 0 || weight > maxTriggerWeight {
			return fmt.Errorf("trigger %q weight %v outside (0, %d]", name, weight, maxTriggerWeight)
		}
	}
	for _, name := range p.EndsOn {
		if name == TriggerSpontaneous || !KnownTrigger(name) {
			return fmt.Errorf("unknown ends_on trigger %q", name)
		}
	}
	if p.Requires != nil && !generic {
		return errors.New("requires is only allowed in _generic.yaml")
	}
	if len(p.Variants) < MinVariants {
		return fmt.Errorf("%d variants, want at least %d", len(p.Variants), MinVariants)
	}
	seen := make(map[string]bool, len(p.Variants))
	for _, variant := range p.Variants {
		if !validSlug(variant.ID) {
			return fmt.Errorf("variant id %q must use a-z, 0-9 and _", variant.ID)
		}
		if seen[variant.ID] {
			return fmt.Errorf("duplicate variant %q", variant.ID)
		}
		seen[variant.ID] = true
		if err := checkName(variant.Name); err != nil {
			return fmt.Errorf("variant %q: %w", variant.ID, err)
		}
		if err := checkProse("variant "+variant.ID, variant.Text, MaxVariantChars); err != nil {
			return err
		}
	}
	return nil
}

// EligiblePhases returns the phases whose requirement the profile meets.
func (c *Catalog) EligiblePhases(meta ProfileMeta) []Phase {
	out := make([]Phase, 0, len(c.Phases))
	for _, phase := range c.Phases {
		if phase.Requires.Met(meta) {
			out = append(out, phase)
		}
	}
	return out
}

// Met reports whether the profile qualifies. A nil or empty requirement
// always holds.
func (r *Requirement) Met(meta ProfileMeta) bool {
	if r == nil {
		return true
	}
	configured := false
	if r.MinVolatility > 0 {
		configured = true
		if meta.Volatility >= r.MinVolatility {
			return true
		}
	}
	if r.MinEmpathyBias > 0 {
		configured = true
		if meta.EmpathyBias >= r.MinEmpathyBias {
			return true
		}
	}
	if r.ConflictResponse != "" {
		configured = true
		if strings.EqualFold(meta.ConflictResponse, r.ConflictResponse) {
			return true
		}
	}
	return !configured
}

// FindPhase looks up a phase by ID.
func (c *Catalog) FindPhase(id string) (Phase, bool) {
	if c == nil {
		return Phase{}, false
	}
	for _, phase := range c.Phases {
		if phase.ID == id {
			return phase, true
		}
	}
	return Phase{}, false
}

// FindVariant looks up a variant by ID.
func (p Phase) FindVariant(id string) (Variant, bool) {
	for _, variant := range p.Variants {
		if variant.ID == id {
			return variant, true
		}
	}
	return Variant{}, false
}

func checkName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxNameChars || strings.ContainsAny(name, "\r\n") {
		return fmt.Errorf("name must be 1-%d characters on one line", maxNameChars)
	}
	return nil
}

func checkProse(label, text string, maxChars int) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("%s is empty", label)
	}
	if n := utf8.RuneCountInString(text); n > maxChars {
		return fmt.Errorf("%s has %d characters, limit %d", label, n, maxChars)
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			return fmt.Errorf("%s contains a heading line", label)
		}
	}
	return nil
}

func checkRange(label string, bounds []int, limit int) error {
	if len(bounds) != 2 || bounds[0] < 1 || bounds[0] > bounds[1] || bounds[1] > limit {
		return fmt.Errorf("%s must be [min, max] with 1 <= min <= max <= %d", label, limit)
	}
	return nil
}

func validSlug(id string) bool {
	if id == "" || len(id) > 40 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return true
}

func validPersonaID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func sortedTriggerNames(triggers map[string]float64) []string {
	names := make([]string, 0, len(triggers))
	for name := range triggers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
