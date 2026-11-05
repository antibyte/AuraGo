package storymakertest

import (
	"context"
	"io"
	"testing"

	"aurago/internal/storymaker"
)

func seedProject(t *testing.T) (*storymaker.ProjectStore, string) {
	t.Helper()
	p := storymaker.NewProjectStore(storymaker.NewMemStore(), "Documents/Storymaker")
	proj, _, _, err := p.Create(context.Background(), "Seed", "16:9", 30)
	if err != nil {
		t.Fatal(err)
	}
	return p, proj.ID
}

func seedStory(voice *storymaker.Voice) *storymaker.Story {
	return &storymaker.Story{
		Schema: storymaker.Schema, Title: "Seed",
		Format: storymaker.Format{Aspect: "16:9", FPS: 30},
		Voice:  voice,
		Scenes: []storymaker.Scene{{ID: "s1", Narration: "Das ist der erste ganze Satz hier. Das ist der zweite ganze Satz hier."}},
	}
}

func TestSeedNarrationFillsEveryPiece(t *testing.T) {
	ctx := context.Background()
	p, id := seedProject(t)
	story := seedStory(&storymaker.Voice{Provider: "Phonon", Voice: "thorsten", Language: "de"})

	before, err := p.AudioAnswer(ctx, id, story, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Missing) != 1 || before.Chars.Missing == 0 {
		t.Fatalf("before the seed: missing %+v, chars %+v", before.Missing, before.Chars)
	}

	if err := SeedNarration(ctx, p, id, story, 1); err != nil {
		t.Fatal(err)
	}
	answer, err := p.AudioAnswer(ctx, id, story, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Missing) != 0 || answer.Chars.Missing != 0 {
		t.Fatalf("after the seed: missing %+v, chars %+v", answer.Missing, answer.Chars)
	}
	blocks := answer.Narration["s1"].Blocks
	if len(blocks) != 1 || !blocks[0].SpeechOK {
		t.Fatalf("blocks = %+v", blocks)
	}
	for _, piece := range blocks[0].Pieces {
		// Speech starts at 100 ms and in_ms is start minus the 40 ms margin.
		if piece.InMS != 60 {
			t.Errorf("in_ms = %d, want 60", piece.InMS)
		}
		rc, _, err := p.OpenAudio(ctx, id, "narration", piece.Hash+"."+piece.Ext)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		decoded, ext, err := storymaker.DecodeAudio(data)
		if err != nil || ext != "wav" || decoded.Rate != 24000 || decoded.Channels != 1 {
			t.Fatalf("decoded rate %d channels %d ext %q err %v", decoded.Rate, decoded.Channels, ext, err)
		}
		env := storymaker.ComputeEnvelope(decoded)
		if env.DurationMS != 1000 {
			t.Fatalf("duration = %d, want 1000", env.DurationMS)
		}
		runs := env.SpeechRuns()
		// The last speech window covers 890 to 900 ms, and SpeechRuns may name either edge of it.
		if len(runs) != 1 || runs[0][0] != 100 || runs[0][1] < 890 || runs[0][1] > 900 {
			t.Fatalf("speech runs = %v, want one from 100 to the end minus 100", runs)
		}
		// -20 dBFS is (-20+60)*255/60 = 170. Window 50 lies at 500 ms, window 0 in the silent margin.
		if level := env.Levels[50]; level < 169 || level > 171 {
			t.Errorf("speech level = %d, want 170", level)
		}
		if env.Levels[0] != 0 {
			t.Errorf("margin level = %d, want 0", env.Levels[0])
		}
	}
}

func TestSeedNarrationSeedsAStoryWithoutAVoice(t *testing.T) {
	ctx := context.Background()
	p, id := seedProject(t)
	story := seedStory(nil)
	block := storymaker.Blocks(story)[0]
	if block.Voice.Provider != "" {
		t.Fatalf("voice = %+v, want the empty voice", block.Voice)
	}
	if err := SeedNarration(ctx, p, id, story, 1); err != nil {
		t.Fatal(err)
	}
	answer, err := p.AudioAnswer(ctx, id, story, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Missing) != 0 {
		t.Fatalf("missing = %+v", answer.Missing)
	}
	files, err := p.PieceFiles(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, piece := range storymaker.PiecesOf(block) {
		f, ok := files[storymaker.PieceKey(storymaker.Voice{}, piece.Text)]
		if !ok || f.Ext != "wav" || !f.HasEnv {
			t.Errorf("piece %q: file %+v, found %v", piece.Text, f, ok)
		}
	}
}

func TestSeedMusicLeavesAFileAndAValidMeta(t *testing.T) {
	ctx := context.Background()
	p, id := seedProject(t)
	key := storymaker.MusicKeyGenerated("calm-ambient", "")
	if err := SeedMusic(ctx, p, id, key, 2); err != nil {
		t.Fatal(err)
	}
	meta, audioExt, err := p.MusicInfo(ctx, id, key)
	if err != nil {
		t.Fatal(err)
	}
	if meta.V != 1 || meta.Ext != "wav" || meta.DurationMS != 2000 || audioExt != "wav" {
		t.Fatalf("meta = %+v, audio ext %q", meta, audioExt)
	}
	rc, _, err := p.OpenAudio(ctx, id, "music", key+".wav")
	if err != nil {
		t.Fatal(err)
	}
	_ = rc.Close()
}
