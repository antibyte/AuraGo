package ui

import (
	"archive/zip"
	"aurago/internal/office"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopWriterAppBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	var mu sync.Mutex
	files := map[string][]byte{}
	versions := map[string]int{}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/api/desktop/office/document", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		path := r.URL.Query().Get("path")
		if path == "Documents/import.md" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"document": map[string]string{"text": "# Imported heading\n\nOriginal Markdown remains untouched."}})
			return
		}
		if r.Method == "PUT" {
			expected := fmt.Sprintf("\"%d\"", versions[path])
			if (versions[path] > 0 && r.Header.Get("If-Match") != expected) || (versions[path] == 0 && r.Header.Get("If-None-Match") != "*") {
				http.Error(w, "Conflict", 412)
				return
			}
			files[path], _ = io.ReadAll(r.Body)
			versions[path]++
		}
		if versions[path] == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", fmt.Sprintf("\"%d\"", versions[path]))
		if r.Method == "GET" {
			w.Write(files[path])
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "{}")
	})
	mux.HandleFunc("/api/desktop/office/assist", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"replacement": "A clear, considered proposal.", "source_revision": body["source_revision"]})
	})
	mux.HandleFunc("/writer-app-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html lang="en"><head><meta charset="utf-8">
<link rel="stylesheet" href="/js/vendor/writer/engine.css"><link rel="stylesheet" href="/css/desktop-app-writer.css">
<style>html,body{margin:0;height:100%;font-family:system-ui}body{--vd-text:#e4e8f0;--vd-theme-app-bg:#141922;--vd-theme-panel-bg:#1b202a;--vd-theme-chrome-bg:#1d2430;--vd-theme-control-bg:#252d3a;--vd-theme-border:#ffffff1a;--vd-theme-muted:#a7b1c3;--vd-accent:#9abfff}body[data-fruity-mode=light]{--vd-text:#212b3c;--vd-theme-app-bg:#e2e7ee;--vd-theme-panel-bg:#e5e9ef;--vd-theme-chrome-bg:#d7dfe9;--vd-theme-control-bg:#f8fafc;--vd-theme-border:#65748c40;--vd-theme-muted:#536279;--vd-accent:#2169bd}#host{height:100%}</style></head><body class="desktop-body" data-theme="default"><div id="host"></div>
<script src="/js/vendor/purify.min.js"></script><script src="/js/vendor/marked.min.js"></script><script src="/js/desktop/apps/writer-session.js"></script><script src="/js/desktop/apps/writer-panels.js"></script><script src="/js/desktop/apps/writer.js"></script>
<script>window.ready=(async()=>{const labels=await(await fetch('/lang/desktop/en.json')).json();window.writerContext={t:(key,params)=>String(labels[key]||key).replace(/\{\{(\w+)\}\}/g,(_,x)=>params?.[x]??''),promptDialog:async(title,value)=>window.promptAnswer??value,confirmDialog:async()=>true,setWindowBeforeClose:(id,fn)=>window.closeGuard=fn,setWindowMenus:(id,menus)=>window.menus=menus,saveFileDialog:async()=>({path:'Documents/copy.docx'}),openFileDialog:async()=>({path:WriterApp.instances.get('test').path})};WriterApp.render(document.getElementById('host'),'test',writerContext);})();</script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/writer-app-fixture").Timeout(30 * time.Second)
	defer page.Close()
	page.MustSetViewport(1366, 768, 1, false)
	page.MustWaitLoad()
	page.Timeout(90 * time.Second).MustWait(`()=>WriterApp.instances.get('test')?.editor && !document.querySelector('[data-loading]').offsetHeight`)
	result := page.MustEval(`async()=>{
        const app=WriterApp.instances.get('test'),editor=app.editor;
        const run=c=>{const r=editor.exec(c);if(!r.ok)throw Error(JSON.stringify({c,r}));return r;};
        run({type:'paste',text:'',html:'<h1>A thoughtful place to write.</h1><p>Every good idea deserves room to grow. Autor brings your words, feedback and ideas together in a calm workspace.</p><h2>Clarity, one page at a time</h2><p>Work with genuine document pages, thoughtful typography and focused tools.</p>'});
        editor.selectMatch(editor.findMatches('Every good idea')[0]);
        const size=document.querySelector('[data-slot="font.size"]');
        for(const points of [12,11.5,1,200]) {
            const before=editor.snapshot().formatting.fontSizePt;
            size.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));size.focus();size.value=String(points);
            size.dispatchEvent(new Event('change',{bubbles:true}));size.blur();
            await new Promise(r=>setTimeout(r,100));
            if(editor.snapshot().formatting.fontSizePt!==points)throw Error('Font size in points was not applied: '+points);
            if(size.disabled || Number(size.value)!==points)throw Error('Font size control lost its value or became disabled');
            if(!document.querySelector('[data-notice]').hidden)throw Error(document.querySelector('[data-notice-text]').textContent);
            run({type:'undo'});if(editor.snapshot().formatting.fontSizePt!==before)throw Error('Font size undo failed');
        }
        for(const invalid of ['',0,200.5,11.25]) {
            const before=editor.snapshot().formatting.fontSizePt,revision=app.session.revision;
            size.focus();size.value=String(invalid);size.dispatchEvent(new Event('change',{bubbles:true}));size.blur();
            if(editor.snapshot().formatting.fontSizePt!==before || app.session.revision!==revision)throw Error('Invalid font size changed the document');
            if(!document.querySelector('[data-notice]').hidden)throw Error('Invalid font size reached the editor');
        }
        const ids=editor.surface.session.paragraphIds();const last=ids[ids.length-1];
        run({type:'setSelection',range:{anchor:{paragraphId:last,offset:editor.query({type:'paragraphs'}).at(-1).text.length},head:{paragraphId:last,offset:editor.query({type:'paragraphs'}).at(-1).text.length}}});
        run({type:'insertBreak',kind:'page'});
        run({type:'paste',text:'The next chapter begins here.'});
        if(editor.getTotalPages()<2)throw Error('No real page break');
        run({type:'selectAll'});
        const bold=editor.exec({type:'toggleMark',mark:'bold'});if(!bold.ok)throw Error(JSON.stringify(bold));
        run({type:'undo'});
        const comment=editor.addComment('The introduction is ready for review.');if(!comment.ok)throw Error(JSON.stringify(comment));
        await app.act('format');editor.scrollToPage(1);await new Promise(r=>setTimeout(r,250));
        return {pages:editor.getTotalPages(),text:editor.surface.session.bodyText(),pageClasses:[...document.querySelectorAll('[data-editor] [class]')].slice(0,12).map(x=>x.className),setup:editor.snapshot().pageSetup,selection:editor.surface.state().selection};
    }`).JSON("", "")
	t.Log(result)
	os.MkdirAll("../reports/autor", 0755)
	for _, variant := range []string{"default-dark", "fruity-dark", "fruity-light"} {
		bits := strings.Split(variant, "-")
		page.MustEval(`(theme,mode)=>{document.body.dataset.theme=theme;document.body.dataset.fruityMode=mode}`, bits[0], bits[1])
		page.MustScreenshot("../reports/autor/" + variant + "-format.png")
	}
	page.MustEval(`async()=>await WriterApp.instances.get('test').act('review')`)
	page.MustScreenshot("../reports/autor/fruity-light-review.png")

	page.MustEval(`async()=>{
        const app=WriterApp.instances.get('test'),editor=app.editor;
        await app.session.save();if(app.session.dirty)throw Error('Save stayed dirty');
        editor.selectMatch(editor.findMatches('Every good idea')[0]);await app.act('assist');
        document.querySelector('[data-panel-command="ai_generate"]').click();
        await new Promise(r=>setTimeout(r,500));
        if(!document.querySelector('[data-panel-command="ai_apply"]'))throw Error(JSON.stringify({text:editor.query({type:'selectedText'}),panel:document.querySelector('[data-right]').innerHTML,notice:document.querySelector('[data-notice]').textContent}));
        const n=document.querySelector('[data-notice]');if(n && !n.hidden && n.dataset.error==='true')throw Error(n.textContent);
    }`)
	page.Timeout(15 * time.Second).MustWait(`()=>!!document.querySelector('[data-panel-command="ai_apply"]')`)
	page.MustEval(`()=>{
        const editor=WriterApp.instances.get('test').editor,before=editor.surface.session.bodyText();
        document.querySelector('[data-panel-command="ai_apply"]').click();
        if(!editor.surface.session.bodyText().includes('A clear, considered proposal.'))throw Error('AI suggestion was not applied');
        editor.exec({type:'undo'});if(editor.surface.session.bodyText()!==before)throw Error('AI application was not one undo step');
        document.querySelector('[data-panel-command="ai_generate"]').click();
    }`)
	page.Timeout(15 * time.Second).MustWait(`()=>!!document.querySelector('[data-panel-command="ai_apply"]')`)
	page.MustEval(`()=>WriterApp.instances.get('test').editor.exec({type:'paste',text:'A changed sentence.'})`)
	page.Timeout(5 * time.Second).MustWait(`()=>document.querySelector('[data-panel-command="ai_apply"]').disabled`)
	page.MustEval(`()=>{
        const editor=WriterApp.instances.get('test').editor,last=editor.surface.session.paragraphIds().at(-1),offset=editor.query({type:'paragraphs'}).at(-1).text.length;
        editor.exec({type:'setSelection',range:{anchor:{paragraphId:last,offset},head:{paragraphId:last,offset}}});editor.surface.focus();window.beforeKeyboardInput=editor.surface.session.bodyText();
    }`)
	if err := (proto.InputInsertText{Text: " Grüße — café — Ελληνικά — Кириллица"}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.Timeout(5 * time.Second).MustWait(`()=>WriterApp.instances.get('test').editor.surface.session.bodyText().includes('Grüße — café — Ελληνικά — Кириллица')`)
	if err := (proto.InputDispatchKeyEvent{Type: "keyDown", Key: "z", Code: "KeyZ", Modifiers: 2, WindowsVirtualKeyCode: 90}).Call(page); err != nil {
		t.Fatal(err)
	}
	if err := (proto.InputDispatchKeyEvent{Type: "keyUp", Key: "z", Code: "KeyZ", WindowsVirtualKeyCode: 90}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.Timeout(5 * time.Second).MustWait(`()=>WriterApp.instances.get('test').editor.surface.session.bodyText()===beforeKeyboardInput`)
	page.MustEval(`async()=>{
        const app=WriterApp.instances.get('test');await app.session.save();
        const host=document.createElement('div');host.id='second-host';host.style.cssText='position:absolute;left:20px;top:20px;width:800px;height:600px';
        document.body.appendChild(host);WriterApp.render(host,'second',{...writerContext,path:app.path});
    }`)
	page.Timeout(20 * time.Second).MustWait(`()=>WriterApp.instances.get('second')?.editor && document.querySelector('#second-host [data-loading]').hidden`)
	page.MustEval(`async()=>{
        const first=WriterApp.instances.get('test'),second=WriterApp.instances.get('second');
        first.editor.exec({type:'paste',text:'First window edit'});await first.session.save();
        second.editor.exec({type:'paste',text:'Second window edit'});
        let conflict=false;try{await second.session.save();}catch(error){conflict=error.status===412;}
        if(!conflict || !second.session.dirty || !second.editor.surface.session.bodyText().includes('Second window edit'))throw Error('Second-instance conflict lost edits');
        await second.act('saveAs');if(second.path!=='Documents/copy.docx')throw Error('Conflict copy was not saved');
        WriterApp.dispose('second');document.getElementById('second-host').remove();
        const host=document.createElement('div');host.id='readonly-host';document.body.appendChild(host);
        WriterApp.render(host,'readonly',{...writerContext,path:first.path,readonly:true});
    }`)
	page.Timeout(20 * time.Second).MustWait(`()=>!!WriterApp.instances.get('readonly')?.editor`)
	page.MustEval(`()=>{
        const app=WriterApp.instances.get('readonly');
        if(app.editor.exec({type:'paste',text:'must not write'}).ok || app.session.dirty)throw Error('Readonly writer accepted edits');
        WriterApp.dispose('readonly');document.getElementById('readonly-host').remove();
        const host=document.createElement('div');host.id='import-host';document.body.appendChild(host);
        WriterApp.render(host,'import',{...writerContext,path:'Documents/import.md',readonly:true});
    }`)
	page.Timeout(20 * time.Second).MustWait(`()=>!!WriterApp.instances.get('import')?.editor && document.querySelector('#import-host [data-loading]').hidden`)
	page.MustEval(`()=>{
        const app=WriterApp.instances.get('import');
        if(!app.editor.surface.session.bodyText().includes('Imported heading') || !app.editor.getOutline().length || app.session.dirty)throw Error('Readonly Markdown import was blank or lost structure: '+JSON.stringify({text:app.editor.surface.session.bodyText(),outline:app.editor.getOutline(),styles:app.editor.getDocumentStyles().map(x=>[x.styleId,x.name]),dirty:app.session.dirty}));
        WriterApp.dispose('import');document.getElementById('import-host').remove();
    }`)
	page.MustEval(`async()=>{
        const canvas=document.createElement('canvas');canvas.width=64;canvas.height=48;canvas.getContext('2d').fillRect(0,0,64,48);
        const blob=await new Promise(r=>canvas.toBlob(r,'image/png')),transfer=new DataTransfer();transfer.items.add(new File([blob],'example.png',{type:'image/png'}));
        const input=document.querySelector('[data-image]');input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));
    }`)
	page.Timeout(10 * time.Second).MustWait(`()=>[...WriterApp.instances.get('test').editor.surface.session.currentPackage().partBytes.keys()].some(x=>x.includes('/media/'))`)
	page.MustEval(`async()=>{
        const observer=new MutationObserver(records=>{for(const r of records)for(const frame of r.addedNodes)if(frame.tagName==='IFRAME'){
            const hook=()=>{frame.contentWindow.print=()=>{
                window.printProbe={pages:frame.contentDocument.querySelectorAll('.docx-page').length,text:frame.contentDocument.body.innerText,html:frame.contentDocument.documentElement.outerHTML};window.printFonts=[...frame.contentDocument.fonts];
            };};hook();frame.addEventListener('load',hook);
        }});
        observer.observe(document.body,{childList:true});
        await WriterApp.instances.get('test').act('print');observer.disconnect();
        if(!window.printProbe || printProbe.pages!==WriterApp.instances.get('test').editor.getTotalPages() || !printProbe.text.includes('First window edit'))throw Error('Print did not capture all current pages: '+JSON.stringify(window.printProbe)+'; notice: '+document.querySelector('[data-notice]').textContent);
    }`)

	page.MustEval(`async()=>{
        WriterApp.dispose('test');
        const html=printProbe.html,faces=printFonts;
        document.open();document.write('<!doctype html>'+html);document.close();
        for(const face of faces)document.fonts.add(face);
        await document.fonts.ready;
    }`)
	pdf, err := (proto.PagePrintToPDF{PrintBackground: true, PreferCSSPageSize: true}).Call(page)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("../reports/autor/print.pdf", pdf.Data, 0644); err != nil {
		t.Fatal(err)
	}
	if pages := bytes.Count(pdf.Data, []byte("/Type /Page\n")); pages != 2 {
		t.Fatalf("Printed PDF has %d pages instead of 2", pages)
	}
}

func writerLongDocument(t *testing.T) []byte {
	t.Helper()
	base, err := office.EncodeDOCX(office.Document{Text: "Reference"})
	if err != nil {
		t.Fatal(err)
	}
	parts, err := office.ReadDOCXParts(base)
	if err != nil {
		t.Fatal(err)
	}
	var body strings.Builder
	body.WriteString(`<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for i := 1; i <= 100; i++ {
		pageBreak := ""
		if i > 1 {
			pageBreak = "<w:pageBreakBefore/>"
		}
		fmt.Fprintf(&body, `<w:p><w:pPr>%s</w:pPr><w:r><w:rPr><w:rFonts w:ascii="Uninstalled Reference Font" w:hAnsi="Uninstalled Reference Font"/><w:lang w:val="de-DE"/></w:rPr><w:t>Reference page %d. Grüße, café, Ελληνικά, Кириллица.</w:t></w:r></w:p>`, pageBreak, i)
	}
	body.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1417" w:right="1417" w:bottom="1417" w:left="1417"/></w:sectPr></w:body></w:document>`)
	parts["word/document.xml"] = []byte(body.String())
	parts["customXml/preserved.xml"] = []byte("<preserved>unchanged metadata</preserved>")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range parts {
		w, e := zw.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write(data); e != nil {
			t.Fatal(e)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDesktopWriterEngineBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	longDocument := writerLongDocument(t)
	mux.HandleFunc("/writer-long.docx", func(w http.ResponseWriter, r *http.Request) { w.Write(longDocument) })
	mux.HandleFunc("/writer-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><link rel="stylesheet" href="/js/vendor/writer/engine.css"><div class="docx-editor__scroll-container" style="height:850px;overflow:auto"><div id="editor" class="docx-editor"></div></div>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/writer-fixture")
	defer page.Close()
	page.MustWaitLoad()
	result := page.Timeout(90*time.Second).MustEval(`async()=>{
        const lib = await import('/js/vendor/writer/engine.js');
        window.writerEngine = lib;
        const fonts = await lib.defaultFonts();
        if(fonts.failures.length) throw Error(JSON.stringify(fonts.failures));
        const editor=window.writerProbe=lib.createDocxEditor({container:document.getElementById('editor'),document:'blank',fonts,author:'Autor Test',modules:[lib.reviewModule]});
        await document.fonts.ready;
        const run=command=>{const result=editor.exec(command);if(!result.ok)throw Error(JSON.stringify({command,result}));return result};
        run({type:'paste',text:'First paragraph.\nSecond paragraph.'});
        run({type:'setPageSetup',pageWidth:11906,pageHeight:16838,marginTop:1417,marginBottom:1417,marginLeft:1417,marginRight:1417});
        run({type:'selectAll'});
        const comment=editor.addComment('Review comment');if(!comment.ok)throw Error(JSON.stringify(comment));
        run({type:'setEditingMode',mode:'suggesting'});
        run({type:'paste',text:' Tracked text'});
        const review=editor.getReviewItems({placement:false});
        if(!review.some(x=>x.kind==='comment')||!review.some(x=>x.kind==='revision'))throw Error('Missing review: '+JSON.stringify(review));
        const bytes=await editor.save();
        const pages=editor.snapshot().page.total;
        editor.load(bytes);
        if(editor.getReviewItems({placement:false}).length!==review.length)throw Error('Review round-trip lost items');

        const probes=[];
        for(const commands of [
            [{type:'paste',text:'Footnote anchor.'},{type:'insertNote',noteKind:'footnote'},{type:'paste',text:'This is a footnote.'}],
            [{type:'insertTable',rows:2,cols:2},{type:'insertRow',where:'below'},{type:'insertColumn',where:'right'},{type:'setCellFill',color:{kind:'hex',value:'DDEEFF'}},{type:'setTableBorders',scope:'all',spec:{style:'single',size:4,color:{kind:'hex',value:'64748B'}}}],
            [{type:'editHeaderFooter',position:'footer'},{type:'insertPageField',field:'PAGE_X_OF_Y'},{type:'exitHeaderFooter'}],
            [{type:'paste',text:'Review text'},{type:'selectAll'},{type:'setEditingMode',mode:'suggesting'},{type:'toggleMark',mark:'bold'},{type:'paste',text:'Revised text'}]
        ]) {
            editor.load('blank');run({type:'setEditingMode',mode:'editing'});
            for(const command of commands)run(command);
            for(const item of editor.getReviewItems({placement:false}).filter(x=>x.kind==='revision')){
                const result=editor.acceptReviewItem(item.key);if(!result.ok)throw Error(JSON.stringify({result,item}));
            }
            const saved=await editor.save();editor.load(saved);
            if(commands.some(x=>x.type==='insertNote')) {
                if(!editor.surface.session.currentPackage().partBytes.has('/word/footnotes.xml') || !document.getElementById('editor').textContent.includes('This is a footnote.'))throw Error('Footnote did not survive or render');
            }
            probes.push({commands:commands.map(x=>x.type),bytes:saved.byteLength,pages:editor.getTotalPages()});
        }

        editor.load('blank');run({type:'setEditingMode',mode:'editing'});run({type:'insertTable',rows:2,cols:2});

        run({type:'paste',text:'Table review anchor'});run({type:'selectAll'});
        const tableComment=editor.addComment('Keep table comment');if(!tableComment.ok)throw Error(JSON.stringify(tableComment));
        const nodes=(n,name)=>[...(n.localName===name?[n]:[]),...(n.children || []).flatMap(x=>nodes(x,name))];
        let table=nodes(editor.surface.session.part().root,'tbl')[0];
        const cells=table.children.filter(x=>x.localName==='tr').flatMap(row=>row.children.filter(x=>x.localName==='tc'));
        editor.surface.setCellSelection({kind:'cells',tableId:table.id,cellIds:cells.map(x=>x.id),rows:{from:0,to:1},columns:{from:0,to:1},text:{anchor:{paragraphId:nodes(cells[0],'p')[0].id,offset:0},head:{paragraphId:nodes(cells[1],'p')[0].id,offset:0}}});
        const merged=lib.tableCommand(editor,{type:'mergeCells'});if(!merged.ok)throw Error(JSON.stringify({merged,selected:editor.getSelectedTable()}));
        table=nodes(editor.surface.session.part().root,'tbl')[0];
        if(table.children.find(x=>x.localName==='tr').children.filter(x=>x.localName==='tc').length!==1)throw Error('Cells did not merge');
        run({type:'undo'});
        table=nodes(editor.surface.session.part().root,'tbl')[0];
        if(table.children.find(x=>x.localName==='tr').children.filter(x=>x.localName==='tc').length!==2)throw Error('Merge undo did not restore cells');
        run({type:'redo'});
        const first=nodes(nodes(editor.surface.session.part().root,'tbl')[0],'p')[0];
        run({type:'setSelection',range:{anchor:{paragraphId:first.id,offset:0},head:{paragraphId:first.id,offset:0}}});
        const split=lib.tableCommand(editor,{type:'splitCell',rows:1,cols:2});if(!split.ok)throw Error(JSON.stringify(split));
        if(nodes(nodes(editor.surface.session.part().root,'tbl')[0],'tc').length!==4)throw Error('Vertical merge did not split back into cells');
        const tableRoundTrip=await editor.save();editor.load(tableRoundTrip);
        if(!editor.getReviewItems({placement:false}).some(x=>x.text==='Keep table comment'))throw Error('Table operation lost comment');
        const tableExtra={merged,split};editor.destroy();
        return {pages,bytes:bytes.byteLength,review:review.length,fonts:fonts.sources.length,probes,tableExtra};
    }`).JSON("", "")
	t.Logf("Native DOCX engine: %s", result)

	t.Log(page.Timeout(90*time.Second).MustEval(`async()=>{
        const lib=await import('/js/vendor/writer/engine.js'), base=await lib.defaultFonts();
        const bytes=await(await fetch('/writer-long.docx')).arrayBuffer(),resources=lib.documentResources(bytes,base);
        if(resources.language!=='de-DE' || !resources.substitutions.some(x=>x.includes('Uninstalled Reference Font')))throw Error('Font/language inspection failed');
        document.getElementById('editor').replaceChildren();
        const start=performance.now(),long=lib.createDocxEditor({container:document.getElementById('editor'),document:bytes,fonts:resources.fonts,modules:[lib.reviewModule]});
        await long.save();await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));const openMs=performance.now()-start;
        if(long.getTotalPages()!==100)throw Error('Expected 100 pages, got '+long.getTotalPages());
        long.scrollToPage(90);await new Promise(r=>setTimeout(r,100));
        const current=long.getCurrentPage('viewport'),materialized=document.querySelectorAll('.docx-page[data-materialized="true"]').length;
        if(Math.abs(current-90)>1 || materialized>12)throw Error(JSON.stringify({current,materialized}));
        const last=long.surface.session.paragraphIds().at(-1),editStart=performance.now();
        long.exec({type:'setSelection',range:{anchor:{paragraphId:last,offset:0},head:{paragraphId:last,offset:0}}});
        const edit=long.exec({type:'paste',text:'Edited '});if(!edit.ok)throw Error(JSON.stringify(edit));
        const editMs=performance.now()-editStart,saved=await long.save();long.load(saved);
        const preserved=long.surface.session.currentPackage().partBytes.get('/customXml/preserved.xml');
        if(new TextDecoder().decode(preserved)!=='<preserved>unchanged metadata</preserved>')throw Error('Unknown package part lost');
        if(long.getTotalPages()!==100)throw Error('Round-trip changed pagination');
        const output=document.createElement('div');document.body.appendChild(output);
        lib.paintSemanticLayout(output,long.surface.layout(),{scale:96/72});
        if(output.querySelectorAll('.docx-page').length!==100 || !output.textContent.includes('Reference page 100'))throw Error('Long-document output truncated');
        if(openMs>15000 || editMs>2000)throw Error('Long document unresponsive: '+JSON.stringify({openMs,editMs}));
        output.remove();long.destroy();return {pages:100,current,materialized,openMs:Math.round(openMs),editMs:Math.round(editMs)};
    }`).JSON("", ""))
}
