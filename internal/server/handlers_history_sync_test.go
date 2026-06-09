package server

import "testing"

func TestShouldAppendHistoryMessage(t *testing.T) {
	tests := []struct {
		name string
		id   int64
		err  error
		want bool
	}{
		{name: "success", id: 42, err: nil, want: true},
		{name: "insert error", id: -1, err: errHistorySyncTest, want: false},
		{name: "invalid id", id: 0, err: nil, want: false},
		{name: "negative id", id: -1, err: nil, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldAppendHistoryMessage(tc.id, tc.err); got != tc.want {
				t.Fatalf("shouldAppendHistoryMessage(%d, err=%v) = %v, want %v", tc.id, tc.err, got, tc.want)
			}
		})
	}
}

var errHistorySyncTest = &historySyncTestError{}

type historySyncTestError struct{}

func (historySyncTestError) Error() string { return "insert failed" }