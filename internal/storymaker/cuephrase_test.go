package storymaker

import (
	"slices"
	"testing"
)

// cueMatchStarts returns the byte offsets in narration where phrase starts.
func cueMatchStarts(narration, phrase string) []int {
	tokens := cueTokens(narration)
	var starts []int
	for _, i := range cuePhraseMatches(tokens, cueTokens(phrase)) {
		starts = append(starts, tokens[i].start)
	}
	return starts
}

func TestCuePhraseMatching(t *testing.T) {
	cases := []struct {
		name, narration, phrase string
		want                    []int
	}{
		{"plain", "The firewall checks every packet.", "every packet", []int{20}},
		{"case folding", "Die ÄRZTIN kommt.", "ärztin", []int{4}},
		{"umlaut stays", "Die Ärztin kommt.", "arztin", nil},
		{"punctuation and spaces", "Wait,   then -- go!", "then go", []int{8}},
		{"word boundary at the start", "The unfirewalled net.", "firewalled", nil},
		{"word boundary at the end", "A firewall blocks.", "fire", nil},
		{"markers are skipped", "The {1}firewall {2}checks the net.", "firewall checks", []int{7}},
		{"marker inside the phrase span", "Look {3}here, now.", "look here", []int{0}},
		{"first and later occurrences", "Go on. Go on.", "go on", []int{0, 7}},
		{"cyrillic", "Это МОЙ дом.", "мой дом", []int{7}},
		{"cjk substring", "防火墙会检查每个数据包。", "检查每个", []int{12}},
		{"cjk across punctuation", "你好，世界。", "你好世界", []int{0}},
		{"cjk inside a longer run", "防火墙", "火", []int{3}},
		{"empty phrase", "Hello world", "  ,. ", nil},
	}
	for _, c := range cases {
		if got := cueMatchStarts(c.narration, c.phrase); !slices.Equal(got, c.want) {
			t.Errorf("%s: starts = %v, want %v", c.name, got, c.want)
		}
	}
}
