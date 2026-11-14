package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// TestEasyDragKitBrowser: below 600 px of window width a dialog opens as a bottom sheet (full width, at the
// bottom, a grab bar), from 600 px it stays a dialog, and a kit disclosure opens with Enter and closes with Space.
func TestEasyDragKitBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmokeWith(t, easyDragSmokeOptions{Width: 390, Height: 844})
	p := s.page
	s.wait(`()=>!!document.querySelector('.ed-app')`)
	open := `()=>{const root=document.querySelector('.ed-app');window.__kitDialog=EasyDrag.core.modal(root,{title:'Sheet',closeLabel:'Close',body:'<p>Body</p>',actions:[{id:'ok',label:'OK',primary:true}]});return true}`
	p.MustEval(open)
	s.wait(`()=>!!document.querySelector('.ed-modal.ed-sheet') && getComputedStyle(document.querySelector('.ed-modal.ed-sheet')).transform==='none'`)
	got := p.MustEval(`()=>{const m=document.querySelector('.ed-modal.ed-sheet'),a=document.querySelector('.ed-app').getBoundingClientRect(),r=m.getBoundingClientRect(),g=m.querySelector('.ed-sheet-grab').getBoundingClientRect(),cs=getComputedStyle(m);
		return JSON.stringify({full:Math.abs(r.width-a.width)<1.5,bottom:Math.abs(r.bottom-a.bottom)<1.5,grab:g.width>0&&g.height>0,topRound:parseFloat(cs.borderTopLeftRadius)>0,bottomSquare:parseFloat(cs.borderBottomLeftRadius)===0})}`).Str()
	if got != `{"full":true,"bottom":true,"grab":true,"topRound":true,"bottomSquare":true}` {
		t.Errorf("390 px: the dialog is not a bottom sheet: %s", got)
	}
	s.shot("kit-sheet-390")

	// the kit disclosure: Enter opens, Space closes (a real button)
	p.MustEval(`()=>{const d=EasyDrag.kit.disclosure({title:'Notes',count:2,body:'<p>More</p>'});document.querySelector('.ed-modal-body').appendChild(d);d.querySelector('.ed-disclosure-head').focus();return true}`)
	p.Keyboard.MustType(input.Enter)
	s.wait(`()=>document.querySelector('.ed-modal-body .ed-disclosure-head').getAttribute('aria-expanded')==='true' && !document.querySelector('.ed-modal-body .ed-disclosure-body').hidden`)
	p.Keyboard.MustType(input.Space)
	s.wait(`()=>document.querySelector('.ed-modal-body .ed-disclosure-head').getAttribute('aria-expanded')==='false' && document.querySelector('.ed-modal-body .ed-disclosure-body').hidden`)
	p.MustEval(`()=>window.__kitDialog.close('ok')`)
	s.wait(`()=>!document.querySelector('.ed-modal')`)

	// from 600 px up the same call stays a dialog
	p.MustSetViewport(1280, 800, 1, false)
	p.MustEval(open)
	s.wait(`()=>!!document.querySelector('.ed-modal')`)
	if p.MustEval(`()=>document.querySelector('.ed-modal').classList.contains('ed-sheet')`).Bool() {
		t.Error("1280 px: the dialog opened as a sheet")
	}
	s.failOnPageErrors("kit browser")
}
