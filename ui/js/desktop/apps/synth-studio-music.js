(function(root,factory){const api=factory();if(typeof module==='object'&&module.exports)module.exports=api;else root.SynthStudioMusic=api;})(globalThis,function(){
    'use strict';
    const scales={major:[0,2,4,5,7,9,11],minor:[0,2,3,5,7,8,10]};
    const clamp=(v,a,b)=>Math.max(a,Math.min(b,v));
    // Curated progressions stay degree based, so they follow the selected
    // key and major/minor scale instead of carrying fixed pitches.
    const PROGRESSIONS={
        pop:[0,4,5,3],
        ballad:[0,5,3,4],
        jazz:[1,4,0,3],
        minor:[0,5,2,6],
        epic:[0,3,4,0],
        canon:[0,4,5,2,3,0,3,4]
    };
    const BASS_STYLES=['root','octave','walk','pulse'];
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
        if(settings.order==='chord'||settings.order==='strum'){
            const values=arpPitches(pitches,settings),spread=settings.order==='strum'?Math.max(1,Math.round(step*.08)):0;
            for(let at=0,index=0;at<length;at+=step,index++){
                values.forEach((pitch,voice)=>{
                    if(out.length>=50000)throw Error('E_PROJECT_LIMIT');
                    const start=Math.min(length-1,at+voice*spread);
                    out.push(note(pitch,start,Math.min(length-start,Math.max(1,Math.round(step*settings.gate))),.78,index*values.length+voice));
                });
            }
            return out.sort((a,b)=>a.start-b.start||a.pitch-b.pitch);
        }
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
    function progressionDegrees(settings){
        return PROGRESSIONS[settings?.progression]||PROGRESSIONS.pop;
    }
    function bassPitch(h,degree){
        const scale=scales[h?.scale||'major'],n=((degree%7)+7)%7,octave=Math.floor(degree/7);
        return clamp(36+((h?.root||0)%12)+scale[n]+12*octave,0,127);
    }
    // Groove lab: one deterministic pass over the existing notes. The seed
    // makes humanisation repeatable, so a preview always matches its apply.
    function groove(notes,length,settings){
        if(!Array.isArray(notes)||!Number.isInteger(length)||length<1||notes.length>50000)throw Error('E_PROJECT_LIMIT');
        const rawGrid=Number(settings.groove_grid??settings.grid),grid=[8,16,32].includes(rawGrid)?rawGrid:16;
        const step=1920/grid,rng=random(settings.groove_seed||settings.seed||'1');
        const quantize=clamp(Number(settings.quantize)||0,0,1),swing=clamp(Number(settings.groove_swing??settings.swing)||0,0,.66);
        const humanize=clamp(Number(settings.groove_humanize??settings.humanize)||0,0,1),transpose=clamp(Math.round(Number(settings.groove_transpose??settings.transpose)||0),-24,24);
        const out=notes.map(n=>{
            let start=Number(n.start)||0;
            if(quantize)start+=(Math.round(start/step)*step-start)*quantize;
            if(swing&&Math.round(start/step)%2===1)start+=step*swing*.5;
            let velocity=Number(n.velocity)||.8;
            if(humanize){
                start+=(rng()-.5)*step*humanize*.45;
                velocity+= (rng()-.5)*.2*humanize;
            }
            start=clamp(Math.round(start),0,length-1);
            const duration=clamp(Math.round(Number(n.duration)||step),1,length-start);
            return {...n,pitch:clamp((Number(n.pitch)||60)+transpose,0,127),start,duration,velocity:clamp(velocity,.05,1)};
        });
        return out.sort((a,b)=>a.start-b.start||a.pitch-b.pitch);
    }
    function reverseNotes(notes,length){
        if(!Array.isArray(notes)||!Number.isInteger(length)||length<1||notes.length>50000)throw Error('E_PROJECT_LIMIT');
        const out=notes.map(n=>{
            const duration=clamp(Math.round(Number(n.duration)||1),1,length);
            const start=clamp(Math.round(length-((Number(n.start)||0)+duration)),0,length-1);
            return {...n,start,duration:Math.min(duration,length-start)};
        });
        return out.sort((a,b)=>a.start-b.start||a.pitch-b.pitch);
    }
    function progression(h,length,settings){
        if(!Number.isInteger(length)||length<1)throw Error('E_PROJECT_LIMIT');
        const degrees=progressionDegrees(settings),span=1920*clamp(Math.round(Number(settings.progression_rate)||2),1,8);
        const gate=clamp(Number(settings.song_gate)||.9,.05,1),seventh=!!settings.seventh,out=[];
        for(let at=0,index=0;at<length;at+=span,index++){
            const pitches=chord(h,degrees[index%degrees.length],seventh);
            const duration=Math.max(1,Math.min(length-at,Math.round(span*gate)));
            pitches.forEach((pitch,voice)=>{if(out.length>=50000)throw Error('E_PROJECT_LIMIT');out.push(note(pitch,at,duration,voice===0?.82:.72,index*4+voice));});
        }
        return out;
    }
    function bassline(h,length,settings){
        if(!Number.isInteger(length)||length<1)throw Error('E_PROJECT_LIMIT');
        const degrees=progressionDegrees(settings),span=1920*clamp(Math.round(Number(settings.progression_rate)||2),1,8);
        const gate=clamp(Number(settings.bass_gate??settings.song_gate)||.8,.05,1),style=BASS_STYLES.includes(settings.bass_style)?settings.bass_style:'root',out=[];
        const push=(pitch,start,duration,velocity)=>{
            if(out.length>=50000)throw Error('E_PROJECT_LIMIT');
            const at=clamp(Math.round(start),0,length-1);
            out.push({id:'bass-'+out.length,pitch:clamp(Math.round(pitch),0,127),start:at,duration:Math.max(1,Math.min(Math.round(duration),length-at)),velocity:clamp(velocity,.05,1)});
        };
        for(let at=0,index=0;at<length;at+=span,index++){
            const degree=degrees[index%degrees.length],next=degrees[(index+1)%degrees.length];
            const end=Math.min(length,at+span),root=bassPitch(h,degree),nextRoot=bassPitch(h,next);
            if(style==='octave'){
                for(let t=at,step=0;t<end;t+=240,step++)push(root+(step%2)*12,t,Math.min(240*gate,end-t),step%2?.62:.82);
            }else if(style==='pulse'){
                for(let t=at;t<end;t+=120)push(root,t,Math.min(120*gate,end-t),(t-at)%480===0?.88:.62);
            }else if(style==='walk'){
                const up=bassPitch(h,degree+2),fifth=clamp(root+7,0,127),approach=nextRoot>root?nextRoot-1:nextRoot+1;
                const pattern=[root,up,fifth,approach];
                for(let t=at,step=0;t<end;t+=480,step++)push(pattern[step%4],t,Math.min(480*gate,end-t),step%4===0?.86:.7);
            }else push(root,at,(end-at)*gate,.86);
        }
        return out.sort((a,b)=>a.start-b.start||a.pitch-b.pitch);
    }
    return {scales,random,inScale,fit,chord,arpPitch,arpPitches,arpeggio,variation,rhythm,groove,reverseNotes,progression,bassline,PROGRESSIONS,BASS_STYLES};
});
