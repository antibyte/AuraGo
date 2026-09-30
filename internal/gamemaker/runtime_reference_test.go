package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
)

// A reference must describe the helper version it is attached to: the
// template's descriptor selects it, and every documented member must exist in
// the template or, for re-exporting templates, in the runtime it names.
func TestRuntimeReferencesMatchInstalledTemplates(t *testing.T) {
	for _, template := range []string{"templates/common.ts", "templates/three-common.ts", "templates/voxel-common.ts"} {
		data, err := gameTemplates.ReadFile(template)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		var descriptor struct {
			Version string `json:"version"`
		}
		lines := strings.Split(source, "\n")
		for _, line := range lines[:min(8, len(lines))] {
			if strings.HasPrefix(line, runtimeContractPrefix) {
				_ = json.Unmarshal([]byte(strings.TrimPrefix(line, runtimeContractPrefix)), &descriptor)
			}
		}
		reference, ok := runtimeReferences[descriptor.Version]
		if !ok {
			t.Fatalf("%s version %q has no API reference", template, descriptor.Version)
		}
		definitions := source
		if reference.Source != "" {
			runtime, err := runtimeFS.ReadFile(reference.Source)
			if err != nil {
				t.Fatal(err)
			}
			definitions = string(runtime)
		}
		for _, member := range reference.Members {
			if !strings.Contains(reference.Text, member) {
				t.Errorf("%s reference omits its declared member %q", descriptor.Version, member)
			}
			if !regexp.MustCompile(`\b` + regexp.QuoteMeta(member) + `\s*(\?\.)?\s*[(:=]`).MatchString(definitions) {
				t.Errorf("%s documents %q, which %s does not define", descriptor.Version, member, template)
			}
		}
	}
}

// 2D games read keys through createInputs; documenting a key it does not
// register (F/Q/E) makes inputKeys.down throw "unknown input".
func TestPhaserReferenceListsExactlyTheRegisteredKeys(t *testing.T) {
	data, err := runtimeFS.ReadFile("runtime/aurago-game-1.js")
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`addKeys\('([A-Z,]+)'\)`).FindStringSubmatch(string(data))
	if match == nil {
		t.Fatal("createInputs key registration not found")
	}
	want := "Keys: " + strings.ReplaceAll(match[1], ",", " ") + "."
	if !strings.Contains(runtimeReferences["phaser-2"].Text, want) {
		t.Fatalf("phaser reference lacks %q", want)
	}
}

func TestRuntimeContextCarriesReferenceForInstalledHelper(t *testing.T) {
	service := newTestService(t)
	project := createTestProject(t, service, "2d")
	var runtime map[string]any
	service.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			return service.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"platformer","objective":"Reach the flag","features":["Jump between ledges"]}`))
		}
		runtime = service.RuntimeContext(ctx, run.Job.ID)
		return errors.New("context captured")
	}))
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	waitJob(t, service, job.ID)
	reference, _ := runtime["api_reference"].(string)
	if runtime["version"] != "phaser-2" || !strings.Contains(reference, "class Main extends GameScene") {
		t.Fatalf("runtime context lacks the phaser-2 reference: version=%v", runtime["version"])
	}
}
