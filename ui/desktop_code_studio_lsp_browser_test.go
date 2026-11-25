package ui

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/go-rod/rod/lib/input"
	"github.com/gorilla/websocket"
)

// fakeCodeStudioLSP plays gopls behind the Code Studio bridge: one JSON-RPC message per WebSocket text frame.
type fakeCodeStudioLSP struct {
	mu         sync.Mutex
	langs      []string
	methods    []string
	replies    []string
	initParams string
	opened     []string
}

func fakeLSPRange(startLine, startCharacter, endLine, endCharacter int) map[string]interface{} {
	return map[string]interface{}{
		"start": map[string]int{"line": startLine, "character": startCharacter},
		"end":   map[string]int{"line": endLine, "character": endCharacter},
	}
}

func (f *fakeCodeStudioLSP) serve(conn *websocket.Conn, lang string) {
	f.mu.Lock()
	f.langs = append(f.langs, lang)
	f.mu.Unlock()
	send := func(message map[string]interface{}) {
		message["jsonrpc"] = "2.0"
		if data, err := json.Marshal(message); err == nil {
			_ = conn.WriteMessage(websocket.TextMessage, data)
		}
	}
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(payload, &msg) != nil {
			continue
		}
		f.mu.Lock()
		if msg.Method != "" {
			f.methods = append(f.methods, msg.Method)
		} else if len(msg.ID) > 0 {
			f.replies = append(f.replies, string(payload))
		}
		f.mu.Unlock()
		switch msg.Method {
		case "initialize":
			f.mu.Lock()
			f.initParams = string(msg.Params)
			f.mu.Unlock()
			send(map[string]interface{}{"id": msg.ID, "result": map[string]interface{}{
				"capabilities": map[string]interface{}{"textDocumentSync": 2, "hoverProvider": true, "definitionProvider": true},
				"serverInfo":   map[string]string{"name": "fake-gopls"},
			}})
		case "initialized":
			// A server-to-client request: the page must answer it itself (lspOnServerRequest default).
			send(map[string]interface{}{"id": "reg-1", "method": "client/registerCapability", "params": map[string]interface{}{"registrations": []interface{}{}}})
		case "textDocument/didOpen":
			var params struct {
				TextDocument struct {
					URI        string `json:"uri"`
					LanguageID string `json:"languageId"`
					Version    int    `json:"version"`
					Text       string `json:"text"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			doc := params.TextDocument
			f.mu.Lock()
			f.opened = append(f.opened, doc.URI+"|"+doc.LanguageID+"|"+doc.Text)
			f.mu.Unlock()
			send(map[string]interface{}{"method": "textDocument/publishDiagnostics", "params": map[string]interface{}{
				"uri":     doc.URI,
				"version": doc.Version,
				"diagnostics": []interface{}{map[string]interface{}{
					"range": fakeLSPRange(8, 1, 8, 5), "severity": 1, "code": "UndeclaredName", "source": "compiler", "message": "undefined: fmtx",
				}},
			}})
		case "textDocument/hover":
			send(map[string]interface{}{"id": msg.ID, "result": map[string]interface{}{"contents": map[string]string{
				"kind":  "markdown",
				"value": "```go\nfunc greet(name string) string\n```\n\nGreet says hello. <img src=\"https://example.invalid/x.png\" onerror=\"window.lspXSS=1\"><script>window.lspXSS=2</script>",
			}}})
		case "textDocument/definition":
			var params struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			send(map[string]interface{}{"id": msg.ID, "result": map[string]interface{}{"uri": params.TextDocument.URI, "range": fakeLSPRange(2, 5, 2, 10)}})
		default:
			if len(msg.ID) > 0 && msg.Method != "" {
				send(map[string]interface{}{"id": msg.ID, "result": nil})
			}
		}
	}
}

// Opening a .go file starts the go connection; a diagnostic reaches the Problems panel, the hover tooltip shows
// sanitized server Markdown and F12 moves the cursor to the definition.
func TestDesktopCodeStudioLSPBrowser(t *testing.T) {
	page, backend := newCodeStudioBrowser(t)
	fake := &fakeCodeStudioLSP{}
	const source = "package main\n\nfunc greet(name string) string {\n\treturn \"hi \" + name\n}\n\nfunc main() {\n\tprintln(greet(\"x\"))\n\tfmtx.Println()\n}\n"
	backend.mu.Lock()
	backend.dirs["/workspace/lsp"] = true
	backend.files["/workspace/lsp/main.go"] = source
	backend.lspHandler = fake.serve
	backend.mu.Unlock()
	browserErrors := func() string { return page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str() }

	opened, err := page.Eval(`async()=>{
		await csWait(()=>csRoot()&&csRoot().querySelectorAll('.cs-tree-item').length>0,20000);
		const id=csState().windowId;
		await CodeStudioApp.openFile('/workspace/lsp/main.go',true,id);
		await csWait(()=>csState().lsp&&csState().lsp.connections.go&&csState().lsp.connections.go.state==='ready',15000);
		await csWait(()=>csState().problems.some(p=>p.source==='gopls'),10000);
		await CodeStudioApp.command('setActivePanel',['problems'],id);
		await csWait(()=>csRoot().querySelector('[data-status-lsp="go"]')&&csRoot().querySelector('.cm-lintRange-error'),5000);
		const problem=csState().problems.find(p=>p.source==='gopls');
		return {
			panel:csRoot().querySelector('[data-problems-panel]').textContent.includes('undefined: fmtx'),
			message:problem.message,line:problem.line,column:problem.column,endColumn:problem.endColumn,
			lspSource:problem.lsp?problem.lsp.source:'',lspCode:problem.lsp?problem.lsp.code:'',lspLine:problem.lsp?problem.lsp.range.start.line:-1,
			panelTabs:csRoot().querySelectorAll('[data-panel-tab]').length,
			status:csRoot().querySelector('[data-status-lsp="go"]').dataset.lspState,
			squiggle:csRoot().querySelector('.cm-lintRange-error').textContent
		};
	}`)
	if err != nil {
		t.Fatalf("Go file did not reach a ready language server with a diagnostic: %v; browser errors: %s", err, browserErrors())
	}
	got := opened.Value
	if !got.Get("panel").Bool() || got.Get("message").Str() != "undefined: fmtx" || got.Get("line").Int() != 9 ||
		got.Get("column").Int() != 2 || got.Get("endColumn").Int() != 6 || got.Get("status").Str() != "ready" ||
		got.Get("squiggle").Str() != "fmtx" || got.Get("lspSource").Str() != "compiler" || got.Get("lspCode").Str() != "UndeclaredName" ||
		got.Get("lspLine").Int() != 8 || got.Get("panelTabs").Int() != 3 {
		t.Fatalf("diagnostic, raw diagnostic, squiggle, status or panel tabs wrong: %s; browser errors: %s", got.JSON("", ""), browserErrors())
	}

	point, err := page.Eval(`()=>{
		const view=csState().openTabs[csState().activeTabIndex].view;
		const line=view.state.doc.line(8);
		const c=view.coordsAtPos(line.from+line.text.indexOf('greet')+2);
		return c?[c.left+1,(c.top+c.bottom)/2]:[];
	}`)
	if err != nil || len(point.Value.Arr()) != 2 {
		t.Fatalf("no screen position for greet: %v", err)
	}
	page.Mouse.MustMoveTo(4, 4)
	page.Mouse.MustMoveTo(point.Value.Arr()[0].Num(), point.Value.Arr()[1].Num())
	hover, err := page.Eval(`async()=>{
		const tip=await csWait(()=>document.querySelector('.cm-lsp-hover-tooltip'),8000);
		return {text:tip.textContent,unsafe:!!tip.querySelector('img,script,[onerror]'),clean:window.lspXSS===undefined};
	}`)
	if err != nil {
		t.Fatalf("hover tooltip did not show: %v; browser errors: %s", err, browserErrors())
	}
	h := hover.Value
	if text := h.Get("text").Str(); !strings.Contains(text, "func greet(name string) string") || !strings.Contains(text, "Greet says hello.") ||
		h.Get("unsafe").Bool() || !h.Get("clean").Bool() {
		t.Fatalf("hover content wrong or unsanitized: %s", h.JSON("", ""))
	}

	if _, err := page.Eval(`()=>{
		const view=csState().openTabs[csState().activeTabIndex].view;
		const line=view.state.doc.line(8);
		view.focus();
		view.dispatch({selection:{anchor:line.from+line.text.indexOf('greet')+1}});
		return true;
	}`); err != nil {
		t.Fatalf("placing the cursor failed: %v", err)
	}
	page.Keyboard.MustType(input.F12)
	if _, err := page.Eval(`async()=>{
		await csWait(()=>{
			const view=csState().openTabs[csState().activeTabIndex].view;
			const head=view.state.selection.main.head,line=view.state.doc.lineAt(head);
			return line.number===3&&head-line.from===5;
		},8000);
		return true;
	}`); err != nil {
		t.Fatalf("F12 did not move the cursor to the definition: %v; browser errors: %s", err, browserErrors())
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if strings.Join(fake.langs, ",") != "go" {
		t.Fatalf("language server sockets = %v, want one for go", fake.langs)
	}
	var init struct {
		RootURI          string `json:"rootUri"`
		WorkspaceFolders []struct {
			URI  string `json:"uri"`
			Name string `json:"name"`
		} `json:"workspaceFolders"`
		Capabilities struct {
			Workspace struct {
				Configuration bool `json:"configuration"`
				ApplyEdit     bool `json:"applyEdit"`
				WorkspaceEdit struct {
					DocumentChanges bool `json:"documentChanges"`
				} `json:"workspaceEdit"`
			} `json:"workspace"`
			TextDocument struct {
				Rename struct {
					PrepareSupport bool `json:"prepareSupport"`
				} `json:"rename"`
				CodeAction struct {
					CodeActionLiteralSupport struct {
						CodeActionKind struct {
							ValueSet []string `json:"valueSet"`
						} `json:"codeActionKind"`
					} `json:"codeActionLiteralSupport"`
					DataSupport    bool `json:"dataSupport"`
					ResolveSupport struct {
						Properties []string `json:"properties"`
					} `json:"resolveSupport"`
				} `json:"codeAction"`
				Hover struct {
					ContentFormat []string `json:"contentFormat"`
				} `json:"hover"`
			} `json:"textDocument"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal([]byte(fake.initParams), &init); err != nil {
		t.Fatalf("initialize params: %v (%s)", err, fake.initParams)
	}
	caps := init.Capabilities
	if init.RootURI != "file:///workspace" || len(init.WorkspaceFolders) != 1 || init.WorkspaceFolders[0].URI != "file:///workspace" ||
		!caps.Workspace.Configuration || !caps.Workspace.ApplyEdit || !caps.Workspace.WorkspaceEdit.DocumentChanges ||
		!caps.TextDocument.Rename.PrepareSupport || !caps.TextDocument.CodeAction.DataSupport ||
		strings.Join(caps.TextDocument.CodeAction.ResolveSupport.Properties, ",") != "edit" ||
		!strings.Contains(","+strings.Join(caps.TextDocument.CodeAction.CodeActionLiteralSupport.CodeActionKind.ValueSet, ",")+",", ",quickfix,") ||
		len(caps.TextDocument.Hover.ContentFormat) == 0 {
		t.Fatalf("initialize params = %s", fake.initParams)
	}
	if len(fake.opened) == 0 || fake.opened[0] != "file:///workspace/lsp/main.go|go|"+source {
		t.Fatalf("didOpen = %q", fake.opened)
	}
	answered := false
	for _, reply := range fake.replies {
		if strings.Contains(reply, `"id":"reg-1"`) && strings.Contains(reply, `"result":null`) {
			answered = true
		}
	}
	if !answered {
		t.Fatalf("client/registerCapability was not answered by the page: replies=%q", fake.replies)
	}
	methods := "," + strings.Join(fake.methods, ",") + ","
	for _, want := range []string{"initialize", "initialized", "textDocument/didOpen", "textDocument/hover", "textDocument/definition"} {
		if !strings.Contains(methods, ","+want+",") {
			t.Fatalf("server never saw %s: %s", want, methods)
		}
	}
	if errs := browserErrors(); errs != "[]" {
		t.Fatalf("browser errors: %s", errs)
	}
}
