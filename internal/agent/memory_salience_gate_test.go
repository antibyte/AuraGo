package agent

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type fakeSalienceEmbedder struct {
	messages map[string][]float32
	err      error
	calls    int
}

func (f *fakeSalienceEmbedder) EmbedQueries(_ context.Context, texts []string) ([][]float32, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	vectors := make([][]float32, 0, len(texts))
	for _, text := range texts {
		switch {
		case f.messages[text] != nil:
			vectors = append(vectors, f.messages[text])
		case slices.Contains(salientMemoryPrototypes, text):
			vectors = append(vectors, []float32{1, 0})
		default:
			vectors = append(vectors, []float32{0, 1})
		}
	}
	return vectors, nil
}

func (f *fakeSalienceEmbedder) EmbeddingFingerprint() string { return "fake-salience" }

func TestMemoryExtractionWorthwhile(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name       string
		msg        string
		vector     []float32
		err        error
		noEmbedder bool
		want       bool
		reason     string
	}{
		{name: "filler words skip without embedding", msg: "Super, vielen Dank dafür!", want: false, reason: "filler_words"},
		{name: "close to salient", msg: "Mein NAS heißt atlas und läuft mit TrueNAS.", vector: []float32{1, 0}, want: true},
		{name: "close to filler", msg: "Zeig mir bitte die Logs vom Container.", vector: []float32{0, 1}, want: false, reason: "filler_prototype"},
		{name: "borderline stays", msg: "Kannst du das kurz prüfen und mir sagen?", vector: []float32{0.7, 0.71}, want: true},
		{name: "embedder failure fails open", msg: "Zeig mir bitte die Logs vom Container.", err: errors.New("down"), want: true},
		{name: "no embedder keeps", msg: "Zeig mir bitte die Logs vom Container.", noEmbedder: true, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeSalienceEmbedder{messages: map[string][]float32{test.msg: test.vector}, err: test.err}
			var embedder memoryQueryEmbedder = fake
			if test.noEmbedder {
				embedder = nil
			}
			got, reason := memoryExtractionWorthwhile(ctx, embedder, test.msg)
			if got != test.want || reason != test.reason {
				t.Fatalf("got %v %q, want %v %q", got, reason, test.want, test.reason)
			}
			if test.reason == "filler_words" && fake.calls != 0 {
				t.Fatal("filler words paid for an embedding")
			}
		})
	}
}

func TestOnlySalienceFillerWords(t *testing.T) {
	for msg, want := range map[string]bool{
		"Super, vielen Dank dafür!":      true,
		"ok danke 👍":                     true,
		"Thanks a lot, perfect.":         true,
		"Ja, mach das morgen":            false,
		"Danke, mein Server heißt atlas": false,
		"":                               false,
	} {
		if got := onlySalienceFillerWords(msg); got != want {
			t.Errorf("onlySalienceFillerWords(%q) = %v, want %v", msg, got, want)
		}
	}
}
