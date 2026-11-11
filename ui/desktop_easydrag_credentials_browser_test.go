package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

// TestDesktopEasyDragCredentialMoveBrowser walks the real controls: the credential warning of an
// address, "move to the vault" with the proposed name, the editor with no trace of the token (model,
// undo history, localStorage), and the purge offer after the next publish.
func TestDesktopEasyDragCredentialMoveBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t, "/testdata/easydrag-credentials-fixture.js")
	p := s.page
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(p); err != nil {
		t.Fatal(err)
	}
	const token = "tok-browser-1234567890"
	s.wait(`()=>!!document.querySelector('.ed-flow-card h3')`)
	p.MustElement(`.ed-flow-card h3`).MustClick()
	s.wait(`()=>!!edFixture.editor() && !!document.querySelector('[data-ed-cmd="test"]')`)
	id := p.MustEval(`token=>{window.__purge=[];window.EasyDrag.credentialsPurge={offer:id=>window.__purge.push(id)};return edFixture.editor().model.addNode('http.request',{x:1280,y:120},{method:'GET',url:'https://api.example.com/v1?api_key='+token+'&q=1'},{node:'n_pdf',port:'out'});}`, token).Str()
	p.MustEval(`id=>{window.__credNode=id}`, id)
	s.wait(`()=>{const last=edFixture.lastSave();return !!last && last.nodes.some(n=>n.type==='http.request')}`)
	p.MustEval(`()=>edFixture.editor().bus.emit('open-detail',{nodeId:window.__credNode})`)
	action := `.ed-detail .ed-field[data-param="url"] [data-ed-credential-action="move"]`
	s.wait(`()=>!!document.querySelector('` + action + `')`)
	p.MustElement(action).MustClick()
	s.wait(`()=>{const i=document.querySelector('.ed-modal [data-ed-credential-name]');return !!i && i.value==='api_example_com_api_key'}`)
	s.shot("credential-move-dialog")
	p.MustElement(`.ed-modal [data-ed-action="move"]`).MustClick()
	s.wait(`()=>edCredentialsFixture.state.moves.length===1 && !document.querySelector('.ed-modal-backdrop:not(.is-closing)')`)

	// The editor holds no trace of the token: the model, every undo step, the emergency copy.
	s.wait(`()=>{const ed=edFixture.editor(),n=ed.model.node(window.__credNode);return n.params.auth_secret==='api_example_com_api_key' && n.params.url==='https://api.example.com/v1?q=1' && ed.saver.state==='saved'}`)
	clean := p.MustEval(`token=>{const ed=edFixture.editor();let ok=!JSON.stringify(ed.model.toJSON()).includes(token);for(let i=0;i<10&&ed.model.canUndo();i++){ed.model.undo();ok=ok&&!JSON.stringify(ed.model.toJSON()).includes(token);}while(ed.model.canRedo())ed.model.redo();return ok&&!JSON.stringify(Object.entries(localStorage)).includes(token)}`, token).Bool()
	if !clean {
		t.Fatal("the token is still in the model, its undo history or localStorage")
	}
	s.failOnPageErrors("credential move")
	s.noRawKeys("credential move")

	// The next publish offers the guided purge, once.
	p.MustElement(`[data-ed-cmd="publish"]`).MustClick()
	s.wait(`()=>!!document.querySelector('.ed-modal [data-ed-action="publish"]')`)
	p.MustElement(`.ed-modal [data-ed-action="publish"]`).MustClick()
	s.wait(`()=>window.__purge.length===1`)
	if got := p.MustEval(`()=>window.__purge[0]`).Str(); got != p.MustEval(`()=>edFixture.editor().flow.id`).Str() {
		t.Fatalf("purge offered for %q", got)
	}
}
