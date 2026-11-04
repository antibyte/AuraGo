package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// ecalcPinoutFixturePatch adds the fixture pinouts of scripts/ecalc-pinout-fixtures.js (the text the node test evaluates, too)
// to the served catalog, in the JSON shape of the contract: every outline of rows, the shapes circular, dip, to, jack and board,
// dense connectors (USB-C with 24 contacts, HDMI with 19, a 2 x 20 header, a 38-pin board, DIP-28), variants, wires and long
// texts. The pinouts of the real catalog are taken out of the served JSON unless window.ecalcWithReal is set (then only they
// are served), so that the fixture assertions do not depend on content; their ids stay in window.ecalcRealPinoutIDs.
func ecalcPinoutFixturePatch(t testing.TB) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "scripts", "ecalc-pinout-fixtures.js"))
	if err != nil {
		t.Fatal(err)
	}
	return `j => {
	const real = j.items.filter(item => item.kind === 'pinout');
	window.ecalcRealPinoutIDs = real.map(item => item.id);
	if (!window.ecalcWithReal) {
		j.items = j.items.filter(item => !real.includes(item));
		const fixtures = (` + string(source) + `)();
		window.ecalcFixtureIDs = fixtures.map(item => item.id);
		j.items.push(...fixtures);
	}
	return j;
}`
}

// pinoutReportJS examines the open pinout page and returns a JSON report. It is the geometry check of the real browser: every pin
// of the shown variant is drawn exactly once and has a table row, the fill of a contact is the hex of its pin type, a number
// written inside a contact lies inside it (a round contact: its corners inside the circle), a name or a number outside a contact
// overlaps no contact and no other text, no contact overlaps another, everything lies inside the svg, no text is mirrored
// (the determinant of its screen matrix is positive), and neither the page host nor the document scrolls sideways.
const pinoutReportJS = `()=>{
	const p=document.querySelector('.ec-pinout-page');
	if(!p)return 'null';
	const svg=p.querySelector('svg.ec-pin-art'),problems=[];
	const rect=el=>{const r=el.getBoundingClientRect();return {l:r.left,t:r.top,r:r.right,b:r.bottom,w:r.width,h:r.height};};
	const overlap=(a,b,tol)=>Math.min(a.r,b.r)-Math.max(a.l,b.l)>tol&&Math.min(a.b,b.b)-Math.max(a.t,b.t)>tol;
	const within=(a,b,tol)=>a.l>=b.l-tol&&a.r<=b.r+tol&&a.t>=b.t-tol&&a.b<=b.b+tol;
	const out={id:p.dataset.pinout,view:svg?svg.dataset.view:'',shape:svg?svg.dataset.shape:'',problems};
	const rows=[...p.querySelectorAll('tbody tr[data-pin]')].map(tr=>tr.dataset.pin);
	out.rows=rows;
	if(!svg){problems.push('no drawing');return JSON.stringify(out);}
	const box=rect(svg),vb=svg.viewBox.baseVal;
	out.width=box.w;out.height=box.h;out.scale=box.w/vb.width;
	if(!(box.w>20&&box.h>20))problems.push('the drawing has no size');
	const groups=[...svg.querySelectorAll('g.ec-pin')];
	const numbers=groups.map(g=>g.getAttribute('data-pin'));
	out.pins=numbers;
	if(new Set(numbers).size!==numbers.length)problems.push('a pin is drawn twice');
	const bodies=groups.map(g=>({g,n:g.getAttribute('data-pin'),el:g.querySelector('.ec-pin-body')}));
	bodies.forEach(b=>{b.r=rect(b.el);});
	bodies.forEach(b=>{if(!within(b.r,box,1))problems.push('contact '+b.n+' lies outside the drawing');});
	for(let i=0;i<bodies.length;i++)for(let j=i+1;j<bodies.length;j++)if(overlap(bodies[i].r,bodies[j].r,1))problems.push('contacts '+bodies[i].n+' and '+bodies[j].n+' overlap');
	const texts=[];
	let minPx=1e9;
	for(const b of bodies){
		const num=b.g.querySelector('.ec-pin-num'),name=b.g.querySelector('.ec-pin-name');
		const fs=parseFloat(num.getAttribute('font-size'));minPx=Math.min(minPx,fs*out.scale);
		const nr=rect(num);
		if(b.g.dataset.num==='in'){
			const tol=1;
			if(b.el.tagName==='circle'){
				const cx=(b.r.l+b.r.r)/2,cy=(b.r.t+b.r.b)/2,rad=b.r.w/2+tol;
				const corners=[[nr.l,nr.t],[nr.r,nr.t],[nr.l,nr.b],[nr.r,nr.b]];
				if(corners.some(c=>Math.hypot(c[0]-cx,c[1]-cy)>rad))problems.push('the number of contact '+b.n+' sticks out of its circle');
			}else if(!within(nr,b.r,tol))problems.push('the number of contact '+b.n+' sticks out of its contact ('+nr.w.toFixed(1)+'x'+nr.h.toFixed(1)+' in '+b.r.w.toFixed(1)+'x'+b.r.h.toFixed(1)+')');
			if(getComputedStyle(num).fill===getComputedStyle(b.el).fill)problems.push('the number of contact '+b.n+' has the colour of its contact');
		}else{
			if(bodies.some(o=>overlap(nr,o.r,0.5)))problems.push('the outside number of contact '+b.n+' overlaps a contact');
			if(!within(nr,box,1))problems.push('the outside number of contact '+b.n+' lies outside the drawing');
			texts.push({n:b.n,kind:'number',r:nr});
		}
		if(name){
			const r=rect(name);minPx=Math.min(minPx,parseFloat(name.getAttribute('font-size'))*out.scale);
			if(bodies.some(o=>overlap(r,o.r,0.5)))problems.push('the name of pin '+b.n+' overlaps a contact');
			if(!within(r,box,1))problems.push('the name of pin '+b.n+' lies outside the drawing');
			texts.push({n:b.n,kind:'name',r});
		}
	}
	for(const t of svg.querySelectorAll('text')){
		const m=t.getScreenCTM();
		if(m&&m.a*m.d-m.b*m.c<=0)problems.push('a text is mirrored: '+t.textContent);
	}
	for(let i=0;i<texts.length;i++)for(let j=i+1;j<texts.length;j++)if(overlap(texts[i].r,texts[j].r,0.5))problems.push(texts[i].kind+' of pin '+texts[i].n+' overlaps '+texts[j].kind+' of pin '+texts[j].n);
	const labels=[...svg.querySelectorAll('.ec-pin-inner-label')].map(rect);
	for(const l of labels){if(bodies.some(o=>overlap(l,o.r,0.5)))problems.push('an inner label overlaps a contact');}
	out.minPx=Math.round(minPx*10)/10;
	const host=document.querySelector('.ec-page-host');
	out.sideways=host.scrollWidth>host.clientWidth+1||document.documentElement.scrollWidth>document.documentElement.clientWidth+1;
	if(out.sideways)problems.push('the page scrolls sideways');
	const stage=rect(p.querySelector('.ec-pin-stage')),table=rect(p.querySelector('.ec-pin-tablewrap'));
	out.stacked=table.t>=stage.b-1;
	out.stage=stage.w;
	if(!within(rect(p.querySelector('.ec-pin-art-host')),stage,1))problems.push('the drawing host leaves the stage');
	return JSON.stringify(out);
}`

type pinoutReport struct {
	ID       string   `json:"id"`
	View     string   `json:"view"`
	Shape    string   `json:"shape"`
	Pins     []string `json:"pins"`
	Rows     []string `json:"rows"`
	Problems []string `json:"problems"`
	Width    float64  `json:"width"`
	Height   float64  `json:"height"`
	Scale    float64  `json:"scale"`
	MinPx    float64  `json:"minPx"`
	Sideways bool     `json:"sideways"`
	Stacked  bool     `json:"stacked"`
	Stage    float64  `json:"stage"`
}

// pinoutHarness drives the pinout pages of the real desktop shell.
type pinoutHarness struct {
	*ecalcBrowserFixture
}

func (h *pinoutHarness) report() pinoutReport {
	h.T.Helper()
	raw := h.Page.MustEval(pinoutReportJS).Str()
	if raw == "null" {
		h.T.Fatal("no pinout page is open")
	}
	var report pinoutReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		h.T.Fatalf("pinout report %s: %v", raw, err)
	}
	return report
}

// open selects the pinouts on the rail and the item in the list (by script: the fixture window reaches under the taskbar).
func (h *pinoutHarness) open(id string) {
	h.T.Helper()
	h.Page.MustEval(`id=>{document.querySelector('.ec-rail-btn[data-rail="pinouts"]').click();document.querySelector('.ec-item[data-id="'+id+'"]').click();}`, id)
	h.Wait(`()=>document.querySelector('.ec-pinout-page')?.dataset.pinout==='` + id + `' && !!document.querySelector('.ec-pinout-page svg.ec-pin-art')`)
	// the page measures its host after the next frames (the resize observer of the shell)
	h.Page.MustEval(`()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>r(true))))`)
}

// check opens a pinout and reports the problems of its drawing in every variant and every view it offers.
func (h *pinoutHarness) check(id string, shotAs string) []pinoutReport {
	h.T.Helper()
	h.open(id)
	var reports []pinoutReport
	variants := h.Page.MustEval(`()=>[...document.querySelectorAll('.ec-pinout-page [data-choice="variant"] input')].map(i=>i.value)`).Arr()
	views := h.Page.MustEval(`()=>[...document.querySelectorAll('.ec-pinout-page [data-choice="view"] input')].map(i=>i.value)`).Arr()
	setChoice := func(kind, value string) {
		h.Page.MustEval(`(k,v)=>{const r=document.querySelector('.ec-pinout-page [data-choice="'+k+'"] input[value="'+v+'"]');r.checked=true;r.dispatchEvent(new Event('change',{bubbles:true}));}`, kind, value)
	}
	run := func(label string) {
		report := h.report()
		if len(report.Problems) > 0 {
			h.Shot("pinout-problem-" + id + "-" + label)
			h.T.Errorf("pinout %s (%s): %s", id, label, strings.Join(report.Problems, "; "))
		}
		if len(report.Rows) != len(report.Pins) {
			h.T.Errorf("pinout %s (%s): %d table rows for %d drawn pins", id, label, len(report.Rows), len(report.Pins))
		}
		reports = append(reports, report)
	}
	for vi := 0; vi < max(1, len(variants)); vi++ {
		if len(variants) > 0 {
			setChoice("variant", variants[vi].Str())
		}
		for wi := 0; wi < max(1, len(views)); wi++ {
			label := "base"
			if len(views) > 0 {
				setChoice("view", views[wi].Str())
				label = views[wi].Str()
			}
			if len(variants) > 0 {
				label += "-" + variants[vi].Str()
			}
			run(label)
			if shotAs != "" {
				h.Shot(shotAs + "-" + id + "-" + label)
			}
		}
	}
	return reports
}

// TestDesktopECalcPinoutsBrowser drives the pinout pages inside the real desktop shell with fixture pinouts injected into the
// served catalog: the Pinouts list sectioned by group, the drawing of every fixture in every view and variant (geometry measured
// in the browser), the mirror image of the other view, the hover and click link between a contact and its row, the selected
// row scrolled into view inside the table, the filter, the copy, the variants and their persistence, the stacked layout of a
// phone-sized window and the real catalog have tests of their own. Set AURAGO_BROWSER_ARTIFACT_DIR to keep
// screenshots.
func TestDesktopECalcPinoutsBrowser(t *testing.T) {
	f := &pinoutHarness{newECalcBrowserFixture(t, ecalcPinoutFixturePatch(t))}
	page := f.Page
	words := f.Words
	same := func(label string, got, want any) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %v, want %v", label, got, want)
		}
	}
	eval := func(js string, args ...any) string {
		t.Helper()
		return page.MustEval(js, args...).Str()
	}

	// ---- the rail and the list: the Pinouts category with the sections in the order of the contract ----------------------------
	f.OpenApp(`.ec-page[data-calc]`)
	f.Wait(`()=>!!document.querySelector('.ec-rail-btn[data-rail="pinouts"]')`)
	ids := []string{}
	if err := json.Unmarshal([]byte(eval(`()=>JSON.stringify(window.ecalcFixtureIDs)`)), &ids); err != nil || len(ids) < 30 {
		t.Fatalf("fixture ids: %v (%v)", ids, err)
	}
	same("the rail count", eval(`()=>document.querySelector('.ec-rail-btn[data-rail="pinouts"] .ec-count').textContent`), strconv.Itoa(len(ids)))
	page.MustEval(`()=>document.querySelector('.ec-rail-btn[data-rail="pinouts"]').click()`)
	same("the sections of the Pinouts list", eval(`()=>JSON.stringify([...document.querySelectorAll('.ec-group-title')].map(h=>h.textContent))`),
		`["`+strings.Join([]string{words["desktop.ecalc_group_data"], words["desktop.ecalc_group_audio"], words["desktop.ecalc_group_pc_power"], words["desktop.ecalc_group_boards"], words["desktop.ecalc_group_debug"], words["desktop.ecalc_group_ics"]}, `","`)+`"]`)

	// A page for every shape has its parts: the legend lists the types in use, the table the pins, the wire column only with wires.
	f.open("fx_pin_dsub")
	same("columns without wires", eval(`()=>document.querySelectorAll('.ec-pin-table thead th').length`), "5")
	same("legend of the types in use", eval(`()=>document.querySelectorAll('.ec-pin-legend-item').length`), "6")
	f.open("fx_pin_rj45")
	same("columns with wires", eval(`()=>document.querySelectorAll('.ec-pin-table thead th').length`), "6")
	same("a wire stub for every wire", eval(`()=>document.querySelectorAll('svg.ec-pin-art .ec-wire-stub').length`), "8")
	same("the striped wire has a stripe", eval(`()=>document.querySelectorAll('svg.ec-pin-art .ec-wire-stripe').length`), "4")

	// A single column keeps its wires on the left and its names on the right; a symbol is a safe pin number.
	f.open("fx_pin_servo")
	same("servo wires", eval(`()=>JSON.stringify([...document.querySelectorAll('svg.ec-pin-art .ec-wire-stub')].map(w=>{const r=w.querySelector('.ec-wire-base').getBBox();return r.width>r.height;}))`), "[true,true,true]")
	same("servo pins", eval(`()=>JSON.stringify([...document.querySelectorAll('g.ec-pin')].map(g=>g.getAttribute('data-pin')))`), `["S","+","-"]`)
	page.MustElement(`g.ec-pin[data-pin="+"] .ec-pin-body`).MustClick()
	same("a symbol pin is selected", eval(`()=>document.querySelector('.ec-pin-detail-num').textContent+document.querySelectorAll('tr[data-selected="true"]').length`), "+1")
	// Seven variants with long names wrap inside the page.
	f.open("fx_pin_many_variants")
	same("the variant selector wraps", eval(`()=>{const s=document.querySelector('[data-choice="variant"] .ec-seg'),h=document.querySelector('.ec-page-host');return (s.getBoundingClientRect().height>40)+'|'+(h.scrollWidth<=h.clientWidth+1);}`), "true|true")

	// ---- the other view is the mirror image: positions mirrored, text readable, the key at the same height -------------------
	for _, id := range []string{"fx_pin_dsub", "fx_pin_usb_c", "fx_pin_din5", "fx_pin_atx24", "fx_pin_sata_data", "fx_pin_rect"} {
		f.open(id)
		centers := `()=>{const s=document.querySelector('svg.ec-pin-art'),w=s.viewBox.baseVal.width;return JSON.stringify({w,view:s.dataset.view,flip:s.querySelector('.ec-pin-shell').getAttribute('transform')||'',c:Object.fromEntries([...s.querySelectorAll('g.ec-pin')].map(g=>{const b=g.querySelector('.ec-pin-body').getBBox(),n=g.querySelector('.ec-pin-num').getBBox();return [g.getAttribute('data-pin'),[b.x+b.width/2,b.y+b.height/2,n.x+n.width/2]];}))});}`
		var plug, socket struct {
			W    float64
			View string
			Flip string
			C    map[string][3]float64
		}
		f.Page.MustEval(`()=>{const r=document.querySelector('.ec-pinout-page [data-choice="view"] input[value="plug"]');r.checked=true;r.dispatchEvent(new Event('change',{bubbles:true}));}`)
		if err := json.Unmarshal([]byte(eval(centers)), &plug); err != nil {
			t.Fatal(err)
		}
		f.Page.MustEval(`()=>{const r=document.querySelector('.ec-pinout-page [data-choice="view"] input[value="socket"]');r.checked=true;r.dispatchEvent(new Event('change',{bubbles:true}));}`)
		if err := json.Unmarshal([]byte(eval(centers)), &socket); err != nil {
			t.Fatal(err)
		}
		if plug.View != "plug" || socket.View != "socket" || len(plug.C) == 0 || len(plug.C) != len(socket.C) {
			t.Fatalf("%s: views %s/%s with %d/%d contacts", id, plug.View, socket.View, len(plug.C), len(socket.C))
		}
		drawn := plug
		other := socket
		if id == "fx_pin_xlr" {
			drawn, other = socket, plug
		}
		if drawn.Flip != "" || !strings.Contains(other.Flip, "scale(-1 1)") {
			t.Errorf("%s: the shell transform is %q in the drawn view and %q in the other", id, drawn.Flip, other.Flip)
		}
		for pin, a := range drawn.C {
			b := other.C[pin]
			if pinoutAbs(a[0]+b[0]-drawn.W) > 0.6 || pinoutAbs(a[1]-b[1]) > 0.01 || pinoutAbs(a[2]+b[2]-drawn.W) > 0.6 {
				t.Errorf("%s: pin %s at x %.1f / %.1f (number %.1f / %.1f) is not the mirror image about the axis of %.0f", id, pin, a[0], b[0], a[2], b[2], drawn.W)
			}
		}
	}
	// A shape with one view has no toggle.
	for _, id := range []string{"fx_pin_dip8", "fx_pin_to92", "fx_pin_trs", "fx_pin_board38"} {
		f.open(id)
		same(id+" has no view toggle", eval(`()=>document.querySelectorAll('.ec-pinout-page [data-choice="view"]').length`), "0")
	}

	// ---- hover and click link the contact and its row; the selected row scrolls inside the table, not the page -----------------
	f.open("fx_pin_dip28")
	linked := func(attr, pin string) bool {
		return page.MustEval(`(a,n)=>{const g=document.querySelector('g.ec-pin[data-pin="'+n+'"]'),r=document.querySelector('tr[data-pin="'+n+'"]');return g.dataset[a]==='true'&&r.dataset[a]==='true'&&document.querySelectorAll('g.ec-pin[data-'+a+'="true"]').length===1&&document.querySelectorAll('tr[data-'+a+'="true"]').length===1;}`, attr, pin).Bool()
	}
	page.MustElement(`g.ec-pin[data-pin="3"] .ec-pin-body`).MustHover()
	if !linked("hover", "3") {
		t.Fatal("hovering the contact of pin 3 must highlight its row and nothing else")
	}
	page.MustElement(`tr[data-pin="5"] .ec-pin-cell-name`).MustHover()
	if !linked("hover", "5") {
		t.Fatal("hovering the row of pin 5 must highlight its contact and nothing else")
	}
	page.MustElement(`g.ec-pin[data-pin="4"] .ec-pin-body`).MustClick()
	if !linked("selected", "4") {
		t.Fatal("clicking the contact of pin 4 must select its row")
	}
	same("the detail panel names the pin", eval(`()=>document.querySelector('.ec-pin-detail-num').textContent+'|'+document.querySelector('.ec-pin-detail-name').textContent`), "4|"+eval(`()=>document.querySelector('tr[data-pin="4"] .ec-pin-cell-name').textContent`))
	same("the detail panel is a polite live region", eval(`()=>document.querySelector('.ec-pin-detail').getAttribute('aria-live')`), "polite")
	// The keyboard path: the button in the first cell of a row.
	page.MustElement(`tr[data-pin="7"] .ec-pin-rowbtn`).MustFocus()
	page.Keyboard.MustType(input.Enter)
	if !linked("selected", "7") || eval(`()=>document.querySelector('tr[data-pin="7"] .ec-pin-rowbtn').getAttribute('aria-pressed')`) != "true" {
		t.Fatal("pressing Enter on the row button must select the pin")
	}
	page.MustElement(`tr[data-pin="7"] .ec-pin-rowbtn`).MustClick()
	same("a second click deselects", eval(`()=>document.querySelectorAll('[data-selected="true"]').length`), "0")
	// Selecting a pin far down the table from the drawing scrolls the table area and leaves the page where it is.
	page.MustEval(`()=>{document.querySelector('.ec-pin-scroll').scrollTop=0;}`)
	scrolled := eval(`()=>{const host=document.querySelector('.ec-page-host'),area=document.querySelector('.ec-pin-scroll'),before=host.scrollTop,y=window.scrollY;
		document.querySelector('g.ec-pin[data-pin="26"]').dispatchEvent(new MouseEvent('click',{bubbles:true}));
		const row=document.querySelector('tr[data-pin="26"]').getBoundingClientRect(),box=area.getBoundingClientRect(),head=document.querySelector('.ec-pin-table thead').getBoundingClientRect().height;
		return JSON.stringify({hostMoved:host.scrollTop!==before||window.scrollY!==y,areaTop:area.scrollTop,visible:row.top>=box.top+head-1&&row.bottom<=box.bottom+1,scrollable:area.scrollHeight>area.clientHeight});}`)
	var sc struct {
		HostMoved, Visible, Scrollable bool
		AreaTop                        float64
	}
	if err := json.Unmarshal([]byte(scrolled), &sc); err != nil {
		t.Fatal(err)
	}
	if !sc.Scrollable || sc.HostMoved || !sc.Visible || sc.AreaTop <= 0 {
		t.Fatalf("selecting pin 26: %s (the row must be visible inside the scrolled table area and the page must not move)", scrolled)
	}

	// ---- the filter: dims the other contacts, hides the other rows, "/" focuses it; the copy is the visible rows ---------------
	f.open("fx_pin_rj45")
	wireName := eval(`()=>{return document.querySelector('tr[data-pin="3"] .ec-pin-wire span').textContent;}`)
	page.MustEval(`()=>{window.copied='';window.ECalcCalcPage.copyText=text=>{window.copied=text;};}`)
	page.MustElement(`body`).MustClick()
	page.Keyboard.MustType(input.Slash)
	f.Wait(`()=>document.activeElement&&document.activeElement.classList.contains('ec-filter-input')`)
	typeFilter := func(text string) {
		t.Helper()
		el := page.MustElement(`.ec-pinout-page .ec-filter-input`)
		el.MustSelectAllText()
		if text == "" {
			el.MustType(input.Backspace)
		} else {
			el.MustInput(text)
		}
	}
	typeFilter("gnd")
	state := eval(`()=>JSON.stringify({rows:[...document.querySelectorAll('tbody tr[data-pin]')].map(r=>r.dataset.pin),dim:[...document.querySelectorAll('g.ec-pin[data-dim="true"]')].length,count:document.querySelector('.ec-table-count').textContent})`)
	same("filter by name", state, `{"rows":["2"],"dim":7,"count":"`+strings.NewReplacer("{shown}", "1", "{total}", "8").Replace(words["desktop.ecalc_pin_rows_filtered"])+`"}`)
	page.MustElement(`.ec-pinout-page [data-action="copy"]`).MustClick()
	copied := strings.Split(eval(`()=>window.copied`), "\n")
	if len(copied) != 2 || len(strings.Split(copied[0], "\t")) != 6 || strings.Split(copied[1], "\t")[0] != "2" || strings.Split(copied[0], "\t")[0] != words["desktop.ecalc_pin_number"] {
		t.Fatalf("the copy of the filtered pinout is %q", copied)
	}
	groundName := eval(`()=>document.querySelector('.ec-pin-legend-item[data-type="ground"] span').textContent`)
	typeFilter(groundName)
	same("filter by type name", eval(`()=>JSON.stringify([...document.querySelectorAll('tbody tr[data-pin]')].map(r=>r.dataset.type))`), `["ground","ground"]`)
	typeFilter(strings.ToLower(wireName))
	if got := page.MustEval(`()=>[...document.querySelectorAll('tbody tr[data-pin]')].every(r=>r.querySelector('.ec-pin-wire span').textContent.toLowerCase().includes(document.querySelector('.ec-filter-input').value.toLowerCase()))&&document.querySelectorAll('tbody tr[data-pin]').length>0`).Bool(); !got {
		t.Fatal("filter by wire name must keep only rows with that wire")
	}
	typeFilter("zzzz-no-pin")
	same("no pin matches", eval(`()=>document.querySelector('.ec-table-empty td').textContent+'|'+document.querySelectorAll('g.ec-pin[data-dim="true"]').length`), words["desktop.ecalc_pin_no_match"]+"|8")
	typeFilter("")
	same("the filter is cleared", eval(`()=>document.querySelectorAll('tbody tr[data-pin]').length+'|'+document.querySelectorAll('g.ec-pin[data-dim="true"]').length`), "8|0")
	page.MustElement(`.ec-pinout-page [data-action="copy"]`).MustClick()
	if lines := strings.Split(eval(`()=>window.copied`), "\n"); len(lines) != 9 {
		t.Fatalf("the copy of the whole pinout has %d lines, want the header and 8 pins", len(lines))
	}

	// ---- variants: the selector switches names and wires and is remembered per item ----------------------------------------
	f.open("fx_pin_variants")
	same("variant A names", eval(`()=>document.querySelector('tr[data-pin="2"] .ec-pin-cell-name').textContent`), "GND")
	f.Page.MustEval(`()=>{const r=document.querySelector('[data-choice="variant"] input[value="vb"]');r.checked=true;r.dispatchEvent(new Event('change',{bubbles:true}));}`)
	same("variant B names", eval(`()=>document.querySelector('tr[data-pin="2"] .ec-pin-cell-name').textContent+'|'+document.querySelector('g.ec-pin[data-pin="2"] .ec-pin-name')?.textContent`), "VGND|undefined")
	f.open("fx_pin_dsub")
	f.open("fx_pin_variants")
	same("the variant is remembered", eval(`()=>document.querySelector('[data-choice="variant"] input:checked').value+'|'+document.querySelector('tr[data-pin="2"] .ec-pin-cell-name').textContent`), "vb|VGND")
	f.open("fx_pin_dsub")
	f.Page.MustEval(`()=>{const r=document.querySelector('[data-choice="view"] input[value="socket"]');r.checked=true;r.dispatchEvent(new Event('change',{bubbles:true}));}`)
	f.open("fx_pin_rj45")
	f.open("fx_pin_dsub")
	same("the view is remembered", eval(`()=>document.querySelector('[data-choice="view"] input:checked').value+'|'+document.querySelector('svg.ec-pin-art').dataset.view`), "socket|socket")
	same("the view captions and the fixed caption", eval(`()=>[...document.querySelectorAll('[data-choice="view"] span')].map(s=>s.textContent).join('|')`),
		"Stecker (Stifte), Blick auf die Steckseite|Buchse (Kontakte), Blick auf die Steckseite|"+words["desktop.ecalc_pin_face"])

	// ---- long texts wrap: no sideways growth even in a narrow window; the favorite and the search keep working -------------------
	f.open("fx_pin_long")
	if r := f.report(); len(r.Problems) > 0 {
		t.Fatalf("fx_pin_long: %v", r.Problems)
	}
	page.MustElement(`.ec-pinout-page [data-action="favorite"]`).MustClick()
	same("favorite", eval(`()=>document.querySelector('.ec-pinout-page [data-action="favorite"]').getAttribute('aria-pressed')`), "true")
	page.MustElement(`.ec-pinout-page [data-action="favorite"]`).MustClick()
	f.Shot("pinout-long-wide")

	if errors := eval(`()=>JSON.stringify(fixtureErrors)`); errors != "[]" {
		t.Fatal(errors)
	}
}

func pinoutAbs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// startPinouts opens the app on the fixture catalog and returns the harness and the ids of the fixture pinouts.
func startPinouts(t *testing.T) (*pinoutHarness, []string) {
	t.Helper()
	f := &pinoutHarness{newECalcBrowserFixture(t, ecalcPinoutFixturePatch(t))}
	f.OpenApp(`.ec-page[data-calc]`)
	f.Wait(`()=>!!document.querySelector('.ec-rail-btn[data-rail="pinouts"]')`)
	ids := []string{}
	if err := json.Unmarshal([]byte(f.Page.MustEval(`()=>JSON.stringify(window.ecalcFixtureIDs)`).Str()), &ids); err != nil || len(ids) < 30 {
		t.Fatalf("fixture ids: %v (%v)", ids, err)
	}
	return f, ids
}

// TestDesktopECalcPinoutDrawingsBrowser measures the drawing of every fixture pinout in every variant and view in the real
// browser (see pinoutReportJS) at the size of the desktop window.
func TestDesktopECalcPinoutDrawingsBrowser(t *testing.T) {
	f, ids := startPinouts(t)
	// ---- every fixture, in every variant and view: the geometry measured by the browser --------------------------------------
	dense := map[string]bool{"fx_pin_usb_c": true, "fx_pin_hdmi_a": true, "fx_pin_header40": true, "fx_pin_board38": true, "fx_pin_dip28": true}
	for _, id := range ids {
		for _, report := range f.check(id, "pinout") {
			t.Logf("%-22s %-14s %4.0fx%-4.0f scale %.2f smallest text %.1f px", id, report.View, report.Width, report.Height, report.Scale, report.MinPx)
			if report.MinPx < 7 && !dense[id] || report.MinPx < 6.5 {
				t.Errorf("pinout %s: the smallest text is %.1f px", id, report.MinPx)
			}
		}
	}
}

// TestDesktopECalcPinoutCompactBrowser opens every fixture pinout in a phone-sized window: the page stacks, nothing moves sideways.
func TestDesktopECalcPinoutCompactBrowser(t *testing.T) {
	f, ids := startPinouts(t)
	page := f.Page
	// ---- a phone-sized window: the page stacks, nothing moves sideways, every fixture still has a sound drawing -----------------
	f.CloseWindows()
	page.MustSetViewport(390, 667, 1, false)
	page.MustEval(`()=>fixtureTheme('fruity-dark')`)
	f.OpenApp(`.ec-app[data-compact="true"]`)
	f.Wait(`()=>!!document.querySelector('.ec-page')`)
	page.MustEval(`()=>document.querySelector('.ec-rail-btn[data-rail="pinouts"]').click()`)
	for _, id := range ids {
		for _, report := range f.check(id, "") {
			if !report.Stacked || report.Sideways {
				t.Errorf("compact %s (%s): stacked %v, sideways %v", id, report.View, report.Stacked, report.Sideways)
			}
			if report.MinPx < 6.5 {
				t.Errorf("compact %s (%s): the smallest text is %.1f px", id, report.View, report.MinPx)
			}
		}
	}
	f.open("fx_pin_usb_c")
	f.Shot("pinout-compact-usb-c")
	f.open("fx_pin_board38")
	f.Shot("pinout-compact-board38")
	f.open("fx_pin_long")
	f.Shot("pinout-compact-long")

	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}

// TestDesktopECalcPinoutRealBrowser opens every pinout of the real catalog (none until the content is assembled) and checks its
// drawing; it must pass with zero items as well.
func TestDesktopECalcPinoutRealBrowser(t *testing.T) {
	f, _ := startPinouts(t)
	page := f.Page
	eval := func(js string) string { return page.MustEval(js).Str() }
	f.CloseWindows()
	// ---- the pinouts of the real catalog (none until the content is assembled): each one opens and draws soundly ---------------
	page.MustEval(`()=>{window.ecalcWithReal=true;fixtureTheme('standard');}`)
	f.OpenApp(`.ec-page`)
	realIDs := []string{}
	if err := json.Unmarshal([]byte(eval(`()=>JSON.stringify(window.ecalcRealPinoutIDs)`)), &realIDs); err != nil {
		t.Fatal(err)
	}
	if len(realIDs) != len(f.Catalog.Pinouts) {
		t.Fatalf("the served catalog holds %d pinouts, the engine %d", len(realIDs), len(f.Catalog.Pinouts))
	}
	for _, p := range f.Catalog.Pinouts {
		if !slices.Contains(realIDs, p.ID) {
			t.Fatalf("the pinout %s is missing from the served catalog", p.ID)
		}
		reports := f.check(p.ID, "real")
		pins, _, _ := p.PinsOf("")
		if len(reports) == 0 || len(reports[0].Pins) != len(pins) {
			t.Fatalf("real pinout %s draws %d pins for %d pins", p.ID, len(reports[0].Pins), len(pins))
		}
	}

	if errors := eval(`()=>JSON.stringify(fixtureErrors)`); errors != "[]" {
		t.Fatal(errors)
	}
}

// pinoutContrastJS measures the drawing of the open pinout page against its stage in the current desktop skin: the fill of every
// contact must be the hex of its pin type (computed style), the outline of the shell, the names and the numbers outside the
// contacts are measured against what lies behind the stage (translucent layers are laid over the next one outward, an image falls
// back to the strong panel colour of the skin, like the code pages do).
const pinoutContrastJS = `()=>{
	const cv=document.createElement('canvas');cv.width=cv.height=1;const cx=cv.getContext('2d',{willReadFrequently:true});
	const rgba=css=>{cx.clearRect(0,0,1,1);cx.fillStyle='#000';cx.fillStyle=css;cx.fillRect(0,0,1,1);const d=cx.getImageData(0,0,1,1).data;return [d[0],d[1],d[2],d[3]/255];};
	const over=(top,under)=>{const a=top[3];return [0,1,2].map(i=>top[i]*a+under[i]*(1-a)).concat([1]);};
	const lum=c=>{const f=v=>{v/=255;return v<=0.03928?v/12.92:Math.pow((v+0.055)/1.055,2.4);};return 0.2126*f(c[0])+0.7152*f(c[1])+0.0722*f(c[2]);};
	const ratio=(a,b)=>{const x=lum(a),y=lum(b);return (Math.max(x,y)+0.05)/(Math.min(x,y)+0.05);};
	const stage=document.querySelector('.ec-pin-stage');
	const strong=over(rgba(getComputedStyle(stage).getPropertyValue('--vd-theme-panel-bg-strong')||'#fff'),[255,255,255,1]);
	const behind=el=>{const layers=[];for(let n=el;n;n=n.parentElement){const cs=getComputedStyle(n);if(cs.backgroundImage&&cs.backgroundImage!=='none')return strong;const c=rgba(cs.backgroundColor);if(c[3]>0)layers.push(c);if(c[3]>=1)break;}
		let base=strong;if(layers.length&&layers[layers.length-1][3]>=1)base=layers.pop();for(let i=layers.length-1;i>=0;i--)base=over(layers[i],base);return base;};
	const bg=behind(stage),out={bg:bg.map(Math.round).join(',')};
	const svg=stage.querySelector('svg.ec-pin-art'),box=svg.getBoundingClientRect();
	out.size=box.width>20&&box.height>20;
	out.fills=[...svg.querySelectorAll('g.ec-pin')].map(g=>g.getAttribute('data-type')+'='+getComputedStyle(g.querySelector('.ec-pin-body')).fill).join('|');
	const stroke=(el)=>{const cs=getComputedStyle(el),c=rgba(cs.stroke);c[3]*=parseFloat(cs.strokeOpacity);return ratio(over(c,bg),bg);};
	out.shell=stroke(svg.querySelector('.ec-shell'));
	out.body=Math.min(...[...svg.querySelectorAll('g.ec-pin')].map(g=>ratio(rgba(getComputedStyle(g.querySelector('.ec-pin-body')).stroke),bg)));
	const names=[...svg.querySelectorAll('.ec-pin-name,.ec-pin-num-out')];
	out.names=names.length?Math.min(...names.map(n=>ratio(over(rgba(getComputedStyle(n).fill),bg),bg))):99;
	out.inks=[...svg.querySelectorAll('g.ec-pin[data-num="in"]')].map(g=>ratio(rgba(getComputedStyle(g.querySelector('.ec-pin-num')).fill),rgba(getComputedStyle(g.querySelector('.ec-pin-body')).fill))).reduce((m,v)=>Math.min(m,v),99);
	return JSON.stringify(out);
}`

// TestDesktopECalcPinoutSkinsBrowser checks the drawing in every desktop skin: it has a size, the contacts have the colour of
// their pin type in every skin (it comes from the data), the numbers inside the contacts are legible on them, and the shell, the
// names and the numbers outside the contacts are legible against the stage.
func TestDesktopECalcPinoutSkinsBrowser(t *testing.T) {
	f, _ := startPinouts(t)
	page := f.Page
	skins := []string{}
	for _, theme := range parseDesktopThemeRegistry(t) {
		if theme.ID == "fruity" {
			skins = append(skins, "fruity-light", "fruity-dark")
		} else {
			skins = append(skins, theme.ID)
		}
	}
	type measure struct {
		Bg    string
		Size  bool
		Fills string
		Shell float64
		Body  float64
		Names float64
		Inks  float64
	}
	const outlineMin, textMin, inkMin = 3.0, 4.5, 4.5
	for _, id := range []string{"fx_pin_usb_c", "fx_pin_atx24", "fx_pin_board38"} {
		f.open(id)
		fills := ""
		for _, skin := range skins {
			page.MustEval(`id=>fixtureTheme(id)`, skin)
			var m measure
			if err := json.Unmarshal([]byte(page.MustEval(pinoutContrastJS).Str()), &m); err != nil {
				t.Fatal(err)
			}
			t.Logf("skin %-14s %-14s shell %.2f contact outline %.2f names %.2f ink %.2f (stage rgb %s)", skin, id, m.Shell, m.Body, m.Names, m.Inks, m.Bg)
			if !m.Size {
				t.Errorf("skin %s, %s: the drawing has no size", skin, id)
			}
			if m.Shell < outlineMin || m.Body < outlineMin || m.Names < textMin || m.Inks < inkMin {
				t.Errorf("skin %s, %s: the drawing is hard to read (shell %.2f, contact outline %.2f, names %.2f, ink on contact %.2f, stage rgb %s)", skin, id, m.Shell, m.Body, m.Names, m.Inks, m.Bg)
			}
			if fills == "" {
				fills = m.Fills
			} else if m.Fills != fills {
				t.Errorf("skin %s, %s draws the contacts in other colours than the first skin: %s vs %s", skin, id, m.Fills, fills)
			}
			if id == "fx_pin_usb_c" && (skin == "fruity-light" || skin == "cyberpunk" || skin == "minimal") {
				f.Shot("pinout-skin-" + skin)
			}
		}
		page.MustEval(`()=>fixtureTheme('standard')`)
	}
	// The contact colour is the hex of the data: the computed fill equals the hex of the catalog pin type.
	f.open("fx_pin_dsub")
	drawn := page.MustEval(`()=>{return JSON.stringify([...document.querySelectorAll('svg.ec-pin-art g.ec-pin')].map(g=>g.querySelector('.ec-pin-body').getAttribute('fill')));}`).Str()
	if !strings.Contains(drawn, "#") {
		t.Fatalf("the contacts carry their hex as an attribute: %s", drawn)
	}
	for _, pt := range f.Catalog.PinTypeStyles {
		if !strings.Contains(drawn, pt.Hex) && pt.ID != "nc" {
			t.Errorf("the pin type %s (%s) is not drawn by the Sub-D fixture: %s", pt.ID, pt.Hex, drawn)
		}
	}
}
