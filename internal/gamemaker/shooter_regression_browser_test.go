package gamemaker

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// Run the production test driver against real Phaser collisions. The delayed
// semi-automatic fixture reproduces a shot expiring before the first spawn.
func TestShooterBrowserDelayedSemiAutomatic(t *testing.T) {
	if os.Getenv("GAMEMAKER_SHOOTER_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_SHOOTER_BROWSER=1 for real Chrome hit checks")
	}
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	l := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer l.Cleanup()
	defer browser.Close()
	for _, mode := range []string{"semi", "automatic", "broken_collision"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			project := Project{Name: "Delayed shooter", Dimension: "2d"}
			if err := WriteScaffold(root, project); err != nil {
				t.Fatal(err)
			}
			plan := ExampleGamePlan(project)
			plan.Template, plan.Assets = "shooter", nil
			if err := installGameTemplate(root, plan); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "src/main.ts")
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			source = bytes.Replace(source, []byte("this.spawn();"), []byte("this.time.delayedCall(2000, () => this.spawn());"), 1)
			source = bytes.Replace(source, []byte("tick() { if (this.builder) return; this.spawn(); }"), []byte("tick() {}"), 1)
			if mode != "automatic" {
				source = bytes.Replace(source, []byte("if (this.inputKeys.down('SPACE')) this.action();"), nil, 1)
			}
			if mode == "broken_collision" {
				source = bytes.Replace(source, []byte("this.physics.add.overlap(this.shots, this.enemies,"), []byte("this.physics.add.overlap(this.shots, this.physics.add.group(),"), 1)
			}
			if err := os.WriteFile(path, source, 0o600); err != nil {
				t.Fatal(err)
			}
			if result := buildDirectory(context.Background(), root, 150, 64<<20); !result.OK {
				t.Fatal(result.Diagnostics)
			}
			server := httptest.NewServer(http.FileServer(http.Dir(root)))
			defer server.Close()
			page := browser.MustPage(server.URL + "/#gm-channel=shooter-fixture").MustWaitLoad()
			defer page.Close()
			page.MustSetViewport(960, 540, 1, false)
			if err := page.Timeout(10 * time.Second).Wait(rod.Eval("()=>!!window.__AURAGO_GAME_TEST__")); err != nil {
				t.Fatal(err)
			}
			driver, err := runtimeFS.ReadFile("runtime/preview-tests.js")
			if err != nil {
				t.Fatal(err)
			}
			if err := page.AddScriptTag("", string(driver)); err != nil {
				t.Fatal(err)
			}
			scenarios := []GameScenario{requiredScenarios("shooter")[2]}
			if mode == "semi" {
				scenarios = append([]GameScenario{{ID: "old_hold", Metric: "hits", Compare: "increased", Steps: []GameTestStep{{Action: "key", Key: "SPACE", MS: 2400}}}}, scenarios...)
			}
			page.MustEval(`scenarios=>{window.__shooterReport=null;addEventListener('message',e=>{if(e.data?.type==='gameplay')window.__shooterReport=e.data});postMessage({source:'aurago-studio',channel:'shooter-fixture',type:'run-tests',scenarios},'*')}`, scenarios)
			if err := page.Timeout(25 * time.Second).Wait(rod.Eval("()=>!!window.__shooterReport")); err != nil {
				t.Fatal(err)
			}
			var report PreviewReport
			if err := json.Unmarshal([]byte(page.MustEval("()=>JSON.stringify(window.__shooterReport)").Str()), &report); err != nil {
				t.Fatal(err)
			}
			checks := compareGameObservations(scenarios, report.Observations)
			want := "passed"
			if mode == "broken_collision" {
				want = "failed"
			}
			if checks[len(checks)-1].Status != want || mode == "semi" && checks[0].Status != "unavailable" {
				t.Fatalf("%s: %+v", mode, checks)
			}
			t.Logf("%s: %+v", mode, checks)
		})
	}
}
