package ui

import (
	"testing"

	"github.com/go-rod/rod"
)

// writerWave1Prelude is shared by the Wave 1a checks: the app, a wait, the open form dialog and
// a way back to the untouched blank document.
const writerWave1Prelude = `
        const app=WriterApp.instances.get('test'),editor=app.editor,wait=ms=>new Promise(r=>setTimeout(r,ms));
        const dialog=()=>document.querySelector('dialog.vd-form-dialog');
        const field=id=>dialog().querySelector('[data-form-field="'+id+'"]');
        const submit=()=>dialog().querySelector('[type=submit]').click();
        const body=()=>editor.surface.session.bodyText();
        const failed=()=>{const n=document.querySelector('[data-notice]');return !n.hidden&&n.dataset.error==='true'?document.querySelector('[data-notice-text]').textContent:'';};
        const rewind=()=>{for(let i=0;i<80&&editor.snapshot().canUndo;i++)editor.exec({type:'undo'});};
`

// verifyWriterDialogs drives the Wave 1a dialogs on the blank document and leaves it blank.
func verifyWriterDialogs(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{` + writerWave1Prelude + `
        try {
            // A link at a collapsed caret asks for text and address, and refuses a script address.
            void app.act('link');await wait(200);
            if(!dialog())throw Error('link dialog did not open');
            if(dialog().querySelector('[data-form-extra]'))throw Error('a new link offers no removal');
            field('text').value='AuraGo';field('href').value='javascript:alert(1)';submit();await wait(80);
            if(!dialog())throw Error('a script address was accepted');
            field('href').value='example.com';submit();await wait(300);
            if(dialog())throw Error('link dialog stayed open');
            if(failed())throw Error('link insert: '+failed());
            if(!body().includes('AuraGo'))throw Error('the link text was not inserted: '+JSON.stringify(body()));
            // The caret sits in the new link: editing shows its address and offers removal.
            editor.exec({type:'selectAll'});
            const made=editor.query({type:'hyperlinkAt'});
            if(made?.href!=='https://example.com')throw Error('hyperlinkAt after insert: '+JSON.stringify(made));
            void app.act('link');await wait(200);
            if(field('text'))throw Error('editing a link must not ask for its text');
            if(field('href').value!=='https://example.com')throw Error('edit shows '+field('href').value);
            field('href').value='https://aurago.example/docs';submit();await wait(300);
            if(failed())throw Error('link edit: '+failed());
            editor.exec({type:'selectAll'});
            const edited=editor.query({type:'hyperlinkAt'});
            if(edited?.href!=='https://aurago.example/docs')throw Error('hyperlinkAt after edit: '+JSON.stringify(edited));
            void app.act('link');await wait(200);
            dialog().querySelector('[data-form-extra]').click();await wait(300);
            if(failed())throw Error('link removal: '+failed());
            editor.exec({type:'selectAll'});
            if(editor.query({type:'hyperlinkAt'}))throw Error('the link was not removed');
            if(!body().includes('AuraGo'))throw Error('removing the link removed its text');
            // Go to page accepts a page of the document.
            void app.act('goToPage');await wait(200);
            if(field('page').max!==String(editor.getTotalPages()))throw Error('go to page allows up to '+field('page').max);
            field('page').value='1';submit();await wait(150);
            if(dialog())throw Error('go to page stayed open');
            // Word count is read-only: rows, one button.
            void app.act('wordCount');await wait(200);
            const rows=[...dialog().querySelectorAll('.vd-form-static')];
            if(rows.length<4)throw Error('word count shows '+rows.length+' rows');
            if(rows[1].querySelector('strong').textContent!=='1')throw Error('one word expected, got '+rows[1].textContent);
            if(dialog().querySelector('[data-form-cancel]'))throw Error('an information dialog has no Cancel');
            submit();await wait(150);
            // The shortcut overview lists what the registry binds.
            void app.act('shortcuts');await wait(200);
            const keys=[...dialog().querySelectorAll('.vd-form-static strong')].map(x=>x.textContent);
            for(const key of ['Ctrl+K','Ctrl+G','Ctrl+S','Ctrl+C'])if(!keys.includes(key))throw Error('shortcut overview lacks '+key);
            submit();await wait(150);
            // The table dialog: pointing at a grid cell fills rows and columns.
            void app.act('table');await wait(200);
            dialog().querySelector('[data-form-cell="2,3"]').dispatchEvent(new PointerEvent('pointerover',{bubbles:true}));
            if(field('rows').value!=='2'||field('cols').value!=='3')throw Error('grid wrote '+field('rows').value+' x '+field('cols').value);
            if(dialog().querySelectorAll('[data-form-cell][data-on]').length!==6)throw Error('grid highlight is wrong');
            dialog().querySelector('[data-form-cancel]').click();await wait(150);
            if(dialog())throw Error('table dialog stayed open');
        } finally { dialog()?.close(); rewind(); }
        if(body().trim())throw Error('the document was not left blank: '+JSON.stringify(body()));
    }`)
}
