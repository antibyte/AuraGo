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
	for _, rotation := range []int{0, 90, 270} {
		for _, flip := range []bool{false, true} {
			t.Run(fmt.Sprintf("rotation=%d/flip=%v", rotation, flip), func(t *testing.T) {
				page.MustEval(`()=>{GalleryLightbox.close();GalleryLightbox.open({items:[{path:'Photos/pan.png',name:'pan.png',web_path:'/image.svg',media_kind:'image'}],context:{animationsEnabled:false}});}`)
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
			})
		}
	}
}
