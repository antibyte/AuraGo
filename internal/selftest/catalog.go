// Package selftest owns bounded prompt tests and their private evidence ledger.
package selftest

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"

	promptsembed "aurago/prompts"
)

type Template struct {
	ID      string   `json:"id"`
	Targets []string `json:"targets"`
	Profile string   `json:"profile"`
	Prompt  string   `json:"prompt"`
}

var caseID = regexp.MustCompile(`^[A-Z]+-[0-9]{2}$`)

func Catalog() ([]Template, error) {
	b, err := promptsembed.FS.ReadFile("self_tests/catalog.md")
	if err != nil {
		return nil, err
	}
	return ParseCatalog(string(b))
}

func ParseCatalog(source string) ([]Template, error) {
	var result []Template
	seen := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(source))
	scanner.Buffer(make([]byte, 4096), 32768)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 2 || !caseID.MatchString(strings.TrimSpace(parts[1])) {
			continue
		}
		if len(parts) != 6 {
			return nil, fmt.Errorf("invalid test row")
		}
		t := Template{ID: strings.TrimSpace(parts[1]), Profile: strings.TrimSpace(parts[3]), Prompt: strings.TrimSpace(parts[4])}
		if seen[t.ID] || !strings.Contains("RWXHL", t.Profile) || len(t.Profile) != 1 || len(t.Prompt) < 30 {
			return nil, fmt.Errorf("invalid or duplicate test %s", t.ID)
		}
		for _, name := range strings.Split(parts[2], ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				return nil, fmt.Errorf("empty target in %s", t.ID)
			}
			t.Targets = append(t.Targets, name)
		}
		seen[t.ID] = true
		result = append(result, t)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("empty self-test catalog")
	}
	return result, nil
}
