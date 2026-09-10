package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"aurago/internal/office"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopSheetsAppBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome required")
	}
	var mu sync.Mutex
	files := map[string][]byte{}
	versions := map[string]int{}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/api/desktop/office/workbook", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		path := r.URL.Query().Get("path")
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "PATCH" {
			expected := fmt.Sprintf("%q", fmt.Sprint(versions[path]))
			if (versions[path] > 0 && r.Header.Get("If-Match") != expected) || (versions[path] == 0 && r.Header.Get("If-None-Match") != "*") {
				w.WriteHeader(412)
				fmt.Fprint(w, `{"error":"Conflict"}`)
				return
			}
			var patch struct {
				office.WorkbookEditorPatch
				SourceData []byte `json:"source_data"`
				SourcePath string `json:"source_path"`
			}
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			original := files[path]
			if versions[path] == 0 {
				if len(patch.SourceData) > 0 {
					original = patch.SourceData
				} else if patch.SourcePath != "" {
					original = files[patch.SourcePath]
				}
			}
			saved, err := office.ApplyWorkbookEditorPatch(original, patch.WorkbookEditorPatch)
			if err != nil {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			files[path] = saved
			versions[path]++
		}
		if versions[path] == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", fmt.Sprintf("%q", fmt.Sprint(versions[path])))
		if r.Method == "GET" {
			doc, err := office.DecodeEditorWorkbook(files[path])
			if err != nil {
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(struct {
				office.WorkbookEditorDocument
				SourceData []byte `json:"source_data"`
			}{doc, files[path]})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"source_data": files[path]})
		}
	})
	mux.HandleFunc("/api/desktop/office/workbook/assist", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			SourceRevision uint64             `json:"source_revision"`
			Range          office.EditorRange `json:"range"`
		}
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			http.Error(w, "invalid", 400)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"explanation": "Use SUM for the selected cells.", "source_revision": body.SourceRevision, "changes": []interface{}{map[string]interface{}{"row": body.Range.StartRow, "column": body.Range.StartColumn, "formula": "=SUM(2,3)"}}})
	})
	mux.HandleFunc("/sheets-app-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><link rel="stylesheet" href="/js/vendor/sheets/engine.css"><link rel="stylesheet" href="/css/desktop-app-sheets.css">
<style>html,body{margin:0;height:100%;font-family:system-ui}body{--vd-text:#e4e8f0;--vd-theme-app-bg:#141922;--vd-theme-panel-bg:#1b202a;--vd-theme-chrome-bg:#1d2430;--vd-theme-control-bg:#252d3a;--vd-theme-border:#ffffff1a;--vd-theme-muted:#a7b1c3;--vd-accent:#9abfff}body[data-fruity-mode=light]{--vd-text:#212b3c;--vd-theme-app-bg:#e2e7ee;--vd-theme-panel-bg:#e5e9ef;--vd-theme-chrome-bg:#d7dfe9;--vd-theme-control-bg:#f8fafc;--vd-theme-border:#65748c40;--vd-theme-muted:#536279;--vd-accent:#2169bd}#host{height:100%}</style></head><body class="desktop-body" data-theme="default"><div id="host"></div>
<script src="/chart.min.js"></script><script src="/js/desktop/apps/writer-session.js"></script><script src="/js/desktop/apps/sheets-data.js"></script><script src="/js/desktop/apps/sheets-panels.js"></script><script src="/js/desktop/apps/sheets-charts.js"></script><script src="/js/desktop/apps/sheets.js"></script>
<script>window.ready=(async()=>{const labels=await(await fetch('/lang/desktop/en.json')).json();window.sheetsContext={t:(key,params)=>String(labels[key]||key).replace(/\{\{(\w+)\}\}/g,(_,x)=>params?.[x]??''),promptDialog:async(title,value)=>value,confirmDialog:async()=>true,setWindowBeforeClose:(id,fn)=>window.closeGuard=fn,setWindowMenus:(id,menus)=>window.menus=menus,saveFileDialog:async()=>({path:'Documents/copy.xlsx'}),openFileDialog:async()=>({path:SheetsApp.instances.get('test').path})};SheetsApp.render(document.getElementById('host'),'test',sheetsContext);})();</script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/sheets-app-fixture").Timeout(60 * time.Second)
	defer page.Close()
	page.MustSetViewport(1366, 768, 1, false)
	page.MustWaitLoad()
	page.MustWait(`()=>!!SheetsApp.instances.get('test')?.book||document.querySelector('[data-notice]')?.dataset.error==='true'`)
	result := page.MustEval(`async()=>{
  const app=SheetsApp.instances.get('test');if(!app.book)throw Error(document.querySelector('[data-notice-text]').textContent);
  const sheet=app.book.getActiveSheet();sheet.getRange('A1:B3').setValues([['Name','Amount'],['Rent',1250.5],['Food',400]]);
  sheet.getRange('B4').setValue({f:'=SUM(B2:B3)'});
  await app.session.save();if(app.session.error)throw app.session.error;
  await app.state.load('Documents/Budget.xlsx','budget');
  if(!app.book)throw Error(document.querySelector('[data-notice-text]').textContent);
  await app.session.save();if(app.session.error)throw app.session.error;
  await app.act('format');
  return {path:app.path,chartCount:document.querySelectorAll('.sheets-chart').length,notice:document.querySelector('[data-notice-text]').textContent,saved:!app.session.dirty,panel:{hidden:document.querySelector('[data-right]').hidden,rect:document.querySelector('[data-right]').getBoundingClientRect().toJSON(),text:document.querySelector('[data-right]').textContent.slice(0,120)}};
 }`)
	t.Logf("Sheets app: %s", result.JSON("", ""))
	page.MustWait(`()=>SheetsApp.instances.get('test').book.getActiveSheet().getRange('B11').getRawValue()===2340`)
	t.Logf("Formula values %s", page.MustEval(`()=>SheetsApp.instances.get('test').book.getActiveSheet().getRange('D5:D9').getRawValues()`).JSON("", ""))
	resources := page.MustEval(`async()=>{
        const app=SheetsApp.instances.get('test'),s=app.book.getActiveSheet(),api=app.api;
        s.getRange('A14:A16').setDataValidation(api.newDataValidation().requireValueInList(['Yes','No']).setAllowInvalid(false).build());
        s.getRange('B14:B16').setValues([[1],[10],[30]]);
        s.addConditionalFormattingRule(s.getRange('B14:B16').createConditionalFormattingRule().whenNumberGreaterThan(5).setBackground('#dcefe6').build());
        s.getRange('A14').createOrUpdateNote({note:'Check this',width:220,height:140,show:false});
        app.book.insertDefinedName('SampleRange',"'"+s.getSheetName()+"'!$B$14:$B$16");
        s.getRange('A3:D11').createFilter();
        return app.book.save();
    }`)
	os.MkdirAll("../reports/sheets", 0755)
	os.WriteFile("../reports/sheets/native-snapshot.json", []byte(resources.JSON("", "  ")), 0644)
	page.MustEval(`async()=>{const app=SheetsApp.instances.get('test');await app.session.save();if(app.session.error)throw app.session.error;await app.act('closeRight');await app.act('format');await new Promise(resolve=>setTimeout(resolve,250));}`)
	if result.Get("chartCount").Int() != 1 {
		t.Fatal("Chart missing")
	}
	_ = os.MkdirAll("../reports/sheets", 0755)
	for _, variant := range []struct {
		Name  string
		Theme string
		Mode  string
	}{{"standard", "default", ""}, {"fruity-dark", "fruity", "dark"}, {"fruity-light", "fruity", "light"}} {
		page.MustEval(`(theme,mode)=>{document.body.dataset.theme=theme;document.body.dataset.fruityMode=mode;}`, variant.Theme, variant.Mode)
		shot, err := page.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile("../reports/sheets/"+variant.Name+"-1366.png", shot, 0644); err != nil {
			t.Fatal(err)
		}
	}
	testSheetsAppInteractions(t, page)
	testSheetsClipboardAIPrint(t, page)
	testSheetsSessions(t, page)
	testSheetsNativeTyping(t, page)
	testSheetsLargeWorkbook(t, page)
	page.MustEval(`()=>SheetsApp.dispose('test')`)
}

func testSheetsAppInteractions(t *testing.T, page *rod.Page) {
	t.Helper()
	value := page.MustEval(`async()=>{
  const app=SheetsApp.instances.get('test'),s=app.book.getActiveSheet(),api=app.api,root=document.querySelector('.sheets-app');
  const check=(yes,message)=>{if(!yes)throw Error(message)},wait=()=>new Promise(r=>setTimeout(r,80));
  app.session.suspend();
  const before=app.state.aux.charts[0].range;s.insertRows(4,1);await wait();check(app.state.aux.charts[0].range==='A4:C10','Chart range did not follow row insertion');
  await api.undo();await wait();check(app.state.aux.charts[0].range===before,'Chart range undo');
  const target=s.getRange('B5');target.activate();await app.act('closeRight');await app.act('format');
  root.querySelector('[data-field="border"]').value='all';root.querySelector('[data-panel-action="setBorder"]').click();await wait();check(target.getCellStyleData()?.bd?.t,'Border control');
  root.querySelector('[data-field="pattern"]').value='0.000';root.querySelector('[data-panel-action="setNumberFormat"]').click();await wait();check(target.getNumberFormat()==='0.000','Number format control');
  s.getRange('K2:L4').setValues([['C',3],['A',1],['B',2]]);s.getRange('K2:L4').activate();await app.act('data');
  root.querySelector('[data-panel-action="sort"]').click();await wait();check(s.getRange('K2').getRawValue()==='A','Sort control: '+JSON.stringify({values:s.getRange('K2:L4').getRawValues(),selection:app.state.selection(),notice:root.querySelector('[data-notice-text]').textContent}));
  await app.act('closeRight');await app.act('navigation');check(!root.querySelector('[data-left]').hidden,'Navigation panel');
  root.querySelector('[data-name-range]').click();await wait();check(s.getActiveRange().getA1Notation().includes('B14'),'Named range navigation');
  await app.act('closeLeft');s.getRange('L2').activate();const scroll=s.getScrollState();await app.act('bold');await wait();check(s.getRange('L2').getCellStyleData().bl===1,'Bold selection');check(JSON.stringify(s.getScrollState())===JSON.stringify(scroll),'Toolbar moved scroll');
  await app.act('search');root.querySelector('[data-field="query"]').value='Housing';root.querySelector('[data-panel-action="searchRun"]').click();await wait();check(root.querySelector('[data-search-results]').textContent.includes('Housing'),'Search results');
  root.querySelector('[data-field="replacement"]').value='Home';root.querySelector('[data-panel-action="replaceOne"]').click();await wait();check(s.getRange('A5').getRawValue()==='Home','Replace');await api.undo();await wait();check(s.getRange('A5').getRawValue()==='Housing','Replace undo');
  s.getRange('P2').setValue({f:'=$B5+C$5+$C$5'});await s.getRange('P2').autoFill(s.getRange('P2:P4'),'COPY');
  const prepared=await app.state.prepareOutput();check(prepared.workbook.sheetOrder.length===1,'Snapshot');check(prepared.workbook.sheets[s.getSheetId()].cellData[3][15].f==='=$B7+C$5+$C$5','Shared formula expansion');
  s.getRange('P6').setValue({f:'=AVG(B5:B9)'});await app.state.prepareOutput();check(s.getRange('P6').getRawValue()===468,'AVG alias');
  await api.executeCommand('sheet.operation.set-selections',{unitId:app.book.getId(),subUnitId:s.getSheetId(),selections:[s.getRange('M20').getRange(),s.getRange('O20').getRange()].map(range=>({range,primary:{actualRow:range.startRow,actualColumn:range.startColumn,startRow:range.startRow,endRow:range.endRow,startColumn:range.startColumn,endColumn:range.endColumn,isMerged:false},style:null}))});await app.act('bold');check(s.getRange('M20').getCellStyleData().bl===1&&s.getRange('O20').getCellStyleData().bl===1,'Disjoint formatting');await api.undo();check(s.getRange('M20').getCellStyleData()?.bl!==1&&s.getRange('O20').getCellStyleData()?.bl!==1,'Disjoint one-step undo');
  const linked=app.book.insertSheet('References');linked.getRange('A1').setValue({f:"='"+s.getSheetName()+"'!B5"});app.book.setActiveSheet(s);const oldName=s.getSheetName();s.setName('Renamed');await app.state.prepareOutput();check(linked.getRange('A1').getFormula().includes('Renamed'),'Cross-sheet rename');s.setName(oldName);app.book.deleteSheet(linked);
  s.getRange('R20:V20').setValues([[{f:'=S20+1'},{f:'=R20+1'},{f:'=R20+1'},{f:'=U20+1'},{f:'=SUM(1,2)'}]]);await app.state.prepareOutput();check(s.getRange('R20:U20').getRawValues()[0].every(x=>x==='#REF!'),'Circular reference errors: '+JSON.stringify(s.getRange('R20:V20').getRawValues()));check(s.getRange('V20').getRawValue()===3,'Cycle affected unrelated formula');s.getRange('S20').setValue(1);await app.state.prepareOutput();check(s.getRange('R20').getRawValue()===2&&s.getRange('T20').getRawValue()===3,'Breaking cycle must restore calculation');s.getRange('R20:V20').clear();
  app.session.resume();await app.session.save();check(!app.session.dirty,'Save after interactions');
  const revision=app.session.revision;await app.state.prepareOutput();await wait();check(revision===app.session.revision,'Recalculation marked document dirty');
  const error=root.querySelector('[data-notice]');check(error.hidden||error.dataset.error!=='true',root.querySelector('[data-notice-text]').textContent);
  return {chartReferences:true,formatting:true,sort:true,searchUndo:true,navigation:true,stableFocus:true,stableSave:true};
 }`)
	t.Logf("Sheets workflows: %s", value.JSON("", ""))
}

func testSheetsNativeTyping(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{await SheetsApp.instances.get('test').state.save();SheetsApp.dispose('test');document.documentElement.lang='de';SheetsApp.render(document.getElementById('host'),'test',sheetsContext);}`)
	page.MustWait(`()=>!!SheetsApp.instances.get('test')?.book`)
	for _, entry := range []struct {
		Cell, Text string
		Value      interface{}
	}{
		{"A1", "1.234,50", 1234.5}, {"A2", "00123", "00123"}, {"A3", "10.09.2026", 46275},
	} {
		rect := page.MustEval(`async cell=>{const a=SheetsApp.instances.get('test');a.session.suspend();const r=a.book.getActiveSheet().getRange(cell);r.activate();await new Promise(resolve=>setTimeout(resolve,150));const c=r.getCellRect(),m=document.querySelector('[data-engine]').getBoundingClientRect();return {x:m.left+c.left+20,y:m.top+c.top+10};}`, entry.Cell)
		page.Mouse.MustMoveTo(rect.Get("x").Num(), rect.Get("y").Num())
		if err := page.Mouse.Click(proto.InputMouseButtonLeft, 2); err != nil {
			t.Fatal(err)
		}
		if err := (proto.InputInsertText{Text: entry.Text}).Call(page); err != nil {
			t.Fatal(err)
		}
		got := page.MustEval(`async cell=>{const a=SheetsApp.instances.get('test');await a.book.endEditingAsync(true);return a.book.getActiveSheet().getRange(cell).getRawValue();}`, entry.Cell)
		if fmt.Sprint(got.Val()) != fmt.Sprint(entry.Value) {
			t.Fatalf("German native input %s: got %v want %v", entry.Text, got.Val(), entry.Value)
		}
	}
	page.MustEval(`async()=>{const a=SheetsApp.instances.get('test');a.session.resume();await a.state.save();}`)
	t.Log("German native input: decimal, date and leading zeros passed")
}
func testSheetsLargeWorkbook(t *testing.T, page *rod.Page) {
	t.Helper()
	result := page.MustEval(`async()=>{
  const a=SheetsApp.instances.get('test'),template=SheetsData.template,doc=template('blank',key=>key),s=doc.workbook.sheets[doc.workbook.sheetOrder[0]];
  s.rowCount=10050;s.columnCount=26;s.cellData={};
  for(let r=0;r<10000;r++){const cells={};for(let c=0;c<19;c++)cells[c]={v:r+c,t:2};cells[19]={f:'=SUM(A'+(r+1)+':S'+(r+1)+')',t:2};s.cellData[r]=cells;}
  SheetsData.template=()=>doc;const start=performance.now();await a.state.load('Documents/Performance.xlsx','blank');SheetsData.template=template;a.session.suspend();const loaded=performance.now()-start;
  if(!a.book)throw Error(document.querySelector('[data-notice-text]').textContent);
  const calculate=performance.now();await a.state.prepareOutput();const recalculation=performance.now()-calculate;
  const sheet=a.book.getActiveSheet();if(sheet.getRange('T10000').getRawValue()!==190152)throw Error('Large workbook formula result '+sheet.getRange('T10000').getRawValue());
  const input=performance.now();sheet.getRange('A10000').setValue(10);const inputMs=performance.now()-input;
  const scroll=performance.now();sheet.scrollToCell(9990,0);await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));const scrollMs=performance.now()-scroll;
  if(loaded>20000||inputMs>1500||recalculation>10000||scrollMs>1500)throw Error(JSON.stringify({loaded,inputMs,recalculation,scrollMs}));
  return {rows:10000,columns:20,loadedMs:Math.round(loaded),recalculationMs:Math.round(recalculation),inputMs:Math.round(inputMs),scrollMs:Math.round(scrollMs)};
 }`)
	t.Logf("Sheets performance: %s", result.JSON("", ""))
}

func testSheetsClipboardAIPrint(t *testing.T, page *rod.Page) {
	t.Helper()
	if err := (proto.BrowserGrantPermissions{Origin: page.MustInfo().URL, Permissions: []proto.BrowserPermissionType{proto.BrowserPermissionTypeClipboardReadWrite, proto.BrowserPermissionTypeClipboardSanitizedWrite}}).Call(page.Browser()); err != nil {
		t.Fatal(err)
	}
	result := page.MustEval(`async()=>{
  const a=SheetsApp.instances.get('test'),s=a.book.getActiveSheet(),root=document.querySelector('.sheets-app'),wait=()=>new Promise(r=>setTimeout(r,100)),check=(yes,message)=>{if(!yes)throw Error(message)};
  a.session.suspend();await a.act('closeLeft');await a.act('closeRight');
  s.getRange('A20:B21').setValues([[10,{f:'=A20*2'}],[30,40]]);await a.state.prepareOutput();s.getRange('A20:B21').activate();await a.act('copy');s.getRange('D20').activate();await a.act('paste');await a.state.prepareOutput();check(s.getRange('E20').getFormula()==='=D20*2','Clipboard relative formula: '+JSON.stringify(s.getRange('D20:E21').getCellDatas()));check(s.getRange('E20').getRawValue()===20,'Clipboard calculation');
  s.getRange('G20').activate();await a.act('pasteValues');check(s.getRange('H20').getRawValue()===20&&!s.getRange('H20').getFormula(),'Paste values');
  s.getRange('K20').activate();await a.act('pasteTranspose');await a.state.prepareOutput();check(s.getRange('K21').getFormula()==='=J21*2','Transpose relative formula: '+s.getRange('K21').getFormula());check(s.getRange('L20').getRawValue()===30,'Transpose value');
  s.getRange('A25').setValue('move me');s.getRange('A25').activate();await a.act('cut');s.getRange('D25').activate();await a.act('paste');check(s.getRange('D25').getRawValue()==='move me'&&!s.getRange('A25').getRawValue(),'Cut paste');await a.api.undo();check(s.getRange('A25').getRawValue()==='move me','Cut undo');
  s.getRange('B5').activate();const before=s.getRange('B5').getRawValue();await a.act('assist');root.querySelector('[data-panel-action="askAI"]').click();for(let i=0;i<30&&!root.querySelector('[data-panel-action="applyAI"]');i++)await wait();
  const apply=root.querySelector('[data-panel-action="applyAI"]');check(apply&&!apply.disabled,'AI preview');apply.click();await a.state.prepareOutput();check(s.getRange('B5').getRawValue()===5,'AI apply');await a.api.undo();check(s.getRange('B5').getRawValue()===before,'AI one-step undo');
  root.querySelector('[data-panel-action="askAI"]').click();await wait();s.getRange('A30').setValue('changed');await wait();check(root.querySelector('[data-panel-action="applyAI"]').disabled,'AI stale proposal');
  await a.act('print');root.querySelector('[data-field="printArea"]').value='A1:J12';root.querySelector('[data-field="repeatRows"]').value='1';
  window.printProbe=null;const observer=new MutationObserver(()=>{const frame=document.querySelector('.sheets-print-frame');if(frame){frame.contentWindow.print=()=>{window.printProbe={text:frame.contentDocument.body.textContent,rows:frame.contentDocument.querySelectorAll('tr').length,headers:frame.contentDocument.querySelectorAll('thead tr').length,charts:frame.contentDocument.querySelectorAll('figure img').length};};}});observer.observe(root,{subtree:true,childList:true});
  root.querySelector('[data-panel-action="printNow"]').click();for(let i=0;i<30&&!window.printProbe;i++)await wait();observer.disconnect();check(window.printProbe?.rows===12&&window.printProbe.headers===1,'Print rows and headers: '+JSON.stringify(window.printProbe));check(window.printProbe.charts===1,'Print chart');check(window.printProbe.text.includes('Monthly budget'),'Print current document');document.querySelector('.sheets-print-frame')?.remove();
  await a.act('closeRight');a.session.resume();await a.state.save();return {clipboard:true,transpose:true,cutUndo:true,aiUndo:true,aiStale:true,print:window.printProbe};
 }`)
	t.Logf("Clipboard, AI and print: %s", result.JSON("", ""))
}

func testSheetsSessions(t *testing.T, page *rod.Page) {
	t.Helper()
	result := page.MustEval(`async()=>{
 const a=SheetsApp.instances.get('test'),wait=()=>new Promise(r=>setTimeout(r,50)),check=(yes,message)=>{if(!yes)throw Error(message)};
 await a.state.save();a.session.suspend();
 const extra=document.createElement('div');extra.style.cssText='width:900px;height:600px;position:fixed;top:0;left:0;z-index:-1';document.body.append(extra);
 SheetsApp.render(extra,'second',{...sheetsContext,path:a.path});
 for(let i=0;i<100&&!SheetsApp.instances.get('second')?.book;i++)await wait();
 const b=SheetsApp.instances.get('second');check(b?.book,'Second workbook');b.session.suspend();
 a.book.getActiveSheet().getRange('A35').setValue('first window');await a.state.save();
 b.book.getActiveSheet().getRange('A35').setValue('second window');
 let conflicted=false;try{await b.state.save()}catch(e){conflicted=e.status===412}check(conflicted&&b.session.dirty,'Conflict must not mark clean');
 const draftKey=location.origin+':'+a.path;check(await OfficeSession.draft('get',draftKey,undefined,'sheets'),'Conflict draft retained');
 await b.act('saveAs');check(b.path==='Documents/copy.xlsx','Conflict copy');
 const copy=await(await fetch('/api/desktop/office/workbook?path=Documents/copy.xlsx')).json();check(copy.workbook.sheets[copy.workbook.sheetOrder[0]].cellData[34][0].v==='second window','Copied current state');check(copy.charts.length===1,'Copy retained chart');
 SheetsApp.dispose('second');extra.remove();
 const host=document.createElement('div');host.style.cssText='width:800px;height:600px;position:fixed;top:0;left:0;z-index:-1';document.body.append(host);
 SheetsApp.render(host,'recovery',{...sheetsContext,path:a.path});
 for(let i=0;i<100&&!SheetsApp.instances.get('recovery')?.book;i++)await wait();
 const recovered=SheetsApp.instances.get('recovery');check(recovered?.book,'Recovery workbook');recovered.session.suspend();
 check(recovered.book.getActiveSheet().getRange('A35').getRawValue()==='second window','Explicit draft restore');
 let conflict=false;try{await recovered.state.save()}catch(e){conflict=e.status===412}check(conflict,'Recovered stale original must stay protected');
 SheetsApp.dispose('recovery');host.remove();await OfficeSession.draft('delete',draftKey,undefined,'sheets');
 const nativeFetch=window.fetch;let release,started;const began=new Promise(r=>started=r);
 window.fetch=async(...args)=>{if(args[1]?.method==='PATCH'){started();await new Promise(r=>release=r)}return nativeFetch(...args)};
 a.book.getActiveSheet().getRange('A36').setValue('captured');const saving=a.state.save();await began;
 a.book.getActiveSheet().getRange('A36').setValue('newer');release();await saving;window.fetch=nativeFetch;
 check(a.session.dirty,'Old save acknowledged newer input');await a.state.save();check(!a.session.dirty,'Newer input saved');
 const all=await(await fetch('/api/desktop/office/workbook?path='+encodeURIComponent(a.path))).json();check(all.workbook.sheets[all.workbook.sheetOrder[0]].cellData[35][0].v==='newer','Saved latest value');
 return {twoWindows:true,copy:true,recovery:true,revisionRace:true};
 }`)
	t.Logf("Sheets save lifecycle: %s", result.JSON("", ""))
}
