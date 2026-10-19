package ui

import (
	"testing"
	"time"
)

func TestDesktopGreenSlimeSpringSurfaceBrowser(t *testing.T) {
	s := newThemeSweepSession(t, "no-preference")
	s.store.set(map[string]string{
		"appearance.theme":   "green_slime",
		"windows.animations": "true",
	})
	s.pushSettings(t)
	s.waitFor(t, "Green Slime effect and motion", 10*time.Second, `()=>AuraDesktopThemeFx?.activeId()==='green_slime' && AuraDesktopThemeFx.inspect().motion`)

	s.eval(t, `()=>{
		const record={
			id:'green-slime-surface-probe', phase:'open',
			rect:{x:170,y:155,width:390,height:270}, z:9000,
			visible:true,minimized:false,silent:true,velocity:{x:0,y:0}
		};
		window.greenSlimeProbe=record;
		window.dispatchEvent(new CustomEvent('aurago:desktop-window',{detail:record}));
	}`)
	s.waitFor(t, "32 contour follower nodes", 5*time.Second, `()=>AuraDesktopThemeFx.inspect().surfaceNodeCount>=32`)
	before := s.eval(t, `()=>AuraDesktopThemeFx.inspect().frameCount`).Value.Int()
	s.eval(t, `()=>{
		const record=window.greenSlimeProbe;
		record.phase='drag';
		record.rect={x:290,y:205,width:390,height:270};
		record.velocity={x:620,y:250};
		window.dispatchEvent(new CustomEvent('aurago:desktop-window',{detail:record}));
	}`)
	s.waitFor(t, "inertial frame deformation", 3*time.Second,
		`before=>{const fx=AuraDesktopThemeFx.inspect();return fx.frameCount>before && fx.maxDisplacement>0 && fx.maxDisplacement<=18}`, before)
	s.eval(t, `()=>{
		const record=window.greenSlimeProbe;
		record.phase='drag-end';
		window.dispatchEvent(new CustomEvent('aurago:desktop-window',{detail:record}));
	}`)
	s.waitFor(t, "jelly release settles", 3*time.Second,
		`()=>AuraDesktopThemeFx.inspect().currentMaxDisplacement<1`)
	dropsBefore := s.eval(t, `()=>AuraDesktopThemeFx.inspect().dropCount`).Value.Int()
	s.waitFor(t, "periodic detached slime drop", 6*time.Second,
		`before=>AuraDesktopThemeFx.inspect().dropCount>before`, dropsBefore)
	if count := s.eval(t, `()=>AuraDesktopThemeFx.inspect().activeDrops`).Value.Int(); count < 1 || count > 72 {
		t.Fatalf("detached drop pool escaped its bounds: active=%v", count)
	}

	s.store.set(map[string]string{"windows.animations": "false"})
	s.pushSettings(t)
	s.waitFor(t, "static Green Slime mode", 5*time.Second,
		`()=>!AuraDesktopThemeFx.inspect().motion && !AuraDesktopThemeFx.inspect().rafPending`)
	maskCount := s.eval(t, `()=>AuraDesktopThemeFx.inspect().maskCount`).Value.Int()
	s.eval(t, `()=>{
		const modal=document.createElement('div');
		modal.className='vd-modal-backdrop';
		modal.style.cssText='position:fixed;left:20px;top:20px;width:31px;height:27px;z-index:999999;background:transparent;';
		modal.id='green-slime-mask-probe';
		document.body.appendChild(modal);
	}`)
	s.waitFor(t, "static modal mask refresh", 3*time.Second,
		`before=>AuraDesktopThemeFx.inspect().maskCount===before+1`, maskCount)
	s.eval(t, `()=>document.getElementById('green-slime-mask-probe')?.remove()`)
	s.waitFor(t, "static modal mask removal refresh", 3*time.Second,
		`before=>AuraDesktopThemeFx.inspect().maskCount===before`, maskCount)
}
