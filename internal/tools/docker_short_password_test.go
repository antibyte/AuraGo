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

func TestProjectDockerInspectMasksCredentialFlagsAfterShortPassword(t *testing.T) {
	for _, flag := range []string{"--requirepass", "--token", "-p", "-password"} {
		t.Run(flag, func(t *testing.T) {
			args := []string{"client", "-p", flag, "adjacent-credential-sentinel", "--port", "3306"}
			input, err := json.Marshal(map[string]any{"Config": map[string]any{
				"Cmd": args, "Env": []string{"CLIENT_ARGS=" + strings.Join(args, " ")},
			}})
			if err != nil {
				t.Fatal(err)
			}
			projected, err := ProjectDockerInspect(input)
			if err != nil {
				t.Fatal(err)
			}
			config := projected["config"].(map[string]interface{})
			for _, key := range []string{"cmd", "env"} {
				encoded, err := json.Marshal(config[key])
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(encoded), "adjacent-credential-sentinel") {
					t.Errorf("%s exposed the credential after %q", key, flag)
				}
				if key == "cmd" && !strings.Contains(string(encoded), "3306") {
					t.Errorf("%s lost the unambiguous port value", key)
				}
			}
		})
	}
}

func TestProjectDockerInspectMasksEntireEnvValueWithCredentialFlags(t *testing.T) {
	for _, value := range []string{
		`-p 'quoted-password-head tail-sentinel' --port 3306`,
		`-p"quoted-password-head tail-sentinel" --port 3306`,
		`"-p" 'quoted-password-head tail-sentinel' --port 3306`,
		`--requirepass "quoted-password-head tail-sentinel" --port 3306`,
		`"--requirepass"="quoted-password-head tail-sentinel" --port 3306`,
		`"--requirepass" "quoted-password-head tail-sentinel" --port 3306`,
	} {
		t.Run(value, func(t *testing.T) {
			input, err := json.Marshal(map[string]any{"Config": map[string]any{"Env": []string{
				"CLIENT_ARGS=" + value, "PUBLIC_ARGS=--port 3306 --verbose", "PLAIN_TEXT=two words kept",
			}}})
			if err != nil {
				t.Fatal(err)
			}
			projected, err := ProjectDockerInspect(input)
			if err != nil {
				t.Fatal(err)
			}
			env := projected["config"].(map[string]interface{})["env"].([]interface{})
			if env[0] != "CLIENT_ARGS="+dockerInspectRedacted {
				t.Error("credential-bearing environment value was not completely masked")
			}
			if env[1] != "PUBLIC_ARGS=--port 3306 --verbose" || env[2] != "PLAIN_TEXT=two words kept" {
				t.Error("flag-free environment values changed")
			}
		})
	}
}
