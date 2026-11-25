package homedash_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"aurago/internal/homedash"
)

func TestCommandSourceUnavailableKeepsSourceAndIsAuditedAsFailed(t *testing.T) {
	fa := gateSource()
	fa.CommandErr = homedash.SourceUnavailable("fa", "the source is not reachable")
	h := newTestHub(t, nil, fa)
	var outcomes []string
	homedash.SetAuditForTest(h, func(_ context.Context, _ homedash.Command, outcome string) { outcomes = append(outcomes, outcome) })
	_, err := h.Command(context.Background(), cmd("fa:light", homedash.CapToggle, nil, session("s")))
	var he *homedash.Error
	if !errors.As(err, &he) || he.Code != homedash.CodeSourceUnavailable || he.Source != "fa" {
		t.Fatalf("got %v", err)
	}
	if !errors.Is(err, &homedash.Error{Code: homedash.CodeSourceUnavailable}) {
		t.Fatal("errors.Is must still match by code")
	}
	if !slices.Equal(outcomes, []string{"failed"}) {
		t.Fatalf("outcomes %v, want [failed]", outcomes)
	}
	raw, _ := json.Marshal(he)
	if !strings.Contains(string(raw), `"source":"fa"`) {
		t.Fatalf("json %s", raw)
	}
	plain, _ := json.Marshal(&homedash.Error{Code: homedash.CodeCommandFailed})
	if strings.Contains(string(plain), "source") {
		t.Fatalf("source must be omitted when empty: %s", plain)
	}
}
