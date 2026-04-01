package agent

import "testing"

func TestAssessMemoryEffectiveness(t *testing.T) {
	candidates := map[string]string{
		"doc-useful":  "Docker deployment for Nextcloud with reverse proxy",
		"doc-useless": "Weekly groceries and kitchen reminders",
	}

	useful, useless := assessMemoryEffectiveness("Use Docker deployment for Nextcloud behind the reverse proxy.", candidates)
	if len(useful) != 1 || useful[0] != "doc-useful" {
		t.Fatalf("useful = %v, want [doc-useful]", useful)
	}
	if len(useless) != 1 || useless[0] != "doc-useless" {
		t.Fatalf("useless = %v, want [doc-useless]", useless)
	}
}
