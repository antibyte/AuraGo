package agent

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

type fakeSalienceEmbedder struct {
	messages map[string][]float32
	err      error
	calls    atomic.Int32
}

func (f *fakeSalienceEmbedder) EmbedQueries(_ context.Context, texts []string) ([][]float32, error) {
	f.calls.Add(1)
	return f.embed(texts)
}

func (f *fakeSalienceEmbedder) EmbedTexts(_ context.Context, texts []string) ([][]float32, error) {
	return f.embed(texts)
}

func (f *fakeSalienceEmbedder) embed(texts []string) ([][]float32, error) {
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

// resetSalienceCacheForTest waits for a running warm-up, then empties the cache.
func resetSalienceCacheForTest(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		salienceCache.mu.Lock()
		if !salienceCache.warming {
			salienceCache.fingerprint, salienceCache.salient, salienceCache.filler = "", nil, nil
			salienceCache.mu.Unlock()
			return
		}
		salienceCache.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("salience warm-up did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

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
			resetSalienceCacheForTest(t)
			t.Cleanup(func() { resetSalienceCacheForTest(t) })
			if embedder != nil {
				_ = warmSaliencePrototypes(ctx, embedder)
			}
			got, reason := memoryExtractionWorthwhile(ctx, embedder, test.msg)
			if got != test.want || reason != test.reason {
				t.Fatalf("got %v %q, want %v %q", got, reason, test.want, test.reason)
			}
			if test.reason == "filler_words" && fake.calls.Load() != 0 {
				t.Fatal("filler words paid for an embedding")
			}
		})
	}
}

func TestMemoryExtractionWorthwhileColdCacheFailsOpen(t *testing.T) {
	resetSalienceCacheForTest(t)
	t.Cleanup(func() { resetSalienceCacheForTest(t) })
	msg := "Zeig mir bitte die Logs vom Container."
	fake := &fakeSalienceEmbedder{messages: map[string][]float32{msg: {0, 1}}}
	start := time.Now()
	got, reason := memoryExtractionWorthwhile(context.Background(), fake, msg)
	if !got || reason != "" {
		t.Fatalf("cold cache got %v %q, want fail open", got, reason)
	}
	if time.Since(start) > time.Second {
		t.Fatal("cold cache blocked the turn")
	}
	resetSalienceCacheForTest(t) // lets the background warm-up finish
	if err := warmSaliencePrototypes(context.Background(), fake); err != nil {
		t.Fatal(err)
	}
	if got, reason := memoryExtractionWorthwhile(context.Background(), fake, msg); got || reason != "filler_prototype" {
		t.Fatalf("warm cache got %v %q, want filler skip", got, reason)
	}
}

func TestMemoryExtractionWorthwhileDimensionMismatchFailsOpen(t *testing.T) {
	resetSalienceCacheForTest(t)
	t.Cleanup(func() { resetSalienceCacheForTest(t) })
	msg := "Zeig mir bitte die Logs vom Container."
	fake := &fakeSalienceEmbedder{messages: map[string][]float32{msg: {0, 1, 0}}}
	if err := warmSaliencePrototypes(context.Background(), fake); err != nil {
		t.Fatal(err)
	}
	if got, reason := memoryExtractionWorthwhile(context.Background(), fake, msg); !got || reason != "" {
		t.Fatalf("dimension mismatch got %v %q, want fail open", got, reason)
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
