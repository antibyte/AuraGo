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
            editor.exec({type:'paste',text:'Before '});
            void app.act('link');await wait(200);
            if(!dialog())throw Error('link dialog did not open');
            if(dialog().querySelector('[data-form-extra]'))throw Error('a new link offers no removal');
            field('text').value='AuraGo';field('href').value='javascript:alert(1)';submit();await wait(80);
            if(!dialog())throw Error('a script address was accepted');
            field('href').value='example.com';submit();await wait(300);
            if(dialog())throw Error('link dialog stayed open');
            if(failed())throw Error('link insert: '+failed());
            editor.exec({type:'paste',text:' after'});
            if(body().trim()!=='Before AuraGo after')throw Error('the document reads '+JSON.stringify(body()));
            // Editing and removing work from a collapsed caret inside the link, in a longer paragraph.
            editor.exec({type:'selectAll'});
            const start=editor.surface.state().selection.anchor;
            const caret=offset=>editor.exec({type:'setSelection',range:{anchor:{...start,offset},head:{...start,offset}}});
            caret(9);
            const made=editor.query({type:'hyperlinkAt'});
            if(made?.href!=='https://example.com')throw Error('hyperlinkAt after insert: '+JSON.stringify(made));
            void app.act('link');await wait(200);
            if(field('text'))throw Error('editing a link must not ask for its text');
            if(field('href').value!=='https://example.com')throw Error('edit shows '+field('href').value);
            if(!dialog().querySelector('[data-form-extra]'))throw Error('editing a link offers no removal');
            field('href').value='https://aurago.example/docs';submit();await wait(300);
            if(failed())throw Error('link edit: '+failed());
            caret(9);
            const edited=editor.query({type:'hyperlinkAt'});
            if(edited?.href!=='https://aurago.example/docs')throw Error('hyperlinkAt after edit: '+JSON.stringify(edited));
            if(body().trim()!=='Before AuraGo after')throw Error('editing the link changed the text: '+JSON.stringify(body()));
            caret(2);
            if(editor.query({type:'hyperlinkAt'}))throw Error('the link spread to the paragraph');
            caret(9);
            void app.act('link');await wait(200);
            dialog().querySelector('[data-form-extra]').click();await wait(300);
            if(failed())throw Error('link removal: '+failed());
            caret(9);
            if(editor.query({type:'hyperlinkAt'}))throw Error('the link was not removed');
            if(body().trim()!=='Before AuraGo after')throw Error('removing the link changed the text: '+JSON.stringify(body()));
            // Go to page accepts a page of the document.
            void app.act('goToPage');await wait(200);
            if(field('page').max!==String(editor.getTotalPages()))throw Error('go to page allows up to '+field('page').max);
            field('page').value='1';submit();await wait(150);
            if(dialog())throw Error('go to page stayed open');
            // Word count is read-only: rows, one button.
            void app.act('wordCount');await wait(200);
            const rows=[...dialog().querySelectorAll('.vd-form-static')];
            if(rows.length<4)throw Error('word count shows '+rows.length+' rows');
            if(rows[1].querySelector('strong').textContent!=='3')throw Error('three words expected, got '+rows[1].textContent);
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

// verifyWriterActions covers the app actions behind the Edit and View menus: paste from the
// system clipboard (stubbed), the tracking switch, zoom presets and Ctrl+wheel.
func verifyWriterActions(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{` + writerWave1Prelude + `
        const own=Object.getOwnPropertyDescriptor(navigator,'clipboard');
        Object.defineProperty(navigator,'clipboard',{configurable:true,value:{
            read:async()=>[{types:['text/html','text/plain'],getType:async type=>new Blob([type==='text/html'?'<p><b>Rich paste</b></p>':'Rich paste'],{type})}],
            readText:async()=>'Plain paste'}});
        try {
            await app.act('paste');await wait(200);
            if(failed())throw Error('paste: '+failed());
            if(!body().includes('Rich paste'))throw Error('paste inserted '+JSON.stringify(body()));
            await app.act('pastePlain');await wait(200);
            if(failed())throw Error('paste without formatting: '+failed());
            if(!body().includes('Plain paste'))throw Error('plain paste inserted '+JSON.stringify(body()));
            editor.exec({type:'selectAll'});
            for(const type of ['copy','cut'])if(!editor.can({type}).ok)throw Error(type+' is refused on a selection: '+editor.can({type}).reason);
            await app.act('tracking');
            if(editor.snapshot().editingMode!=='suggesting')throw Error('tracking did not switch on');
            await app.act('tracking');
            if(editor.snapshot().editingMode!=='editing')throw Error('tracking did not switch off');
            await app.act('zoom150');
            if(Math.abs(editor.getZoom()-1.5)>.001)throw Error('zoom150 gave '+editor.getZoom());
            if(document.querySelector('[data-zoom]').textContent!=='150%')throw Error('the status bar shows '+document.querySelector('[data-zoom]').textContent);
            await app.act('zoomPage');await wait(300);
            const whole=editor.getZoom(),sheet=document.querySelector('.docx-page').getBoundingClientRect(),view=document.querySelector('[data-scroll]');
            if(sheet.height>view.clientHeight||sheet.width>view.clientWidth)throw Error('whole page does not fit: '+Math.round(sheet.width)+'x'+Math.round(sheet.height)+' in '+view.clientWidth+'x'+view.clientHeight);
            const turn=new WheelEvent('wheel',{deltaY:-100,ctrlKey:true,bubbles:true,cancelable:true});
            view.dispatchEvent(turn);
            if(!turn.defaultPrevented||Math.abs(editor.getZoom()-(whole+.1))>.011)throw Error('Ctrl+wheel: '+editor.getZoom()+' after '+whole);
            const scroll=new WheelEvent('wheel',{deltaY:-100,bubbles:true,cancelable:true});
            view.dispatchEvent(scroll);
            if(scroll.defaultPrevented)throw Error('a plain wheel must scroll');
            await app.act('fit');
        } finally {
            if(own)Object.defineProperty(navigator,'clipboard',own);else delete navigator.clipboard;
            rewind();
        }
        if(body().trim())throw Error('the document was not left blank: '+JSON.stringify(body()));
    }`)
}
