package storymaker

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"strings"
	"unicode"
)

const (
	fallbackMaxWords = 8
	fallbackMaxToken = 40
)

// FallbackHeadline is the key sentence of a scene for the fallback board: the narration without markers (the title when
// that is empty) up to its first sentence end, at most 8 words, without a trailing comma or dash. A sentence end is
// . ! ? : ; or the CJK forms of them followed by white space or the end of the text; a final ? or ! stays. One token is cut to
// 40 characters, and the words are dropped from the end until the headline fits the 80 characters of a headline block.
// An empty result means there is nothing to show.
func FallbackHeadline(narration, title string) string {
	text := strings.TrimSpace(StripMarkers(narration))
	if text == "" {
		text = strings.TrimSpace(title)
	}
	text = strings.ReplaceAll(text, "*", "")
	runes := []rune(text)
	for i, r := range runes {
		if !strings.ContainsRune(".!?:;。！？", r) || (i+1 < len(runes) && !unicode.IsSpace(runes[i+1])) {
			continue
		}
		runes = runes[:i]
		if strings.ContainsRune("!?！？", r) {
			runes = append(runes, r)
		}
		break
	}
	words := strings.Fields(string(runes))
	if len(words) > fallbackMaxWords {
		words = words[:fallbackMaxWords]
	}
	for i, w := range words {
		if r := []rune(w); len(r) > fallbackMaxToken {
			words[i] = string(r[:fallbackMaxToken])
		}
	}
	trim := func() string { return strings.TrimRight(strings.Join(words, " "), ",–- ") }
	for len(words) > 1 && runeLen(trim()) > blockHeadlineMax {
		words = words[:len(words)-1]
	}
	return trim()
}

// ApplyFallbackBoard turns a scene that stayed empty into a board with one headline. A catalog scene gets a headline show
// beat on env.board; a free scene becomes a code scene on env.board and the source it needs is returned. The narration and
// the rest of the scene stay.
func ApplyFallbackBoard(sc *Scene, headline, mode string) (source string) {
	sc.Environment = BoardEnvironmentID
	sc.Actors, sc.Props = nil, nil
	if EffectiveMode(mode) == ModeFree {
		sc.Kind, sc.Beats = "code", nil
		quoted, _ := json.Marshal(headline)
		return "export default function scene(api) {\n  api.headline(" + string(quoted) + ", { at: 'full', entry: 'fade' });\n}\n"
	}
	sc.Kind = ""
	sc.Beats = []Beat{{Do: "show", Text: headline, Style: "headline", At: SpotAt("full"), Entry: "fade"}}
	return ""
}

// FallbackBoards gives each listed scene that is still empty a board with its key sentence, so the film has no blank
// frame, and returns the ids that got one. A free scene first saves its source (replacing a saved one that was never
// activated); a scene whose source does not save is skipped. The story is saved once. A read-only job changes nothing.
func (s *Studio) FallbackBoards(ctx context.Context, sceneIDs []string) ([]string, error) {
	if len(sceneIDs) == 0 || (s.deps.ReadOnly != nil && s.deps.ReadOnly()) {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, etag, err := s.loadStory(ctx)
	if err != nil {
		return nil, err
	}
	var done []string
	for _, id := range sceneIDs {
		sc := studioFindScene(st, id)
		if sc == nil || studioSceneFilled(sc) {
			continue
		}
		headline := FallbackHeadline(sc.Narration, st.Title)
		if headline == "" {
			continue
		}
		board := *sc
		source := ApplyFallbackBoard(&board, headline, s.job.Mode)
		if source != "" {
			match := MustNotExist
			if _, existing, err := s.deps.Projects.LoadCodeSource(ctx, s.job.ProjectID, id); err == nil {
				match = existing
			} else if !errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if _, res, err := s.deps.Projects.SaveCodeScene(ctx, s.job.ProjectID, id, []byte(source), match); err != nil || len(res.Errors) != 0 {
				continue
			}
		}
		*sc = board
		done = append(done, id)
	}
	if len(done) == 0 {
		return nil, nil
	}
	if _, err := s.saveStory(ctx, st, etag); err != nil {
		return nil, err
	}
	return done, nil
}
