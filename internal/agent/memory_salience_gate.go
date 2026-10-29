package agent

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// memoryQueryEmbedder is the part of the vector store the salience gate needs.
type memoryQueryEmbedder interface {
	EmbedQueries(ctx context.Context, texts []string) ([][]float32, error)
	EmbedTexts(ctx context.Context, texts []string) ([][]float32, error)
	EmbeddingFingerprint() string
}

const (
	// The gate skips only when a message is clearly closer to filler than to
	// anything worth remembering. Missed facts still reach the nightly
	// consolidation of the archived transcript.
	salienceSkipMargin   = 0.05
	salienceMaxGateRunes = 300
	salienceGateTimeout  = 5 * time.Second
	salienceMaxFillerLen = 8
	// The prototype warm-up runs in the background, so it may outlast a turn.
	salienceWarmupTimeout = 60 * time.Second
)

var errSaliencePrototypesWarming = errors.New("salience prototypes are still warming up")

var salienceFillerWords = map[string]struct{}{}

func init() {
	for _, word := range strings.Fields(`
		danke dankeschön vielen dank dafür super prima perfekt passt ok okay okey alles klar gut sehr toll cool top
		ja nein jo jep gerne bitte hallo hi hey moin servus tschüss ciao das war es ist
		thanks thank you a lot cheers awesome perfect fine good great nice got it yes no yep sure bye hello for that`) {
		salienceFillerWords[word] = struct{}{}
	}
}

// Prototype sentences in German and English; the embedder is multilingual.
var salientMemoryPrototypes = []string{
	"Ich heiße Maria und wohne in Hamburg.",
	"My name is Alex and I live in Berlin.",
	"Mein Homeserver läuft mit Proxmox auf einem Dell R730.",
	"The NAS uses ZFS and is reachable at 192.168.1.20.",
	"Ich bevorzuge Go gegenüber Python.",
	"I prefer short answers without emojis.",
	"Merk dir bitte, dass ich montags keine Meetings habe.",
	"Remember that the backup runs every night at 3 am.",
	"Nein, das stimmt nicht, ich nutze Podman statt Docker.",
	"That is wrong, I moved from Berlin to Munich.",
	"Mach das in Zukunft bitte immer auf Deutsch.",
	"From now on always ask before deleting files.",
	"Ich bin bis Freitag im Urlaub.",
	"The maintenance window is tonight from 10 pm.",
	"Meine Frau heißt Julia und arbeitet als Ärztin.",
	"Our project deadline is the 15th of November.",
}

var fillerMemoryPrototypes = []string{
	"Danke, das hat geholfen.",
	"Thanks, that works now.",
	"Okay, passt so.",
	"Alright, sounds good.",
	"Zeig mir die Logs vom Container.",
	"Show me the logs of the web server.",
	"Starte den Dienst neu.",
	"Restart the container please.",
	"Fasse diesen Artikel zusammen.",
	"Summarize this page for me.",
	"Übersetze den Text ins Englische.",
	"Translate this into German.",
	"Wie wird das Wetter morgen?",
	"What time is it in Tokyo?",
	"Erklär mir, wie DNS funktioniert.",
	"Explain how a reverse proxy works.",
}

var salienceCache struct {
	mu          sync.Mutex
	fingerprint string
	salient     [][]float32
	filler      [][]float32
	warming     bool
}

// memoryExtractionWorthwhile decides before the extraction LLM call whether a
// turn can hold anything worth remembering. It fails open: without an
// embedder, or when embedding fails, the turn goes to the extractor.
func memoryExtractionWorthwhile(ctx context.Context, embedder memoryQueryEmbedder, userMsg string) (bool, string) {
	if onlySalienceFillerWords(userMsg) {
		return false, "filler_words"
	}
	userMsg = strings.TrimSpace(userMsg)
	if embedder == nil || utf8.RuneCountInString(userMsg) > salienceMaxGateRunes {
		return true, ""
	}
	ctx, cancel := context.WithTimeout(ctx, salienceGateTimeout)
	defer cancel()
	salient, filler, err := salienceProtoVectors(embedder)
	if err != nil {
		return true, ""
	}
	vectors, err := embedder.EmbedQueries(ctx, []string{userMsg})
	if err != nil || len(vectors) != 1 {
		return true, ""
	}
	if len(salient) == 0 || len(filler) == 0 || len(vectors[0]) != len(salient[0]) || len(vectors[0]) != len(filler[0]) {
		return true, ""
	}
	if maxCosine(vectors[0], filler)-maxCosine(vectors[0], salient) >= salienceSkipMargin {
		return false, "filler_prototype"
	}
	return true, ""
}

func onlySalienceFillerWords(msg string) bool {
	words := strings.FieldsFunc(strings.ToLower(msg), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(words) == 0 || len(words) > salienceMaxFillerLen {
		return false
	}
	for _, word := range words {
		if _, ok := salienceFillerWords[word]; !ok {
			return false
		}
	}
	return true
}

// salienceProtoVectors returns the prototype vectors of the current embedding
// model. While they are missing it starts one background warm-up and returns
// errSaliencePrototypesWarming, so the gate fails open without adding latency.
func salienceProtoVectors(embedder memoryQueryEmbedder) (salient, filler [][]float32, err error) {
	fingerprint := embedder.EmbeddingFingerprint()
	salienceCache.mu.Lock()
	defer salienceCache.mu.Unlock()
	if salienceCache.fingerprint == fingerprint && salienceCache.salient != nil {
		return salienceCache.salient, salienceCache.filler, nil
	}
	if !salienceCache.warming {
		salienceCache.warming = true
		go func() {
			defer func() {
				salienceCache.mu.Lock()
				salienceCache.warming = false
				salienceCache.mu.Unlock()
			}()
			ctx, cancel := context.WithTimeout(context.Background(), salienceWarmupTimeout)
			defer cancel()
			_ = warmSaliencePrototypes(ctx, embedder)
		}()
	}
	return nil, nil, errSaliencePrototypesWarming
}

// warmSaliencePrototypes embeds the prototypes with the current model, never
// through the query cache, and stores them unless the model changed meanwhile.
func warmSaliencePrototypes(ctx context.Context, embedder memoryQueryEmbedder) error {
	fingerprint := embedder.EmbeddingFingerprint()
	texts := append(append([]string(nil), salientMemoryPrototypes...), fillerMemoryPrototypes...)
	vectors, err := embedder.EmbedTexts(ctx, texts)
	if err != nil {
		return err
	}
	if len(vectors) != len(texts) {
		return fmt.Errorf("embedder returned %d prototype vectors for %d texts", len(vectors), len(texts))
	}
	if embedder.EmbeddingFingerprint() != fingerprint {
		return nil
	}
	salienceCache.mu.Lock()
	defer salienceCache.mu.Unlock()
	salienceCache.fingerprint = fingerprint
	salienceCache.salient = vectors[:len(salientMemoryPrototypes)]
	salienceCache.filler = vectors[len(salientMemoryPrototypes):]
	return nil
}

func maxCosine(vector []float32, prototypes [][]float32) float64 {
	best := -1.0
	for _, prototype := range prototypes {
		if score := cosine32(vector, prototype); score > best {
			best = score
		}
	}
	return best
}

func cosine32(left, right []float32) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	var dot, leftNorm, rightNorm float64
	for i := range left {
		dot += float64(left[i]) * float64(right[i])
		leftNorm += float64(left[i]) * float64(left[i])
		rightNorm += float64(right[i]) * float64(right[i])
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / math.Sqrt(leftNorm*rightNorm)
}
