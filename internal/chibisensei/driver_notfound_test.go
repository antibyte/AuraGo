package chibisensei

import (
	"errors"
	"fmt"
	"testing"

	"aurago/internal/kaggle"
)

func TestIsKaggleNotFound(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"403", &kaggle.APIError{Status: 403}, true},
		{"404", &kaggle.APIError{Status: 404}, true},
		{"wrapped 403", fmt.Errorf("read: %w", &kaggle.APIError{Status: 403}), true},
		{"fake not found", fakeNotFound(), true},
		{"401", &kaggle.APIError{Status: 401}, false},
		{"429", &kaggle.APIError{Status: 429}, false},
		{"500", &kaggle.APIError{Status: 500}, false},
		{"uncertain", &kaggle.APIError{Status: 404, Uncertain: true}, false},
		{"nil", nil, false},
		{"plain error", errors.New("boom"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsKaggleNotFound(tc.err); got != tc.want {
				t.Fatalf("IsKaggleNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
