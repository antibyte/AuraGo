package storymaker

import (
	"os"
	"testing"
)

// loadAgentFixture reads a story of testdata/stories, decodes it and applies the defaults, as the job's check sees it.
func loadAgentFixture(t *testing.T, name string) Story {
	t.Helper()
	data, err := os.ReadFile("testdata/stories/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	s := mustDecode(t, string(data))
	ApplyDefaults(&s)
	return s
}

// The two agent fixtures are real stories: they decode, and the real catalog accepts them.
func TestAgentFixturesValidateAgainstTheRealCatalog(t *testing.T) {
	cat := loadReal(t)
	for _, name := range []string{"agent-firewall-run", "agent-varied"} {
		s := loadAgentFixture(t, name)
		if issues := Validate(&s, cat, DefaultLimits()); HasErrors(issues) {
			t.Errorf("%s: %s", name, dump(issues))
		}
	}
}
