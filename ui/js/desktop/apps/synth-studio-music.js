(function(root,factory){const api=factory();if(typeof module==='object'&&module.exports)module.exports=api;else root.SynthStudioMusic=api;})(globalThis,function(){
    'use strict';
    const scales={major:[0,2,4,5,7,9,11],minor:[0,2,3,5,7,8,10]};
    const clamp=(v,a,b)=>Math.max(a,Math.min(b,v));
    function random(seed){
        let state=2166136261;for(const c of String(seed))state=Math.imul(state^c.charCodeAt(0),16777619);
        return ()=>{state+=0x6D2B79F5;let t=state;t=Math.imul(t^(t>>>15),t|1);t^=t+Math.imul(t^(t>>>7),t|61);return ((t^(t>>>14))>>>0)/4294967296;};
    }
    function inScale(p,h){return scales[h?.scale||'major'].includes(((p-(h?.root||0))%12+12)%12);}
    function fit(p,h){p=clamp(Math.round(p),0,127);for(let d=0;d<12;d++){if(p-d>=0&&inScale(p-d,h))return p-d;if(p+d<=127&&inScale(p+d,h))return p+d;}return p;}
    function chord(h,degree=0,seventh=false){
        const s=scales[h.scale],base=60+h.root;
        return Array.from({length:seventh?4:3},(_,i)=>{const n=degree+i*2;return base+s[n%7]+12*Math.floor(n/7);});
    }
    function arpPitches(pitches,settings){
        const values=[...new Set(pitches)].sort((a,b)=>a-b),out=[];
        for(let octave=0;octave<settings.octaves;octave++)for(const p of values)if(p+octave*12<=127)out.push(p+octave*12);
        if(settings.order==='down')out.reverse();
        if(settings.order==='pendulum'&&out.length>1)out.push(...out.slice(1,-1).reverse());
        return out;
    }
    function arpPitch(pitches,settings,index){
        const values=arpPitches(pitches,settings);if(!values.length)return null;
        // Per-step hashing keeps live playback identical to a printed pattern.
        const n=settings.order==='random'?Math.floor(random(settings.seed+':'+index)()*values.length):index%values.length;
        return values[n];
    }
    function note(pitch,start,duration,velocity,index){return {id:'generated-'+index,pitch,start,duration,velocity};}
    function arpeggio(pitches,length,settings){
        if(![8,16,32].includes(settings.division)||!Number.isInteger(length)||length<1||Math.ceil(length/(1920/settings.division))>50000)throw Error('E_PROJECT_LIMIT');
        const step=1920/settings.division,out=[];
        for(let at=0,i=0;at<length;at+=step,i++){
            const pitch=arpPitch(pitches,settings,i);if(pitch===null)break;
            out.push(note(pitch,at,Math.min(length-at,Math.max(1,Math.round(step*settings.gate))),.78,i));
        }return out;
    }
    function variation(notes,length,h,settings){
        const rng=random(settings.seed),out=[],density=clamp(settings.density,0,2);
        if(notes.length*(settings.lockRhythm?1:Math.ceil(density))>50000)throw Error('E_PROJECT_LIMIT');
        notes.forEach((n,index)=>{
            if(!settings.lockRhythm&&density<1&&rng()>density)return;
            const pitch=settings.lockPitch?n.pitch:fit(n.pitch+Math.round((rng()-.5)*10),h);
            const start=settings.lockRhythm?n.start:clamp(Math.round(n.start+(Math.floor(n.start/120)%2?settings.swing*120:0)+(rng()-.5)*settings.humanize*30),0,length-1);
            out.push({...n,pitch,start,duration:Math.min(n.duration,length-start),velocity:clamp(n.velocity+(rng()-.5)*settings.humanize*.2,.05,1)});
            if(!settings.lockRhythm&&density>1&&rng()<density-1&&start+120<length)out.push(note(pitch,start+120,Math.min(n.duration,length-start-120),n.velocity*.85,'extra-'+index));
        });return out.sort((a,b)=>a.start-b.start||a.pitch-b.pitch);
    }
    function rhythm(length,settings){
        if(![16,32].includes(settings.steps)||!Number.isInteger(length)||length<1||Math.ceil(length/(1920/settings.steps))>50000)throw Error('E_PROJECT_LIMIT');
        const steps=settings.steps,hits=clamp(settings.hits,0,steps),step=1920/steps,out=[];
        for(let at=0,i=0;at<length;at+=step,i++){
            const position=((i-settings.rotation)%steps+steps)%steps;
            if((position*hits)%steps<hits)out.push(note(settings.pitch,at,Math.min(length-at,Math.round(step*.65)),position%settings.accent===0?.95:.62,i));
        }return out;
    }
    return {scales,random,inScale,fit,chord,arpPitch,arpeggio,variation,rhythm};
});

