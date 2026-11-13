package ui

import (
	"testing"

	"github.com/go-rod/rod"
)

// verifyWriterRecent runs after the file operations of the file-ops fixture. The fixture keeps the
// shell's recent list in window.recent and the window menus in window.menus.
func verifyWriterRecent(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{
        const app=WriterApp.instances.get('test'),wait=ms=>new Promise(r=>setTimeout(r,ms));
        const until=async(test,what)=>{for(let i=0;i<120;i++){if(test())return;await wait(50);}throw Error('timed out: '+what);};
        const shown=target=>app.path===target&&!!app.editor&&document.querySelector('[data-loading]').hidden;
        const paths=()=>(window.recent||[]).map(entry=>entry.path).join();
        const recent=()=>window.menus.find(menu=>menu.id==='file').items.find(item=>item.id==='recent').items;
        const labels=()=>recent().map(item=>item.label).join();
        const pick=label=>{const item=recent().find(entry=>entry.label===label);if(!item)throw Error('the recent menu lacks '+label+': '+labels());return item.action();};
        const body=()=>app.editor.surface.session.bodyText();
        const dialog=()=>document.querySelector('dialog.vd-form-dialog');
        const select=()=>dialog()?.querySelector('[data-form-field="draft"]');
        // A scroll event makes Writer refresh, which rebuilds the menus when the list changed.
        const nudge=async()=>{document.querySelector('[data-scroll]').dispatchEvent(new Event('scroll'));await wait(250);};
        window.confirmAnswer=true;
        if(app.session.dirty)await app.session.save();
        // Rename, Save as and the retried copy were recorded; the name renamed away was forgotten.
        if(paths()!=='Documents/race.docx,Documents/taken.docx,Documents/renamed.docx')throw Error('recorded '+paths());
        if(window.recent.some(entry=>entry.appId!=='writer'))throw Error('recorded for another app');
        if(labels()!=='taken.docx,renamed.docx')throw Error('the menu lists '+labels());
        // An entry opens its document and moves to the top of the list.
        await pick('renamed.docx');
        await until(()=>shown('Documents/renamed.docx'),'the recent document did not open');
        if(!body().includes('renamed'))throw Error('the recent document reads '+JSON.stringify(body()));
        if(!paths().startsWith('Documents/renamed.docx,Documents/race.docx,'))throw Error('opening did not record: '+paths());
        if(labels()!=='race.docx,taken.docx')throw Error('after opening the menu lists '+labels());
        // A file that is gone leaves the list, and the menu still leads back.
        window.recent.unshift({path:'Documents/gone.docx',appId:'writer'});
        await nudge();
        await pick('gone.docx');
        await until(()=>!document.querySelector('[data-load-actions]').hidden,'the missing document did not fail');
        if(paths().includes('gone.docx'))throw Error('the missing document stayed in the list: '+paths());
        await pick('race.docx');
        await until(()=>shown('Documents/race.docx'),'the way back did not open');
        // A new document is listed once it holds text and was saved, not before.
        await app.act('new');
        await until(()=>app.path.includes('untitled-')&&shown(app.path),'the new document did not open');
        const fresh=app.path;
        if(app.session.dirty)await app.session.save();
        if(paths().includes(fresh))throw Error('an empty new document was recorded');
        app.editor.exec({type:'paste',text:'First words'});
        await app.session.save();
        if(!paths().startsWith(fresh+','))throw Error('the new document with text was not recorded: '+paths());
        // Kept drafts: one of a document that no longer exists, and an older one.
        for(const entry of await WriterSession.drafts())await WriterSession.draft('delete',entry.key);
        app.editor.exec({type:'paste',text:' lost words'});
        const bytes=await app.editor.save();
        await app.session.save();
        const key=target=>location.origin+':'+target;
        await WriterSession.draft('put',key('Documents/lost.docx'),{id:'lost',bytes,revision:1,etag:null,updated:Date.now(),path:'Documents/lost.docx'});
        await WriterSession.draft('put',key('Documents/old.docx'),{id:'old',bytes,revision:1,etag:null,updated:Date.now()-60000,path:'Documents/old.docx'});
        const listed=await WriterSession.drafts();
        if(listed.map(entry=>entry.id).join()!=='lost,old'||'bytes' in listed[0]||!listed[0].size)throw Error('draft list '+JSON.stringify(listed));
        void app.act('drafts');
        await until(()=>!!select(),'the drafts dialog did not open');
        if(select().options.length!==2||!select().options[0].textContent.startsWith('lost.docx'))throw Error('drafts shown: '+select().textContent);
        if(select().size!==2||select().multiple)throw Error('the drafts are not shown as a list of two rows');
        // Delete acts on the picked draft and the dialog comes back with the rest.
        select().value=key('Documents/old.docx');
        dialog().querySelector('[data-form-extra]').click();
        await until(()=>select()?.options.length===1,'deleting a draft did not leave one');
        if((await WriterSession.drafts()).map(entry=>entry.id).join()!=='lost')throw Error('the wrong draft was deleted');
        // Restore starts a new document at the draft's path, with the draft in it.
        dialog().querySelector('[type=submit]').click();
        await until(()=>shown('Documents/lost.docx'),'the draft did not open');
        await until(()=>body().includes('lost words'),'the draft text is missing');
        await app.session.save();
        if((await WriterSession.drafts()).length)throw Error('the restored draft stayed after saving');
        if(!paths().startsWith('Documents/lost.docx,'))throw Error('the restored document was not recorded: '+paths());
        // Without drafts the dialog says so and offers no choice.
        void app.act('drafts');
        await until(()=>!!dialog(),'the empty drafts dialog did not open');
        if(select())throw Error('an empty draft list offers a choice');
        dialog().querySelector('[type=submit]').click();await wait(150);
        if(dialog())throw Error('the empty drafts dialog stayed open');
    }`)
}
