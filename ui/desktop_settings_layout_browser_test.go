package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDesktopSettingsLayoutBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	words := readDesktopAssetText(t, "lang/desktop/de.json")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html lang="de"><link rel="stylesheet" href="/fonts/fonts.css"><link rel="stylesheet" href="/shared-variables.css"><link rel="stylesheet" href="/shared-utilities.css"><link rel="stylesheet" href="/shared-components.css"><link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/desktop-app-common.css"><link rel="stylesheet" href="/css/desktop-app-settings.css"><style>#settings{height:100vh;max-width:100%;overflow:hidden}</style><body class="desktop-body" data-theme="standard"><div id="settings" class="vd-window-content"></div><script src="/js/desktop/apps/settings.js"></script><script>
const words=`+words+`;const settings={'agent.provider':'local','agent.show_chat_button':'true'};
const ctx={t:key=>words[key]||key,esc:value=>String(value).replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';'),iconMarkup:()=>'',settingValue:key=>settings[key]||'',settingBool:key=>settings[key]==='true',state:{bootstrap:{
workspace:{root:'/srv/aurago/agent_workspace/'+'a-long-workspace-directory/'.repeat(5)},readonly:false,allow_agent_control:true,installed_apps:Array(19),widgets:Array(7),
providers:[{id:'local',name:'Lokaler KI-Assistent',model:'Qwen3-235B-A22B-Thinking-2507-custom-inference-provider'}]
}}};
window.renderCategory=category=>SettingsApp.render(document.querySelector('#settings'),{...ctx,category});renderCategory('system');
</script></body></html>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(30 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
		page.MustEval(`theme=>{document.body.dataset.theme=theme.startsWith('fruity')?'fruity':'standard';document.body.dataset.fruityMode=theme.endsWith('dark')?'dark':'light';}`, theme)
		for _, width := range []int{1212, 820, 390} {
			page.MustSetViewport(width, 700, 1, false)
			for _, category := range []string{"system", "agent"} {
				page.MustEval(`category=>renderCategory(category)`, category)
				if !page.MustEval(`()=>[...document.querySelectorAll('.vd-setting-row')].every(row=>{
                    const r=row.getBoundingClientRect(),label=row.querySelector('.vd-setting-label'),l=label.getBoundingClientRect(),control=row.lastElementChild,c=control.getBoundingClientRect();
                    return l.left>=r.left+12 && c.right<=r.right-12 && row.scrollWidth<=row.clientWidth+1 && label.scrollWidth<=label.clientWidth+1 && l.width>=80 && (c.left>=l.right+8 || c.top>=l.bottom) && c.bottom<=r.bottom;
                })`).Bool() {
					t.Errorf("%s/%s at %dpx needs inset text, readable columns and no overflow", theme, category, width)
				}
				if category == "system" && !page.MustEval(`()=>document.querySelector('.vd-setting-value').textContent===ctx.state.bootstrap.workspace.root && Number(getComputedStyle(document.querySelector('.vd-setting-value')).fontWeight)<=600`).Bool() {
					t.Error("info values must retain their full text without excessive bold weight")
				}
				if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" && width == 1212 {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					page.MustScreenshot(filepath.Join(dir, "settings-"+category+"-"+theme+".png"))
				}
			}
		}
	}
}
