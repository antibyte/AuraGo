package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func TestGoldfishAppRegistration(t *testing.T) {
	t.Parallel()
	found := false
	for _, app := range desktop.BuiltinApps() {
		if app.ID != "goldfish-adventure" {
			continue
		}
		found = true
		if app.Category != "games" || app.Entry != "builtin://goldfish-adventure" || app.Metadata["logo_path"] != "/img/goldfish-adventure/icon.png" {
			t.Fatalf("incomplete game registration: %+v", app)
		}
	}
	if !found {
		t.Fatal("game missing from built-in catalog")
	}
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	previous := strings.Index(loader, "'goldfish-adventure': {")
	if previous < 0 {
		t.Fatal("game lazy loader missing")
	}
	for _, name := range []string{"content", "content-act1", "content-act2", "content-act3", "content-act4", "state", "save", "renderer", "interactions", "app"} {
		path := "/js/desktop/apps/goldfish-adventure/" + name + ".js"
		index := strings.Index(loader, path)
		if index <= previous {
			t.Fatalf("missing or misordered module %s", name)
		}
		previous = index
		if _, err := os.Stat(filepath.FromSlash(strings.TrimPrefix(path, "/"))); err != nil {
			t.Fatal(err)
		}
	}
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var words map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+locale+".json")), &words); err != nil {
			t.Fatal(err)
		}
		if words["desktop.app_goldfish_adventure"] == "" {
			t.Fatalf("%s launcher title missing", locale)
		}
	}
}

func goldfishDismissDialogue(page *rod.Page) {
	page.MustEval(`() => {
        for (let n = 0; n < 100 && !document.querySelector('.agfish-dialogue').hidden; n++)
            document.querySelector('.agfish-dialogue .agfish-primary').click();
    }`)
}

func goldfishUse(page *rod.Page, hotspot, action string) {
	page.MustEval(`(hotspot) => document.querySelector('[data-agfish-hotspot="' + hotspot + '"]').click()`, hotspot)
	page.MustWait(`() => !document.querySelector('.agfish-app').classList.contains('agfish-busy')`)
	page.MustEval(`(action) => document.querySelector('[data-agfish-action="' + action + '"]').click()`, action)
	page.MustWait(`() => !document.querySelector('.agfish-app').classList.contains('agfish-busy')`)
}

func TestGoldfishAdventureOpeningBrowser(t *testing.T) {
	page := auraDriftFixture(t)
	page = page.Timeout(45 * time.Second)
	page.MustSetViewport(1440, 1000, 1, false)
	page.MustEval(`async () => { await fixtureOpen('goldfish-adventure'); }`)
	page.MustWait(`() => !!document.querySelector('.agfish-start .agfish-primary') && !document.querySelector('.agfish-app').classList.contains('agfish-art-loading')`)
	if base := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); base != "" {
		if err := os.MkdirAll(base, 0755); err != nil {
			t.Fatal(err)
		}
		page.MustScreenshot(filepath.Join(base, "goldfish-before-start.png"))
	}
	page.MustElement(".agfish-start .agfish-primary:not([hidden])").MustClick()
	page.MustWait(`() => !document.querySelector('.agfish-app').classList.contains('agfish-art-loading')`)
	goldfishDismissDialogue(page)
	goldfishUse(page, "conveyor", "r01_stop")
	page.MustWait(`() => !document.querySelector('[data-agfish-action="r01_stop"]')`)
	goldfishDismissDialogue(page)
	goldfishUse(page, "grabber", "r01_clamp")
	page.MustWait(`() => !!document.querySelector('[data-agfish-item="tool_clamp"]')`)
	goldfishDismissDialogue(page)
	goldfishUse(page, "magnet", "r01_brake")
	page.MustWait(`() => !!document.querySelector('.agfish-challenge')`)
	page.MustEval(`() => {
        const record = GoldfishContent.actions.find(a => a.id === 'r01_brake');
        for (const id of record.challenge.answer) {
            const label = GoldfishContent.texts[record.challenge.options.find(o => o.id === id).label];
            [...document.querySelectorAll('.agfish-choices button')].find(b => b.textContent === label).click();
        }
        document.querySelector('.agfish-challenge .agfish-primary').click();
    }`)
	page.MustWait(`() => !document.querySelector('.agfish-app').classList.contains('agfish-busy')`)
	goldfishDismissDialogue(page)
	goldfishUse(page, "gate", "r01_open")
	page.MustWait(`() => !document.querySelector('[data-agfish-action="r01_open"]')`)
	goldfishDismissDialogue(page)
	page.MustElement("[data-agfish-exit=R02]").MustClick()
	page.MustWait(`() => document.querySelector('.agfish-app').dataset.room === 'R02' && !document.querySelector('.agfish-app').classList.contains('agfish-art-loading')`)
	goldfishDismissDialogue(page)
	if messages := page.MustEval(`() => fixtureErrors.join('\n')`).Str(); messages != "" {
		t.Fatal(messages)
	}
	base := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if base != "" {
		if err := os.MkdirAll(base, 0755); err != nil {
			t.Fatal(err)
		}
		page.MustScreenshot(filepath.Join(base, "goldfish-standard.png"))
	}
	page.MustEval(`() => fixtureTheme('fruity-light')`)
	page.MustWait(`() => document.querySelector('.agfish-canvas').width > 0`)
	if base != "" {
		page.MustScreenshot(filepath.Join(base, "goldfish-fruity.png"))
	}
	page.MustEval(`() => [...aurora.state.windows.keys()].forEach(id => aurora.closeWindow(id))`)
	page.MustWait(`() => !document.querySelector('.agfish-app')`)
	page.MustEval(`async () => { await fixtureOpen('goldfish-adventure'); }`)
	page.MustWait(`() => document.querySelector('.agfish-app')?.dataset.room === 'R02'`)
	if page.MustEval(`() => !document.querySelector('.agfish-start') || document.querySelector('.agfish-start').hidden`).Bool() {
		t.Fatal("reopening must show the start/continue screen without advancing the game")
	}
}

func TestGoldfishAdventureScenesBrowser(t *testing.T) {
	page := auraDriftFixture(t).Timeout(120 * time.Second)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustSetViewport(1440, 1000, 1, false)
	page.MustEval(`async () => { await fixtureOpen('goldfish-adventure'); }`)
	page.MustWait(`() => !!window.GoldfishRenderer && !!document.querySelector('.agfish-start')`)
	page.MustEval(`() => {
        const overlay = document.createElement('div');
        overlay.id = 'goldfish-review-overlay';
        overlay.style.cssText = 'position:fixed;inset:0;background:#18202a;z-index:2147483647;display:grid;place-items:center';
        const canvas = document.createElement('canvas');
        canvas.style.cssText = 'width:1340px;aspect-ratio:16/9;max-width:100vw';
        overlay.append(canvas); document.body.append(overlay);
        window.goldfishMomentStarted = false;
        window.goldfishStage = GoldfishRenderer.create(canvas, {onMoment: active => { if (active) window.goldfishMomentStarted = true; }});
        window.goldfishSnapshots = {};
        const C = GoldfishContent, S = GoldfishState;
        const path = (state, destination) => {
            const queue = [[state.room]], seen = new Set([state.room]);
            while (queue.length) {
                const route = queue.shift(), current = route.at(-1);
                if (current === destination) return route.slice(1);
                for (const exit of C.rooms.find(room => room.id === current).exits) {
                    if (seen.has(exit.to) || !S.conditions(state, exit)) continue;
                    seen.add(exit.to); queue.push(route.concat(exit.to));
                }
            }
            return null;
        };
        let state = S.create();
        goldfishSnapshots[state.room] = structuredClone(state);
        for (let pass = 0; pass < C.actions.length && !state.ending; pass++) {
            let changed = false;
            for (const action of C.actions) {
                if (action.repeatable || state.applied.includes(action.id)) continue;
                const route = path(state, action.room);
                if (route === null || !S.available({...state,room:action.room}, action)) continue;
                for (const room of route) {
                    state = S.move(state, room).state;
                    goldfishSnapshots[room] = structuredClone(state);
                }
                state = S.apply(state, action.id, {item:action.item,answer:action.challenge?.answer}).state;
                goldfishSnapshots[state.room] = structuredClone(S.validate(state));
                changed = true;
            }
            if (!changed) break;
        }
        for (const room of path(state, 'R16') || []) {
            state = S.move(state, room).state;
            goldfishSnapshots[room] = structuredClone(S.validate(state));
        }
        if (Object.keys(goldfishSnapshots).length !== 16) throw new Error('Scene journey missing: ' + C.rooms.filter(room => !goldfishSnapshots[room.id]).map(room => room.id));
    }`)
	base := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if base != "" {
		if err := os.MkdirAll(base, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, room := range []string{"R01", "R02", "R03", "R04", "R05", "R06", "R07", "R08", "R09", "R10", "R11", "R12", "R13", "R14", "R15", "R16"} {
		page.MustEval(`async room => { await goldfishStage.load(room, goldfishSnapshots[room]); goldfishStage.setPaused(false); await new Promise(requestAnimationFrame); goldfishStage.setPaused(true); }`, room)
		if base != "" {
			page.MustScreenshot(filepath.Join(base, "scene-"+room+".png"))
		}
	}
	for _, action := range []string{"r01_open", "r03_travel", "r04_depart", "r06_read", "r08_agreement", "r09_old_test", "r09_new_test", "r10_test", "r11_restore", "r13_trace", "r15_test", "r15_end_keys", "r15_end_custody", "r15_end_purge"} {
		page.MustEval(`async id => {
            const action = GoldfishContent.actions.find(value => value.id === id);
            await goldfishStage.load(action.room, goldfishSnapshots[action.room]);
            goldfishStage.setPaused(false);
            window.goldfishMomentStarted = false;
            window.goldfishMoment = goldfishStage.act(id);
            await new Promise(resolve => setTimeout(resolve, 950));
            goldfishStage.setPaused(true);
        }`, action)
		if base != "" {
			page.MustScreenshot(filepath.Join(base, "moment-"+action+".png"))
		}
		if !page.MustEval(`async () => { goldfishStage.skipMoment(); return goldfishMomentStarted && await goldfishMoment; }`).Bool() {
			t.Fatalf("cinematic %s did not settle after skipping", action)
		}
	}
	page.MustEval(`() => goldfishStage.dispose()`)
	page.MustEval(`async () => {
        document.getElementById('goldfish-review-overlay').remove();
        const store = await GoldfishSave.open();
        const prior = await store.read('auto');
        // This visual fixture represents an already-read journey to the ship.
        goldfishSnapshots.R05.seenLines = [...new Set(goldfishSnapshots.R05.seenLines.concat(GoldfishState.pendingResults(goldfishSnapshots.R05).map(line => line.text)))];
        await store.write('auto', goldfishSnapshots.R05, {expectedRevision: prior?.revision || 0});
        store.close();
        for (const id of [...aurora.state.windows.keys()]) aurora.closeWindow(id);
    }`)
	page.MustWait(`() => !document.querySelector('.agfish-app')`)
	page.MustEval(`async () => { await fixtureOpen('goldfish-adventure'); }`)
	page.MustWait(`() => document.querySelector('.agfish-app')?.dataset.room === 'R05' && !document.querySelector('.agfish-app').classList.contains('agfish-art-loading')`)
	page.MustElement(".agfish-start .agfish-primary:not([hidden])").MustClick()
	if base != "" {
		page.MustScreenshot(filepath.Join(base, "goldfish-ship-gameplay.png"))
	}
	if messages := page.MustEval(`() => fixtureErrors.join('\n')`).Str(); messages != "" {
		t.Fatal(messages)
	}
}

func TestGoldfishAdventureLocaleLayoutBrowser(t *testing.T) {
	page := auraDriftFixture(t).Timeout(180 * time.Second)
	page.MustSetViewport(1280, 900, 1, false)
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var story, controls map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/goldfish/"+locale+".json")), &story); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/goldfish-interface/"+locale+".json")), &controls); err != nil {
			t.Fatal(err)
		}
		page.MustEval(`async locale => { window.SYSTEM_LANG = locale; document.documentElement.lang = locale; await fixtureOpen('goldfish-adventure'); }`, locale)
		page.MustWait(`() => !!document.querySelector('.agfish-start') && !document.querySelector('.agfish-app').classList.contains('agfish-art-loading')`)
		if !page.MustEval(`(title, settings) => GoldfishContent.texts['goldfish.R01.title'] === title && [...document.querySelectorAll('.agfish-menu button')].some(button => button.textContent === settings)`, story["goldfish.R01.title"], controls["goldfish.ui.settings"]).Bool() {
			t.Fatalf("%s story/interface locale was not selected", locale)
		}
		page.MustElement(".agfish-start .agfish-primary:not([hidden])").MustClick()
		page.MustWait(`() => !document.querySelector('.agfish-busy')`)
		page.MustEval(`() => {
            const root = document.querySelector('.agfish-app'), win = root.closest('.vd-window');
            win.style.width = '700px'; win.style.height = '660px';
            const longest = Object.entries(GoldfishContent.texts).filter(([key]) => /\.(line|look|entry)\./.test(key)).sort((a,b) => b[1].length-a[1].length)[0][1];
            document.querySelector('.agfish-line').textContent = longest;
            document.querySelector('.agfish-dialogue').hidden = false;
        }`)
		page.MustEval(`async () => { await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))); }`)
		if result := page.MustEval(`() => {
            const root = document.querySelector('.agfish-app'), line = root.querySelector('.agfish-line');
            return {root: root.scrollWidth-root.clientWidth, line:line.scrollWidth-line.clientWidth, text:line.textContent};
        }`); result.Get("root").Int() > 1 || result.Get("line").Int() > 1 || result.Get("text").Str() == "" {
			t.Fatalf("%s layout overflow: %s", locale, result.JSON("", ""))
		}
		if base := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); base != "" {
			if err := os.MkdirAll(base, 0755); err != nil {
				t.Fatal(err)
			}
			page.MustScreenshot(filepath.Join(base, "layout-"+locale+".png"))
		}
		page.MustEval(`() => [...aurora.state.windows.keys()].forEach(id => aurora.closeWindow(id))`)
		page.MustWait(`() => !document.querySelector('.agfish-app')`)
	}
	if messages := page.MustEval(`() => fixtureErrors.join('\n')`).Str(); messages != "" {
		t.Fatal(messages)
	}
}
