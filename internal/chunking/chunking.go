package chunking

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	StrategyLegacy    = "legacy"
	StrategyRecursive = "recursive"
)

const (
	DefaultStrategy          = StrategyRecursive
	DefaultMaxChars          = 3500
	DefaultOverlapChars      = 200
	DefaultMaxChunks         = 200
	defaultOversizeSplitStep = 32
)

type Chunk struct {
	Text  string
	Index int
	Total int
}

type Options struct {
	Strategy     string
	MaxChars     int
	OverlapChars int
	MaxChunks    int
}

type Chunker interface {
	Chunk(text string) ([]Chunk, error)
}

func DefaultOptions() Options {
	return Options{
		Strategy:     DefaultStrategy,
		MaxChars:     DefaultMaxChars,
		OverlapChars: DefaultOverlapChars,
		MaxChunks:    DefaultMaxChunks,
	}
}

func NormalizeOptions(opts Options) Options {
	opts.Strategy = strings.ToLower(strings.TrimSpace(opts.Strategy))
	switch opts.Strategy {
	case StrategyLegacy, StrategyRecursive:
	default:
		opts.Strategy = DefaultStrategy
	}
	if opts.MaxChars <= 0 {
		opts.MaxChars = DefaultMaxChars
	}
	if opts.OverlapChars < 0 {
		opts.OverlapChars = 0
	}
	if opts.OverlapChars >= opts.MaxChars {
		opts.OverlapChars = opts.MaxChars / 4
	}
	if opts.MaxChunks <= 0 {
		opts.MaxChunks = DefaultMaxChunks
	}
	return opts
}

func NormalizeOptionsWithDefaults(opts Options) Options {
	defaults := DefaultOptions()
	if strings.TrimSpace(opts.Strategy) != "" {
		defaults.Strategy = opts.Strategy
	}
	if opts.MaxChars > 0 {
		defaults.MaxChars = opts.MaxChars
	}
	if opts.OverlapChars > 0 {
		defaults.OverlapChars = opts.OverlapChars
	}
	if opts.MaxChunks > 0 {
		defaults.MaxChunks = opts.MaxChunks
	}
	normalized := NormalizeOptions(defaults)
	if opts.OverlapChars <= 0 || opts.OverlapChars >= normalized.MaxChars {
		normalized.OverlapChars = DefaultOverlapChars
		if normalized.OverlapChars >= normalized.MaxChars {
			normalized.OverlapChars = normalized.MaxChars / 4
		}
	}
	return normalized
}

func Fingerprint(opts Options) string {
	opts = NormalizeOptions(opts)
	return fmt.Sprintf("chunking=%s;max_chars=%d;overlap_chars=%d;max_chunks=%d", opts.Strategy, opts.MaxChars, opts.OverlapChars, opts.MaxChunks)
}

func NewChunker(opts Options) Chunker {
	opts = NormalizeOptions(opts)
	if opts.Strategy == StrategyLegacy {
		return legacyChunker{opts: opts}
	}
	return recursiveChunker{opts: opts}
}

func ChunkText(text string, opts Options) ([]Chunk, error) {
	return NewChunker(opts).Chunk(text)
}

func ChunkStrings(text string, opts Options) []string {
	chunks, err := ChunkText(text, opts)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		out = append(out, chunk.Text)
	}
	return out
}

type legacyChunker struct {
	opts Options
}

func (c legacyChunker) Chunk(text string) ([]Chunk, error) {
	parts := legacySplit(text, c.opts.MaxChars, c.opts.OverlapChars)
	return finalizeChunks(parts, c.opts.MaxChunks), nil
}

type recursiveChunker struct {
	opts Options
}

func (c recursiveChunker) Chunk(text string) ([]Chunk, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	blocks := markdownBlocks(text)
	var chunks []string
	var current strings.Builder

	flush := func() {
		if s := strings.TrimSpace(current.String()); s != "" {
			chunks = append(chunks, s)
		}
		current.Reset()
	}

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		if runeLen(block) > c.opts.MaxChars {
			flush()
			chunks = append(chunks, splitOversizeBlock(block, c.opts.MaxChars)...)
			continue
		}
		candidate := block
		if current.Len() > 0 {
			candidate = strings.TrimSpace(current.String()) + "\n\n" + block
		}
		if runeLen(candidate) <= c.opts.MaxChars {
			current.Reset()
			current.WriteString(candidate)
			continue
		}
		flush()
		current.WriteString(block)
	}
	flush()

	if c.opts.OverlapChars > 0 && len(chunks) > 1 {
		chunks = applyOverlap(chunks, c.opts.OverlapChars, c.opts.MaxChars)
	}
	return finalizeChunks(chunks, c.opts.MaxChunks), nil
}

func markdownBlocks(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var blocks []string
	var current []string
	var inFence bool
	var inTable bool

	flush := func() {
		if len(current) == 0 {
			return
		}
		block := strings.TrimSpace(strings.Join(current, "\n"))
		if block != "" {
			blocks = append(blocks, block)
		}
		current = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			current = append(current, line)
			inFence = !inFence
			if !inFence {
				flush()
			}
			continue
		}
		if inFence {
			current = append(current, line)
			continue
		}
		if isMarkdownTableLine(trimmed) {
			if !inTable {
				flush()
				inTable = true
			}
			current = append(current, line)
			continue
		}
		if inTable {
			flush()
			inTable = false
		}
		if trimmed == "" {
			flush()
			continue
		}
		if isMarkdownHeading(trimmed) || isMarkdownListStart(trimmed) {
			flush()
		}
		current = append(current, line)
	}
	flush()
	return blocks
}

func isMarkdownHeading(line string) bool {
	if !strings.HasPrefix(line, "#") {
		return false
	}
	hashes := 0
	for _, r := range line {
		if r != '#' {
			break
		}
		hashes++
	}
	return hashes > 0 && hashes <= 6 && len([]rune(line)) > hashes && unicode.IsSpace([]rune(line)[hashes])
}

func isMarkdownListStart(line string) bool {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}
	dot := strings.Index(line, ". ")
	if dot <= 0 || dot > 4 {
		return false
	}
	for _, r := range line[:dot] {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func isMarkdownTableLine(line string) bool {
	return strings.Count(line, "|") >= 2
}

func splitOversizeBlock(text string, maxChars int) []string {
	if runeLen(text) <= maxChars {
		return []string{text}
	}
	var out []string
	remaining := strings.TrimSpace(text)
	for runeLen(remaining) > maxChars {
		head, tail := splitAtBestBoundary(remaining, maxChars)
		if strings.TrimSpace(head) == "" {
			break
		}
		out = append(out, strings.TrimSpace(head))
		remaining = strings.TrimSpace(tail)
	}
	if remaining != "" {
		out = append(out, remaining)
	}
	return out
}

func splitAtBestBoundary(text string, maxChars int) (string, string) {
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text, ""
	}
	window := string(runes[:maxChars])
	minSplit := maxChars / 2
	for _, sep := range []string{"\n\n", "\n", ". ", "! ", "? ", "; ", ", ", " "} {
		idx := strings.LastIndex(window, sep)
		if idx <= 0 {
			continue
		}
		splitRunes := utf8.RuneCountInString(window[:idx+len(sep)])
		if splitRunes >= minSplit {
			return string(runes[:splitRunes]), string(runes[splitRunes:])
		}
	}
	if maxChars > defaultOversizeSplitStep {
		return string(runes[:maxChars]), string(runes[maxChars:])
	}
	return string(runes[:maxChars]), string(runes[maxChars:])
}

func applyOverlap(chunks []string, overlap, maxChars int) []string {
	out := make([]string, len(chunks))
	out[0] = chunks[0]
	for i := 1; i < len(chunks); i++ {
		prefix := tailRunes(chunks[i-1], overlap)
		if prefix == "" {
			out[i] = chunks[i]
			continue
		}
		combined := strings.TrimSpace(prefix + "\n" + chunks[i])
		if runeLen(combined) > maxChars {
			combined = string([]rune(combined)[:maxChars])
		}
		out[i] = strings.TrimSpace(combined)
	}
	return out
}

func tailRunes(text string, count int) string {
	runes := []rune(strings.TrimSpace(text))
	if count <= 0 || len(runes) == 0 {
		return ""
	}
	if count >= len(runes) {
		return string(runes)
	}
	return string(runes[len(runes)-count:])
}

func finalizeChunks(parts []string, maxChunks int) []Chunk {
	if maxChunks <= 0 {
		maxChunks = DefaultMaxChunks
	}
	if len(parts) > maxChunks {
		parts = parts[:maxChunks]
	}
	chunks := make([]Chunk, 0, len(parts))
	total := len(parts)
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		chunks = append(chunks, Chunk{Text: part, Index: i, Total: total})
	}
	if len(chunks) != total {
		total = len(chunks)
		for i := range chunks {
			chunks[i].Index = i
			chunks[i].Total = total
		}
	}
	return chunks
}

func runeLen(text string) int {
	return utf8.RuneCountInString(text)
}

func legacySplit(text string, chunkSize, overlap int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	runes := []rune(text)
	if chunkSize <= 0 {
		chunkSize = len(runes)
		if chunkSize == 0 {
			return nil
		}
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}
	if len(runes) <= chunkSize {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < len(runes) {
		end := start + chunkSize
		if end >= len(runes) {
			if chunk := strings.TrimSpace(string(runes[start:])); chunk != "" {
				chunks = append(chunks, chunk)
			}
			break
		}

		chunkStr := string(runes[start:end])
		splitAt := strings.LastIndex(chunkStr, "\n\n")
		if splitAt > len(chunkStr)/2 {
			end = start + utf8.RuneCountInString(chunkStr[:splitAt])
		} else {
			splitAt = strings.LastIndex(chunkStr, ". ")
			if splitAt > len(chunkStr)/2 {
				end = start + utf8.RuneCountInString(chunkStr[:splitAt+2])
			}
		}

		if end <= start {
			end = start + chunkSize
			if end > len(runes) {
				end = len(runes)
			}
		}
		if chunk := strings.TrimSpace(string(runes[start:end])); chunk != "" {
			chunks = append(chunks, chunk)
		}

		nextStart := end - overlap
		if nextStart <= start {
			nextStart = end
		}
		start = nextStart
	}
	return chunks
}
