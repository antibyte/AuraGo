package chibisensei

var lfmLanguages = map[string]bool{"en": true, "ar": true, "zh": true, "fr": true, "de": true, "ja": true, "ko": true, "es": true}

// SuggestProfile applies the Goal rule in order. All launch profiles are verified (proof runs of
// 2026-11-25), so the "prefer verified" clause has no effect: it would only matter for a profile that
// ships experimental.
func SuggestProfile(recipe string, rows, p95 int, lang string) Suggestion {
	switch {
	case recipe == RecipeSpecialist && (rows == 0 || p95 <= 512):
		return Suggestion{ID: "gemma3-270m", ReasonKey: "chibi_sensei.suggest.specialist_short_inputs"}
	case recipe == RecipeAssistant && lang != "" && !lfmLanguages[lang]:
		return Suggestion{ID: "qwen35-0p8b", ReasonKey: "chibi_sensei.suggest.assistant_other_language"}
	case recipe == RecipeAssistant && lfmLanguages[lang]:
		return Suggestion{ID: "lfm25-1p2b", ReasonKey: "chibi_sensei.suggest.assistant_lfm_language"}
	}
	return Suggestion{ID: "qwen35-0p8b", ReasonKey: "chibi_sensei.suggest.default"}
}
