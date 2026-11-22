package chibisensei

import "testing"

func TestDetectLanguageAndSuggest(t *testing.T) {
	langs := []struct{ text, want string }{
		{"Wie geht es dir? Ich bin müde und das ist nicht gut.", "de"},
		{"Ciao, come stai? Sono qui per te e non ho paura di niente, come sempre.", "it"},
		{"Привет, как дела у тебя сегодня утром", "ru"},
		{"これは日本語のテストです。よろしくお願いします", "ja"},
		{"abc", ""},
	}
	for _, l := range langs {
		if got := DetectLanguage([]string{l.text}); got != l.want {
			t.Errorf("DetectLanguage(%q) = %q, want %q", l.text, got, l.want)
		}
	}
	sugg := []struct {
		recipe        string
		rows, p95     int
		lang, id, key string
	}{
		{"specialist", 0, 0, "", "gemma3-270m", "chibi_sensei.suggest.specialist_short_inputs"},
		{"specialist", 300, 900, "en", "qwen35-0p8b", "chibi_sensei.suggest.default"},
		{"assistant", 0, 0, "it", "qwen35-0p8b", "chibi_sensei.suggest.assistant_other_language"},
		{"assistant", 0, 0, "de", "lfm25-1p2b", "chibi_sensei.suggest.assistant_lfm_language"},
	}
	for _, s := range sugg {
		got := SuggestProfile(s.recipe, s.rows, s.p95, s.lang)
		if got.ID != s.id || got.ReasonKey != s.key {
			t.Errorf("SuggestProfile(%s,%d,%d,%q) = %+v", s.recipe, s.rows, s.p95, s.lang, got)
		}
	}
}
