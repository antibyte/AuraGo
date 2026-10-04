package ui

import "testing"

// Exercise the production status renderer with deterministic API responses.
func TestHomepageStatusOKEnvelopeDoesNotImplyOnlineBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(newPrecisionSmokeOrigin(t))
	defer page.MustClose()
	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	result := page.MustEval(`async source=>{
 const fn=source.match(/async function loadStatus\(\) \{[\s\S]*?\n        \}/);
 if(!fn)throw new Error('production status renderer not found');
 const observed=[];
 for(const running of [false,true]){
  const statusDot={},statusText={},statusPill={},state={modules:{}};
  const data={status:'ok',mode:'python_fallback',python_server:{running}};
  const api=async()=>data,t=key=>key,setStatus=(kind,text)=>observed.push(kind);
  const homepageStatusPreviewURL=()=>'',updatePreviewUrl=()=>{},refreshHomepageTargets=()=>{};
  await eval('('+fn[0]+')')();
 }
 return observed;
 }`, source).Arr()
	if len(result) != 2 || result[0].Str() != "offline" || result[1].Str() != "online" {
		t.Fatalf("status renderer returned %v", result)
	}
}
