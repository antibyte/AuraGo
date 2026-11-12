package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aurago/internal/flows"
)

const (
	blockTriggers = "flows-triggers"
	blockSteps    = "flows-steps"
)

// target is one document with the generated blocks it carries.
type target struct {
	path   string // slash-separated, relative to the repository root
	lang   string // language of the labels and of the generator's own words
	agent  bool   // the agent manual: a list with type ids instead of tables
	blocks []string
}

// targets[0] is the English chapter; the test relies on it.
var targets = []target{
	{path: "documentation/manuals/en/24-easydrag.md", lang: "en", blocks: []string{blockTriggers, blockSteps}},
	{path: "documentation/manuals/de/24-easydrag.md", lang: "de", blocks: []string{blockTriggers, blockSteps}},
	{path: "prompts/tools_manuals/manage_missions.md", lang: "en", agent: true, blocks: []string{blockTriggers}},
}

type words struct {
	headers [5]string
	none    string
	sim     map[flows.SimulationBehavior]string
}

var wordsByLang = map[string]words{
	"en": {
		headers: [5]string{"Name", "Category", "What it does", "Effects (default settings)", "In Simulation"},
		none:    "None",
		sim: map[flows.SimulationBehavior]string{
			flows.SimulationRunsLocally:  "Runs locally",
			flows.SimulationNeedsSample:  "Replaced by your sample output",
			flows.SimulationUnsupported:  "Cannot be simulated",
			flows.SimulationSuppliesData: "Supplies the test data",
		},
	},
	"de": {
		headers: [5]string{"Name", "Kategorie", "Was er tut", "Wirkungen (Standardeinstellungen)", "In der Simulation"},
		none:    "Keine",
		sim: map[flows.SimulationBehavior]string{
			flows.SimulationRunsLocally:  "Läuft lokal",
			flows.SimulationNeedsSample:  "Wird durch deine Beispielausgabe ersetzt",
			flows.SimulationUnsupported:  "Lässt sich nicht simulieren",
			flows.SimulationSuppliesData: "Liefert die Testdaten",
		},
	},
}

// docEnv is the catalog environment of the manuals: every tool is available, so no node
// type is hidden behind a missing setup.
type docEnv struct{}

func (docEnv) ToolAvailability(string) flows.Availability {
	return flows.Availability{State: flows.AvailableState}
}

// source is the catalog and the locale texts the blocks are rendered from.
type source struct {
	reg     *flows.Registry
	locales map[string]map[string]string
}

func loadSource(root string) (*source, error) {
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, docEnv{}); err != nil {
		return nil, fmt.Errorf("register the catalog: %w", err)
	}
	src := &source{reg: reg, locales: map[string]map[string]string{}}
	for lang := range wordsByLang {
		raw, err := os.ReadFile(filepath.Join(root, "ui", "lang", "easydrag", lang+".json"))
		if err != nil {
			return nil, err
		}
		var texts map[string]string
		if err := json.Unmarshal(raw, &texts); err != nil {
			return nil, fmt.Errorf("ui/lang/easydrag/%s.json: %w", lang, err)
		}
		src.locales[lang] = texts
	}
	return src, nil
}

// apply returns doc with every block of tg rewritten. A missing marker pair is an error.
func (s *source) apply(tg target, doc string) (string, error) {
	if strings.Contains(doc, "\r") {
		return "", fmt.Errorf("%s has CR line ends; the generator writes LF only", tg.path)
	}
	for _, name := range tg.blocks {
		body, err := s.renderBlock(tg, name)
		if err != nil {
			return "", fmt.Errorf("%s: %w", tg.path, err)
		}
		if doc, err = replaceBlock(doc, name, body); err != nil {
			return "", fmt.Errorf("%s: %w", tg.path, err)
		}
	}
	return doc, nil
}

// replaceBlock replaces the text between the two marker lines of name. body ends with a
// line feed, so the end marker stays on its own line.
func replaceBlock(doc, name, body string) (string, error) {
	begin := "<!-- BEGIN GENERATED: " + name + " -->"
	end := "<!-- END GENERATED: " + name + " -->"
	b, e := strings.Index(doc, begin), strings.Index(doc, end)
	if b < 0 || e < 0 || e < b || strings.Count(doc, begin) != 1 || strings.Count(doc, end) != 1 {
		return "", fmt.Errorf("block %q needs exactly one line %s followed by one line %s", name, begin, end)
	}
	return doc[:b+len(begin)] + "\n" + body + doc[e:], nil
}

func (s *source) renderBlock(tg target, name string) (string, error) {
	switch {
	case tg.agent && name == blockTriggers:
		return s.renderAgentList(tg.lang)
	case name == blockTriggers || name == blockSteps:
		return s.renderTable(tg.lang, name == blockTriggers)
	}
	return "", fmt.Errorf("unknown block %q", name)
}

// describe returns the catalog in the language, sorted by palette category, label and type.
// A type without a label or a description is an error: the manuals never show a key.
func (s *source) describe(lang string) ([]flows.NodeTypeInfo, error) {
	texts := s.locales[lang]
	infos := flows.DescribeNodeTypes(s.reg, func(key string) string { return texts[key] })
	for _, info := range infos {
		if info.Label == info.Type || info.Description == "" ||
			strings.HasPrefix(info.Label, "easydrag.") || strings.HasPrefix(info.Description, "easydrag.") {
			return nil, fmt.Errorf("%s has no label or no description in ui/lang/easydrag/%s.json", info.Type, lang)
		}
	}
	sort.SliceStable(infos, func(i, j int) bool {
		a, b := infos[i], infos[j]
		if ra, rb := categoryRank(a.Category), categoryRank(b.Category); ra != rb {
			return ra < rb
		}
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		if la, lb := strings.ToLower(a.Label), strings.ToLower(b.Label); la != lb {
			return la < lb
		}
		return a.Type < b.Type
	})
	return infos, nil
}

func categoryRank(category string) int {
	for i, c := range flows.CategoryOrder {
		if c == category {
			return i
		}
	}
	return len(flows.CategoryOrder)
}

func (s *source) renderTable(lang string, triggers bool) (string, error) {
	infos, err := s.describe(lang)
	if err != nil {
		return "", err
	}
	texts, w := s.locales[lang], wordsByLang[lang]
	var b strings.Builder
	b.WriteString("| " + strings.Join(w.headers[:], " | ") + " |\n|---|---|---|---|---|\n")
	for _, info := range infos {
		if info.Trigger != triggers {
			continue
		}
		category, err := word(texts, lang, "easydrag.category."+info.Category)
		if err != nil {
			return "", err
		}
		effects := []string{}
		for _, e := range info.Effects {
			text, err := word(texts, lang, "easydrag.ui.effect_"+string(e))
			if err != nil {
				return "", err
			}
			effects = append(effects, text)
		}
		effectText := w.none
		if len(effects) > 0 {
			effectText = strings.Join(effects, ", ")
		}
		simText := w.sim[flows.SimulationBehaviorOf(info.Type, info.Trigger)]
		b.WriteString("| " + strings.Join([]string{cell(info.Label), cell(category), cell(info.Description), cell(effectText), cell(simText)}, " | ") + " |\n")
	}
	return b.String(), nil
}

// renderAgentList is the trigger list of the agent manual: the type id first, because the
// agent names types.
func (s *source) renderAgentList(lang string) (string, error) {
	infos, err := s.describe(lang)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, info := range infos {
		if info.Trigger {
			fmt.Fprintf(&b, "- `%s` (%s): %s\n", info.Type, oneLine(info.Label), oneLine(info.Description))
		}
	}
	return b.String(), nil
}

func word(texts map[string]string, lang, key string) (string, error) {
	if texts[key] == "" {
		return "", fmt.Errorf("ui/lang/easydrag/%s.json has no key %s", lang, key)
	}
	return texts[key], nil
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// cell is oneLine for a table cell: a pipe would end it.
func cell(s string) string { return strings.ReplaceAll(oneLine(s), "|", "\\|") }
