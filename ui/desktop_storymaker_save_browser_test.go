package ui

import (
	"net/http"
	"testing"
	"time"
)

func TestStorymakerAppDelayedSavesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newStorymakerFixture(t)
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/storymaker/": fixture}).CancelTimeout().Timeout(3 * time.Minute)
	page.MustEval(`async words=>{
 Object.assign(fixtureWords,words);await AuraDesktopModules.loadAppAssets('storymaker');
 const render=StorymakerApp.render;StorymakerApp.render=function(...args){window.testStorymaker=render(...args);return testStorymaker;};
 await auditTest.openApp('storymaker');
 const fetch=window.fetch.bind(window);window.holdStoryAck=false;
 window.fetch=async(url,init={})=>{
  const response=await fetch(url,init);
  if(window.holdStoryAck&&String(url).endsWith('/story')&&init.method==='PUT'){
   window.holdStoryAck=false;await new Promise(resolve=>{window.storyAckPending=true;window.releaseStoryAck=()=>{window.storyAckPending=false;resolve();};});
  }
  return response;
 };
}`, storymakerWords(t, "en"))
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="create-project"]')`)
	page.MustElement(`[data-stm="new-title"]`).MustInput("Delayed saves")
	storymakerClick(t, page, `[data-stm="create-project"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="project-title"]')`)
	id := fixture.projectIDs()[0]
	storymakerClick(t, page, `[data-stm="tab"][data-tab="code"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="code-editor"] .cm-editor')`)
	storymakerEditorReplace(page, `"variant": "day"`, `"variant": "night"`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-state"]').dataset.state==='dirty'&&!document.querySelector('[data-stm="code-save"]').disabled`)
	page.MustEval(`()=>{window.holdStoryAck=true;}`)
	storymakerClick(t, page, `[data-stm="code-save"]`)
	storymakerWait(t, page, `()=>window.storyAckPending===true`)
	storymakerEditorReplace(page, `"variant": "night"`, `"variant": "day"`)
	// Let the editor's update listener publish the newer draft before releasing the response.
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	page.MustEval(`()=>window.releaseStoryAck()`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-state"]').dataset.state==='dirty'&&!document.querySelector('[data-stm="code-save"]').disabled`)
	story, _ := fixture.storedStory(id)
	if story["scenes"].([]any)[0].(map[string]any)["variant"] != "night" {
		t.Fatal("first save did not store its sent snapshot")
	}
	if !page.MustEval(`()=>document.querySelector('[data-stm="code-editor"] .cm-content').textContent.includes('"variant": "day"')`).Bool() {
		t.Fatal("save acknowledgment overwrote newer typing")
	}
	storymakerClick(t, page, `[data-stm="code-save"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-state"]').dataset.state==='saved'`)
	story, _ = fixture.storedStory(id)
	if story["scenes"].([]any)[0].(map[string]any)["variant"] != "day" {
		t.Fatal("newer draft was lost")
	}

	// An answer for a previous editor generation must not touch the next project.
	storymakerEditorReplace(page, `"variant": "day"`, `"variant": "night"`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-state"]').dataset.state==='dirty'&&!document.querySelector('[data-stm="code-save"]').disabled`)
	page.MustEval(`()=>{window.holdStoryAck=true;}`)
	storymakerClick(t, page, `[data-stm="code-save"]`)
	storymakerWait(t, page, `()=>window.storyAckPending===true`)
	page.MustEval(`async()=>{
 const next=await testStorymaker.api.createProject({title:'Next project',aspect:'16:9',fps:30});
 testStorymaker.ctx.confirmDialog=async()=>true;
 await StorymakerApp.open(testStorymaker.windowId,next.project.id);
 window.nextStory=testStorymaker.story;window.nextETag=testStorymaker.etag;
 window.releaseStoryAck();
 await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
}`)
	if !page.MustEval(`()=>testStorymaker.project.title==='Next project'&&testStorymaker.story===window.nextStory&&testStorymaker.etag===window.nextETag`).Bool() {
		t.Fatal("late save changed the next project")
	}
	storymakerClick(t, page, `[data-stm="tab"][data-tab="code"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="code-editor"] .cm-editor')`)
	storymakerEditorReplace(page, `"variant": "day"`, `"variant": "night"`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-state"]').dataset.state==='dirty'&&!document.querySelector('[data-stm="code-save"]').disabled`)
	page.MustEval(`()=>{window.holdStoryAck=true;}`)
	storymakerClick(t, page, `[data-stm="code-save"]`)
	storymakerWait(t, page, `()=>window.storyAckPending===true`)
	page.MustEval(`async()=>{StorymakerApp.dispose(testStorymaker.windowId);window.releaseStoryAck();await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));}`)
	if !page.MustEval(`()=>testStorymaker.disposed&&testStorymaker.story===null&&!testStorymaker.parts.code`).Bool() {
		t.Fatal("late response revived a disposed editor")
	}
	storymakerNoErrors(t, page)
}
