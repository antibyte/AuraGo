package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopCodeStudioC(t *testing.T) {
	page, backend := newCodeStudioBrowser(t)
	backend.mu.Lock()
	backend.files["/workspace/hello.go"] = "package main\nfunc main() {}\n"
	backend.files["/workspace/hello.py"] = "print('Hello from Code Studio')\n"
	backend.files["/workspace/hello.c"] = "#include <stdio.h>\nint main(void) { puts(\"Hello from Code Studio\"); return 0; }\n"
	backend.files["/workspace/hello.h"] = "int greeting(void);\n"
	backend.files["/workspace/hello.ts"] = "export const n: number = 1;\n"
	backend.mu.Unlock()
	page.MustReload().MustWaitLoad()
	result := page.MustEval(`async()=>{
        await fixtureReady;
        await csWait(()=>csState()?.terminalSessions.length && csState().openTabs.length===3,20000);
        const app=CodeStudioApp,target=csState(),id=target.windowId;
        const examples=target.openTabs.map(tab=>tab.path).join(',')==='/workspace/hello.go,/workspace/hello.py,/workspace/hello.c';
        await app.openFile('/workspace/hello.c',true,id);
        const tab=target.openTabs[target.activeTabIndex];
        await csWait(()=>tab.view.contentDOM.querySelector('span[class]'));
        const highlighted=tab.language==='c' && typeof target.cmModule.cpp==='function';
        tab.view.dispatch({changes:{from:0,insert:'// edited before Run\n'}});
        await app.command('runCurrentFile',[],id);
        const saved=!tab.modified && (await app.api.file(tab.path)).content===tab.content;
        const quotedPath="/workspace/it's a sample.c";
        await app.api.writeFile(quotedPath,tab.content);
        await app.openFile(quotedPath,true,id);
        await app.command('runCurrentFile',[],id);
        await app.openFile('/workspace/hello.h',true,id);
        const header=target.openTabs[target.activeTabIndex].language==='c';
        await app.command('runCurrentFile',[],id);
        const headerIdle=csRoot().querySelector('.cs-status-message')?.textContent==='Für diese Datei gibt es keinen Startbefehl';
        await app.openFile('/workspace/hello.ts',true,id);
        await app.command('runCurrentFile',[],id);
        const tsTab=target.openTabs[target.activeTabIndex];
        const noTypeScript=tsTab.path==='/workspace/hello.ts' && csRoot().querySelector('.cs-status-message')?.textContent==='Für diese Datei gibt es keinen Startbefehl';
        await app.command('closeTab',[target.openTabs.indexOf(tsTab),true],id);
        await app.openFile('/workspace/hello.c',true,id);
        return examples && highlighted && saved && header && headerIdle && noTypeScript && fixtureErrors.length===0;
    }`)
	if !result.Bool() {
		t.Fatalf("C examples, highlighting or save/run failed: %s", page.MustEval(`()=>fixtureErrors`).JSON("", ""))
	}
	backend.mu.Lock()
	execs := append([]string(nil), backend.execs...)
	inputs := append([]string(nil), backend.inputs...)
	backend.mu.Unlock()
	if len(execs) != 0 {
		t.Fatalf("run posted /api/code-studio/exec: %q", execs)
	}
	var commands []string
	for _, input := range inputs {
		if strings.Contains(input, "gcc ") || strings.Contains(input, "node ") || strings.Contains(input, "cat ") {
			commands = append(commands, input)
		}
	}
	if len(commands) != 2 || !strings.Contains(commands[0], "gcc -std=c17 -Wall -Wextra -x c '/workspace/hello.c'") ||
		!strings.Contains(commands[0], "mktemp -d") || !strings.Contains(commands[0], "trap '") ||
		!strings.Contains(commands[1], "'/workspace/it'\"'\"'s a sample.c'") {
		t.Fatalf("unexpected C commands: %q", inputs)
	}
	for _, input := range inputs {
		if strings.Contains(input, "hello.h") || strings.Contains(input, "hello.ts") {
			t.Fatalf("header or TypeScript ran: %q", inputs)
		}
	}
	for _, theme := range []string{"standard", "fruity"} {
		page.MustEval(`async theme=>{
            document.body.dataset.theme=theme;document.body.dataset.fruityMode='light';
            await CodeStudioApp.openFile('/workspace/hello.go',true,csState().windowId);
            await CodeStudioApp.openFile('/workspace/hello.c',true,csState().windowId);
        }`, theme)
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "cs-c-"+theme+".png"), page.MustScreenshot(), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	page.MustSetViewport(760, 850, 1, false)
	page.MustReload().MustWaitLoad()
	if !page.MustEval(`async()=>{
        await fixtureReady;await csWait(()=>csState()?.terminalSessions.length,20000);
        return csState().openTabs.length===5 && csState().openTabs[csState().activeTabIndex].path==='/workspace/hello.c' && fixtureErrors.length===0;
    }`).Bool() {
		t.Fatal("restoring the C tab replaced existing tabs or the active selection")
	}
	if !page.MustEval(`()=>{
        const root=csRoot().getBoundingClientRect(),run=csRoot().querySelector('[data-action="run"]').getBoundingClientRect();
        return run.width>0 && run.right<=innerWidth && run.right<=root.right+1 && csRoot().querySelector('.cm-editor').getBoundingClientRect().width>100;
    }`).Bool() {
		t.Fatal("C example or Run became inaccessible in the narrow layout")
	}
}
