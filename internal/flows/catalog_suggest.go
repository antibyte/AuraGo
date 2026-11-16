package flows

import "strings"

// curatedSuggestions is the short list of next steps the quick-add offers after a step of a
// category. The key is the category of the step the quick-add was opened for; every category of
// CategoryOrder that has steps with outputs has a list, and "tool" serves the generic tool
// categories ("tool:<name>"). A list holds at most maxSuggestions type ids, best first. Every id is a
// step of RegisterCatalog that is not a trigger and has an input; TestCuratedSuggestionsAreValid
// pins that. The list is a hint for the editor (NodeTypeInfo.Suggest): a type that is not available
// is still listed, the editor greys it out.
var curatedSuggestions = map[string][]string{
	"trigger":    {TypeAIStep, TypeWebSearch, TypeHTTPRequest, TypeTelegram, TypeIf},
	"ai":         {TypeIf, TypeTelegram, TypeEmail, TypePDFCreate, TypeSet},
	"web":        {TypeAIStep, TypeIf, TypeTelegram, TypePDFCreate, TypeSet},
	"documents":  {TypeAIStep, TypeEmail, TypeTelegram, TypeIf, TypePDFCreate},
	"notify":     {TypeIf, TypeWait, TypeTodoAdd, TypeHTTPRequest, TypeAIStep},
	"smart_home": {TypeIf, TypeTelegram, TypePush, TypeWait, TypeAIStep},
	"logic":      {TypeAIStep, TypeTelegram, TypeHTTPRequest, TypeSet, TypeWait},
	"planner":    {TypeTelegram, TypePush, TypeIf, TypeAIStep, TypeEmail},
	"tool":       {TypeAIStep, TypeIf, TypeTelegram, TypeSet, TypeHTTPRequest},
}

// maxSuggestions is the most entries a list may have; the editor shows at most five.
const maxSuggestions = 5

// suggestionCategory maps the category of a step to the key of curatedSuggestions.
func suggestionCategory(category string) string {
	if strings.HasPrefix(category, "tool:") {
		return "tool"
	}
	return category
}

// suggestionsFor returns the curated next steps after a step of type typ in category: the list of
// its category, without the step's own type and without a type that is not in present (a
// registry may hold fewer types than the full catalog, as the block registry does). A step
// without any output has no next step and gets nil.
func suggestionsFor(typ, category string, hasOutputs bool, present map[string]bool) []string {
	if !hasOutputs {
		return nil
	}
	var out []string
	for _, candidate := range curatedSuggestions[suggestionCategory(category)] {
		if candidate != typ && present[candidate] {
			out = append(out, candidate)
		}
	}
	return out
}
