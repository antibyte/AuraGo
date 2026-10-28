package injectionguard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTokenWindowsCoverAllInput(t *testing.T) {
	for _, n := range []int{0, 1, 2046, 2047, 6000, 100000} {
		ids := make([]int64, n)
		for i := range ids {
			ids[i] = int64(i + 10)
		}
		windows := tokenWindows(ids)
		seen := make([]bool, n)
		for _, w := range windows {
			if len(w) > WindowTokens || w[0] != 2 || w[len(w)-1] != 1 {
				t.Fatalf("bad window at length %d", n)
			}
			for _, id := range w[1 : len(w)-1] {
				seen[id-10] = true
			}
		}
		for i, ok := range seen {
			if !ok {
				t.Fatalf("unscanned token %d", i)
			}
		}
	}
}

func TestTokenizerReference(t *testing.T) {
	root := os.Getenv("AURAGO_INJECTION_GUARD_CACHE")
	if root == "" {
		t.Skip("set AURAGO_INJECTION_GUARD_CACHE to the prepared data directory")
	}
	tok, err := loadTokenizer(filepath.Join(root, "injectionguard", "models", "tokenizer.json"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/tokenizer_reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Text string
		IDs  []int64
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for i, tc := range cases {
		got, err := tok.encode(tc.Text)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) == 0 && len(tc.IDs) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tc.IDs) {
			t.Errorf("case %d: %q\ngot %v\nwant %v", i, tc.Text, got, tc.IDs)
		}
	}
}
