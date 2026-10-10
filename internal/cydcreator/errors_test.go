package cydcreator

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestErrorFormatting(t *testing.T) {
	cases := []struct {
		err  *Error
		want string
	}{
		{NewError(CodeBadRequest, "unknown board", nil), "CYD_BAD_REQUEST: unknown board"},
		{NewError(CodeBusy, "", nil), "CYD_BUSY"},
		{nil, "<nil>"},
	}
	for _, tc := range cases {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("Error() = %q, want %q", got, tc.want)
		}
	}
}

func TestErrorCode(t *testing.T) {
	base := NewError(CodeInvalidLayout, "bad", map[string]any{"index": 2})
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"direct", base, CodeInvalidLayout},
		{"wrapped", fmt.Errorf("apply: %w", base), CodeInvalidLayout},
		{"joined", errors.Join(errors.New("x"), base), CodeInvalidLayout},
		{"plain", errors.New("plain"), ""},
		{"nil", nil, ""},
		{"typed nil", (*Error)(nil), ""},
	}
	for _, tc := range cases {
		if got := ErrorCode(tc.err); got != tc.want {
			t.Errorf("%s: ErrorCode = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestErrorCodesComplete(t *testing.T) {
	codes := ErrorCodes()
	if len(codes) != 33 {
		t.Fatalf("ErrorCodes() has %d codes, want 33 (overview error table)", len(codes))
	}
	seen := map[string]bool{}
	for i, c := range codes {
		if !strings.HasPrefix(c, "CYD_") {
			t.Errorf("code %q lacks CYD_ prefix", c)
		}
		if seen[c] {
			t.Errorf("duplicate code %q", c)
		}
		seen[c] = true
		if i > 0 && codes[i-1] > c {
			t.Errorf("codes not sorted at %d", i)
		}
	}
	codes[0] = "MUTATED"
	if ErrorCodes()[0] == "MUTATED" {
		t.Fatal("ErrorCodes must return a copy")
	}
}
