package server

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"aurago/internal/llm"
	"aurago/internal/newspaper"
)

var errNewspaperDeclined = errors.New("editorial decline")

// Stable codes are safe to persist and to use in a bounded regeneration prompt.
func newspaperEditFailure(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, errNewspaperDeclined):
		return "editorial_decline"
	case errors.Is(err, llm.ErrJSONCompletionEmpty):
		return "json_empty"
	case errors.Is(err, llm.ErrJSONCompletionTruncated):
		return "json_truncated"
	case errors.Is(err, llm.ErrJSONCompletionInvalid):
		return "json_invalid"
	default:
		return "model_error"
	}
}

func newspaperValidateStory(story *newspaper.Story, article newspaperArticle, p newspaper.Profile, number int) string {
	source := article.source
	story.ID, story.Section = fmt.Sprintf("story-%d", number), article.candidate.Query.Section
	story.SourceIDs, story.SingleSource = []string{source.ID}, true
	for i := range story.Paragraphs {
		para := &story.Paragraphs[i]
		para.SourceIDs = []string{source.ID}
		if n := len([]rune(para.EvidenceQuote)); n < 20 || n > 500 {
			return "quote_missing"
		}
		if !strings.Contains(source.Excerpt, para.EvidenceQuote) {
			return "quote_mismatch"
		}
	}
	if err := newspaper.ValidateDraft(newspaper.Draft{Stories: []newspaper.Story{*story}, Sources: []newspaper.Source{source}}, p, time.Now().UTC()); err != nil {
		return "invalid_structure"
	}
	return ""
}

func newspaperRepairable(reason string) bool {
	switch reason {
	case "json_empty", "json_invalid", "json_truncated", "invalid_structure", "quote_missing", "quote_mismatch":
		return true
	}
	return false
}
