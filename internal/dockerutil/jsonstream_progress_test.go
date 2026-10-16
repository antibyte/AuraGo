package dockerutil

import (
	"errors"
	"strings"
	"testing"
)

func TestPullProgressPreservesStreamFailure(t *testing.T) {
	stream := `{"id":"layer","status":"Downloading","progressDetail":{"current":3,"total":10}}
{"id":"layer","status":"Extracting","progressDetail":{"current":5,"total":10}}
{"errorDetail":{"message":"denied"}}
`
	calls := 0
	err := DrainJSONMessagesWithProgress(strings.NewReader(stream), func(id string, current, total int64) {
		calls++
		if id != "layer" || current != 3 || total != 10 {
			t.Fatalf("invalid progress: %s %d/%d", id, current, total)
		}
	})
	var failure *JSONMessageError
	if calls != 1 || !errors.As(err, &failure) || failure.Message != "denied" {
		t.Fatalf("calls=%d, error=%v", calls, err)
	}
}
