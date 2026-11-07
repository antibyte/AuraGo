package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The IDE surface of Code Studio: the Visual Studio style bottom panel with its
// Problems / Output / Terminal views, the inline diagnostics that feed it, the
// find and replace widget and the document outline.
func TestDesktopCodeStudioIDEPanelBrowser(t *testing.T) {
	page, backend := newCodeStudioBrowser(t)
	backend.mu.Lock()
	backend.execOutputs["go vet"] = "/workspace/main.go:6:2: undefined: missingSymbol\n"
	backend.mu.Unlock()

	check := func(name, js string) {
		t.Helper()
		result, err := page.Eval(js)
		if err != nil {
			t.Errorf("%s: eval failed: %v; browser errors: %s", name, err, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
			return
		}
		if !result.Value.Bool() {
			t.Errorf("%s; browser errors: %s", name, page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
		}
	}
	snapshot := func(name string) {
		t.Helper()
		dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
		if dir == "" {
			return
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".png"), page.MustScreenshot(), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := page.Eval(`async()=>{await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);}`); err != nil {
		t.Fatalf("Code Studio did not open: %v", err)
	}

	// The bottom panel carries the three Visual Studio views and hosts the terminal.
	check("bottom panel exposes problems, output and terminal", `()=>{
		const tabs=[...csRoot().querySelectorAll('[data-panel-tab]')].map(b=>b.dataset.panelTab);
		return tabs.length===3 && tabs.includes('problems') && tabs.includes('output') && tabs.includes('terminal');
	}`)
	check("terminal lives inside the bottom panel", `()=>{
		const panel=csRoot().querySelector('[data-panel]');
		return !!panel && !!panel.querySelector('[data-terminal]') && !!panel.querySelector('[data-terminal-screen]');
	}`)
	check("terminal view is the active one on start", `()=>{
		const tab=csRoot().querySelector('[data-panel-tab="terminal"]');
		const pane=csRoot().querySelector('[data-panel-pane="terminal"]');
		return tab.classList.contains('active') && !pane.hidden;
	}`)

	// Switching to the problems view hides the terminal pane.
	page.MustEval(`()=>{csRoot().querySelector('[data-panel-tab="problems"]').click();}`)
	check("problems view replaces the terminal view", `()=>{
		return csRoot().querySelector('[data-panel-tab="problems"]').classList.contains('active')
			&& !csRoot().querySelector('[data-panel-pane="problems"]').hidden
			&& csRoot().querySelector('[data-panel-pane="terminal"]').hidden;
	}`)

	// Ctrl+Shift+M opens the problems panel from anywhere in the studio.
	page.MustEval(`()=>{csRoot().querySelector('[data-panel-tab="terminal"]').click();}`)
	page.MustEval(`async()=>{[...csRoot().querySelectorAll('.cs-tree-item')].find(el=>el.dataset.filePath==='/workspace/main.go').click();await csWait(()=>csRoot().querySelector('.cm-editor'),10000);}`)
	page.MustEval(`()=>{csRoot().querySelector('.cm-content').focus();csKey(csRoot().querySelector('.cm-content'),{key:'m',ctrlKey:true,shiftKey:true});}`)
	check("Ctrl+Shift+M focuses the problems panel", `()=>csState().activePanel==='problems' && csState().panelVisible`)

	// Running the checks turns the compiler output into problems.
	if _, err := page.Eval(`async()=>{csRoot().querySelector('[data-panel-action="diagnostics"]').click();await csWait(()=>csState().problems.length>0,10000);}`); err != nil {
		t.Fatalf("the check run did not report a problem: %v; state=%s", err, page.MustEval(`()=>JSON.stringify({active:csState().activePanel,running:csState().diagnosticsRunning,problems:csState().problems,files:csState().files.map(f=>f.name),tabs:csState().openTabs.map(x=>x.path),status:csRoot().querySelector('[data-statusbar]').textContent})`).Str())
	}
	check("the check run reports the compiler error", `()=>{
		const problems=csState().problems;
		return problems.length===1 && problems[0].file==='/workspace/main.go' && problems[0].line===6
			&& problems[0].severity==='error' && problems[0].message.includes('missingSymbol');
	}`)
	check("the problems list renders the finding", `()=>{
		const item=csRoot().querySelector('.cs-problem');
		return !!item && item.textContent.includes('missingSymbol');
	}`)
	snapshot("cs-ide-01-problems")
	check("the status bar counts the errors", `()=>{
		const badge=csRoot().querySelector('.cs-status-problems .cs-status-badge.error');
		return !!badge && badge.textContent.includes('1');
	}`)
	check("the check output lands in the output view", `()=>{
		csRoot().querySelector('[data-panel-tab="output"]').click();
		return csRoot().querySelector('[data-output-panel]').textContent.includes('missingSymbol');
	}`)

	// Clicking a problem jumps to the offending line.
	page.MustEval(`()=>{csRoot().querySelector('[data-panel-tab="problems"]').click();}`)
	page.MustEval(`()=>{csRoot().querySelector('.cs-problem').click();}`)
	check("clicking a problem jumps to its line", `()=>{
		const text=csRoot().querySelector('.code-studio-statusbar').textContent;
		return text.includes('6:') || text.includes('Zeile 6');
	}`)

	// The inline gutter marks the same finding.
	check("the editor shows an inline diagnostic", `()=>!!csRoot().querySelector('.cm-lintRange-error, .cm-lint-marker-error')`)

	// Find and replace works on the active editor.
	page.MustEval(`()=>{csRoot().querySelector('.cm-content').focus();csKey(csRoot().querySelector('.cm-content'),{key:'f',ctrlKey:true});}`)
	check("Ctrl+F opens the find widget", `()=>{
		const widget=csRoot().querySelector('[data-find-widget]');
		return !!widget && !widget.hidden && !!widget.querySelector('[data-find-input]');
	}`)
	page.MustEval(`()=>{const input=csRoot().querySelector('[data-find-input]');input.focus();input.value='fmt';input.dispatchEvent(new Event('input',{bubbles:true}));}`)
	check("the find widget counts the matches", `()=>{
		const counter=csRoot().querySelector('[data-find-count]');
		return !!counter && /1/.test(counter.textContent);
	}`)
	check("the find input keeps the focus while typing", `()=>document.activeElement===csRoot().querySelector('[data-find-input]')`)
	snapshot("cs-ide-02-find")
	page.MustEval(`()=>{csRoot().querySelector('[data-find-replace-toggle]').click();}`)
	check("the replace row can be opened", `()=>{const row=csRoot().querySelector('[data-find-replace-row]');return !!row && !row.hidden;}`)
	page.MustEval(`()=>{const input=csRoot().querySelector('[data-find-replace]');input.focus();input.value='format';input.dispatchEvent(new Event('input',{bubbles:true}));}`)
	page.MustEval(`()=>{csRoot().querySelector('[data-find-replace-all]').click();}`)
	check("replace all rewrites the document", `()=>csState().openTabs.some(tab=>tab.content.includes('format.Println'))`)

	// The outline lists the symbols of the active file.
	page.MustEval(`()=>{csRoot().querySelector('[data-activity="outline"]').click();}`)
	check("the outline lists the symbols of the active file", `()=>{
		const names=[...csRoot().querySelectorAll('.cs-outline-name')].map(el=>el.textContent);
		return names.includes('main');
	}`)
	snapshot("cs-ide-03-outline")

	check("no browser errors after the IDE tour", `()=>fixtureErrors.length===0`)
}

func TestDesktopCodeStudioIDEParseAndChecks(t *testing.T) {
	page, _ := newCodeStudioBrowser(t)
	if _, err := page.Eval(`async()=>{await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);}`); err != nil {
		t.Fatalf("Code Studio did not open: %v", err)
	}

	// The client side checker reports unbalanced brackets as a problem of the open file.
	page.MustEval(`async()=>{const app=window.CodeStudioApp;const id=[...app.instances.keys()][0];await app.openFile('/workspace/src/app.js',true,id);await csWait(()=>csState().openTabs.length>0,10000);}`)
	page.MustEval(`()=>{const view=csState().openTabs[0].view;view.dispatch({changes:{from:view.state.doc.length,to:view.state.doc.length,insert:'\nconst pattern = /[a-z]+/g;\n'}});}`)
	if _, err := page.Eval(`async()=>{await csWait(()=>csState().problems.length>0||true,2000);}`); err != nil {
		t.Fatalf("the document check failed: %v", err)
	}
	regex := page.MustEval(`()=>JSON.stringify(csState().problems.filter(p=>p.source==='syntax'))`)
	if strings.Contains(regex.Str(), "Unclosed") {
		t.Fatalf("a regular expression literal must not be reported as an unbalanced bracket: %s", regex.Str())
	}

	page.MustEval(`()=>{const view=csState().openTabs[0].view;view.dispatch({changes:{from:view.state.doc.length,to:view.state.doc.length,insert:'\nfunction broken() {'}});}`)
	if _, err := page.Eval(`async()=>{await csWait(()=>csState().problems.some(p=>p.source==='syntax'),10000);}`); err != nil {
		t.Fatalf("the syntax check did not report the unbalanced bracket: %v", err)
	}
	result := page.MustEval(`()=>{
		const problem=csState().problems.find(p=>p.source==='syntax');
		return problem && problem.severity==='error' && problem.message.length>0;
	}`)
	if !result.Bool() {
		t.Fatalf("syntax problem not recorded: %s", page.MustEval(`()=>JSON.stringify(csState().problems)`).Str())
	}

	// The compiler output parser understands the common Go and gcc formats.
	out := page.MustEval(`()=>{
		const app=window.CodeStudioApp;
		const id=[...app.instances.keys()][0];
		return JSON.stringify(app.parseDiagnostics('main.go:12:5: undefined: x\nmain.go:9:1: declared and not used: y','go build',id));
	}`)
	if !strings.Contains(out.Str(), `"line":12`) || !strings.Contains(out.Str(), "undefined: x") {
		t.Fatalf("go diagnostics not parsed: %s", out.Str())
	}
	gcc := page.MustEval(`()=>{
		const app=window.CodeStudioApp;
		const id=[...app.instances.keys()][0];
		return JSON.stringify(app.parseDiagnostics('main.c:3:9: error: expected \';\'','gcc',id));
	}`)
	if !strings.Contains(gcc.Str(), `"severity":"error"`) || !strings.Contains(gcc.Str(), "expected ';'") {
		t.Fatalf("gcc diagnostics not parsed: %s", gcc.Str())
	}
}
