package storymaker

import (
	"context"
	"reflect"
	"testing"
)

// A voice that changed after the speech leaves the file under a hash nobody looks for. The answer
// finds the recording by its words and by the voice the story file names, so the film keeps the
// voice it had. The exact answer still counts the block as missing: the narration job has to make
// it again, because there the voice of the story decides what is said.
func TestAudioAnswerFindsTheAudioOfAnotherVoice(t *testing.T) {
	ps, _, id := audioStoreProject(t)
	ctx := context.Background()
	story := audioStoreStory()
	old := Voice{Provider: "sanotts", Voice: "thorsten", Language: "de"}
	story.Voice = &old
	data, err := CanonicalJSON(*story)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.SaveStory(ctx, id, data, ""); err != nil {
		t.Fatal(err)
	}
	// The speech of the story: two pieces under the voice of that moment.
	block := Blocks(story)[0]
	for _, piece := range PiecesOf(block) {
		hash := PieceKey(old, piece.Text)
		env := audioStoreEnvelope()
		env.Text = piece.Text
		env.Language = old.Language
		if err := ps.WritePiece(ctx, id, hash, "wav", []byte("stored audio"), env); err != nil {
			t.Fatal(err)
		}
	}

	// The policy has since changed the voice of the story.
	effective := *story
	effective.Voice = &Voice{Provider: "phonon", Voice: "female-de", Language: "de"}

	answer, err := ps.AudioAnswer(ctx, id, &effective, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Missing) != 0 {
		t.Fatalf("the speech of another voice counts as missing: %+v", answer.Missing)
	}
	blocks := answer.Narration["s1"].Blocks
	if len(blocks) != 1 || len(blocks[0].Pieces) != 2 {
		t.Fatalf("the narration of the scene: %+v", answer.Narration["s1"])
	}
	for _, piece := range blocks[0].Pieces {
		if piece.Hash == "" || piece.Ext != "wav" {
			t.Fatalf("the piece of the answer: %+v", piece)
		}
	}
	chars := 0
	for _, piece := range PiecesOf(block) {
		chars += runeLen(piece.Text)
	}
	if want := []MissingBlock{{SceneID: "s1", BlockID: "s1:n", Chars: chars}}; !reflect.DeepEqual(answer.Revoiced, want) {
		t.Fatalf("revoiced = %+v, want %+v", answer.Revoiced, want)
	}

	// The narration job asks the exact answer: there the voice decides what has to be made.
	exact, err := ps.AudioAnswerExact(ctx, id, &effective, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(exact.Missing) != 1 || exact.Missing[0].BlockID != "s1:n" {
		t.Fatalf("the exact answer: %+v", exact.Missing)
	}
	if len(exact.Narration) != 0 {
		t.Fatalf("the exact answer has narration: %+v", exact.Narration)
	}
}

// The story file names the voice that spoke it. A piece is found under that voice, even when the
// envelope of an older piece does not name its text.
func TestAudioAnswerFindsTheVoiceOfTheStoryFile(t *testing.T) {
	ps, _, id := audioStoreProject(t)
	ctx := context.Background()
	story := audioStoreStory()
	old := Voice{Provider: "sanotts", Voice: "thorsten", Language: "de"}
	story.Voice = &old
	data, err := CanonicalJSON(*story)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.SaveStory(ctx, id, data, ""); err != nil {
		t.Fatal(err)
	}
	block := Blocks(story)[0]
	for _, piece := range PiecesOf(block) {
		hash := PieceKey(old, piece.Text)
		// The envelope of an older piece names no text.
		if err := ps.WritePiece(ctx, id, hash, "wav", []byte("stored audio"), audioStoreEnvelope()); err != nil {
			t.Fatal(err)
		}
	}

	// The voice of the story file is the one that speaks now; the policy did not change it.
	effective := *story
	effective.Voice = &Voice{Provider: "sanotts", Voice: "thorsten", Language: "de"}
	answer, err := ps.AudioAnswer(ctx, id, &effective, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Missing) != 0 || len(answer.Narration["s1"].Blocks) != 1 {
		t.Fatalf("the answer: missing %+v, narration %+v", answer.Missing, answer.Narration)
	}
	if len(answer.Revoiced) != 0 {
		t.Fatalf("the voice did not change: %+v", answer.Revoiced)
	}
}

// The words of a piece are found under any voice: the envelope names them, so a piece that was
// spoken with a voice the story no longer names is still heard.
func TestAudioAnswerFindsTheWordsOfAPiece(t *testing.T) {
	ps, _, id := audioStoreProject(t)
	ctx := context.Background()
	story := audioStoreStory()
	old := Voice{Provider: "supertonic", Voice: "amber", Language: "de"}
	story.Voice = &old
	block := Blocks(story)[0]
	for _, piece := range PiecesOf(block) {
		hash := PieceKey(old, piece.Text)
		env := audioStoreEnvelope()
		env.Text = piece.Text
		env.Language = old.Language
		if err := ps.WritePiece(ctx, id, hash, "mp3", []byte("stored audio"), env); err != nil {
			t.Fatal(err)
		}
	}

	// The story file names another voice, and the policy changed the voice of the story.
	effective := *story
	effective.Voice = &Voice{Provider: "phonon", Voice: "male-de", Language: "de"}
	answer, err := ps.AudioAnswer(ctx, id, &effective, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Missing) != 0 {
		t.Fatalf("the words of the piece were not found: %+v", answer.Missing)
	}
	blocks := answer.Narration["s1"].Blocks
	if len(blocks) != 1 || len(blocks[0].Pieces) != 2 {
		t.Fatalf("the narration of the scene: %+v", answer.Narration["s1"])
	}
	for _, piece := range blocks[0].Pieces {
		if piece.Ext != "mp3" {
			t.Fatalf("the piece of the answer: %+v", piece)
		}
	}
}

// A piece of another language is not the piece of this one.
func TestAudioAnswerKeepsTheLanguagesApart(t *testing.T) {
	ps, _, id := audioStoreProject(t)
	ctx := context.Background()
	story := audioStoreStory()
	old := Voice{Provider: "phonon", Voice: "female-de", Language: "de"}
	story.Voice = &old
	block := Blocks(story)[0]
	for _, piece := range PiecesOf(block) {
		hash := PieceKey(old, piece.Text)
		env := audioStoreEnvelope()
		env.Text = piece.Text
		env.Language = "de"
		if err := ps.WritePiece(ctx, id, hash, "wav", []byte("stored audio"), env); err != nil {
			t.Fatal(err)
		}
	}
	effective := *story
	effective.Voice = &Voice{Provider: "phonon", Voice: "female-en", Language: "en"}
	answer, err := ps.AudioAnswer(ctx, id, &effective, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Missing) != 1 {
		t.Fatalf("a German piece is the piece of an English story: %+v", answer.Missing)
	}
}
