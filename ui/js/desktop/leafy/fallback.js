/* Atlas baked locally from renderer.js; same deterministic plant and hit testing. */
(function () {
    'use strict';
    function create(canvas) {
        const ctx = canvas.getContext('2d'), atlas = new Image();
        let model = null, plant = null, light = false, selection = null, disposed = false, frames = 0;
        atlas.onload = () => { if (!disposed && model) render(); };
        atlas.src = window.AuraLazyAssets.versionedURL('/img/leafy/fallback.png');
        function sprite(index, x, y, size, angle = 0, aspect = 1) {
            if (!atlas.complete || !atlas.naturalWidth) return;
            ctx.save(); ctx.translate(x, y); ctx.rotate(angle);
            ctx.drawImage(atlas, index % 4 * 256, Math.floor(index / 4) * 256, 256, 256, -size * aspect / 2, -size * .86, size * aspect, size);
            ctx.restore();
        }
        function render() {
            if (disposed || !model) return;
            frames++; ctx.clearRect(0, 0, model.width, model.height);
            const ids = selection ? window.AuraLeafyGeometry.descendants(model, selection.branch, selection.node) : new Set();
            ctx.lineJoin = ctx.lineCap = 'round';
            for (const b of model.branches) {
                ctx.beginPath(); b.points.forEach((p,i) => i ? ctx.lineTo(p.x,model.height-p.y) : ctx.moveTo(p.x,model.height-p.y));
                ctx.strokeStyle = plant.dead ? '#736047' : '#557630'; ctx.lineWidth = 3.5 * model.scale; ctx.stroke();
                if (ids.has(b.id)) {
                    ctx.beginPath(); b.points.slice(b.id === selection.branch ? selection.node : 0).forEach((p,i) => i ? ctx.lineTo(p.x,model.height-p.y) : ctx.moveTo(p.x,model.height-p.y));
                    ctx.strokeStyle = '#ffbd74'; ctx.lineWidth = 6 * model.scale; ctx.stroke();
                }
            }
            ctx.strokeStyle=plant.dead?'#736047':'#557630';ctx.lineWidth=1.2*model.scale;
            for(const l of model.leaves){ctx.beginPath();ctx.moveTo(l.originX,model.height-l.originY);ctx.quadraticCurveTo((l.originX+l.x)/2,model.height-(l.originY+l.y)/2-2,l.x,model.height-l.y);ctx.stroke();}
            ctx.filter = plant.dead ? 'sepia(1) saturate(.6) brightness(.7)' : plant.vitality < 30 ? 'saturate(.55)' : 'brightness(.8)';
            for (const l of model.leaves) sprite(l.variant%4,l.x,model.height-l.y,l.size*1.43,-l.angle+(plant.moisture<20?.35:0),l.aspect);
            ctx.filter = 'none';
            for (const f of model.flowers) sprite(6+f.variant,f.x,model.height-f.y,f.size*3*f.open);
            sprite(light?5:4,model.root.x,model.height-model.root.y+145*model.scale,218*model.scale);
        }
        return {
            update(state,width,height,anchor,isLight) {
                plant=state; light=isLight; model=window.AuraLeafyGeometry.layout(state,width,height,anchor);
                const dpr=Math.min(devicePixelRatio||1,1.5,Math.sqrt(4000000/(width*height)));
                canvas.width=Math.ceil(width*dpr); canvas.height=Math.ceil(height*dpr); ctx.setTransform(dpr,0,0,dpr,0,0); render();
            },
            render, select(value) {selection=value;render();}, get layout() {return model;},
            metrics:()=>({frames,calls:0,triangles:0,textures:1}),
            dispose() {disposed=true;atlas.onload=null;canvas.width=canvas.height=1;}
        };
    }
    window.AuraLeafyFallback={create};
})();
