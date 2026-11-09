package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDesktopCodeStudioNode(t *testing.T) {
	page, backend := newCodeStudioBrowser(t)
	backend.mu.Lock()
	backend.files["/workspace/hello.go"] = "package main\nfunc main() {}\n"
	backend.files["/workspace/hello.py"] = "print('Hello from Code Studio')\n"
	backend.files["/workspace/hello.c"] = "#include <stdio.h>\nint main(void) { puts(\"Hello from Code Studio\"); return 0; }\n"
	backend.files["/workspace/hello.js"] = "console.log(\"Hello from Code Studio\");\n"
	backend.files["/workspace/hello.ts"] = "const greeting: string = \"Hello from Code Studio\";\n\nconsole.log(greeting);\n"
	backend.files["/workspace/hello.mjs"] = "export const value = 1;\n"
	backend.files["/workspace/hello.tsx"] = "export const label = \"Hello\";\n"
	backend.mu.Unlock()
	page.MustReload().MustWaitLoad()
	result := page.MustEval(`async()=>{
        await fixtureReady;
        await csWait(()=>csState()?.terminalSessions.length && csState().openTabs.length===5,20000);
        // The terminal socket drops input while it is still connecting, so wait for it.
        await csWait(()=>{const session=csState().terminalSessions[0];return session&&session.ws&&session.ws.readyState===1;},20000);
        const app=CodeStudioApp,target=csState(),id=target.windowId;
        const examples=target.openTabs.map(tab=>tab.path).join(',')==='/workspace/hello.go,/workspace/hello.py,/workspace/hello.c,/workspace/hello.js,/workspace/hello.ts';
        const runner=()=>csRoot().querySelector('.cs-status-message')?.textContent;
        await app.openFile('/workspace/hello.js',true,id);
        const jsTab=target.openTabs[target.activeTabIndex];
        await csWait(()=>jsTab.view.contentDOM.querySelector('span[class]'));
        const jsLanguage=jsTab.language==='javascript';
        await app.command('runCurrentFile',[],id);
        const jsRan=runner()!=='Für diese Datei gibt es keinen Startbefehl';
        await app.openFile('/workspace/hello.ts',true,id);
        const tsTab=target.openTabs[target.activeTabIndex];
        await csWait(()=>tsTab.view.contentDOM.querySelector('span[class]'));
        const tsLanguage=tsTab.language==='typescript';
        await app.command('runCurrentFile',[],id);
        const tsRan=runner()!=='Für diese Datei gibt es keinen Startbefehl';
        await app.openFile('/workspace/hello.mjs',true,id);
        const mjsLanguage=target.openTabs[target.activeTabIndex].language==='javascript';
        await app.command('runCurrentFile',[],id);
        const mjsRan=runner()!=='Für diese Datei gibt es keinen Startbefehl';
        await app.openFile('/workspace/hello.tsx',true,id);
        const tsxLanguage=target.openTabs[target.activeTabIndex].language==='tsx';
        await app.command('runCurrentFile',[],id);
        const tsxRan=runner()!=='Für diese Datei gibt es keinen Startbefehl';
        return examples&&jsLanguage&&jsRan&&tsLanguage&&tsRan&&mjsLanguage&&mjsRan&&tsxLanguage&&tsxRan&&fixtureErrors.length===0;
    }`)
	if !result.Bool() {
		t.Fatalf("Node or TypeScript examples failed: %s", page.MustEval(`()=>fixtureErrors`).JSON("", ""))
	}
	backend.mu.Lock()
	execs := append([]string(nil), backend.execs...)
	backend.mu.Unlock()
	if len(execs) != 0 {
		t.Fatalf("run posted /api/code-studio/exec: %q", execs)
	}
	want := []string{
		"node '/workspace/hello.js'",
		"tsx '/workspace/hello.ts'",
		"node '/workspace/hello.mjs'",
		"tsx '/workspace/hello.tsx'",
	}
	// The terminal sends the run commands over the socket, so the fixture receives
	// them one round trip after the browser call returned.
	var commands []string
	deadline := time.Now().Add(15 * time.Second)
	for {
		backend.mu.Lock()
		commands = nil
		for _, input := range backend.inputs {
			if strings.Contains(input, "node ") || strings.Contains(input, "tsx ") {
				commands = append(commands, input)
			}
		}
		backend.mu.Unlock()
		if len(commands) >= len(want) || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(commands) != len(want) {
		t.Fatalf("unexpected Node commands: %q", commands)
	}
	for index, command := range want {
		if !strings.Contains(commands[index], command) {
			t.Fatalf("Node command %d = %q, want %q", index, commands[index], command)
		}
	}
	for _, theme := range []string{"standard", "fruity"} {
		page.MustEval(`async theme=>{
            document.body.dataset.theme=theme;document.body.dataset.fruityMode='light';
            await CodeStudioApp.openFile('/workspace/hello.js',true,csState().windowId);
            await CodeStudioApp.openFile('/workspace/hello.ts',true,csState().windowId);
        }`, theme)
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "cs-node-"+theme+".png"), page.MustScreenshot(), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
