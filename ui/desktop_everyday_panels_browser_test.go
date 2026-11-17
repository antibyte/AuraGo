package ui

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func captureEverydayPanel(t *testing.T, page *rod.Page, name string) {
	t.Helper()
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".png"), page.MustScreenshot(), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopEverydayQuickSettingsBrowser(t *testing.T) {
	var writes atomic.Int32
	settings := desktopEverydaySettingsHandler()
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/desktop/settings": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				writes.Add(1)
			}
			settings.ServeHTTP(w, r)
		}),
	})
	page.MustElement("#vd-quick-settings-button").MustClick()
	page.MustWait(`()=>!!document.querySelector('#vd-quick-settings:not([hidden])')`)
	if !page.MustEval(`()=>{
        const p=document.querySelector('#vd-quick-settings');
        return p.contains(document.activeElement)&&getComputedStyle(p).backgroundColor!=='rgba(0, 0, 0, 0)'
            &&!p.querySelector('[data-quick-setting="appearance.fruity_mode"]');
    }`).Bool() {
		t.Fatal("standard quick settings must have an opaque themed surface, focus and no unsupported mode")
	}
	captureEverydayPanel(t, page, "quick-standard")
	page.MustEval(`()=>{const v=document.querySelector('[data-quick-setting="sound.volume"]');v.value='35';v.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustWait(`()=>auditTest.state.bootstrap.settings['sound.volume']==='0.35'`)
	if writes.Load() != 1 {
		t.Fatalf("volume writes = %d; want one", writes.Load())
	}
	page.MustEval(`()=>{auditTest.state.bootstrap.readonly=true;window.dispatchEvent(new CustomEvent('aurago:desktop-settings'));}`)
	page.MustElement("[data-quick-dnd]").MustClick()
	if !page.MustEval(`()=>AuraDesktopNotificationCenter.policy().dnd&&[...document.querySelectorAll('#vd-quick-settings [data-quick-setting]')].every(el=>el.disabled)&&!document.querySelector('[data-quick-dnd]').disabled`).Bool() {
		t.Fatal("browser-local DND must remain usable while server preferences are read-only")
	}
	if writes.Load() != 1 {
		t.Fatal("DND must not write server preferences")
	}
	page.MustEval(`async()=>{
        auditTest.state.bootstrap.readonly=false;
        await auditTest.saveDesktopPreference('appearance.theme','fruity');
        window.mediaTestOwner={};window.mediaTestActions=[];
        AuraDesktopMediaSession.claim(mediaTestOwner,{priority:60,playbackState:'playing',metadata:{title:'Test track',artist:'Test artist'},handlers:{
            pause:()=>{mediaTestActions.push('pause');AuraDesktopMediaSession.claim(mediaTestOwner,{priority:60,playbackState:'paused',metadata:{title:'Test track',artist:'Test artist'},handlers:{play:()=>mediaTestActions.push('play')}});}
        }});
    }`)
	page.MustWait(`()=>!!document.querySelector('[data-quick-setting="appearance.fruity_mode"]')`)
	page.MustElement("[data-media-action=pause]").MustClick()
	page.MustWait(`()=>!!document.querySelector('[data-media-action=play]:not(:disabled)')`)
	if !page.MustEval(`()=>!document.querySelector('#vd-quick-settings').hidden&&mediaTestActions.join(',')==='pause'&&document.querySelector('[data-media-action=play]')===document.activeElement`).Bool() {
		t.Fatal("media dispatch must preserve focus on the logical transport control")
	}
	captureEverydayPanel(t, page, "quick-fruity-media")
	page.MustEval(`()=>{document.dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true}));}`)
	if !page.MustEval(`()=>document.querySelector('#vd-quick-settings').hidden&&document.activeElement.id==='vd-quick-settings-button'`).Bool() {
		t.Fatal("Escape must close quick settings and return focus to its launcher")
	}
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustSetViewport(390, 640, 1, false)
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	page.MustElement("#vd-quick-settings-button").MustClick()
	captureEverydayPanel(t, page, "quick-small")
	if !page.MustEval(`()=>{
        const r=document.querySelector('#vd-quick-settings').getBoundingClientRect();
        return r.left>=0&&r.top>=0&&r.right<=innerWidth+1&&r.bottom<=innerHeight+1;
    }`).Bool() {
		t.Fatalf("quick settings must fit a small viewport: %s", page.MustEval(`()=>JSON.stringify({panel:document.querySelector('#vd-quick-settings').getBoundingClientRect().toJSON(),anchor:document.querySelector('#vd-quick-settings-button').getBoundingClientRect().toJSON(),style:document.querySelector('#vd-quick-settings').getAttribute('style'),viewport:[innerWidth,innerHeight]})`).Str())
	}
	page.MustSetViewport(640, 450, 2, false)
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{
		{Name: "forced-colors", Value: "active"}, {Name: "prefers-reduced-motion", Value: "reduce"},
	}}).Call(page); err != nil {
		t.Fatal(err)
	}
	if !page.MustEval(`()=>{
        const p=document.querySelector('#vd-quick-settings'),r=p.getBoundingClientRect();
        return matchMedia('(forced-colors: active)').matches&&r.top>=0&&r.bottom<=innerHeight+1&&p.scrollHeight>=p.clientHeight;
    }`).Bool() {
		t.Fatal("quick settings must remain reachable at 200% equivalent zoom with forced colors")
	}
	if !page.MustEval(`()=>{
        const control=document.querySelector('[data-media-action=play]');control.focus();
        const r=control.getBoundingClientRect();return document.activeElement===control&&r.top>=0&&r.bottom<=innerHeight;
    }`).Bool() {
		t.Fatal("keyboard focus must scroll media controls into view in the constrained panel")
	}
	captureEverydayPanel(t, page, "quick-forced-colors-zoom")
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatal(got)
	}
}

func TestDesktopEverydayMonthCalendarBrowser(t *testing.T) {
	var calls atomic.Int32
	today := time.Now()
	nextMonth := time.Date(today.Year(), today.Month()+1, 1, 12, 0, 0, 0, today.Location())
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/desktop/integrations/appointments": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			call := calls.Add(1)
			if call == 1 {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "title": "Current appointment", "date_time": today.Format(time.RFC3339), "status": "pending"},
				{"id": 2, "title": "Next appointment", "date_time": nextMonth.Format(time.RFC3339), "status": "pending"},
				{"id": 3, "title": "Cancelled appointment", "date_time": today.Format(time.RFC3339), "status": "cancelled"},
			})
		}),
	})
	page.MustEval(`()=>{window.SYSTEM_LANG='de';}`)
	page.MustElement("#vd-clock").MustClick()
	page.MustWait(`()=>!!document.querySelector('[data-clock-action=retry]')`)
	if !page.MustEval(`()=>!document.querySelector('.vd-clock-calendar-agenda').textContent.includes(window.t('desktop.clock_no_events'))`).Bool() {
		t.Fatal("appointment errors must not pretend there are no events")
	}
	page.MustElement("[data-clock-action=retry]").MustClick()
	if page.MustEval(`()=>document.querySelector('#vd-clock-popup').hidden`).Bool() {
		t.Fatal("retry must not count as a click outside the re-rendered calendar")
	}
	page.MustWait(`()=>document.querySelector('.vd-clock-calendar-agenda')?.textContent.includes('Current appointment')===true`)
	if !page.MustEval(`()=>{
        const p=document.querySelector('#vd-clock-popup');
        return p.contains(document.activeElement)&&p.querySelectorAll('[data-clock-day]').length>=28&&!p.textContent.includes('Cancelled appointment')&&p.querySelector('.vd-clock-calendar-weekdays span').textContent==='Mo';
    }`).Bool() {
		t.Fatal("calendar load must keep keyboard focus and omit cancelled events")
	}
	page.MustEval(`()=>window.monthBefore=document.getElementById('vd-clock-calendar-title').textContent`)
	page.MustElement("[data-clock-action=next-month]").MustClick()
	if !page.MustEval(`()=>document.getElementById('vd-clock-calendar-title').textContent!==monthBefore`).Bool() {
		t.Fatal("next month did not advance")
	}
	page.MustEval(`(date)=>document.querySelector('[data-clock-day="'+date+'"]').click()`, nextMonth.Format("2006-01-02"))
	page.MustWait(`()=>document.querySelector('.vd-clock-calendar-agenda').textContent.includes('Next appointment')`)
	if calls.Load() != 2 {
		t.Fatal("month navigation must reuse the loaded appointment list")
	}
	page.MustElement("[data-clock-action=today]").MustClick()
	if !page.MustEval(`()=>document.querySelector('.vd-clock-calendar-agenda').textContent.includes('Current appointment')`).Bool() {
		t.Fatal("Today must restore the current day and agenda")
	}
	if !page.MustEval(`() => {
        const busy = document.querySelector('.vd-clock-calendar-day.has-events');
        const top = busy.getBoundingClientRect().top;
        const plain = [...document.querySelectorAll('.vd-clock-calendar-day:not(.has-events)')]
            .find(day => Math.abs(day.getBoundingClientRect().top - top) < 1);
        return plain && Math.abs(plain.firstElementChild.getBoundingClientRect().top -
            busy.firstElementChild.getBoundingClientRect().top) < 1;
    }`).Bool() {
		t.Fatal("appointment marks must not lift the day number above its neighbours")
	}
	captureEverydayPanel(t, page, "calendar-standard")
	page.MustSetViewport(390, 640, 1, false)
	page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	if !page.MustEval(`()=>{const r=document.querySelector('#vd-clock-popup').getBoundingClientRect();return r.left>=0&&r.top>=0&&r.right<=innerWidth+1&&r.bottom<=innerHeight+1;}`).Bool() {
		t.Fatal("month calendar must fit a narrow viewport")
	}
	page.MustElement("#vd-clock").MustClick()
	if !page.MustEval(`()=>document.querySelector('#vd-clock-popup').hidden`).Bool() {
		t.Fatal("clock launcher should toggle the calendar closed")
	}
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatal(got)
	}
}

func TestDesktopEverydayFileHistoryBrowser(t *testing.T) {
	var undoCalls atomic.Int32
	var trashCalls atomic.Int32
	releaseUndo := make(chan struct{})
	lateTrashStarted := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseUndo) }) }
	defer release()
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/desktop/trash": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if trashCalls.Add(1) == 2 {
				close(lateTrashStarted)
				select {
				case <-releaseUndo:
				case <-r.Context().Done():
					return
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "moves": []map[string]string{
				{"from": "Desktop/example.txt", "path": "Trash/example.txt", "undo_receipt": "receipt-original"},
			}})
		}),
		"/api/desktop/file-undo": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Receipt string `json:"receipt"`
			}
			if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&body) != nil {
				http.Error(w, "invalid", 400)
				return
			}
			count := undoCalls.Add(1)
			want := "receipt-original"
			if count >= 2 {
				want = "receipt-redo"
			}
			if body.Receipt != want {
				http.Error(w, "unexpected receipt", 409)
				return
			}
			if count == 3 {
				select {
				case <-releaseUndo:
				case <-r.Context().Done():
					return
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "from": "Trash/example.txt", "path": "Desktop/example.txt", "undo_receipt": "receipt-redo"})
		}),
	})
	page.MustEval(`async()=>{await auditTest.movePathsToTrash(['Desktop/example.txt']);}`)
	page.MustWait(`()=>AuraDesktopFileHistory.snapshot().canUndo&&!!document.querySelector('[data-file-undo] .vd-toast-action')`)
	if !page.MustEval(`()=>!AuraDesktopNotificationCenter.history().some(item=>item.message===window.t('desktop.context_delete'))`).Bool() {
		t.Fatal("Undo callbacks must not enter persisted notification history")
	}
	page.MustElement("[data-file-undo] .vd-toast-action").MustClick()
	page.MustWait(`()=>AuraDesktopFileHistory.snapshot().canRedo&&!AuraDesktopFileHistory.snapshot().busy`)
	if undoCalls.Load() != 1 {
		t.Fatal("Undo toast must invoke exactly the current receipt")
	}
	page.MustEval(`()=>document.getElementById('vd-workspace').dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:100,clientY:100}))`)
	page.MustEval(`()=>{
        const redo=[...document.querySelectorAll('.vd-context-menu button')].find(node=>node.querySelector('.vd-context-label')?.textContent===window.t('desktop.menu_redo'));
        if(!redo||redo.disabled)throw new Error('Redo menu unavailable');redo.click();
    }`)
	page.MustWait(`()=>AuraDesktopFileHistory.snapshot().canUndo&&!AuraDesktopFileHistory.snapshot().busy`)
	if undoCalls.Load() != 2 {
		t.Fatal("Desktop menu must share the same redo history")
	}
	page.MustEval(`()=>{
        window.undoScopeInput=document.createElement('textarea');document.body.appendChild(undoScopeInput);undoScopeInput.focus();
        undoScopeInput.dispatchEvent(new KeyboardEvent('keydown',{key:'z',ctrlKey:true,bubbles:true,cancelable:true}));
    }`)
	if undoCalls.Load() != 2 {
		t.Fatal("text undo must not invoke file undo")
	}
	page.MustEval(`()=>undoScopeInput.remove()`)
	page.MustElement("[data-file-undo] .vd-toast-action").MustClick()
	page.MustWait(`()=>AuraDesktopFileHistory.snapshot().busy`)
	page.MustEval(`()=>{window.lateMutation=auditTest.movePathsToTrash(['Desktop/late.txt']);}`)
	select {
	case <-lateTrashStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("late producer request did not start")
	}
	page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:auth-ended'))`)
	if !page.MustEval(`()=>!AuraDesktopFileHistory.snapshot().canUndo&&AuraDesktopFileHistory.snapshot().undoCount===0`).Bool() {
		t.Fatal("ending auth must clear file history")
	}
	if !page.MustEval(`()=>!document.querySelector('[data-file-undo]:not(.vd-toast-closing)')`).Bool() {
		t.Fatal("auth clear must immediately remove an in-flight Undo toast")
	}
	release()
	page.MustEval(`async()=>{await lateMutation;}`)
	page.MustWait(`()=>!AuraDesktopFileHistory.snapshot().busy`)
	if !page.MustEval(`()=>AuraDesktopFileHistory.snapshot().undoCount===0&&AuraDesktopFileHistory.snapshot().redoCount===0`).Bool() {
		t.Fatal("late Undo or producer responses must not restore auth-cleared history")
	}
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		t.Fatal(got)
	}
}
