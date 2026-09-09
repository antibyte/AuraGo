/* Leafy generation v1. Shared by WebGL, Canvas 2D and picking. */
(function () {
    'use strict';
    const random = (seed, salt) => {
        let n = (seed ^ Math.imul(salt + 1, 0x9e3779b9)) >>> 0;
        n = Math.imul(n ^ (n >>> 16), 0x21f0aaad);
        n = Math.imul(n ^ (n >>> 15), 0x735a2d97);
        return ((n ^ (n >>> 15)) >>> 0) / 4294967296;
    };
    function layout(state, width, height, anchor) {
        const seed = state.seed ?? 731, scale = Math.min(1.1, Math.max(.62, height / 950));
        const root = {x: width * anchor.x, y: height * (1-anchor.y) + 151*scale};
        const sx = width / 1920, sy = height / 1080, branches = [], leaves = [], flowers = [], paths = new Map();
        for (const b of state.branches || []) {
            const parent = paths.get(b.parent), at = parent?.points[Math.min(b.attach, parent.points.length-1)];
            let x = at?.x || 0, y = at?.y || 0;
            let angle = parent ? (at.angle + (random(seed,b.id*67)>.5?1:-1)*(.6+random(seed,b.id*71)*1.3)) : [2.45,1.55,.65][b.id%3];
            const points = [{x,y,angle}];
            for(let i=1;i<b.nodes.length;i++){
                const noise = Math.sin(i*.57+b.id*1.7+seed*.001)*.12;
                angle += noise+(random(seed,b.id*613+i*19+b.nodes[i]*7)-.5)*.16;
                if(y<30 && Math.sin(angle)<0) angle += Math.cos(angle)>0?.24:-.24;
                if(y>720 && Math.sin(angle)>0) angle += Math.cos(angle)>0?-.22:.22;
                if(x < -740 && Math.cos(angle)<0) angle += Math.sin(angle)>0?-.26:.26;
                if(x > 1010 && Math.cos(angle)>0) angle += Math.sin(angle)>0?.26:-.26;
                const stride = 27 + random(seed,b.id*199+i)*10;
                x += Math.cos(angle)*stride; y += Math.sin(angle)*stride;
                points.push({x,y,angle});
            }
            const entry = {id:b.id, parent:b.parent, attach:b.attach, points};
            paths.set(b.id,entry);
            const screen = points.map((p,i)=>({x:root.x+p.x*sx,y:root.y+p.y*sy,z:30+Math.sin(i*.5+b.id)*5,angle:p.angle}));
            branches.push({...entry,points:screen,capped:b.capped});
            for(let i=1;i<points.length;i++){
                const r=random(seed,b.id*997+i*7), p=screen[i], maturity=Math.min(1,(state.age_hours-b.nodes[i]+3)/18);
                const direction=p.angle+(i%2?1:-1)*(.95+r*.72),petiole=(5+random(seed,b.id*659+i)*8)*scale;
                leaves.push({branch:b.id,node:i,originX:p.x,originY:p.y,originZ:p.z,
                    x:p.x+Math.cos(direction)*petiole,y:p.y+Math.sin(direction)*petiole,z:p.z+8+random(seed,b.id*463+i)*54,
                    angle:direction-Math.PI/2+(random(seed,b.id*373+i)-.5)*.18,
                    size:(56+random(seed,b.id*541+i)*54)*scale*(.35+.65*maturity),
                    variant:Math.floor(random(seed,b.id*761+i*13)*8),phase:r*6.28,
                    aspect:.88+random(seed,b.id*823+i)*.24,turn:(random(seed,b.id*617+i)-.5)*.65,
                    priority:random(seed,b.id*1729+i*43),tilt:(r-.5)*.42,tint:random(seed,b.id*887+i),maturity});
                const age=state.age_hours-b.nodes[i], cycle=(age-168)%216;
                if(age>=168 && cycle<120 && state.vitality>45 && state.nutrients>10 && !state.dead && r>.7 && flowers.length<80)
                    flowers.push({branch:b.id,node:i,x:p.x+12*scale,y:p.y+8*scale,z:110,size:scale*(12+r*8),open:Math.min(1,(cycle+2)/18)*Math.min(1,(120-cycle)/20),variant:i%2,phase:r*6.28});
            }
        }
        // Stable sampling spreads the foliage budget across every branch.
        leaves.sort((a,b)=>a.priority-b.priority);leaves.length=Math.min(leaves.length,960);
        return {branches,leaves,flowers,root,scale,width,height};
    }
    function descendants(layout, branch, node) {
        const ids=new Set([branch]);
        for(const b of layout.branches)if(ids.has(b.parent) && (b.parent!==branch || b.attach>=node))ids.add(b.id);
        return ids;
    }
    function pick(layout,x,y) {
        let best=null,distance=24;
        for(const b of layout.branches)for(let i=1;i<b.points.length;i++){
            const p=b.points[i],d=Math.hypot(p.x-x,p.y-y);
            if(d<distance){distance=d;best={branch:b.id,node:i};}
        }
        return best;
    }
    window.AuraLeafyGeometry={random,layout,descendants,pick};
})();
