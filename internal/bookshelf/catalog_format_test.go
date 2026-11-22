package bookshelf

import "testing"

func TestValidFormatAcceptsConvertibleInputs(t *testing.T) {
	tests := []struct {
		format string
		want   bool
	}{
		{"epub", true},
		{"fb2.zip", true},
		{"lit", true},
		{"pdb", true},
		{"lrf", true},
		{"exe", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			if got := validFormat(tc.format); got != tc.want {
				t.Fatalf("validFormat(%q) = %v, want %v", tc.format, got, tc.want)
			}
		})
	}
}
