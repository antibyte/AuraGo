package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func TestGitHubPermissionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "de", true) + "/config#overview")
	defer func() {
		_, _ = page.Timeout(5 * time.Second).Eval(`()=>{document.querySelector('#shared-modal-cancel')?.click();AuraConfigState.init(configData);resetDirtySnapshot();}`)
		_ = page.Timeout(5 * time.Second).Close()
	}()
	click := func(selector string) {
		t.Helper()
		control := page.Timeout(10 * time.Second).MustElement(selector)
		control.MustEval(`()=>this.scrollIntoView({block:'center',inline:'center',behavior:'instant'})`)
		box := control.MustShape().Box()
		page.Mouse.MustMoveTo(box.X+box.Width/2, box.Y+box.Height/2).MustClick(proto.InputMouseButtonLeft)
	}
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async ()=>{
  configData.github={enabled:true,owner:'owner',allowed_repos:[]}; AuraConfigState.init(configData); resetDirtySnapshot();
  await selectSection('github');
  _githubReposData=[{name:'legacy',full_name:'owner/legacy',trust_migration_required:true},{name:'created',full_name:'owner/created',agent_created:true}];
  document.querySelector('#github-repos-list').innerHTML=githubBuildRepoList(_githubReposData,[]);
  resetDirtySnapshot();
 }`)
	if !page.MustEval(`()=>{
  const boxes=[...document.querySelectorAll('.gh-repo-check')];
  return !document.querySelector('[data-path="github.allow_delete"]').classList.contains('on') &&
   boxes.length===2 && !boxes[0].checked && !boxes[0].disabled && boxes[1].checked && boxes[1].disabled &&
   document.querySelector('.gh-trust-migration').textContent.includes('ausdrücklich');
 }`).Bool() {
		t.Fatal("default deletion or migration policy is incorrect")
	}
	click(".gh-repo-item")
	click(`[data-path="github.allow_delete"]`)
	waitForJSBool(t, page, `()=>document.querySelector('#shared-modal-overlay')?.classList.contains('active')`)
	if page.MustEval(`()=>AuraConfigState.get('github.allow_delete')===true`).Bool() {
		t.Fatal("deletion enabled before administrator confirmation")
	}
	click("#shared-modal-confirm")
	waitForJSBool(t, page, `()=>AuraConfigState.get('github.allow_delete')===true`)
	if !page.MustEval(`()=>document.querySelector('#github-allowed-repos-input').value==='owner/legacy' && AuraConfigState.get('github.allow_delete')===true`).Bool() {
		t.Fatal("explicit administrator approval was not retained")
	}
	page.MustEval(`()=>{
  configData.github.owner = 'owner';
  document.querySelector('[data-path="github.owner"]').value = 'owner';
  document.querySelector('#github-allowed-repos-input').value = 'kept/elsewhere, Owner/Legacy';
  _githubReposData = [
    {name:'legacy', full_name:'owner/legacy', trust_migration_required:true},
    {name:'legacy', full_name:'stranger/legacy'},
    {name:'created', full_name:'owner/created', agent_created:true}
  ];
  document.querySelector('#github-repos-list').innerHTML = githubBuildRepoList(_githubReposData, ['kept/elsewhere', 'Owner/Legacy']);
 }`)
	if !page.MustEval(`()=>{
  const boxes=[...document.querySelectorAll('.gh-repo-check')];
  return boxes.length===3 && boxes[0].checked && !boxes[0].disabled && !boxes[1].checked && boxes[2].checked && boxes[2].disabled;
 }`).Bool() {
		t.Fatal("allowlist matching ignored case or treated a bare name as global")
	}
	click("#github-repos-list .gh-repo-item:nth-of-type(2)")
	if !page.MustEval(`()=>{
  const parts=document.querySelector('#github-allowed-repos-input').value.split(',').map(entry=>entry.trim());
  return parts.includes('kept/elsewhere') && parts.includes('owner/legacy') && parts.includes('stranger/legacy') && !parts.some(entry=>entry.toLowerCase()==='legacy');
 }`).Bool() {
		t.Fatal("visible page replaced allowlist entries that were not listed")
	}
	for _, width := range []int{390, 1440} {
		page.MustSetViewport(width, 1000, 1, width == 390)
		for _, theme := range []string{"dark", "light"} {
			page.MustEval(`async theme=>{document.documentElement.dataset.theme=theme;document.body.dataset.theme=theme;document.querySelector('.gh-trust-migration').scrollIntoView({block:'center'});await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));}`, theme)
			if !page.MustEval(`()=>{const c=document.getElementById('content'),m=document.querySelector('.gh-trust-migration');return c.scrollWidth<=c.clientWidth+1 && m.getClientRects().length>0 && m.scrollWidth<=m.clientWidth+1 && getComputedStyle(m).whiteSpace!=='nowrap'}`).Bool() {
				t.Fatalf("migration layout overflow at %d %s", width, theme)
			}
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("github-trust-%s-%d.png", theme, width)), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
