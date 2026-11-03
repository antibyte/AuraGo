package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectDockerInspectMasksShortPasswordArguments(t *testing.T) {
	for _, args := range [][]string{
		{"mysql", "-p", "short-password-sentinel", "--port", "3306"},
		{"mysql", "-p=short-password-sentinel", "--port", "3306"},
		{"mysql", "-pshort-password-sentinel", "--port", "3306"},
		{"mysql", "-pshort-password-sentinel_TOKEN"},
		{"mysql", "-pshort-password-sentinel_TOKEN=value"},
		{"mysql", "-password", "short-password-sentinel", "--port", "3306"},
		{"service", "-p", "8080", "--port", "3306"},
	} {
		input, err := json.Marshal(map[string]any{"Config": map[string]any{"Cmd": args,
			"Env": []string{"CLIENT_ARGS=-p short-password-sentinel --port 3306"}}})
		if err != nil {
			t.Fatal(err)
		}
		projected, err := ProjectDockerInspect(input)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(projected)
		if err != nil {
			t.Fatal(err)
		}
		for _, leaked := range []string{"short-password-sentinel", "8080"} {
			if strings.Contains(string(encoded), leaked) {
				t.Errorf("inspect exposed a short -p value for %q", args)
			}
		}
		if strings.Contains(strings.Join(args, " "), "--port") && (!strings.Contains(string(encoded), "--port") || !strings.Contains(string(encoded), "3306")) {
			t.Error("inspect lost the unambiguous long port flag")
		}
	}
}
