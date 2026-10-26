package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestRustFSS3LocalesAreComplete(t *testing.T) {
	var english map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/config/sections/en.json"), &english); err != nil {
		t.Fatal(err)
	}
	if english["config.s3.provider"] == "" || english["config.s3.phase_iam"] == "" {
		t.Fatal("missing S3 locale baseline")
	}
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var values map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, fmt.Sprintf("lang/config/sections/%s.json", locale)), &values); err != nil {
			t.Fatal(err)
		}
		for key := range english {
			if strings.HasPrefix(key, "config.s3.") && strings.TrimSpace(values[key]) == "" {
				t.Errorf("%s is missing %s", locale, key)
			}
		}
		if locale != "en" {
			for _, key := range []string{"provider_help", "permissions_help", "prefix_help", "remove_confirm"} {
				if values["config.s3."+key] == english["config.s3."+key] {
					t.Errorf("%s has untranslated S3 help %s", locale, key)
				}
			}
		}
		var dashboard map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, fmt.Sprintf("lang/dashboard/%s.json", locale)), &dashboard); err != nil {
			t.Fatal(err)
		}
		if dashboard["dashboard.rustfs_running_unready"] == "" {
			t.Errorf("%s is missing RustFS dashboard state", locale)
		}
	}
}

func TestRustFSDashboardSeparatesConfiguredAndObservedState(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for the dashboard renderer regression")
	}
	const check = `
const assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs');
const grid={innerHTML:''};
const ctx={document:{getElementById:()=>grid},dashIcon:key=>key,t:key=>key,esc:String,escAttr:String,DashSections:{register(){}}};
vm.createContext(ctx);vm.runInContext(fs.readFileSync(0,'utf8'),ctx);
function render(active,runtime){ctx.renderIntegrations({integrations:{rustfs:active},rustfs:runtime});return grid.innerHTML}
let html=render(true,{ready:false,running:false});
assert.match(html,/class="int-badge active-warning"/);assert.match(html,/href="\/config#s3"/);
assert.match(html,/dashboard.cronjobs_status_enabled/);assert.match(html,/dashboard.daemons_stopped/);
html=render(true,{ready:true,running:true});assert.match(html,/class="int-badge active"/);assert.match(html,/dashboard.agent_banner_status_ready/);
html=render(true,{ready:true,running:false});assert.match(html,/active-warning/);assert.doesNotMatch(html,/dashboard.agent_banner_status_ready/);
html=render(true,{ready:false,running:true});assert.match(html,/dashboard.rustfs_running_unready/);
html=render(true);assert.match(html,/dashboard.overview.pill_unavailable/);assert.match(html,/active-warning/);
html=render(false,{ready:false,running:false});assert.match(html,/class="int-badge inactive"/);assert.match(html,/dashboard.operations_disabled/);
`
	command := exec.Command(node, "-e", check)
	command.Stdin = bytes.NewReader(mustReadUIFile(t, "js/dashboard/sections/system.js"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("dashboard state regression: %v\n%s", err, output)
	}
}
