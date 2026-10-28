package ui

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

// dockerManagerScaleContainers is a host with thousands of containers: `running` web containers and `stopped` exited ones from one
// image (a CI runner leaves one per job). Every hundredth stopped container failed (exit code 1).
func dockerManagerScaleContainers(running, stopped int) []map[string]any {
	list := make([]map[string]any, 0, running+stopped)
	add := func(index int, name, state, status, image string) {
		full := fmt.Sprintf("%064x", index+1)
		list = append(list, map[string]any{
			"id": full[:12], "full_id": full, "names": []string{name}, "image": image, "image_id": "sha256:aaa", "state": state, "status": status,
			"created": 1760000000 + index, "ports": []any{}, "mounts": []any{}, "networks": []string{"bridge"},
		})
	}
	for i := 0; i < running; i++ {
		add(i, fmt.Sprintf("web-%02d", i), "running", "Up 3 hours", "nginx:1.27")
	}
	for i := 0; i < stopped; i++ {
		status := "Exited (0) 3 hours ago"
		if i%100 == 0 {
			status = "Exited (1) 3 hours ago"
		}
		add(running+i, fmt.Sprintf("ci-job-%04d", i), "exited", status, "ci-runner:1")
	}
	return list
}

const (
	dmScaleTableRows = `()=>document.querySelectorAll('[data-tbody] > tr').length`
	dmScaleListItems = `()=>document.querySelectorAll('[data-stack-list] .dm-srow').length`
	dmScaleGroupRow  = `document.querySelector('[data-tbody] tr.dm-image-row')`
	dmScaleGroupItem = `document.querySelector('[data-stack-list] [data-group-key]')`
)

// A host with 3,300 containers (49 running, 3,251 stopped from one image): the table and the Stacks list render a bounded number of rows,
// the stopped ones are one collapsed group that opens by click and by keyboard, "Show more" adds 200, search and filter act inside the
// group, a member opens the drawer, and a paint without new data sorts nothing and leaves the rows that did not change alone.
func TestDesktopDockerManagerScaleBrowser(t *testing.T) {
	fixture := &dockerManagerScaleFixture{dockerManagerFixture: newDockerManagerFixture()}
	fixture.setLocal(dockerManagerScaleContainers(49, 3251))
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/docker-manager/": fixture})
	page.MustEval(`()=>auditTest.openApp('docker-manager')`)
	dockerManagerWaitFor(t, "the stream subscription", func() bool { return strings.Contains(fixture.lastSubscription(), `"hosts":["local","h_nas","h_edge"]`) })
	count := func(script string) int { return page.MustEval(script).Int() }
	must := func(what, script string) {
		t.Helper()
		if !page.MustEval(script).Bool() {
			t.Fatal(what)
		}
	}

	// Stacks (the default section): the 49 running containers and ONE item for the 3,251 stopped ones
	dmScaleWait(t, page, `()=>!!`+dmScaleGroupItem)
	if got := count(dmScaleListItems); got > 250 {
		t.Fatalf("the Stacks list renders %d items for 3,300 containers, want at most about 250", got)
	}
	must("the group item must be collapsed and say how many containers it holds", `()=>{const b=`+dmScaleGroupItem+`;return b.getAttribute('aria-expanded')==='false' && /3[.,]?251/.test(b.textContent) && b.textContent.includes('ci-runner:1')}`)
	must("a collapsed group renders none of its members", `()=>document.querySelectorAll('[data-stack-list] [data-action="open-container"]').length===49`)
	page.MustEval(`()=>` + dmScaleGroupItem + `.click()`)
	dmScaleWait(t, page, `()=>document.querySelectorAll('[data-stack-list] [data-action="open-container"]').length===49+200`)
	if got := count(dmScaleListItems); got < 250 || got > 262 {
		t.Fatalf("the Stacks list with the group open has %d items, want 49 running + 200 members + the group, the stacks and the show-more item", got)
	}
	page.MustEval(`()=>document.querySelector('[data-stack-list] [data-action="show-more"]').click()`)
	dmScaleWait(t, page, `()=>document.querySelectorAll('[data-stack-list] [data-action="open-container"]').length===49+400`)
	// the arrow keys: Left closes the group, Right opens it, Left on a member goes back to the group
	page.MustEval(`()=>` + dmScaleGroupItem + `.focus()`)
	page.Keyboard.MustType(input.ArrowLeft)
	dmScaleWait(t, page, `()=>`+dmScaleGroupItem+`.getAttribute('aria-expanded')==='false' && document.activeElement===`+dmScaleGroupItem)
	page.Keyboard.MustType(input.ArrowRight)
	dmScaleWait(t, page, `()=>`+dmScaleGroupItem+`.getAttribute('aria-expanded')==='true' && document.activeElement===`+dmScaleGroupItem)
	if got := count(`()=>document.querySelectorAll('[data-stack-list] [data-action="open-container"]').length`); got != 49+400 {
		t.Fatalf("members after closing and opening again = %d, want the 400 shown before (the step stays)", got)
	}
	page.MustEval(`()=>document.querySelector('[data-stack-list] .is-nested [data-action="open-container"]').focus()`)
	page.Keyboard.MustType(input.ArrowLeft)
	dmScaleWait(t, page, `()=>document.activeElement===`+dmScaleGroupItem)
	// a member opens the drawer
	page.MustEval(`()=>document.querySelector('[data-stack-list] .is-nested [data-action="open-container"]').click()`)
	dmScaleWait(t, page, `()=>!document.querySelector('[data-drawer]').hidden && /^ci-job-/.test(document.querySelector('[data-drawer] h2')?.textContent||'')`)
	page.MustEval(`()=>document.querySelector('[data-action="close-detail"]').click()`)
	dmScaleWait(t, page, `()=>document.querySelector('[data-drawer]').hidden`)
	// an image group is open or closed for both sections: closing it here leaves it closed in the table
	page.MustEval(`()=>` + dmScaleGroupItem + `.click()`)
	dmScaleWait(t, page, `()=>`+dmScaleGroupItem+`.getAttribute('aria-expanded')==='false'`)

	// Containers: 49 running rows and the collapsed group
	page.MustEval(`()=>document.querySelector('[data-action="pick-section"][data-section="containers"]').click()`)
	dmScaleWait(t, page, `()=>!!`+dmScaleGroupRow)
	if got := count(dmScaleTableRows); got > 80 {
		t.Fatalf("the table renders %d rows with the group closed, want the running containers, the group and the project headers", got)
	}
	must("the table group row is a collapsed treegrid parent", `()=>{const r=`+dmScaleGroupRow+`;return r.getAttribute('aria-expanded')==='false' && r.dataset.action==='toggle-group' && /3[.,]?251/.test(r.textContent) && document.querySelector('[data-table]').getAttribute('role')==='treegrid'}`)
	if got := count(`()=>document.querySelector('[data-tbody]').querySelectorAll('*').length`); got > 3000 {
		t.Fatalf("the table body holds %d elements, want a few hundred rows' worth", got)
	}

	// Search inside the group: its matches show, the group opens by itself; fewer than five matches are plain rows; clearing closes it again
	search := page.MustElement(`[data-search]`)
	search.MustInput("ci-job-004")
	dmScaleWait(t, page, dmScaleMembersIs(10)+`&&`+dmScaleGroupRow+`?.getAttribute('aria-expanded')==='true'`)
	search.MustSelectAllText().MustInput("ci-job-0042")
	dmScaleWait(t, page, dmScaleMembersIs(1)+`&&!`+dmScaleGroupRow)
	search.MustSelectAllText().MustInput("ci-job-00")
	dmScaleWait(t, page, dmScaleMembersIs(100)+`&&`+dmScaleGroupRow+`?.getAttribute('aria-expanded')==='true'`)
	// the admin may close an opened group while the search is active
	page.MustEval(`()=>` + dmScaleGroupRow + `.click()`)
	dmScaleWait(t, page, dmScaleMembersIs(0)+`&&`+dmScaleGroupRow+`?.getAttribute('aria-expanded')==='false'`)
	search.MustSelectAllText().MustType(input.Backspace)
	dmScaleWait(t, page, dmScaleMembersIs(50)+`&&`+dmScaleGroupRow+`?.getAttribute('aria-expanded')==='false'`)
	// the state filter: the failed ones (every hundredth stopped container) are the matches inside the group
	page.MustEval(`()=>document.querySelector('[data-filter="problem"]').click()`)
	dmScaleWait(t, page, dmScaleMembersIs(33)+`&&`+dmScaleGroupRow+`?.getAttribute('aria-expanded')==='true'`)
	page.MustEval(`()=>document.querySelector('[data-filter="running"]').click()`)
	dmScaleWait(t, page, dmScaleMembersIs(50)+`&&!`+dmScaleGroupRow)
	page.MustEval(`()=>document.querySelector('[data-filter="all"]').click()`)
	dmScaleWait(t, page, `()=>!!`+dmScaleGroupRow)

	// Opening the group: 200 members and "Show 200 more" with how many are hidden; by click, then by keyboard
	page.MustEval(`()=>{const r=` + dmScaleGroupRow + `;r.focus();r.click()}`)
	dmScaleWait(t, page, dmScaleMembersIs(50+200))
	if got := count(dmScaleTableRows); got < 250 || got > 262 {
		t.Fatalf("the table with the group open has %d rows, want about 250", got)
	}
	must("the group's own show-more row names the hidden containers one level below the group", `()=>{const b=document.querySelector('[data-tbody] [data-action="show-more"]');return !!b && /3[.,]?051/.test(b.textContent) && Number(b.closest('tr').getAttribute('aria-level'))===Number(`+dmScaleGroupRow+`.getAttribute('aria-level'))+1}`)
	page.MustEval(`()=>document.querySelector('[data-tbody] [data-action="show-more"]').click()`)
	dmScaleWait(t, page, dmScaleMembersIs(50+400))
	page.MustEval(`()=>` + dmScaleGroupRow + `.focus()`)
	page.Keyboard.MustType(input.ArrowLeft)
	dmScaleWait(t, page, dmScaleMembersIs(50)+`&&document.activeElement===`+dmScaleGroupRow)
	page.Keyboard.MustType(input.Enter)
	dmScaleWait(t, page, dmScaleMembersIs(50+400)+`&&document.activeElement===`+dmScaleGroupRow)
	page.Keyboard.MustType(input.Space)
	dmScaleWait(t, page, dmScaleMembersIs(50)+`&&document.activeElement===`+dmScaleGroupRow)
	page.Keyboard.MustType(input.ArrowRight)
	dmScaleWait(t, page, dmScaleMembersIs(50+400))
	must("exactly one row of the table is in the tab order", `()=>document.querySelectorAll('[data-tbody] [data-nav-key][tabindex="0"]').length===1`)
	// Enter on the focused show-more row adds the next 200
	page.MustEval(`()=>document.querySelector('[data-tbody] tr.dm-more-row').focus()`)
	page.Keyboard.MustType(input.Enter)
	dmScaleWait(t, page, dmScaleMembersIs(50+600))

	// A member of the group opens the drawer (click); a paint without new data sorts nothing; the drawer stays when its group is closed
	page.MustEval(`()=>{window.__groupCalls=0;const m=DockerManagerModel,g=m.groupRows;m.groupRows=function(){window.__groupCalls++;return g.apply(this,arguments)}}`)
	page.MustEval(`()=>document.querySelector('[data-tbody] tr.dm-row[data-row-key*="000000000100"] .dm-cell-name').click()`)
	dmScaleWait(t, page, `()=>!document.querySelector('[data-drawer]').hidden && /^ci-job-/.test(document.querySelector('[data-drawer] h2')?.textContent||'')`)
	time.Sleep(300 * time.Millisecond)
	if calls := count(`()=>window.__groupCalls`); calls != 0 {
		t.Fatalf("a paint without new data sorted the groups %d times, want none", calls)
	}
	page.MustEval(`()=>` + dmScaleGroupRow + `.click()`)
	dmScaleWait(t, page, dmScaleMembersIs(50))
	must("the drawer of a member stays when its group is closed", `()=>!document.querySelector('[data-drawer]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-action="close-detail"]').click()`)

	// A reload replaces only the rows that changed: the element of an unchanged row survives, the new container shows
	page.MustEval(`()=>{window.__web=document.querySelector('[data-tbody] tr.dm-row[data-row-key*="000000000003"]');window.__web.dataset.mark='kept'}`)
	next := append(dockerManagerScaleContainers(49, 3251), map[string]any{"id": "ffffffffffff", "full_id": strings.Repeat("f", 64), "names": []string{"zz-new"}, "image": "nginx:1.27", "image_id": "sha256:aaa", "state": "running", "status": "Up 1 second", "created": 1760009999, "ports": []any{}, "mounts": []any{}, "networks": []string{"bridge"}})
	fixture.setLocal(next)
	fixture.pushStream("event", dockerManagerContainerEvent("zz-new", ""))
	dmScaleWait(t, page, dmScaleMembersIs(51))
	must("a reload must patch the table: the row of an unchanged container is the same element", `()=>window.__web.isConnected && window.__web.dataset.mark==='kept' && document.querySelectorAll('[data-tbody] tr.dm-row[data-row-key*="ffffffffffff"]').length===1`)
	if errors := page.MustEval(`()=>window.fixtureErrors.join('|')`).String(); errors != "" {
		t.Fatalf("page errors: %s", errors)
	}
}

// dmScaleMembersIs is a wait condition: the table holds n container rows.
func dmScaleMembersIs(n int) string {
	return fmt.Sprintf(`()=>document.querySelectorAll('[data-tbody] > tr.dm-row[data-row-key]').length===%d`, n)
}

// dmScaleWait waits up to 15 s for a condition of the page and, when it does not come true, names it and shows what the table holds.
func dmScaleWait(t *testing.T, page *rod.Page, script string) {
	t.Helper()
	if err := page.Timeout(15 * time.Second).Wait(rod.Eval(script)); err != nil {
		state := page.MustEval(`()=>JSON.stringify({rows:document.querySelectorAll('[data-tbody] > tr').length,members:document.querySelectorAll('[data-tbody] > tr.dm-row[data-row-key]').length,group:document.querySelector('[data-tbody] tr.dm-image-row')?.getAttribute('aria-expanded'),query:document.querySelector('[data-search]').value,filter:document.querySelector('[data-filter][aria-pressed="true"]')?.dataset.filter,list:document.querySelectorAll('[data-stack-list] .dm-srow').length,active:document.activeElement?.outerHTML.slice(0,160),errors:window.fixtureErrors})`).String()
		t.Fatalf("waiting for %s: %v\npage: %s", script, err, state)
	}
}
