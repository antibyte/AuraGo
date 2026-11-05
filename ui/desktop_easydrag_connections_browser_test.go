package ui

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopEasyDragConnectionPreviewBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	s := startEasyDragSmoke(t)
	p := s.page
	p.MustElement(".ed-flow-card h3").MustClick()
	s.wait(`()=>document.querySelectorAll('.ed-node').length===4`)
	s.settle()
	p.MustEval(`()=>edFixture.editor().model.disconnect(['e_1'])`)
	points := p.MustEval(`()=>['.ed-node[data-node-id="n_zeitplan"] [data-ed-side="out"]','.ed-node[data-node-id="n_suche"] [data-ed-side="in"]'].map(s=>{const r=document.querySelector(s).getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})`)
	sx, sy := points.Get("0.0").Num(), points.Get("0.1").Num()
	tx, ty := points.Get("1.0").Num(), points.Get("1.1").Num()
	assertPreview := func(x, y float64) {
		t.Helper()
		p.MustEval(`point=>{
			const path=document.querySelector('.ed-wire-preview'), style=getComputedStyle(path);
			if(path.hasAttribute('hidden') || style.display==='none' || style.visibility==='hidden' || style.stroke==='none' || Number(style.opacity)===0 || path.getTotalLength()<20)
				throw Error('Connection preview is invisible: '+JSON.stringify({hidden:path.hasAttribute('hidden'),property:path.hidden,display:style.display,d:path.getAttribute('d')}));
			const end=path.getPointAtLength(path.getTotalLength()), screen=new DOMPoint(end.x,end.y).matrixTransform(path.getScreenCTM());
			if(Math.hypot(screen.x-point[0],screen.y-point[1])>1)throw Error('Connection preview does not follow the pointer');
		}`, []float64{x, y})
	}
	p.Mouse.MustMoveTo(sx, sy).MustDown(proto.InputMouseButtonLeft).MustMoveTo(sx+32, sy-60)
	s.wait(`()=>!!document.querySelector('.ed-wire-preview').getAttribute('d')`)
	assertPreview(sx+32, sy-60)
	s.shot("connection-preview-drag")
	p.Mouse.MustMoveTo(sx+48, sy-80)
	assertPreview(sx+48, sy-80)
	p.Mouse.MustMoveTo(tx, ty)
	s.wait(`()=>document.querySelector('.ed-wire-preview').classList.contains('is-snap')`)
	p.Mouse.MustUp(proto.InputMouseButtonLeft)
	s.wait(`()=>edFixture.editor().model.doc.edges.length===3 && getComputedStyle(document.querySelector('.ed-wire-preview')).display==='none'`)

	// Losing pointer capture cancels a picked-up wire and restores its connection.
	p.MustEval(`()=>document.querySelector('.ed-canvas').addEventListener('pointerdown',event=>window.__edPreviewPointerId=event.pointerId,{once:true})`)
	p.Mouse.MustMoveTo(tx, ty).MustDown(proto.InputMouseButtonLeft).MustMoveTo(tx-32, ty-60)
	s.wait(`()=>edFixture.editor().model.doc.edges.length===2`)
	assertPreview(tx-32, ty-60)
	p.MustEval(`()=>document.querySelector('.ed-canvas').releasePointerCapture(window.__edPreviewPointerId)`)
	p.Mouse.MustMoveTo(tx-33, ty-60).MustUp(proto.InputMouseButtonLeft)
	s.wait(`()=>!edFixture.editor().dragging && edFixture.editor().model.doc.edges.length===3 && getComputedStyle(document.querySelector('.ed-wire-preview')).display==='none'`)
	s.failOnPageErrors("connection preview")
}
