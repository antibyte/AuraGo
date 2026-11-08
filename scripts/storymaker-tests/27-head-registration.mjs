import assert from 'node:assert/strict';
import fs from 'node:fs';
import {decodePNG} from '../../assets/storymaker/lib/png.mjs';
import {headAnchor} from '../../assets/storymaker/lib/raster-attachments.mjs';
import {realCatalog,readJSON,root} from '../storymaker-test-support.mjs';
import {t} from '../storymaker-test-context.mjs';

t.check('every facial drawing shares a fixed crown, skull center and scale through speech and blinking',async()=>{
    // Let the short worker deadline checks settle before decoding the full cast's atlases.
    await Promise.all(t.pending.slice());
    for(const character of realCatalog().entries.filter(e=>e.kind==='character')){
        const descriptor=readJSON('ui/img/storymaker/'+character.file);
        const file=descriptor.views.front.find(s=>s.kind==='head').variants.neutral.file;
        const bitmap=decodePNG(fs.readFileSync(root+'/ui/img/storymaker/'+file));
        for(const [view,slots]of Object.entries(descriptor.views)){
            const variants=slots.find(s=>s.kind==='head').variants,ref=variants.neutral,a=headAnchor(bitmap,ref.rect);
            const center=ref.box[0]+a.x*ref.box[2]/ref.rect[2],width=a.width*ref.box[2]/ref.rect[2];
            for(const [name,image]of Object.entries(variants)){
                const anchor=headAnchor(bitmap,image.rect),scale=image.box[2]/image.rect[2];
                assert.ok(Math.abs(image.box[0]+anchor.x*scale-center)<1e-8,character.id+'/'+view+'/'+name+' shifts the skull');
                assert.ok(Math.abs(anchor.width*scale-width)<1e-8,character.id+'/'+view+'/'+name+' changes skull size');
                assert.equal(image.box[1],ref.box[1],character.id+'/'+view+'/'+name+' shifts the crown');
                assert.ok(Math.abs(scale-image.box[3]/image.rect[3])<1e-8,'A face must not be stretched');
            }
        }
    }
});
