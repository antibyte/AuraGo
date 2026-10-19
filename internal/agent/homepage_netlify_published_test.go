package agent

import "testing"

func TestHomepageNetlifyResultPublished(t *testing.T) {
	for _, tc := range []struct {
		name   string
		parsed map[string]interface{}
		want   bool
	}{
		{name: "nil result stays published", parsed: nil, want: true},
		{name: "legacy result without a flag", parsed: map[string]interface{}{"verified": true}, want: true},
		{name: "published deploy", parsed: map[string]interface{}{"published": true}, want: true},
		{name: "draft deploy", parsed: map[string]interface{}{"published": false}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := homepageNetlifyResultPublished(tc.parsed); got != tc.want {
				t.Fatalf("homepageNetlifyResultPublished(%v) = %v, want %v", tc.parsed, got, tc.want)
			}
		})
	}
}
