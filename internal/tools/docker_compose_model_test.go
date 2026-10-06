package tools

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The agent preflight used its own copy of this decoder; the shared helper must
// read depends_on, networks, secrets and configs exactly like it did.
func TestDockerComposeRefNamesReadsMapsListsAndSources(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
	}{
		{`{"db":{"condition":"service_healthy","required":true},"cache":{}}`, []string{"cache", "db"}},
		{`["web","api"]`, []string{"web", "api"}},
		{`[{"source":"npmrc","target":"/run/npmrc"},"token",{"target":"x"}]`, []string{"npmrc", "token"}},
		{`null`, nil},
		{``, nil},
		{`42`, nil},
	}
	for _, tc := range cases {
		got := DockerComposeRefNames(json.RawMessage(tc.raw))
		if len(got) == 0 && len(tc.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("DockerComposeRefNames(%s) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestSortedDockerComposeKeysSortsAnyModelMap(t *testing.T) {
	if got := SortedDockerComposeKeys(map[string]int{"b": 1, "a": 2, "c": 3}); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("SortedDockerComposeKeys() = %q", got)
	}
	if got := SortedDockerComposeKeys(map[string]DockerComposeService(nil)); len(got) != 0 {
		t.Fatalf("SortedDockerComposeKeys(nil) = %q", got)
	}
}
