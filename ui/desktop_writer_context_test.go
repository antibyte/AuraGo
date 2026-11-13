package ui

import (
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// writerContextPrelude adds what the context checks share: the bar, the menu the fixture's
// showContextMenu received, a caret setter and a right-click on an element.
const writerContextPrelude = `
        const bar=document.querySelector('[data-contextbar]'),tool=id=>bar.querySelector('[data-tool="'+id+'"]');
        const ids=items=>items.filter(x=>!x.separator).map(x=>x.id),entry=id=>window.contextMenu.items.find(x=>x.id===id);
        const caret=(from,to)=>{editor.exec({type:'selectAll'});const s=editor.surface.state().selection.anchor;
            editor.exec({type:'setSelection',range:{anchor:{...s,offset:from},head:{...s,offset:to===undefined?from:to}}});};
        const rightClick=el=>{const b=el.getBoundingClientRect();window.contextMenu=null;
            const e=new MouseEvent('contextmenu',{bubbles:true,cancelable:true,button:2,clientX:b.left+b.width/2,clientY:b.top+b.height/2});el.dispatchEvent(e);return e;};
        const toTop=async()=>{document.querySelector('[data-scroll]').scrollTop=0;await wait(120);};
        const run=text=>[...document.querySelectorAll('.docx-page .layout-run-text')].find(x=>x.textContent.includes(text));
`

// verifyWriterContext covers the right-click menu and the contextual bar on the blank document
// and leaves it blank: a selection survives a right-click, a plain caret follows it, the menu
// fits a link, a table and an image, and the bar appears for a table, a header and an image.
func verifyWriterContext(t *testing.T, page *rod.Page) {
	t.Helper()
	script := func(body string) string {
		return `async()=>{` + writerWave1Prelude + writerContextPrelude + body + `}`
	}
	settle := func() { page.MustEval(`()=>new Promise(r=>setTimeout(r,300))`) }
	rightClickAt := func(x, y float64) {
		page.Mouse.MustMoveTo(x, y)
		page.Mouse.MustClick(proto.InputMouseButtonRight)
		settle()
	}
	point := page.MustEval(script(`
        editor.exec({type:'paste',text:'Alpha beta gamma delta epsilon'});await wait(200);
        caret(0,10);window.contextMenu=null;await toTop();
        const b=run('gamma').getBoundingClientRect();
        const view=document.querySelector('[data-scroll]').getBoundingClientRect();
        if(!(b.top>view.top+4&&b.bottom<view.bottom-4))throw Error('the click target is not inside the viewport: '+JSON.stringify([b.top,b.bottom,view.top,view.bottom]));
        return {x:b.left+b.width*0.8,y:b.top+b.height/2};
    `))
	x, y := point.Get("x").Num(), point.Get("y").Num()
	rightClickAt(x, y)
	page.MustEval(script(`
        if(editor.query({type:'selectedText'})!=='Alpha beta')throw Error('a right-click dropped the selection: '+JSON.stringify(editor.query({type:'selectedText'})));
        if(!window.contextMenu)throw Error('no menu opened on a right-click');
        const got=ids(window.contextMenu.items).join();
        if(got!=='cut,copy,paste,pastePlain,link,comment,paragraphFormat,assist')throw Error('menu for a text selection: '+got);
        if(entry('copy').disabled||entry('cut').disabled)throw Error('copy and cut must be enabled for a selection');
        caret(0);window.contextMenu=null;await toTop();
    `))
	rightClickAt(x, y)
	page.MustEval(script(`
        const s=editor.surface.state().selection;
        if(!editor.snapshot().selectionCollapsed||s.anchor.offset===0)throw Error('a right-click with a plain caret must move it: '+JSON.stringify(s));
        if(!window.contextMenu)throw Error('no menu opened for a plain caret');
        // A link under the caret can be removed from the menu.
        caret(0,5);
        if(!editor.exec({type:'insertHyperlink',href:'https://example.com'}).ok)throw Error('could not link the selection');
        caret(2);
        const event=rightClick(run('Alpha'));
        if(!event.defaultPrevented)throw Error('the browser menu was not suppressed');
        if(!ids(window.contextMenu.items).includes('unlink'))throw Error('no unlink in '+ids(window.contextMenu.items).join());
        entry('unlink').action();await wait(250);
        caret(2);
        if(editor.query({type:'hyperlinkAt'}))throw Error('the link was not removed');
        // Shift+F10 arrives without a pointer position; the menu opens inside the document.
        window.contextMenu=null;
        document.querySelector('.docx-pages').dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,button:-1}));
        const box=document.querySelector('[data-editor]').getBoundingClientRect(),menu=window.contextMenu;
        if(!menu||!(menu.x>=box.left&&menu.x<=box.right&&menu.y>=box.top&&menu.y<=box.bottom))throw Error('keyboard menu position: '+JSON.stringify(menu&&[menu.x,menu.y]));
        // The table bar appears with the caret in a table and acts on it.
        if(!bar.hidden)throw Error('the contextual bar shows outside any context');
        editor.exec({type:'insertTable',rows:2,cols:2});await wait(350);
        if(bar.hidden||bar.querySelector('strong').textContent!=='Table')throw Error('no table bar: '+bar.outerHTML.slice(0,200));
        tool('rowBelow').click();await wait(300);
        if(failed())throw Error('row below: '+failed());
        if(editor.getSelectedTable()?.rowCount!==3)throw Error('row below gave '+JSON.stringify(editor.getSelectedTable()));
        rightClick(document.querySelector('.docx-table-cell'));
        const table=entry('table');
        if(!table||!ids(table.items).includes('deleteTable'))throw Error('no table submenu in '+ids(window.contextMenu.items).join());
        tool('deleteTable').click();await wait(350);
        if(document.querySelector('.docx-table-fragment')||!bar.hidden)throw Error('the table or its bar stayed after deleting the table');
        // The header bar switches the header options and leads back to the document.
        editor.exec({type:'editHeaderFooter',position:'header'});await wait(350);
        if(bar.hidden||bar.querySelector('strong').textContent!=='Header')throw Error('no header bar: '+bar.outerHTML.slice(0,200));
        if(tool('headerFirstPage').getAttribute('aria-pressed')!=='false')throw Error('the first page option starts pressed');
        tool('headerFirstPage').click();await wait(300);
        if(editor.getHeaderFooterState()?.titlePage!==true||tool('headerFirstPage').getAttribute('aria-pressed')!=='true')throw Error('first page option: '+JSON.stringify(editor.getHeaderFooterState()));
        tool('backDocument').click();await wait(350);
        if(editor.snapshot().scope.kind!=='body'||!bar.hidden)throw Error('back to the document left scope '+editor.snapshot().scope.kind);
        // An image for the next step.
        const canvas=document.createElement('canvas');canvas.width=64;canvas.height=48;canvas.getContext('2d').fillRect(0,0,64,48);
        const blob=await new Promise(r=>canvas.toBlob(r,'image/png')),transfer=new DataTransfer();transfer.items.add(new File([blob],'context.png',{type:'image/png'}));
        const input=document.querySelector('[data-image]');input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));
    `))
	page.Timeout(10 * time.Second).MustWait(`()=>[...WriterApp.instances.get('test').editor.surface.session.currentPackage().partBytes.keys()].some(x=>x.includes('/media/'))`)
	settle()
	drawing := page.MustEval(`async()=>{document.querySelector('[data-scroll]').scrollTop=0;await new Promise(r=>setTimeout(r,150));const b=document.querySelector('.docx-page .docx-drawing').getBoundingClientRect();return {x:b.left+b.width/2,y:b.top+b.height/2};}`)
	page.Mouse.MustMoveTo(drawing.Get("x").Num(), drawing.Get("y").Num())
	page.Mouse.MustClick(proto.InputMouseButtonLeft)
	settle()
	page.MustEval(script(`
        if(!editor.snapshot().image)throw Error('a click did not select the image');
        if(bar.hidden||bar.querySelector('strong').textContent!=='Image')throw Error('no image bar: '+bar.outerHTML.slice(0,200));
        tool('rotateRight').click();await wait(300);
        if(editor.snapshot().image?.rotationDegrees!==90)throw Error('rotate right gave '+JSON.stringify(editor.snapshot().image?.rotationDegrees));
    `))
	// A user leaves an image with a click; a programmatic caret keeps the engine's image selection.
	text := page.MustEval(`async()=>{document.querySelector('[data-scroll]').scrollTop=0;await new Promise(r=>setTimeout(r,150));const b=[...document.querySelectorAll('.docx-page .layout-run-text')].find(x=>x.textContent.includes('beta')).getBoundingClientRect();return {x:b.left+b.width/2,y:b.top+b.height/2};}`)
	page.Mouse.MustMoveTo(text.Get("x").Num(), text.Get("y").Num())
	page.Mouse.MustClick(proto.InputMouseButtonLeft)
	settle()
	page.MustEval(script(`
        if(editor.snapshot().image||!bar.hidden)throw Error('the image bar stayed after a click into the text');
        // A right-click on an image that is not selected selects it; the menu is then about the image.
        rightClick(document.querySelector('.docx-page .docx-drawing'));await wait(400);
        if(!editor.snapshot().image)throw Error('a right-click did not select the image');
        if(!window.contextMenu)throw Error('no menu opened on an image');
        const about=ids(window.contextMenu.items);
        if(!about.includes('deleteImage')||!about.includes('rotateRight'))throw Error('no image commands in '+about.join());
        if(bar.hidden||bar.querySelector('strong').textContent!=='Image')throw Error('the image bar did not follow the right-click');
        entry('deleteImage').action();await wait(400);
        if(failed())throw Error('delete image: '+failed());
        if(document.querySelector('.docx-page .docx-drawing'))throw Error('the image was not deleted');
        rewind();
        if(body().trim())throw Error('the document was not left blank: '+JSON.stringify(body()));
        if(JSON.stringify(editor.snapshot().pageSetup)!==paper)throw Error('the page setup was not restored: '+JSON.stringify(editor.snapshot().pageSetup));
    `))
}
