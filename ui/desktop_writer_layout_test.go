package ui

import (
	"testing"

	"github.com/go-rod/rod"
)

// verifyWriterLayout drives the Page setup and Paragraph dialogs, the Format panel's lengths and the
// page fields on the blank document and leaves it blank. Lengths are computed in the unit the page
// itself uses, so the check holds in centimetres and in inches.
func verifyWriterLayout(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{` + writerWave1Prelude + `
        const u=WriterLayout.units(document.documentElement.lang,navigator.language);
        const type=(id,value)=>{const el=field(id);el.value=value;el.dispatchEvent(new Event('input',{bubbles:true}));};
        const error=()=>{const node=dialog().querySelector('[data-form-error]');return node.hidden?'':node.textContent;};
        const opened=!!document.querySelector('.writer-right [data-field="marginTop"]');
        try {
            // Page setup: fields follow each other, an invalid length is refused, one command lands.
            void app.act('pageSetup');await wait(250);
            if(!dialog())throw Error('page setup did not open');
            if([field('paper').value,field('orientation').value,field('preset').value,field('scope').value].join()!=='a4,portrait,normal,document')throw Error('page setup shows '+[field('paper').value,field('orientation').value,field('preset').value,field('scope').value]);
            if(field('top').value!==u.show(1417)||!dialog().textContent.includes(u.unit==='in'?'in':'cm'))throw Error('page setup margins show '+field('top').value);
            type('paper','a5');
            if(field('width').value!==u.show(8391)||field('height').value!==u.show(11906))throw Error('A5 shows '+field('width').value+' x '+field('height').value);
            type('orientation','landscape');
            if(field('width').value!==u.show(11906)||field('height').value!==u.show(8391))throw Error('landscape shows '+field('width').value+' x '+field('height').value);
            type('preset','narrow');
            if(field('left').value!==u.show(720))throw Error('narrow margins show '+field('left').value);
            type('left','wide');submit();await wait(120);
            if(!dialog()||!error())throw Error('an invalid margin was accepted');
            type('left',u.show(1134));
            if(field('preset').value!=='custom')throw Error('a typed margin left the preset at '+field('preset').value);
            submit();await wait(350);
            if(dialog())throw Error('page setup stayed open: '+error());
            if(failed())throw Error('page setup: '+failed());
            const setup=editor.getPageSetup();
            if(setup.pageWidthTwips!==11906||setup.pageHeightTwips!==8391||setup.orientation!=='landscape'||setup.marginsTwips.top!==720||setup.marginsTwips.left!==u.read(u.show(1134)))throw Error('page setup gave '+JSON.stringify(setup));
            // The Format panel shows the same lengths in the same unit and takes them back.
            if(!document.querySelector('.writer-right [data-field="marginTop"]'))await app.act('format');
            await wait(200);
            const panel=key=>document.querySelector('.writer-right [data-field="'+key+'"]');
            if(panel('marginTop').value!==u.show(720)||panel('paper').value!=='a5')throw Error('the panel shows '+panel('marginTop').value+' '+panel('paper').value);
            if(!panel('marginTop').closest('label').textContent.includes(u.unit==='in'?'in':'cm'))throw Error('the panel names no unit: '+panel('marginTop').closest('label').textContent);
            panel('marginTop').value=u.show(1417);panel('marginTop').dispatchEvent(new Event('change',{bubbles:true}));await wait(250);
            if(editor.getPageSetup().marginsTwips.top!==u.read(u.show(1417)))throw Error('the panel margin gave '+editor.getPageSetup().marginsTwips.top);
            for(const id of ['pageSetup','paragraphFormat'])if(!document.querySelector('.writer-right [data-action="'+id+'"]'))throw Error('the panel has no button for '+id);
            // Paragraph: several fields in one undo step; an untouched dialog sends nothing.
            editor.exec({type:'paste',text:'Layout paragraph'});
            void app.act('paragraphFormat');await wait(250);
            if(!dialog())throw Error('the paragraph dialog did not open');
            if([field('alignment').value,field('special').value,field('keepNext').checked,field('widowControl').checked].join()!=='left,none,false,true')throw Error('the paragraph dialog shows '+[field('alignment').value,field('special').value,field('keepNext').checked,field('widowControl').checked]);
            type('alignment','justify');type('left',u.show(567));type('special','hanging');type('by',u.show(283));type('before','6');type('lineRule','exact');type('lineValue','18');
            field('keepNext').click();
            submit();await wait(350);
            if(dialog())throw Error('the paragraph dialog stayed open: '+error());
            if(failed())throw Error('paragraph: '+failed());
            const f=editor.snapshot().formatting;
            if(f.alignment!=='both'||f.indent.left!==u.read(u.show(567))||f.indent.firstLine!==-u.read(u.show(283))||f.spaceBeforePt!==6||f.lineSpacing.rule!=='exact'||f.lineSpacing.value!==18||f.paragraphFlags.keepNext!==true||f.paragraphFlags.keepLines!==false)
                throw Error('paragraph gave '+JSON.stringify({alignment:f.alignment,indent:f.indent,before:f.spaceBeforePt,line:f.lineSpacing,flags:f.paragraphFlags}));
            editor.exec({type:'undo'});
            const back=editor.snapshot().formatting;
            if(back.alignment!=='left'||back.paragraphFlags.keepNext!==false||back.indent.left!==0)throw Error('the paragraph dialog is more than one undo step');
            const revision=app.session.revision;
            void app.act('paragraphFormat');await wait(250);
            submit();await wait(250);
            if(dialog()||app.session.revision!==revision)throw Error('an untouched paragraph dialog changed the document');
            // Page fields and the date, from the footer bar.
            if(!editor.exec({type:'editHeaderFooter',position:'footer'}).ok)throw Error('the footer did not open');
            await wait(350);
            const bar=document.querySelector('[data-contextbar]'),tools=[...bar.querySelectorAll('button[data-tool]')].map(button=>button.dataset.tool).join();
            if(tools!=='fieldPage,pageNumber,fieldPages,insertDate,headerFirstPage,headerOddEven,backDocument')throw Error('the footer bar holds '+tools);
            for(const id of ['fieldPage','fieldPages','insertDate']) {
                const before=app.session.revision;
                bar.querySelector('[data-tool="'+id+'"]').click();await wait(250);
                if(failed())throw Error(id+': '+failed());
                if(app.session.revision===before)throw Error(id+' changed nothing');
            }
            bar.querySelector('[data-tool="backDocument"]').click();await wait(350);
            if(editor.snapshot().scope.kind!=='body')throw Error('the footer stayed open');
            const fields=window.menus.find(menu=>menu.id==='insert').items.find(item=>item.id==='fields');
            if(!fields||!fields.items.find(item=>item.id==='fieldPage').disabled||fields.items.find(item=>item.id==='insertDate').disabled)throw Error('in the body the page fields must be off and the date on');
        } finally {
            dialog()?.querySelector('[data-form-cancel]')?.click();
            if(editor.snapshot().scope.kind!=='body')editor.exec({type:'exitHeaderFooter'});
            rewind();
            if(!!document.querySelector('.writer-right [data-field="marginTop"]')!==opened)await app.act('format');
        }
        await wait(200);
        if(JSON.stringify(editor.getPageSetup())!==paper)throw Error('the layout check left the page as '+JSON.stringify(editor.getPageSetup()));
    }`)
}
