package ui

import (
	"bytes"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// verifyWriterPdf runs after verifyWriterTemplates in the file-ops fixture. The fixture's PDF endpoint keeps the
// page it was sent; this check renders that page the way the server does, in a blank page under the same policy.
func verifyWriterPdf(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{
        const app=WriterApp.instances.get('test'),wait=ms=>new Promise(r=>setTimeout(r,ms));
        const until=async(test,what)=>{for(let i=0;i<160;i++){if(await test())return;await wait(50);}throw Error('timed out: '+what);};
        const labels=await(await fetch('/lang/desktop/en.json')).json(),text=key=>labels['desktop.writer_'+key];
        const banner=()=>document.querySelector('[data-notice]'),said=()=>document.querySelector('[data-notice-text]').textContent;
        const sent=async()=>(await (await fetch('/test/traces')).json()).filter(item=>item.method==='PDF');
        window.confirmAnswer=true;
        // A new document with a picture and a landscape section.
        await app.act('blank');
        await until(()=>!!app.editor&&document.querySelector('[data-loading]').hidden,'the blank document');
        const editor=app.editor,run=command=>{const result=editor.exec(command);if(!result.ok)throw Error(JSON.stringify({command:command.type,result}));};
        const toEnd=()=>{const last=editor.surface.session.paragraphIds().at(-1),end={paragraphId:last,offset:editor.query({type:'paragraphs'}).at(-1).text.length};run({type:'setSelection',range:{anchor:end,head:end}});};
        run({type:'paste',text:'Server PDF body'});
        const canvas=document.createElement('canvas');canvas.width=64;canvas.height=48;canvas.getContext('2d').fillRect(0,0,64,48);
        const blob=await new Promise(r=>canvas.toBlob(r,'image/png')),transfer=new DataTransfer();transfer.items.add(new File([blob],'pdf.png',{type:'image/png'}));
        const input=document.querySelector('[data-image]');input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));
        await until(()=>[...editor.surface.session.currentPackage().partBytes.keys()].some(part=>part.includes('/media/')),'the picture');
        toEnd();
        for(const command of [{type:'insertBreak',kind:'section'},{type:'paste',text:'Wide page'},{type:'setPageSetup',orientation:'landscape',scope:'section'}])run(command);
        const name=app.path.split('/').pop().replace(/\.docx$/,'')+'.pdf';

        // Download as PDF: one request without a path, the file saved under the document's name, nothing left behind.
        const clicks=[],click=HTMLAnchorElement.prototype.click;
        HTMLAnchorElement.prototype.click=function(){if(this.download)clicks.push({name:this.download,href:this.href});else click.call(this);};
        let before=(await sent()).length;
        await app.act('pdf');
        HTMLAnchorElement.prototype.click=click;
        let all=await sent();
        if(all.length!==before+1||all.at(-1).status!==200||all.at(-1).url.includes('path='))throw Error('download sent '+JSON.stringify(all.slice(before))+'; notice: '+said());
        if(clicks.length!==1||clicks[0].name!==name)throw Error('download saved '+JSON.stringify(clicks)+' instead of '+name);
        if(!(await (await fetch(clicks[0].href)).text()).startsWith('%PDF-'))throw Error('the downloaded file is not the PDF');
        if(!banner().hidden)throw Error('the notice stayed: '+said());
        if(document.querySelector('iframe.writer-print-frame'))throw Error('the print frame stayed');

        // Save as PDF in the workspace: a new file, then the question about the existing one.
        window.saveTarget='Documents/Exports/report.pdf';
        await app.act('pdfSave');
        all=await sent();
        if(all.at(-1).status!==200||!decodeURIComponent(all.at(-1).url).includes('path=Documents/Exports/report.pdf')||all.at(-1).url.includes('overwrite'))throw Error('save sent '+JSON.stringify(all.at(-1)));
        if(said()!==text('pdf_saved')||banner().dataset.error!=='false')throw Error('save says '+said());
        window.confirmAnswer=false;before=all.length;
        await app.act('pdfSave');
        all=await sent();
        if(all.length!==before+1||all.at(-1).status!==409)throw Error('a refused overwrite sent '+JSON.stringify(all.slice(before)));
        if(!banner().hidden)throw Error('a refused overwrite says '+said());
        window.confirmAnswer=true;before=all.length;
        await app.act('pdfSave');
        all=await sent();
        if(all.length!==before+2||all.at(-2).status!==409||all.at(-1).status!==200||!all.at(-1).url.includes('overwrite=1'))throw Error('an overwrite sent '+JSON.stringify(all.slice(before)));
        if(said()!==text('pdf_saved'))throw Error('an overwrite says '+said());

        // Without a renderer the workspace entry says so and the download entry opens the print dialog with its hint.
        await fetch('/test/pdf-down?on=1',{method:'POST'});
        await app.act('pdfSave');
        if(said()!==text('pdf_unavailable')||banner().dataset.error!=='true')throw Error('no renderer says '+said());
        let printed=0;
        const observer=new MutationObserver(records=>{for(const record of records)for(const frame of record.addedNodes)if(frame.tagName==='IFRAME'){
            const hook=()=>{frame.contentWindow.print=()=>{printed++;};};hook();frame.addEventListener('load',hook);
        }});
        observer.observe(document.body,{childList:true});
        await app.act('pdf');observer.disconnect();
        await fetch('/test/pdf-down?on=0',{method:'POST'});
        if(printed!==1||said()!==text('pdf_hint'))throw Error('the fallback printed '+printed+' times and says '+said());
    }`)

	packed := page.MustEval(`async()=>(await fetch('/test/pdf-page')).text()`).Str()
	for _, banned := range []string{"<script", "<link", "blob:", `class="docx-drawing docx-drawing-placeholder"`, `id="source"`} {
		if strings.Contains(packed, banned) {
			t.Fatalf("the packed page contains %q", banned)
		}
	}
	for _, needed := range []string{`@font-face{font-family:"calibri";font-weight:400;font-style:normal;src:url(data:font/ttf;base64,`, `src="data:image/png;base64,`, "@page writer-sheet-0{", "@page writer-sheet-1{", "<title>"} {
		if !strings.Contains(packed, needed) {
			t.Fatalf("the packed page lacks %q", needed)
		}
	}

	fresh := page.Browser().MustPage("about:blank")
	defer fresh.Close()
	policy := `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:">`
	if err := fresh.SetDocumentContent(strings.Replace(packed, "<head>", "<head>"+policy, 1)); err != nil {
		t.Fatal(err)
	}
	state := fresh.MustEval(`async()=>{
        await document.fonts.ready;
        await Promise.all([...document.images].map(image=>image.decode().catch(()=>{})));
        return {pages:document.querySelectorAll('.docx-page').length,faces:[...document.fonts].filter(face=>face.status==='loaded').length,images:[...document.images].map(image=>image.naturalWidth).join(),text:document.body.innerText};
    }`)
	if state.Get("pages").Int() != 2 || state.Get("faces").Int() < 1 || state.Get("images").Str() != "64" {
		t.Fatalf("the packed page renders as %s", state.JSON("", ""))
	}
	if text := state.Get("text").Str(); !strings.Contains(text, "Server PDF body") || !strings.Contains(text, "Wide page") {
		t.Fatalf("the packed page reads %q", text)
	}
	pdf, err := (proto.PagePrintToPDF{PrintBackground: true, PreferCSSPageSize: true}).Call(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll("../reports/autor", 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("../reports/autor/server.pdf", pdf.Data, 0644); err != nil {
		t.Fatal(err)
	}
	portrait, landscape := 0, 0
	for _, box := range regexp.MustCompile(`/MediaBox \[0 0 ([0-9.]+) ([0-9.]+)\]`).FindAllSubmatch(pdf.Data, -1) {
		width, _ := strconv.ParseFloat(string(box[1]), 64)
		height, _ := strconv.ParseFloat(string(box[2]), 64)
		if width > height {
			landscape++
		} else {
			portrait++
		}
	}
	if portrait != 1 || landscape != 1 {
		t.Fatalf("the rendered PDF has %d portrait and %d landscape pages instead of 1 and 1", portrait, landscape)
	}
	if !regexp.MustCompile(`/FontName\s*/[A-Z]+\+Carlito`).Match(pdf.Data) {
		t.Fatal("the rendered PDF does not embed the document's font")
	}
	if !bytes.Contains(pdf.Data, []byte("/Subtype /Image")) {
		t.Fatal("the rendered PDF has no picture")
	}
}
