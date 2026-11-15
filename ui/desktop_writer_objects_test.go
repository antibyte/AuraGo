package ui

import (
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// verifyWriterObjects runs after verifyWriterPdf in the file-ops fixture: table borders drawn with the pen on a
// dragged cell selection, a picture's size, crop and replacement, and the style list's previews.
func verifyWriterObjects(t *testing.T, page *rod.Page) {
	t.Helper()
	helpers := `const app=WriterApp.instances.get('test'),editor=app.editor,wait=ms=>new Promise(r=>setTimeout(r,ms));
        const until=async(test,what)=>{for(let i=0;i<160;i++){if(await test())return;await wait(50);}throw Error('timed out: '+what);};
        const labels=await(await fetch('/lang/desktop/en.json')).json(),text=key=>labels['desktop.writer_'+key];
        const said=()=>document.querySelector('[data-notice-text]').textContent;
        const field=key=>document.querySelector('[data-right] [data-field="'+key+'"]');
        const set=async(key,value)=>{const el=field(key);if(el.type==='checkbox')el.checked=value;else el.value=String(value);el.dispatchEvent(new Event('change',{bubbles:true}));await wait(250);};
        const run=command=>{const result=editor.exec(command);if(!result.ok)throw Error(JSON.stringify({command:command.type,result}));};
        const u=WriterLayout.units(document.documentElement.lang,navigator.language);`
	script := func(body string) string { return `async()=>{` + helpers + body + `}` }

	// A blank document with a 3 x 3 table and the Format panel open.
	centers := page.MustEval(`async()=>{
        const app=WriterApp.instances.get('test'),wait=ms=>new Promise(r=>setTimeout(r,ms));
        window.confirmAnswer=true;
        await app.act('blank');
        for(let i=0;i<160&&!(app.editor&&document.querySelector('[data-loading]').hidden);i++)await wait(50);
        const result=app.editor.exec({type:'insertTable',rows:3,cols:3});
        if(!result.ok)throw Error('no table: '+JSON.stringify(result));
        if(document.querySelector('[data-right]').hidden)await app.act('format');
        await wait(400);
        if(!document.querySelector('[data-right] [data-field="borderStyle"]'))throw Error('the Format panel has no border pen: '+document.querySelector('[data-right]').textContent.slice(0,200));
        document.querySelector('[data-scroll]').scrollTop=0;
        await wait(150);
        const cells=[...document.querySelectorAll('.docx-page .docx-table-cell')].map(cell=>cell.getBoundingClientRect());
        if(cells.length!==9)throw Error('the table shows '+cells.length+' cells');
        return {ax:cells[0].left+20,ay:cells[0].top+cells[0].height/2,mx:cells[4].left+cells[4].width/2,my:cells[4].top+cells[4].height/2,bx:cells[8].left+cells[8].width/2,by:cells[8].top+cells[8].height/2};
    }`)
	page.Mouse.MustMoveTo(centers.Get("ax").Num(), centers.Get("ay").Num())
	page.Mouse.MustDown(proto.InputMouseButtonLeft)
	if err := page.Mouse.MoveLinear(proto.NewPoint(centers.Get("mx").Num(), centers.Get("my").Num()), 8); err != nil {
		t.Fatal(err)
	}
	if err := page.Mouse.MoveLinear(proto.NewPoint(centers.Get("bx").Num(), centers.Get("by").Num()), 8); err != nil {
		t.Fatal(err)
	}
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	time.Sleep(300 * time.Millisecond)
	page.MustEval(script(`
        if(editor.getTableCellSelection()?.cellIds.length!==9)throw Error('dragging selected '+JSON.stringify(editor.getTableCellSelection()));
        // Red thick 1.5 pt lines; the edge buttons draw with that pen, each as one undo step.
        const red=()=>[...document.querySelectorAll('.docx-page .docx-table-cell')].map(cell=>[...cell.querySelectorAll('.docx-table-border-edge-stroke')].filter(line=>/rgb\(255, 0, 0\)/.test(line.getAttribute('style'))).length);
        const all=()=>[...document.querySelectorAll('.docx-page .docx-table-cell')].map(cell=>cell.querySelectorAll('.docx-table-border-edge-stroke,.docx-table-border-double-stroke').length).reduce((a,b)=>a+b,0);
        const press=async edge=>{document.querySelector('[data-right] [data-border="'+edge+'"]').click();await wait(300);};
        const edges=[...document.querySelectorAll('[data-right] [data-border]')].map(button=>button.dataset.border+':'+button.textContent).join();
        if(edges!=='all:Borders,outside:Outside,inside:Inside,top:Top,bottom:Bottom,left:Left,right:Right,none:No borders')throw Error('the edge buttons read '+edges);
        if([...field('borderStyle').options].map(option=>option.textContent).join()!=='Solid,Double,Triple,Thick')throw Error('the line styles read '+field('borderStyle').textContent);
        if(field('borderWidth').options.length!==6||!/pt$/.test(field('borderWidth').options[0].textContent))throw Error('the line widths read '+field('borderWidth').textContent);
        await press('none');
        if(all()!==0)throw Error('No borders left '+all()+' lines');
        await set('borderStyle','thick');await set('borderWidth',12);await set('borderColor','#ff0000');
        if(all()!==0)throw Error('choosing a pen changed the document');
        await press('outside');
        const outline=red();
        if(outline[4]!==0||outline.reduce((a,b)=>a+b,0)!==12)throw Error('Outside drew '+JSON.stringify(outline));
        await press('inside');
        if(red().reduce((a,b)=>a+b,0)!==24)throw Error('Inside drew '+JSON.stringify(red()));
        run({type:'undo'});await wait(300);
        if(red().reduce((a,b)=>a+b,0)!==12)throw Error('one undo did not take Inside back: '+JSON.stringify(red()));
        // Double lines are drawn too, and the pen stays for the next click and for a reopened panel.
        await set('borderStyle','double');
        await press('all');
        if(!document.querySelector('.docx-page .docx-table-cell .docx-table-border-double-stroke'))throw Error('All drew no double line');
        await app.act('format');await wait(200);await app.act('format');await wait(300);
        if(field('borderStyle').value!=='double'||field('borderWidth').value!=='12'||field('borderColor').value!=='#ff0000')throw Error('the pen was forgotten: '+field('borderStyle').value+' '+field('borderWidth').value+' '+field('borderColor').value);
        // Outside a table the edge buttons are off.
        const last=editor.surface.session.paragraphIds().at(-1);
        run({type:'setSelection',range:{anchor:{paragraphId:last,offset:0},head:{paragraphId:last,offset:0}}});
        await wait(400);
        if(editor.snapshot().table||!document.querySelector('[data-right] [data-border="all"]').disabled)throw Error('the edge buttons stay on outside a table');
        if(!document.querySelector('[data-right] [data-action="imageReplace"]').disabled)throw Error('Replace image is on without a picture');
        // A picture after the table.
        const canvas=document.createElement('canvas');canvas.width=64;canvas.height=48;canvas.getContext('2d').fillRect(0,0,64,48);
        const blob=await new Promise(r=>canvas.toBlob(r,'image/png')),transfer=new DataTransfer();transfer.items.add(new File([blob],'objects.png',{type:'image/png'}));
        const input=document.querySelector('[data-image]');input.files=transfer.files;input.dispatchEvent(new Event('change',{bubbles:true}));
        await until(()=>[...editor.surface.session.currentPackage().partBytes.keys()].some(part=>part.includes('/media/')),'the picture');
        await until(()=>document.querySelector('.docx-page .docx-drawing-ready'),'the decoded picture');
        document.querySelector('.docx-page .docx-drawing').scrollIntoView({block:'center'});
        await wait(200);
    `))
	box := page.MustEval(`()=>{const b=document.querySelector('.docx-page .docx-drawing').getBoundingClientRect();return {x:b.left+b.width/2,y:b.top+b.height/2};}`)
	page.Mouse.MustMoveTo(box.Get("x").Num(), box.Get("y").Num())
	page.Mouse.MustClick(proto.InputMouseButtonLeft)
	time.Sleep(400 * time.Millisecond)
	page.MustEval(script(`
        const image=()=>editor.snapshot().image;
        if(!image())throw Error('a click did not select the picture');
        await wait(300);
        if(document.querySelector('[data-right] [data-action="imageReplace"]').disabled)throw Error('Replace image is off for a selected picture');
        const w0=image().widthEmu,h0=image().heightEmu;
        if(field('imageWidth').value!==u.show(w0/635)||field('imageHeight').value!==u.show(h0/635))throw Error('the size fields read '+field('imageWidth').value+' x '+field('imageHeight').value);
        if(!field('imageLock').checked)throw Error('the aspect lock starts off');
        // Locked: the height follows the width. Unlocked: one side only.
        await set('imageWidth',u.show(w0*2/635));
        if(Math.abs(image().widthEmu-w0*2)>5000||Math.abs(image().heightEmu-h0*2)>5000)throw Error('locked width gave '+image().widthEmu+' x '+image().heightEmu+' from '+w0+' x '+h0);
        await set('imageLock',false);
        const w1=image().widthEmu;
        await set('imageHeight',u.show(h0/635));
        if(image().widthEmu!==w1||Math.abs(image().heightEmu-h0)>5000)throw Error('unlocked height gave '+image().widthEmu+' x '+image().heightEmu);
        if(field('imageHeight').value!==u.show(image().heightEmu/635))throw Error('the height field reads '+field('imageHeight').value);
        // Crop in percent; what leaves nothing of the picture is refused with a reason.
        await set('cropLeft',10);await set('cropTop',20);
        if(image().crop.left!==10||image().crop.top!==20||image().crop.right!==0)throw Error('crop gave '+JSON.stringify(image().crop));
        field('cropRight').value='95';await set('cropLeft',10);
        if(said()!==text('crop_invalid')||image().crop.right!==0)throw Error('a crop of everything says '+said()+' and gave '+JSON.stringify(image().crop));
        document.querySelector('[data-right] [data-command*="crop"]').click();await wait(300);
        if(image().crop.left!==0||image().crop.top!==0||field('cropLeft').value!=='0')throw Error('Reset crop left '+JSON.stringify(image().crop)+' and the field at '+field('cropLeft').value);
        // A picture in the text cannot change its wrap, so the list is off instead of failing.
        if(image().canChangeWrap||!field('imageWrap').disabled)throw Error('the wrap list is on for a picture in the text');
        // Replace image: a square JPEG for a 4:3 PNG. The engine takes PNG only, so Writer converts; the height follows.
        const media=()=>[...editor.surface.session.currentPackage().partBytes.entries()].filter(([part])=>part.includes('/media/')).map(([part,bytes])=>part+':'+bytes.byteLength).join();
        const before=media(),w2=image().widthEmu;
        const square=document.createElement('canvas');square.width=120;square.height=120;const paint=square.getContext('2d');paint.fillStyle='#0a0';paint.fillRect(0,0,120,120);
        const jpeg=await new Promise(r=>square.toBlob(r,'image/jpeg')),chosen=new DataTransfer();chosen.items.add(new File([jpeg],'square.jpg',{type:'image/jpeg'}));
        const bar=[...document.querySelectorAll('[data-contextbar] button')].map(button=>button.textContent.trim()).join('|');
        if(!bar.includes(text('image_replace')))throw Error('the picture bar reads '+bar);
        const replace=document.querySelector('[data-image-replace]');replace.files=chosen.files;replace.dispatchEvent(new Event('change',{bubbles:true}));
        await until(()=>media()!==before,'the replaced picture; notice: '+said());
        await until(()=>image()&&Math.abs(image().heightEmu-image().widthEmu)<=1,'the height of the replaced picture: '+JSON.stringify({w:image()?.widthEmu,h:image()?.heightEmu,notice:said()}));
        if(image().widthEmu!==w2)throw Error('replacing changed the width from '+w2+' to '+image().widthEmu);
        if(!/\.png:/.test(media()))throw Error('the replaced file is '+media());
        // The style list: every paragraph style, drawn the way it looks where the browser can.
        const styles=document.querySelector('[data-slot="styles.style"]'),options=[...styles.options];
        if(options.length<16)throw Error('the style list has '+options.length+' entries');
        if(CSS.supports('appearance','base-select')) {
            const look=id=>options.find(option=>option.value===id).getAttribute('style')||'';
            if(!/font-size:22px/.test(look('Title'))||!/color:#2F5496/.test(look('Heading1'))||!/font-style:italic/.test(look('Quote')))throw Error('the previews read '+look('Title')+' | '+look('Heading1')+' | '+look('Quote'));
            if(getComputedStyle(styles).appearance!=='base-select')throw Error('the style list is drawn as '+getComputedStyle(styles).appearance);
        } else if(options.some(option=>option.hasAttribute('style')))throw Error('a plain list carries previews');
        const first=editor.surface.session.paragraphIds().at(-1);
        run({type:'setSelection',range:{anchor:{paragraphId:first,offset:0},head:{paragraphId:first,offset:0}}});
        styles.value='Heading1';styles.dispatchEvent(new Event('change',{bubbles:true}));await wait(300);
        if(editor.snapshot().formatting.styleId!=='Heading1')throw Error('choosing a style gave '+editor.snapshot().formatting.styleId);
        document.querySelector('[data-scroll]').scrollTop=0;
        field('cellFill').scrollIntoView({block:'start'});
    `))

	// The panel's table and picture sections and the open list, for the eye.
	if err := os.MkdirAll("../reports/autor", 0755); err != nil {
		t.Fatal(err)
	}
	page.MustScreenshot("../reports/autor/objects-panel.png")
	list := page.MustEval(`()=>{const b=document.querySelector('[data-slot="styles.style"]').getBoundingClientRect();return {x:b.left+b.width/2,y:b.top+b.height/2};}`)
	page.Mouse.MustMoveTo(list.Get("x").Num(), list.Get("y").Num())
	page.Mouse.MustClick(proto.InputMouseButtonLeft)
	time.Sleep(400 * time.Millisecond)
	page.MustScreenshot("../reports/autor/styles-gallery.png")
	page.Keyboard.MustType(input.Escape)
}
