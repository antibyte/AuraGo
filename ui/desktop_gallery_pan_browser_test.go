package ui

import (
	"fmt"
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopGalleryPanUsesLayoutSizeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(galleryMetadataBrowserOrigin(t))
	defer page.MustClose()
	page.MustSetViewport(1400, 960, 1, false)
	page.MustWaitLoad()
	t.Run("cached-image-before-load-event", func(t *testing.T) {
		if !page.MustEval(`()=>{
 const create=document.createElement;
 document.createElement=function(tag,...args){const node=create.call(this,tag,...args);if(tag==='img')Object.defineProperties(node,{complete:{value:true},naturalWidth:{value:1200},naturalHeight:{value:400}});return node;};
 try{
  GalleryLightbox.open({items:[{path:'Photos/cached.png',name:'cached.png',web_path:'data:image/svg+xml,'+encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="400"/>'),media_kind:'image'}],context:{animationsEnabled:false}});
  const root=document.querySelector('.vd-lightbox'),img=root.querySelector('img.vd-lightbox-image');
  img.style.cssText='width:600px;height:200px;max-width:none;max-height:none;transition:none';
  root.querySelector('[data-lb-zoom-actual]').click();
  const r=img.getBoundingClientRect();
  return Math.abs(r.width-1200)<0.01&&Math.abs(r.height-400)<0.01&&root.querySelector('[data-lb-zoom-reset]').textContent==='100%';
 }finally{document.createElement=create;GalleryLightbox.close();}
}`).Bool() {
			t.Fatal("a cache-complete image must publish its native dimensions before a later load event")
		}
	})
	for _, rotation := range []int{0, 90, 270} {
		for _, flip := range []bool{false, true} {
			t.Run(fmt.Sprintf("rotation=%d/flip=%v", rotation, flip), func(t *testing.T) {
				page.MustEval(`()=>{GalleryLightbox.close();GalleryLightbox.open({items:[{path:'Photos/pan.png',name:'pan.png',web_path:'data:image/svg+xml,'+encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="400"><rect width="1200" height="400" fill="teal"/></svg>'),media_kind:'image'}],context:{animationsEnabled:false}});}`)
				waitForJSBool(t, page, `()=>document.querySelector('.vd-lightbox-image')?.complete && document.querySelector('.vd-lightbox-image')?.naturalWidth>0`)
				page.MustEval(`(rotation,flip)=>{
 const root=document.querySelector('.vd-lightbox'),stage=root.querySelector('[data-lb-stage]'),img=root.querySelector('img.vd-lightbox-image');
 stage.style.cssText='position:absolute;left:400px;top:200px;width:400px;height:350px';
 img.style.cssText='width:600px;height:200px;max-width:none;max-height:none;transition:none';
 root.querySelector('[data-lb-zoom-in]').click();root.querySelector('[data-lb-zoom-in]').click();root.querySelector('[data-lb-zoom-in]').click();
 for(let i=0;i<rotation/90;i++)root.querySelector('[data-lb-rotate-right]').click();
 if(flip)root.querySelector('[data-lb-flip]').click();
}`, rotation, flip)
				page.Mouse.MustMoveTo(600, 360).MustDown(proto.InputMouseButtonLeft).MustMoveTo(1300, 900).MustUp(proto.InputMouseButtonLeft)
				if !page.MustEval(`rotation=>{
 const img=document.querySelector('img.vd-lightbox-image'),stage=document.querySelector('[data-lb-stage]').getBoundingClientRect(),m=new DOMMatrix(getComputedStyle(img).transform);
 const scale=Math.hypot(m.a,m.b),sideways=rotation===90||rotation===270;
 const maxX=Math.max(0,((sideways?img.clientHeight:img.clientWidth)*scale-stage.width)/2),maxY=Math.max(0,((sideways?img.clientWidth:img.clientHeight)*scale-stage.height)/2);
 return Math.abs(m.e-maxX)<1&&Math.abs(m.f-maxY)<1;
}`, rotation).Bool() {
					t.Fatal("pan limits rotated transformed dimensions twice or changed screen-axis dragging")
				}
				page.MustEval(`()=>{const stage=document.querySelector('[data-lb-stage]');stage.style.width='700px';stage.style.height='650px';window.dispatchEvent(new Event('resize'));}`)
				if !page.MustEval(`rotation=>{
 const img=document.querySelector('img.vd-lightbox-image'),stage=document.querySelector('[data-lb-stage]').getBoundingClientRect(),m=new DOMMatrix(getComputedStyle(img).transform);
 const scale=Math.hypot(m.a,m.b),sideways=rotation===90||rotation===270;
 return Math.abs(m.e-Math.max(0,((sideways?img.clientHeight:img.clientWidth)*scale-stage.width)/2))<1&&Math.abs(m.f-Math.max(0,((sideways?img.clientWidth:img.clientHeight)*scale-stage.height)/2))<1;
}`, rotation).Bool() {
					t.Fatal("resize did not reclamp pan to the new stage")
				}
				page.MustSetViewport(1300, 900, 1, false)
				for _, width := range []float64{433.125, 75.375} {
					page.MustEval(`width=>{const root=document.querySelector('.vd-lightbox'),img=root.querySelector('img.vd-lightbox-image');img.style.width=width+'px';img.style.height=(width/3)+'px';root.querySelector('[data-lb-zoom-actual]').click();}`, width)
					actual := page.MustEval(`(rotation,flip)=>{
 const root=document.querySelector('.vd-lightbox'),img=root.querySelector('img.vd-lightbox-image'),rect=img.getBoundingClientRect(),m=new DOMMatrix(getComputedStyle(img).transform),sideways=rotation===90||rotation===270;
 const label=root.querySelector('[data-lb-zoom-reset]').textContent, mirrored=(m.a*m.d-m.b*m.c)<0;
 return {valid:Math.abs(rect.width-(sideways?img.naturalHeight:img.naturalWidth))<0.01&&Math.abs(rect.height-(sideways?img.naturalWidth:img.naturalHeight))<0.01&&label==='100%'&&mirrored===flip,
   width:rect.width,height:rect.height,naturalWidth:img.naturalWidth,naturalHeight:img.naturalHeight,label,mirrored};
}`, rotation, flip)
					if !actual.Get("valid").Bool() {
						t.Fatalf("1:1 zoom at fitted width %g did not match natural dimensions, label or flip: %s", width, actual.JSON("", ""))
					}
				}
				page.MustSetViewport(1400, 960, 1, false)
			})
		}
	}
}
