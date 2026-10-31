package ui

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type w4vLeaf struct {
	Path  string `json:"path"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

func w4vLeaves(prefix string, data map[string]any, out *[]w4vLeaf) {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		switch value := data[key].(type) {
		case map[string]any:
			w4vLeaves(path, value, out)
		case int:
			*out = append(*out, w4vLeaf{path, "int", value})
		case float64:
			*out = append(*out, w4vLeaf{path, "float", value})
		case string:
			*out = append(*out, w4vLeaf{path, "string", value})
		}
	}
}

// The harness loads the real catalog and state modules and, for every scalar of config_template.yaml,
// asks for the derived rule and checks the template's own value against it.
const w4vRulesHarness = `
const fs = require('fs');
const vm = require('vm');
const win = {};
const ctx = vm.createContext({ window: win, document: { dispatchEvent() {} }, CustomEvent: function () {}, URL, console });
vm.runInContext(fs.readFileSync(process.argv[2], 'utf8'), ctx);
vm.runInContext(fs.readFileSync(process.argv[3], 'utf8'), ctx);
const leaves = JSON.parse(fs.readFileSync(process.argv[4], 'utf8'));
const S = win.AuraConfigState;
const derive = win.AuraConfigCatalog.deriveValidationRule;
const rules = {};
leaves.forEach(leaf => { const rule = derive(leaf.path, leaf.type); if (rule) rules[leaf.path] = rule; });
const templateErrors = [];
leaves.forEach(leaf => {
  if (!rules[leaf.path] || leaf.value === '') return;
  S.init({});
  S.setRules({ [leaf.path]: rules[leaf.path] });
  const parts = leaf.path.split('.');
  S.set(leaf.path, leaf.value);
  const v = S.validate();
  if (!v.valid) templateErrors.push(leaf.path + '=' + leaf.value + ' ' + JSON.stringify(v.errors));
});
function check(path, rule, value) {
  S.init({});
  S.setRules({ [path]: rule });
  S.set(path, value);
  const v = S.validate();
  return v.valid ? '' : v.errors.map(e => e.code).join(',');
}
const probes = {
  portHigh: check('a.port', derive('a.port', 'int'), 70000),
  portNegative: check('a.port', derive('a.port', 'int'), -1),
  portZero: check('a.port', derive('a.port', 'int'), 0),
  portOk: check('a.port', derive('a.port', 'int'), 8088),
  urlNoScheme: check('a.base_url', derive('a.base_url', 'string'), 'example.com'),
  urlFtp: check('a.url', derive('a.url', 'string'), 'ftp://example.com'),
  urlWs: check('a.url', derive('a.url', 'string'), 'ws://192.168.6.50/websocket'),
  urlOk: check('a.url', derive('a.url', 'string'), 'https://example.com:8443/x'),
  urlSpace: check('a.url', derive('a.url', 'string'), 'https://exa mple.com'),
  endpointHostPort: check('a.endpoint', derive('a.endpoint', 'string'), 'minio.local:9000'),
  endpointUrl: check('a.endpoint', derive('a.endpoint', 'string'), 'https://s3.amazonaws.com'),
  endpointBad: check('a.endpoint', derive('a.endpoint', 'string'), 'minio local:9000'),
  timeoutNegative: check('a.timeout_seconds', derive('a.timeout_seconds', 'int'), -5),
  timeoutZero: check('a.timeout_seconds', derive('a.timeout_seconds', 'int'), 0),
  timeoutOk: check('a.timeout_seconds', derive('a.timeout_seconds', 'int'), 30),
  bareTimeoutNegative: check('a.request_timeout', derive('a.request_timeout', 'int'), -1),
  intervalFraction: check('a.poll_interval_seconds', derive('a.poll_interval_seconds', 'int'), 2.5),
  floatTimeoutFraction: check('a.timeout_seconds', derive('a.timeout_seconds', 'float'), 2.5),
  explicitWins: win.AuraConfigCatalog.validationRules['server.port'].min,
  noRuleForString: derive('a.timeout_seconds', 'string') === null,
  noRuleForArray: derive('a.retry_intervals', 'array') === null
};
console.log(JSON.stringify({ rules, templateErrors, probes }));
`

func TestConfigDerivedValidationRulesCoverTemplate(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	raw, err := os.ReadFile("../config_template.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var template map[string]any
	if err := yaml.Unmarshal(raw, &template); err != nil {
		t.Fatal(err)
	}
	var leaves []w4vLeaf
	w4vLeaves("", template, &leaves)
	dir := t.TempDir()
	leavesFile := filepath.Join(dir, "leaves.json")
	encoded, _ := json.Marshal(leaves)
	if err := os.WriteFile(leavesFile, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "harness.js")
	if err := os.WriteFile(script, []byte(w4vRulesHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog, _ := filepath.Abs("js/config/catalog.js")
	state, _ := filepath.Abs("js/config/state.js")
	out, err := exec.Command(node, script, catalog, state, leavesFile).CombinedOutput()
	if err != nil {
		t.Fatalf("node harness failed: %v\n%s", err, out)
	}
	var result struct {
		Rules          map[string]map[string]any `json:"rules"`
		TemplateErrors []string                  `json:"templateErrors"`
		Probes         map[string]any            `json:"probes"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("harness output: %v\n%s", err, out)
	}

	// Independent expectation, written from the plan (E14), not from the implementation.
	portName := regexp.MustCompile(`(^|_)port(_|$)`)
	unitName := regexp.MustCompile(`_(seconds|secs|sec|ms|minutes)$`)
	bareName := regexp.MustCompile(`(^|_)(timeout|interval)(_|$)`)
	urlName := regexp.MustCompile(`(^|_)url$`)
	counts := map[string]int{}
	for _, leaf := range leaves {
		name := leaf.Path[strings.LastIndex(leaf.Path, ".")+1:]
		rule := result.Rules[leaf.Path]
		numeric := leaf.Type == "int" || leaf.Type == "float"
		wantKind := ""
		switch {
		case numeric && portName.MatchString(name):
			wantKind = "port"
		case numeric && (unitName.MatchString(name) || bareName.MatchString(name)):
			wantKind = "time"
		case leaf.Type == "string" && (urlName.MatchString(name) || name == "endpoint"):
			wantKind = "url"
		}
		if wantKind == "" {
			if rule != nil {
				t.Errorf("%s (%s) must not get a derived rule: %v", leaf.Path, leaf.Type, rule)
			}
			continue
		}
		counts[wantKind]++
		if rule == nil {
			t.Errorf("%s (%s) should get a %s rule", leaf.Path, leaf.Type, wantKind)
			continue
		}
		if rule["changedOnly"] != true {
			t.Errorf("%s: derived rules must be changedOnly so untouched saved values never block a save", leaf.Path)
		}
		switch wantKind {
		case "port":
			if rule["min"] != float64(0) || rule["max"] != float64(65535) || rule["integer"] != true {
				t.Errorf("%s: port rule %v", leaf.Path, rule)
			}
		case "time":
			if rule["min"] != float64(0) {
				t.Errorf("%s: time rule %v", leaf.Path, rule)
			}
			if unitName.MatchString(name) && rule["unit"] == nil {
				t.Errorf("%s: a unit suffix in the name must show a unit", leaf.Path)
			}
		case "url":
			if rule["type"] != "url" {
				t.Errorf("%s: url rule %v", leaf.Path, rule)
			}
		}
	}
	for kind, minimum := range map[string]int{"port": 20, "time": 40, "url": 20} {
		if counts[kind] < minimum {
			t.Errorf("only %d %s paths found in the template, expected at least %d", counts[kind], kind, minimum)
		}
	}
	if len(result.TemplateErrors) > 0 {
		t.Errorf("the template's own values fail their derived rule (rule too strict for the backend):\n%s", strings.Join(result.TemplateErrors, "\n"))
	}
	want := map[string]any{
		"portHigh": "max", "portNegative": "min", "portZero": "", "portOk": "",
		"urlNoScheme": "url", "urlFtp": "url", "urlWs": "", "urlOk": "", "urlSpace": "url",
		"endpointHostPort": "", "endpointUrl": "", "endpointBad": "url",
		"timeoutNegative": "min", "timeoutZero": "", "timeoutOk": "", "bareTimeoutNegative": "min",
		"intervalFraction": "integer_required", "floatTimeoutFraction": "",
		"explicitWins": float64(1), "noRuleForString": true, "noRuleForArray": true,
	}
	for name, expected := range want {
		if result.Probes[name] != expected {
			t.Errorf("probe %s = %v, want %v", name, result.Probes[name], expected)
		}
	}
}

func TestConfigValidationMessagesCarryRangeAndScheme(t *testing.T) {
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var bundle map[string]string
		if err := json.Unmarshal([]byte(wave0Read(t, "lang/config/common/"+lang+".json")), &bundle); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(bundle["config.precision.validation_min"], "{min}") {
			t.Errorf("%s: validation_min must name the minimum", lang)
		}
		if !strings.Contains(bundle["config.precision.validation_max"], "{max}") {
			t.Errorf("%s: validation_max must name the maximum", lang)
		}
		if !strings.Contains(bundle["config.precision.validation_url"], "https://") {
			t.Errorf("%s: validation_url must show a URL with its scheme", lang)
		}
	}
}
