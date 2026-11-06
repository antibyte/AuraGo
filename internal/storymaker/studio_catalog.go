package storymaker

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
)

const (
	catalogVoiceKind      = "voice"
	catalogMissNextAction = "Use the nearest alternative, or a show beat with text."
	catalogKindList       = "character, prop, environment, pose, gesture, expression, transition, text_style, sfx or voice"
)

// catalogModelKinds are the kinds a miss may suggest when the call named no kind.
var catalogModelKinds = nameSet("character", "prop", "environment", "pose", "gesture", "expression", "transition", "text_style", "sfx")

type catalogHit struct {
	ID           string          `json:"id"`
	Kind         string          `json:"kind"`
	Tags         []string        `json:"tags"`
	UsageExample json.RawMessage `json:"usage_example,omitempty"`
	Spots        []string        `json:"spots,omitempty"`
	Variants     []string        `json:"variants,omitempty"`
}

type catalogVoiceHit struct {
	Provider  string   `json:"provider"`
	Voice     string   `json:"voice"`
	Languages []string `json:"languages"`
	Gender    string   `json:"gender,omitempty"`
}

type catalogVoiceRow struct {
	hit  catalogVoiceHit
	rank int
	text string
}

func init() {
	registerStudioOp(ToolCatalog, "search", studioCatalogOp("search"))
	registerStudioOp(ToolCatalog, "describe", studioCatalogOp("describe"))
}

// studioCatalogOp returns the handler of one storymaker_catalog operation. Studio.Call has checked
// the phase and the read-only gate.
func studioCatalogOp(op string) studioOp {
	return func(ctx context.Context, s *Studio, args map[string]any) StudioResult {
		return s.runCatalog(ctx, op, args)
	}
}

// runCatalog answers a search or describe call.
func (s *Studio) runCatalog(ctx context.Context, op string, args map[string]any) StudioResult {
	query := catalogOpArg(args, "query")
	id := catalogOpArg(args, "id")
	rawKind := catalogOpArg(args, "kind")
	if strings.EqualFold(rawKind, catalogVoiceKind) {
		return s.catalogVoiceSearch(ctx, op, query, catalogOpLimit(args))
	}
	kind := NormalizeKind(rawKind)
	if rawKind != "" && kind == "" {
		return catalogOpRefused(op, "This kind does not exist.", "Use kind "+catalogKindList+".")
	}
	if op == "describe" {
		target := id
		if target == "" {
			target = query
		}
		return s.catalogDescribe(target, kind)
	}
	if query == "" {
		query = id
	}
	hits := s.deps.Catalog.Search(query, kind, catalogOpLimit(args))
	if len(hits) == 0 {
		return s.catalogMiss(op, query, kind)
	}
	results := make([]catalogHit, 0, len(hits))
	for _, e := range hits {
		results = append(results, s.catalogHitOf(e))
	}
	return catalogOpAnswer(op, "ok", map[string]any{"results": results})
}

func (s *Studio) catalogDescribe(target, kind string) StudioResult {
	const op = "describe"
	entry, found := s.deps.Catalog.Lookup(target)
	if !found && kind != "" {
		entry, found = s.deps.Catalog.Lookup(kindPrefixes[kind] + target)
	}
	if !found {
		return s.catalogMiss(op, target, kind)
	}
	return catalogOpAnswer(op, "ok", map[string]any{"entry": s.catalogHitOf(entry)})
}

func (s *Studio) catalogHitOf(e Entry) catalogHit {
	hit := catalogHit{ID: e.ID, Kind: e.Kind, Tags: e.Tags, UsageExample: e.UsageExample}
	if hit.Tags == nil {
		hit.Tags = []string{}
	}
	if e.Kind == "environment" {
		hit.Spots = s.deps.Catalog.SpotNames(e.ID)
		hit.Variants = s.deps.Catalog.Variants(e.ID)
	}
	return hit
}

func (s *Studio) catalogMiss(op, query, kind string) StudioResult {
	var pool []string
	if s.deps.Catalog != nil {
		for _, e := range s.deps.Catalog.Entries() {
			if e.Kind == kind || (kind == "" && catalogModelKinds[e.Kind]) {
				pool = append(pool, e.ID)
			}
		}
	}
	return catalogOpAnswer(op, "ok", map[string]any{
		"results": []catalogHit{}, "alternatives": closestNames(query, pool, 5), "next_action": catalogMissNextAction,
	})
}

// catalogVoiceSearch lists ready voices: the active provider first, inside that voices of the job
// language first. Query terms filter on provider, voice, name, gender and language.
func (s *Studio) catalogVoiceSearch(ctx context.Context, op, query string, limit int) StudioResult {
	var providers []VoiceProvider
	if s.deps.Voices != nil {
		providers = s.deps.Voices(ctx)
	}
	base := LanguageBase(s.job.Language)
	var rows []catalogVoiceRow
	add := func(p *VoiceProvider, voice, name, gender string, languages []string) {
		rank := 2
		if p.Active {
			rank = 0
		}
		if !catalogSpeaks(languages, base) {
			rank++
		}
		id := NormalizeProvider(p.ID)
		text := strings.ToLower(strings.Join(append([]string{id, voice, name, gender}, languages...), " "))
		rows = append(rows, catalogVoiceRow{
			hit:  catalogVoiceHit{Provider: id, Voice: voice, Languages: append([]string{}, languages...), Gender: gender},
			rank: rank, text: text,
		})
	}
	for i := range providers {
		p := &providers[i]
		if !p.Ready || (!p.FreeText && len(p.Voices) == 0) {
			continue
		}
		if len(p.Voices) == 0 {
			add(p, p.DefaultVoice, "", "", p.Languages)
			continue
		}
		for _, o := range p.Voices {
			if o.Installed != nil && !*o.Installed {
				continue
			}
			languages := p.Languages
			if o.Language != "" {
				languages = []string{o.Language}
			}
			add(p, o.ID, o.Name, o.Gender, languages)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].rank < rows[j].rank })
	terms := strings.Fields(strings.ToLower(query))
	results := []catalogVoiceHit{}
	for _, row := range rows {
		if len(results) == limit {
			break
		}
		matches := true
		for _, term := range terms {
			matches = matches && strings.Contains(row.text, term)
		}
		if matches {
			results = append(results, row.hit)
		}
	}
	body := map[string]any{"results": results}
	if len(results) == 0 {
		body["next_action"] = "No voice fits. Leave the voice as it is."
	}
	return catalogOpAnswer(op, "ok", body)
}

func catalogSpeaks(languages []string, base string) bool {
	if base == "" {
		return true
	}
	for _, language := range languages {
		if language == "*" || LanguageBase(language) == base {
			return true
		}
	}
	return false
}

func catalogOpArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func catalogOpLimit(args map[string]any) int {
	limit := 0
	switch v := args["limit"].(type) {
	case float64:
		limit = int(v)
	case int:
		limit = v
	}
	if limit <= 0 {
		return defaultSearchLimit
	}
	return min(limit, maxSearchLimit)
}

func catalogOpAnswer(op, status string, body map[string]any) StudioResult {
	body["status"] = status
	data, err := json.Marshal(body)
	if err != nil {
		status = "refused"
		data = []byte(`{"status":"refused","code":"invalid_data","message":"The answer could not be encoded."}`)
	}
	return StudioResult{JSON: data, Status: status, Operation: op}
}

func catalogOpRefused(op, message, next string) StudioResult {
	return catalogOpAnswer(op, "refused", map[string]any{"code": "invalid_data", "message": message, "next_action": next})
}
