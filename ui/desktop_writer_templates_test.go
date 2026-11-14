package ui

import (
	"testing"

	"github.com/go-rod/rod"
)

// verifyWriterTemplates runs after verifyWriterRecent in the file-ops fixture. The fixture keeps every written
// document in memory, lists a folder under /api/desktop/files and records each request under /test/traces.
func verifyWriterTemplates(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{
        const app=WriterApp.instances.get('test'),T=WriterTemplates,wait=ms=>new Promise(r=>setTimeout(r,ms));
        const until=async(test,what)=>{for(let i=0;i<160;i++){if(await test())return;await wait(50);}throw Error('timed out: '+what);};
        const labels=await(await fetch('/lang/desktop/en.json')).json(),text=key=>labels['desktop.writer_'+key];
        const shown=()=>!!app.editor&&document.querySelector('[data-loading]').hidden;
        const body=()=>app.editor.surface.session.bodyText();
        const dialog=()=>document.querySelector('dialog.vd-form-dialog');
        const field=id=>dialog()?.querySelector('[data-form-field="'+id+'"]');
        const submit=()=>dialog().querySelector('[type=submit]').click();
        const notice=()=>document.querySelector('[data-notice]');
        const puts=async target=>(await (await fetch('/test/traces')).json()).filter(item=>item.method==='PUT'&&decodeURIComponent(item.url).includes(target)&&item.status===200);
        const u=WriterLayout.units(document.documentElement.lang,navigator.language);
        window.confirmAnswer=true;
        if(app.session.dirty)await app.session.save();
        // The File menu offers the three new entries where they belong.
        const fileIds=window.menus.find(menu=>menu.id==='file').items.map(item=>item.id||'-').join();
        for(const run of ['new,newFromTemplate,open','saveAs,saveTemplate,-','docx,pdf,html'])if(!fileIds.includes(run))throw Error('the File menu reads '+fileIds);
        // Every built-in template builds: its paper and margins, its texts, a footer, no undo steps.
        const paper=u.unit==='in'?[12240,15840]:[11906,16838],marks={letter:'template_subject',invoice:'invoice_intro',minutes:'agenda',report:'summary'};
        for(const id of T.ids) {
            const before=app.path;
            await app.act(id);
            await until(()=>shown()&&app.path!==before,id+' did not open');
            const setup=app.editor.getPageSetup(),margins=setup.marginsTwips;
            if(setup.pageWidthTwips!==paper[0]||setup.pageHeightTwips!==paper[1]||[margins.top,margins.right,margins.bottom,margins.left].join()!=='1417,1134,1134,1417')throw Error(id+' page: '+JSON.stringify(setup));
            if(!text(marks[id])||!body().includes(text(marks[id])))throw Error(id+' lacks "'+text(marks[id])+'": '+body().slice(0,200));
            if(body().includes('undefined')||body().includes('desktop.writer_'))throw Error(id+' shows a raw key: '+body().slice(0,300));
            if(app.editor.snapshot().canUndo)throw Error(id+' starts with undo steps');
            if(![...app.editor.surface.session.currentPackage().partBytes.keys()].some(key=>key.includes('footer')))throw Error(id+' has no footer');
            if(!/untitled-[0-9a-f]{8}\.docx$/.test(app.path))throw Error(id+' opened at '+app.path);
            if((id==='minutes'||id==='report')&&app.editor.getOutline().length!==3)throw Error(id+' outline: '+JSON.stringify(app.editor.getOutline()));
            if(id==='invoice') {
                const at=app.editor.query({type:'paragraphs'}).findIndex(paragraph=>paragraph.text===text('invoice_description'));
                const start={paragraphId:app.editor.surface.session.paragraphIds()[at],offset:0};
                app.editor.exec({type:'setSelection',range:{anchor:start,head:start}});
                const table=app.editor.getSelectedTable();
                if(!table||table.columnCount!==5||table.rowCount!==6)throw Error('the invoice table is '+JSON.stringify(table));
            }
        }
        // An untouched document from a template is not written.
        const untouched=app.path.split('/').pop();
        await wait(1500);
        if(app.session.dirty||(await puts(untouched)).length)throw Error('an untouched template document was written');
        // The Insert panel shows the built-in templates and leads to the list.
        if(!document.querySelector('.writer-right .writer-template-grid'))await app.act('insert');
        await wait(150);
        const tiles=[...document.querySelectorAll('.writer-right .writer-template-grid button')].map(button=>button.dataset.action).join();
        if(tiles!=='blank,letter,invoice,minutes,report')throw Error('the Insert panel offers '+tiles);
        if(!document.querySelector('.writer-right [data-action="newFromTemplate"]'))throw Error('the Insert panel has no way to the template list');
        await app.act('insert');
        // Without own templates the list holds the four built-in ones and names the folder.
        void app.act('newFromTemplate');
        await until(()=>!!field('template'),'the template list did not open');
        if(field('template').options.length!==4||field('template').size!==4||field('template').multiple)throw Error('the list offers '+field('template').options.length+' rows');
        if(!dialog().textContent.includes(T.folder))throw Error('the list does not name the folder');
        dialog().querySelector('[data-form-cancel]').click();await wait(150);
        // Save as template: an invalid name is refused in the dialog, a valid one writes into the folder.
        app.editor.exec({type:'paste',text:'House style marker '});
        const home=app.path,target=T.folder+'/House style.docx';
        void app.act('saveTemplate');
        await until(()=>!!field('name'),'the template dialog did not open');
        if(field('name').value!=='')throw Error('an unnamed document offers the name '+field('name').value);
        field('name').value='a/b';submit();await wait(150);
        if(!dialog()||dialog().querySelector('[data-form-error]').hidden)throw Error('an invalid template name was accepted');
        field('name').value='House style';submit();
        await until(async()=>(await puts(target)).length===1,'the template was not written');
        await until(()=>!notice().hidden&&notice().textContent.includes('House style'),'saving a template says nothing');
        if(dialog())throw Error('the template dialog stayed open');
        if(app.path!==home)throw Error('saving a template moved the document to '+app.path);
        const first=(await puts(target))[0];
        if(first.ifNone!=='*')throw Error('the first template write carried '+JSON.stringify(first));
        // The same name again asks, then replaces the file with its version.
        void app.act('saveTemplate');
        await until(()=>!!field('name'),'the template dialog did not open again');
        field('name').value='House style.docx';submit();
        await until(async()=>(await puts(target)).length===2,'the template was not replaced');
        if((await puts(target))[1].ifMatch!==first.etag)throw Error('the replacement did not name the version it replaces');
        // New from template now lists the own template and opens it as a new document.
        void app.act('newFromTemplate');
        await until(()=>!!field('template'),'the template list did not open again');
        const options=[...field('template').options].map(option=>option.value).join();
        if(options!=='builtin:letter,builtin:invoice,builtin:minutes,builtin:report,own:'+target)throw Error('the list offers '+options);
        if(field('template').size!==5||!field('template').options[4].textContent.startsWith('House style · '))throw Error('the own template is shown as '+field('template').options[4].textContent);
        field('template').value='own:'+target;submit();
        await until(()=>shown()&&app.path!==home&&body().includes('House style marker'),'the own template did not open');
        if(!/untitled-[0-9a-f]{8}\.docx$/.test(app.path))throw Error('a document from a template opened at '+app.path);
        if(app.editor.snapshot().canUndo)throw Error('a document from an own template starts with undo steps');
        await app.act('save');
        if(app.session.dirty)throw Error('the new document was not saved');
        if((await puts(target)).length!==2)throw Error('saving the new document wrote the template');
    }`)
}
