package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGitHubPermissionsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	page := newSmokeBrowser(t).MustPage(configRefreshFixtureOrigin(t, "de", true) + "/config#overview")
	defer page.MustClose()
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
	page.MustEval(`()=>{
  document.querySelector('.gh-repo-check').click();
  document.querySelector('[data-path="github.allow_delete"]').click();
 }`)
	if !page.MustEval(`()=>document.querySelector('#github-allowed-repos-input').value==='owner/legacy' && AuraConfigState.get('github.allow_delete')===true`).Bool() {
		t.Fatal("explicit administrator approval was not retained")
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
	page.MustEval(`()=>resetDirtySnapshot()`)
}
