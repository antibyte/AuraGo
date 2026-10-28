package injectionguard

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

type addedToken struct {
	ID         int64  `json:"id"`
	Content    string `json:"content"`
	LStrip     bool   `json:"lstrip"`
	RStrip     bool   `json:"rstrip"`
	SingleWord bool   `json:"single_word"`
	Normalized bool   `json:"normalized"`
}

type normalizer struct {
	Type    string            `json:"type"`
	Pattern map[string]string `json:"pattern"`
	Content string            `json:"content"`
	re      *regexp.Regexp
}

type tokenizer struct {
	vocab       map[string]int64
	ranks       map[[2]string]int
	added       []addedToken
	normalizers []normalizer
}

// loadTokenizer supports the pinned model's exact pipeline, not arbitrary HF tokenizers.
func loadTokenizer(path string) (*tokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f struct {
		Added      []addedToken `json:"added_tokens"`
		Normalizer struct {
			Type        string
			Normalizers []normalizer
		}
		Pre struct {
			Type, Replacement string
			Prepend           string `json:"prepend_scheme"`
			Split             bool
		} `json:"pre_tokenizer"`
		Model struct {
			Type         string
			Vocab        map[string]int64
			Merges       [][]string
			ByteFallback bool `json:"byte_fallback"`
			IgnoreMerges bool `json:"ignore_merges"`
		}
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("tokenizer JSON: %w", err)
	}
	if f.Model.Type != "BPE" || !f.Model.ByteFallback || f.Model.IgnoreMerges || f.Pre.Type != "Metaspace" || f.Pre.Replacement != "▁" || f.Pre.Prepend != "always" || !f.Pre.Split || f.Normalizer.Type != "Sequence" || len(f.Model.Vocab) != 256000 {
		return nil, fmt.Errorf("unsupported Horizon tokenizer pipeline")
	}
	t := &tokenizer{vocab: f.Model.Vocab, ranks: make(map[[2]string]int, len(f.Model.Merges)), added: f.Added, normalizers: f.Normalizer.Normalizers}
	for i, pair := range f.Model.Merges {
		if len(pair) != 2 {
			return nil, fmt.Errorf("invalid BPE merge")
		}
		t.ranks[[2]string{pair[0], pair[1]}] = i
	}
	for i := range t.normalizers {
		n := &t.normalizers[i]
		switch n.Type {
		case "NFKC":
		case "Replace":
			if pattern, ok := n.Pattern["Regex"]; ok {
				n.re, err = regexp.Compile(pattern)
				if err != nil {
					return nil, fmt.Errorf("normalizer regex: %w", err)
				}
			} else if value, ok := n.Pattern["String"]; !ok || value == "" {
				return nil, fmt.Errorf("unsupported normalizer replacement")
			}
		default:
			return nil, fmt.Errorf("unsupported normalizer")
		}
	}
	for _, token := range t.added {
		if token.Normalized || token.SingleWord || token.Content == "" {
			return nil, fmt.Errorf("unsupported added token")
		}
	}
	sort.SliceStable(t.added, func(i, j int) bool { return len(t.added[i].Content) > len(t.added[j].Content) })
	return t, nil
}

func (t *tokenizer) normalize(text string) string {
	for _, n := range t.normalizers {
		switch {
		case n.Type == "NFKC":
			text = norm.NFKC.String(text)
		case n.re != nil:
			text = n.re.ReplaceAllString(text, n.Content)
		default:
			text = strings.ReplaceAll(text, n.Pattern["String"], n.Content)
		}
	}
	return text
}

// encode returns all content tokens; the tokenizer JSON's embedded truncation is intentionally ignored.
func (t *tokenizer) encode(text string) ([]int64, error) {
	if len(text) > MaxTextBytes || !utf8.ValidString(text) {
		return nil, fmt.Errorf("invalid or excessive classifier input")
	}
	var ids []int64
	for text != "" {
		at, which := len(text), -1
		for i, token := range t.added {
			if p := strings.Index(text, token.Content); p >= 0 && (p < at || (p == at && which < 0)) {
				at, which = p, i
			}
		}
		plain := text[:at]
		if which >= 0 && t.added[which].LStrip {
			plain = strings.TrimRightFunc(plain, unicode.IsSpace)
		}
		ids = append(ids, t.plain(plain)...)
		if which < 0 {
			break
		}
		token := t.added[which]
		ids = append(ids, token.ID)
		text = text[at+len(token.Content):]
		if token.RStrip {
			text = strings.TrimLeftFunc(text, unicode.IsSpace)
		}
	}
	return ids, nil
}

func (t *tokenizer) plain(text string) []int64 {
	text = t.normalize(text)
	if text == "" {
		return nil
	}
	if !strings.HasPrefix(text, "▁") {
		text = "▁" + text
	}
	var ids []int64
	// Metaspace splits before every replacement, retaining it in the next piece.
	start := 0
	for i, r := range text {
		if r == '▁' && i > start {
			ids = append(ids, t.piece(text[start:i])...)
			start = i
		}
	}
	return append(ids, t.piece(text[start:])...)
}

type bpeNode struct {
	value               string
	prev, next, version int
	live                bool
}
type bpeMerge struct{ rank, left, right, leftVersion, rightVersion int }
type mergeHeap []bpeMerge

func (h mergeHeap) Len() int { return len(h) }
func (h mergeHeap) Less(i, j int) bool {
	if h[i].rank != h[j].rank {
		return h[i].rank < h[j].rank
	}
	return h[i].left < h[j].left
}
func (h mergeHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *mergeHeap) Push(v any)   { *h = append(*h, v.(bpeMerge)) }
func (h *mergeHeap) Pop() any     { a := *h; v := a[len(a)-1]; *h = a[:len(a)-1]; return v }

func (t *tokenizer) piece(text string) []int64 {
	nodes := make([]bpeNode, 0, utf8.RuneCountInString(text))
	for _, r := range text {
		i := len(nodes)
		nodes = append(nodes, bpeNode{value: string(r), prev: i - 1, next: i + 1, live: true})
	}
	if len(nodes) == 0 {
		return nil
	}
	nodes[len(nodes)-1].next = -1
	h := &mergeHeap{}
	add := func(i int) {
		if i < 0 || !nodes[i].live {
			return
		}
		j := nodes[i].next
		if j < 0 {
			return
		}
		if rank, ok := t.ranks[[2]string{nodes[i].value, nodes[j].value}]; ok {
			heap.Push(h, bpeMerge{rank, i, j, nodes[i].version, nodes[j].version})
		}
	}
	for i := range nodes {
		add(i)
	}
	for h.Len() > 0 {
		m := heap.Pop(h).(bpeMerge)
		l, r := &nodes[m.left], &nodes[m.right]
		if !l.live || !r.live || l.next != m.right || l.version != m.leftVersion || r.version != m.rightVersion {
			continue
		}
		l.value += r.value
		l.next = r.next
		l.version++
		r.live = false
		if r.next >= 0 {
			nodes[r.next].prev = m.left
		}
		add(l.prev)
		add(m.left)
	}
	var ids []int64
	for i := 0; i >= 0; i = nodes[i].next {
		if id, ok := t.vocab[nodes[i].value]; ok {
			ids = append(ids, id)
			continue
		}
		for _, b := range []byte(nodes[i].value) {
			if id, ok := t.vocab[fmt.Sprintf("<0x%02X>", b)]; ok {
				ids = append(ids, id)
			} else {
				ids = append(ids, 3)
			}
		}
	}
	return ids
}

func tokenWindows(ids []int64) [][]int64 {
	var windows [][]int64
	for start := 0; ; start += WindowTokens - 2 - WindowOverlap {
		end := min(len(ids), start+WindowTokens-2)
		window := make([]int64, 0, end-start+2)
		window = append(window, 2)
		window = append(window, ids[start:end]...)
		window = append(window, 1)
		windows = append(windows, window)
		if end == len(ids) {
			return windows
		}
	}
}
