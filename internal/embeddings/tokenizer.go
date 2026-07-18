package embeddings

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

const granitePretokenizerPattern = `[^\r\n\p{L}\p{N}]?[\p{Lu}\p{Lt}\p{Lm}\p{Lo}\p{M}]*[\p{Ll}\p{Lm}\p{Lo}\p{M}]+(?i:'s|'t|'re|'ve|'m|'ll|'d)?|[^\r\n\p{L}\p{N}]?[\p{Lu}\p{Lt}\p{Lm}\p{Lo}\p{M}]+[\p{Ll}\p{Lm}\p{Lo}\p{M}]*(?i:'s|'t|'re|'ve|'m|'ll|'d)?|\p{N}{1,3}| ?[^\s\p{L}\p{N}]+[\r\n/]*|\s*[\r\n]+|\s+(?!\S)|\s+`

type tokenizerFile struct {
	Model struct {
		Type         string         `json:"type"`
		Vocab        map[string]int `json:"vocab"`
		Merges       [][]string     `json:"merges"`
		IgnoreMerges bool           `json:"ignore_merges"`
	} `json:"model"`
	AddedTokens []struct {
		ID      int    `json:"id"`
		Content string `json:"content"`
		Special bool   `json:"special"`
	} `json:"added_tokens"`
	PostProcessor struct {
		SpecialTokens map[string]struct {
			IDs []int `json:"ids"`
		} `json:"special_tokens"`
	} `json:"post_processor"`
	Padding struct {
		PadID int `json:"pad_id"`
	} `json:"padding"`
}

type tokenPair struct {
	left  string
	right string
}

type graniteTokenizer struct {
	vocab        map[string]int
	mergeRanks   map[tokenPair]int
	ignoreMerges bool
	byteEncoder  [256]string
	splitter     *regexp2.Regexp
	specials     map[string]int
	specialOrder []string
	clsID        int
	eosID        int
	padID        int
	cacheMu      sync.RWMutex
	cache        map[string][]int
}

type tokenBatch struct {
	InputIDs      []int64
	AttentionMask []int64
	BatchSize     int
	SequenceSize  int
}

func loadGraniteTokenizer(path string) (*graniteTokenizer, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tokenizer: %w", err)
	}
	var config tokenizerFile
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("parse tokenizer JSON: %w", err)
	}
	if config.Model.Type != "BPE" {
		return nil, fmt.Errorf("tokenizer model type %q is not BPE", config.Model.Type)
	}
	if len(config.Model.Vocab) == 0 || len(config.Model.Merges) == 0 {
		return nil, fmt.Errorf("tokenizer vocabulary or merge table is empty")
	}
	splitter, err := regexp2.Compile(granitePretokenizerPattern, regexp2.None)
	if err != nil {
		return nil, fmt.Errorf("compile Granite pre-tokenizer: %w", err)
	}
	splitter.MatchTimeout = 2 * time.Second

	tokenizer := &graniteTokenizer{
		vocab:        config.Model.Vocab,
		mergeRanks:   make(map[tokenPair]int, len(config.Model.Merges)),
		ignoreMerges: config.Model.IgnoreMerges,
		byteEncoder:  buildByteEncoder(),
		splitter:     splitter,
		specials:     make(map[string]int),
		padID:        config.Padding.PadID,
		cache:        make(map[string][]int),
	}
	for rank, merge := range config.Model.Merges {
		if len(merge) != 2 {
			return nil, fmt.Errorf("tokenizer merge %d has %d elements", rank, len(merge))
		}
		tokenizer.mergeRanks[tokenPair{left: merge[0], right: merge[1]}] = rank
	}
	for _, token := range config.AddedTokens {
		if token.Special {
			tokenizer.specials[token.Content] = token.ID
			tokenizer.specialOrder = append(tokenizer.specialOrder, token.Content)
		}
	}
	sort.Slice(tokenizer.specialOrder, func(i, j int) bool {
		return len(tokenizer.specialOrder[i]) > len(tokenizer.specialOrder[j])
	})
	tokenizer.clsID = firstSpecialID(config.PostProcessor.SpecialTokens, "<|startoftext|>", 179934)
	tokenizer.eosID = firstSpecialID(config.PostProcessor.SpecialTokens, "<|return|>", 179938)
	if tokenizer.padID == 0 {
		tokenizer.padID = 179935
	}
	return tokenizer, nil
}

func firstSpecialID(values map[string]struct {
	IDs []int `json:"ids"`
}, token string, fallback int) int {
	entry, ok := values[token]
	if !ok || len(entry.IDs) == 0 {
		return fallback
	}
	return entry.IDs[0]
}

func (t *graniteTokenizer) encodeBatch(texts []string, contextSize int) (tokenBatch, error) {
	if len(texts) == 0 {
		return tokenBatch{}, fmt.Errorf("at least one text is required")
	}
	if contextSize < 2 {
		return tokenBatch{}, fmt.Errorf("context size must be at least 2")
	}
	encoded := make([][]int, len(texts))
	maxLength := 0
	for i, text := range texts {
		ids, err := t.encode(text, contextSize)
		if err != nil {
			return tokenBatch{}, fmt.Errorf("tokenize text %d: %w", i, err)
		}
		encoded[i] = ids
		if len(ids) > maxLength {
			maxLength = len(ids)
		}
	}
	result := tokenBatch{
		InputIDs:      make([]int64, len(texts)*maxLength),
		AttentionMask: make([]int64, len(texts)*maxLength),
		BatchSize:     len(texts),
		SequenceSize:  maxLength,
	}
	for batchIndex, ids := range encoded {
		offset := batchIndex * maxLength
		for tokenIndex := 0; tokenIndex < maxLength; tokenIndex++ {
			if tokenIndex < len(ids) {
				result.InputIDs[offset+tokenIndex] = int64(ids[tokenIndex])
				result.AttentionMask[offset+tokenIndex] = 1
			} else {
				result.InputIDs[offset+tokenIndex] = int64(t.padID)
			}
		}
	}
	return result, nil
}

func (t *graniteTokenizer) encode(text string, contextSize int) ([]int, error) {
	ids := make([]int, 0, min(contextSize, len(text)/3+2))
	ids = append(ids, t.clsID)
	for len(text) > 0 {
		index, special := t.nextSpecial(text)
		if index < 0 {
			plain, err := t.encodePlain(text)
			if err != nil {
				return nil, err
			}
			ids = append(ids, plain...)
			break
		}
		if index > 0 {
			plain, err := t.encodePlain(text[:index])
			if err != nil {
				return nil, err
			}
			ids = append(ids, plain...)
		}
		ids = append(ids, t.specials[special])
		text = text[index+len(special):]
	}
	maxBody := contextSize - 2
	if len(ids)-1 > maxBody {
		ids = ids[:maxBody+1]
	}
	ids = append(ids, t.eosID)
	return ids, nil
}

func (t *graniteTokenizer) nextSpecial(text string) (int, string) {
	bestIndex := -1
	bestToken := ""
	for _, token := range t.specialOrder {
		index := strings.Index(text, token)
		if index >= 0 && (bestIndex < 0 || index < bestIndex || (index == bestIndex && len(token) > len(bestToken))) {
			bestIndex = index
			bestToken = token
		}
	}
	return bestIndex, bestToken
}

func (t *graniteTokenizer) encodePlain(text string) ([]int, error) {
	var result []int
	match, err := t.splitter.FindStringMatch(text)
	if err != nil {
		return nil, fmt.Errorf("pre-tokenize: %w", err)
	}
	consumedRunes := 0
	for match != nil {
		if match.Index > consumedRunes {
			prefix := sliceRunes(text, consumedRunes, match.Index)
			ids, err := t.encodePiece(prefix)
			if err != nil {
				return nil, err
			}
			result = append(result, ids...)
		}
		piece := match.String()
		ids, err := t.encodePiece(piece)
		if err != nil {
			return nil, err
		}
		result = append(result, ids...)
		consumedRunes = match.Index + match.Length
		match, err = t.splitter.FindNextMatch(match)
		if err != nil {
			return nil, fmt.Errorf("continue pre-tokenize: %w", err)
		}
	}
	totalRunes := utf8.RuneCountInString(text)
	if consumedRunes < totalRunes {
		ids, err := t.encodePiece(sliceRunes(text, consumedRunes, totalRunes))
		if err != nil {
			return nil, err
		}
		result = append(result, ids...)
	}
	return result, nil
}

func sliceRunes(value string, start, end int) string {
	if start == 0 && end == utf8.RuneCountInString(value) {
		return value
	}
	runes := []rune(value)
	return string(runes[start:end])
}

func (t *graniteTokenizer) encodePiece(piece string) ([]int, error) {
	if piece == "" {
		return nil, nil
	}
	encoded := t.byteEncode(piece)
	t.cacheMu.RLock()
	cached, ok := t.cache[encoded]
	t.cacheMu.RUnlock()
	if ok {
		return append([]int(nil), cached...), nil
	}
	if t.ignoreMerges {
		if id, ok := t.vocab[encoded]; ok {
			result := []int{id}
			t.cacheMu.Lock()
			t.cache[encoded] = result
			t.cacheMu.Unlock()
			return append([]int(nil), result...), nil
		}
	}

	symbols := make([]string, 0, utf8.RuneCountInString(encoded))
	for _, symbol := range encoded {
		symbols = append(symbols, string(symbol))
	}
	for len(symbols) > 1 {
		bestRank := int(^uint(0) >> 1)
		bestPair := tokenPair{}
		found := false
		for i := 0; i+1 < len(symbols); i++ {
			pair := tokenPair{left: symbols[i], right: symbols[i+1]}
			rank, ok := t.mergeRanks[pair]
			if ok && rank < bestRank {
				bestRank = rank
				bestPair = pair
				found = true
			}
		}
		if !found {
			break
		}
		merged := make([]string, 0, len(symbols))
		for i := 0; i < len(symbols); {
			if i+1 < len(symbols) && symbols[i] == bestPair.left && symbols[i+1] == bestPair.right {
				merged = append(merged, symbols[i]+symbols[i+1])
				i += 2
			} else {
				merged = append(merged, symbols[i])
				i++
			}
		}
		symbols = merged
	}

	result := make([]int, len(symbols))
	for i, symbol := range symbols {
		id, ok := t.vocab[symbol]
		if !ok {
			return nil, fmt.Errorf("BPE token %q is missing from vocabulary", symbol)
		}
		result[i] = id
	}
	t.cacheMu.Lock()
	if len(t.cache) < 32_768 {
		t.cache[encoded] = append([]int(nil), result...)
	}
	t.cacheMu.Unlock()
	return result, nil
}

func (t *graniteTokenizer) byteEncode(value string) string {
	var builder strings.Builder
	for _, valueByte := range []byte(value) {
		builder.WriteString(t.byteEncoder[valueByte])
	}
	return builder.String()
}

func buildByteEncoder() [256]string {
	var bytes []int
	for value := int('!'); value <= int('~'); value++ {
		bytes = append(bytes, value)
	}
	for value := 0xA1; value <= 0xAC; value++ {
		bytes = append(bytes, value)
	}
	for value := 0xAE; value <= 0xFF; value++ {
		bytes = append(bytes, value)
	}
	codePoints := append([]int(nil), bytes...)
	included := make(map[int]bool, len(bytes))
	for _, value := range bytes {
		included[value] = true
	}
	extra := 0
	for value := 0; value < 256; value++ {
		if included[value] {
			continue
		}
		bytes = append(bytes, value)
		codePoints = append(codePoints, 256+extra)
		extra++
	}
	var encoder [256]string
	for i, value := range bytes {
		encoder[value] = string(rune(codePoints[i]))
	}
	return encoder
}
