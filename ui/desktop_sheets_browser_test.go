package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopSheetsEngineBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/sheets-engine-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html><head><meta charset="utf-8"><link rel="stylesheet" href="/js/vendor/sheets/engine.css"></head><body style="margin:0"><main id="editor" style="height:100vh;width:100vw"></main></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/sheets-engine-fixture").Timeout(45 * time.Second)
	defer page.Close()
	page.MustSetViewport(1366, 768, 1, false)
	page.MustWaitLoad()
	value := page.MustEval(`async()=>{
  window.workerErrors=[];const NativeWorker=window.Worker;window.Worker=class extends NativeWorker{constructor(...args){super(...args);this.addEventListener('error',e=>workerErrors.push(e.message))}};
  const lib=await import('/js/vendor/sheets/engine.js');
  const {univer,univerAPI:api}=lib.createUniver({locale:lib.LocaleType.EN_US,locales:{[lib.LocaleType.EN_US]:lib.locales['en-US']},presets:[
   lib.UniverSheetsCorePreset({container:document.getElementById('editor'),header:false,toolbar:false,formulaBar:false,footer:false,disableAutoFocus:true,workerURL:'/js/vendor/sheets/worker.js'}),
   lib.UniverSheetsFilterPreset(),lib.UniverSheetsSortPreset(),lib.UniverSheetsDataValidationPreset(),lib.UniverSheetsConditionalFormattingPreset(),lib.UniverSheetsFindReplacePreset(),lib.UniverSheetsNotePreset(),lib.UniverSheetsHyperLinkPreset(),lib.UniverSheetsTablePreset()
  ]});
  window.nativeSheets={univer,api,lib};
  let aux={charts:[]};const bridge=lib.installSheetActions(univer,{get:()=>aux,set:v=>{aux=v}});
  const book=api.createWorkbook({id:'probe',name:'Budget',appVersion:'0.25.1',locale:'enUS',styles:{},sheetOrder:['budget'],sheets:{budget:{id:'budget',name:'Budget',rowCount:1000,columnCount:26,defaultRowHeight:26,defaultColumnWidth:116,cellData:{0:{0:{v:'Category',t:1},1:{v:'Amount',t:1}},1:{0:{v:'Rent',t:1},1:{v:1250.5,t:2}},2:{0:{v:'Food',t:1},1:{v:400,t:2}},3:{0:{v:'Total',t:1},1:{f:'=SUM(B2:B3)'}}},rowData:{},columnData:{},mergeData:[],freeze:{xSplit:0,ySplit:1,startRow:1,startColumn:0},zoomRatio:1,showGridlines:1,rowHeader:{width:48},columnHeader:{height:26}}}});
  window.nativeSheets.book=book;
  const sheet=book.getActiveSheet();sheet.getRange('A1:B1').setFontWeight('bold').setBackgroundColor('#dae8f7');sheet.getRange('B2:B4').setNumberFormat('#,##0.00');
  bridge.commit({charts:[{title:'Budget'}]},book.getId());if(aux.charts.length!==1)throw Error('Auxiliary mutation failed');
  await api.undo();if(aux.charts.length)throw Error('Auxiliary undo failed');
  await api.redo();if(aux.charts.length!==1)throw Error('Auxiliary redo failed');
  sheet.getRange('A1:B4').activate();api.getFormula().executeCalculation();
  await new Promise(resolve=>{const timer=setTimeout(resolve,5000);api.getFormula().calculationEnd(()=>{clearTimeout(timer);resolve()})});
  return {errors:workerErrors,value:sheet.getRange('B4').getValue(),styles:Object.keys(book.save().styles).length,resources:book.save().resources,rect:sheet.getRange('F2').getCellRect().toJSON()};
 }`)
	t.Logf("Native engine probe: %s", value.JSON("", ""))
	page.MustEval(`()=>{const v=nativeSheets.book.getActiveSheet().getRange('B4').getRawValue();if(v!==1650.5)throw Error('Formula result: '+JSON.stringify(v)+' worker: '+workerErrors.join(';'));}`)
	page.MustEval(`async()=>{const {api,book}=nativeSheets;const sheet=book.getActiveSheet();sheet.insertRows(1,1);if(sheet.getRange('B5').getFormula()!=='=SUM(B3:B4)')throw Error('Reference shift failed: '+sheet.getRange('B5').getFormula());await api.undo();if(sheet.getRange('B4').getFormula()!=='=SUM(B2:B3)')throw Error('Structure undo failed');}`)
	_ = os.MkdirAll("../reports/sheets", 0755)
	shot, err := page.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join("../reports/sheets", "engine.png"), shot, 0644); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>nativeSheets.univer.dispose()`)
}
