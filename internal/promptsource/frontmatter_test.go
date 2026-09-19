package promptsource

import "testing"

func TestSharedFrontmatterFraming(t *testing.T) {
	for _, tc := range []struct {
		raw, body string
		valid     bool
	}{
		{"plain markdown", "plain markdown", true}, {"---\n---\nbody", "body", true}, {"---\nname: test\n---", "", true},
		{"---\nname: test\n---\nbody", "body", true}, {"---\nname: invalid: value\n---\nbody", "", false}, {"---\nname: test", "", false},
	} {
		_, body, _, err := Split(tc.raw)
		if (err == nil) != tc.valid || (tc.valid && body != tc.body) {
			t.Errorf("%q: body=%q err=%v", tc.raw, body, err)
		}
	}
}
