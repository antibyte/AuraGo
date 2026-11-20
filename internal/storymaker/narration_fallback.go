package storymaker

import (
	"context"
	"encoding/json"
)

// The voice fallback of the audio answer. A piece is stored under the hash of its voice, so a voice
// that changed after the speech leaves the file unreachable by that name. The words of a piece are
// stored with it, and the story file still names the voice it was spoken with, so the recording of
// a film survives a change of the voice policy.

// pieceTextIndex finds a stored piece by the words it speaks. The envelopes of the project are read
// once, on the first lookup.
type pieceTextIndex struct {
	p      *ProjectStore
	ctx    context.Context
	id     string
	files  map[string]PieceFile
	byText map[string]string
	loaded bool
}

// lookup returns the hash of the stored piece that speaks the text in the language.
func (x *pieceTextIndex) lookup(text, language string) (string, bool) {
	if x == nil {
		return "", false
	}
	if !x.loaded {
		x.loaded = true
		x.byText = map[string]string{}
		for hash, f := range x.files {
			if !f.HasEnv {
				continue
			}
			data, _, err := x.p.st.ReadFile(x.ctx, x.p.file(x.id, "audio", "narration", hash+".env.json"))
			if err != nil {
				continue
			}
			var env struct {
				Text     string `json:"text"`
				Language string `json:"language"`
			}
			if json.Unmarshal(data, &env) != nil || env.Text == "" {
				continue
			}
			key := pieceTextKey(env.Text, env.Language)
			if _, ok := x.byText[key]; !ok {
				x.byText[key] = hash
			}
		}
	}
	hash, ok := x.byText[pieceTextKey(text, language)]
	return hash, ok
}

// pieceTextKey names a piece by its words and its language. The language is compared by its base,
// so "de" and "de-DE" are the same voice.
func pieceTextKey(text, language string) string {
	return LanguageBase(language) + "\x00" + text
}

// storedBlockVoices returns the voice of every block as the story file names it. The voice policy
// may have changed a voice since the speech was made; the file still holds the voice that spoke it.
func (p *ProjectStore) storedBlockVoices(ctx context.Context, id string) map[string]Voice {
	out := map[string]Voice{}
	data, _, err := p.LoadStory(ctx, id)
	if err != nil {
		return out
	}
	raw, _, err := DecodeStory(data, DefaultLimits())
	if err != nil {
		return out
	}
	for _, b := range Blocks(&raw) {
		out[b.ID] = b.Voice
	}
	return out
}

// storedBlockWithVoice returns the audio of a block that was spoken with another voice. The pieces
// are split again with the chunk limit of that provider, so their hashes are the ones of that voice.
// A voice without a provider has no files of its own.
func (p *ProjectStore) storedBlockWithVoice(ctx context.Context, id string, b NarrationBlock, v Voice, files map[string]PieceFile, index *pieceTextIndex) ([]AudioPieceInput, bool, error) {
	if v.Provider == "" || v == b.Voice {
		return nil, false, nil
	}
	inputs := make([]AudioPieceInput, 0, 4)
	for _, piece := range SplitPieces(b.Text, ChunkLimit(v.Provider)) {
		in, ok, err := p.storedPiece(ctx, id, v, piece, files, index)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, nil
		}
		inputs = append(inputs, in)
	}
	return inputs, len(inputs) > 0, nil
}
