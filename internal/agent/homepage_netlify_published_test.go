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

// The deployment ledger requires a result that is both verified and published.
// A draft deploy can pass the permalink check and must still stay out of it.
func TestHomepageNetlifyVerifiedDraftIsNeverLedgerWorthy(t *testing.T) {
	draft := map[string]interface{}{
		"verified":     true,
		"verified_url": "https://deploy-123--site.netlify.app",
		"published":    false,
	}
	if !homepageNetlifyResultVerified(draft) {
		t.Fatal("a verified draft must still report its permalink verification")
	}
	if homepageNetlifyResultVerified(draft) && homepageNetlifyResultPublished(draft) {
		t.Fatal("a verified draft must never enter the deployment ledger")
	}

	published := map[string]interface{}{
		"verified":     true,
		"verified_url": "https://site.netlify.app",
		"published":    true,
	}
	if !homepageNetlifyResultVerified(published) || !homepageNetlifyResultPublished(published) {
		t.Fatal("a verified published deploy must enter the deployment ledger")
	}
}
