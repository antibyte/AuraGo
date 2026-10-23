(function(root,factory){const api=factory(root);if(typeof module==='object'&&module.exports)module.exports=api;else root.SynthStudioPerformance=api;})(globalThis,function(root){
    'use strict';
    const BAR=1920,MAX_TICKS=64*BAR;
    const model=()=>root.SynthStudioModel;
    function valueAt(track,parameter,tick){let value=track.params[parameter];for(const p of track.automation||[])if(p.parameter===parameter&&p.tick<=tick)value=p.value;return value;}
    function reducePoints(points){
        const out=[];
        for(let i=0;i<points.length;i++){
            const point=points[i];
            const p={tick:Math.max(0,Math.round(point.tick)),parameter:point.parameter,value:Math.max(0,Math.min(1,Number(point.value)))};
            const last=out.at(-1);
            if(last&&last.parameter===p.parameter&&last.tick===p.tick){out[out.length-1]=p;continue;}
            if(last&&last.parameter===p.parameter&&Math.abs(last.value-p.value)<.004&&i<points.length-1)continue;
            out.push(p);
        }return out;
    }
    function mergeGesture(track,points){
        for(const parameter of ['tone','reverb']){
            const unique=new Map();for(const point of points)if(point.parameter===parameter)unique.set(Math.round(point.tick),point);
            const incoming=reducePoints([...unique.values()].sort((a,b)=>a.tick-b.tick));if(!incoming.length)continue;
            const from=incoming[0].tick,to=incoming.at(-1).tick;
            track.automation=track.automation.filter(p=>p.parameter!==parameter||p.tick<from||p.tick>to).concat(incoming);
        }
        track.automation.sort((a,b)=>a.tick-b.tick||a.parameter.localeCompare(b.parameter));
    }
    function sections(changes,end){
        const output=[];
        changes.forEach((change,index)=>{
            let at=change.at,start=change.start,stop=Math.min(end,changes[index+1]?.at??end);
            while(at<stop){const length=Math.min(change.end-start,stop-at);if(length<=0)throw Error('invalid_project');output.push({at,start,end:start+length,length,name:change.name});at+=length;start=change.start;}
        });return output;
    }
    function flatten(source,changes,end,gestures=[]){
        if(!Number.isInteger(end)||end<1||end>MAX_TICKS)throw Error('export_limit');
        const M=model(),project=M.upgrade(source);project.name=(source.name+' · '+(source.takeLabel||'Take')).slice(0,96);project.scenes=[];project.loop={enabled:false,start:0,end:Math.ceil(end/BAR)*BAR};
        let count=0,noteCount=0;const slices=sections(changes,end),prefix=M.newID('take');
        project.tracks.forEach((track,ti)=>{
            const original=source.tracks[ti],controllers=original.clips.flatMap(c=>c.controllers.map(p=>({...p,tick:c.start+p.tick}))).sort((a,b)=>a.tick-b.tick);
            const notes=original.clips.flatMap(c=>c.notes.map(n=>({...n,start:c.start+n.start})));
            const sustain=controllers.filter(c=>c.type==='sustain'),offs=sustain.filter(c=>c.value<.5);
            const after=(events,tick)=>{let lo=0,hi=events.length;while(lo<hi){const mid=(lo+hi)>>>1;if(events[mid].tick<=tick)lo=mid+1;else hi=mid;}return lo;};
            track.clips=[];track.automation=[];
            slices.forEach((part,si)=>{
                const clip={id:prefix+'-c-'+ti+'-'+si,name:(part.name||original.name).slice(0,96),start:part.at,length:part.length,notes:[],controllers:[]};
                for(const n of notes){
                    if(n.start<part.start||n.start>=part.end)continue;
                    let off=n.start+n.duration;
                    if(sustain[after(sustain,off)-1]?.value>=.5)off=offs[after(offs,off)]?.tick??part.end;
                    clip.notes.push({...n,id:prefix+'-n-'+ti+'-'+si+'-'+clip.notes.length,start:n.start-part.start,duration:Math.max(1,Math.min(off,part.end)-n.start)});
                    if(++noteCount>12000)throw Error('export_limit');
                }
                for(const type of ['bend','modulation']){
                    let value=0;for(const c of controllers)if(c.type===type&&c.tick<part.start)value=c.value;
                    clip.controllers.push({tick:0,type,value});
                }
                for(const c of controllers)if(c.type!=='sustain'&&c.tick>=part.start&&c.tick<part.end)clip.controllers.push({...c,tick:c.tick-part.start});
                for(const parameter of ['tone','reverb'])track.automation.push({tick:part.at,parameter,value:valueAt(original,parameter,part.start)});
                for(const point of original.automation||[])if(point.tick>part.start&&point.tick<part.end)track.automation.push({...point,tick:part.at+point.tick-part.start});
                count+=clip.notes.length+clip.controllers.length;if(count>50000)throw Error('E_PROJECT_LIMIT');
                track.clips.push(clip);
            });
            for(const gesture of gestures)if(gesture.id===track.id)mergeGesture(track,gesture.points.filter(p=>p.tick<end));
            const points=new Map();for(const point of track.automation)points.set(point.tick+':'+point.parameter,point);track.automation=[...points.values()];
            count+=track.automation.length;if(count>50000)throw Error('E_PROJECT_LIMIT');
        });
        return M.validate(project);
    }
    function createTake(source,range,checkRender){
        const project=model().upgrade(source);
        let changes=[{at:0,start:range.start,end:range.end,name:range.name}],gestures=[],limit=MAX_TICKS;
        const render=end=>flatten(project,changes,Math.max(1,Math.round(end)),gestures);
        function capacity(minimum=0){
            let low=Math.ceil(minimum/BAR),high=64,best=0;
            while(low<=high){const middle=Math.floor((low+high)/2);if(!middle){low=1;continue;}
                try{const p=render(middle*BAR);checkRender(p);best=middle;low=middle+1;}catch(_){high=middle-1;}
            }
            if(best*BAR<minimum)throw Error('export_limit');limit=best*BAR;return limit;
        }
        capacity();if(!limit)throw Error('export_limit');
        return {
            render, get limit(){return limit;},
            scene(range,at){
                const old=changes;changes=changes.filter(c=>c.at<at).concat({at:Math.round(at/BAR)*BAR,start:range.start,end:range.end,name:range.name});
                try{return capacity(at);}catch(error){changes=old;throw error;}
            },
            automation(id,points,elapsed){
                const safe=reducePoints(points);if(!safe.length)return limit;
                const old=gestures;gestures=[...gestures,{id,points:safe}];
                try{checkRender(render(elapsed));return capacity(elapsed);}catch(error){gestures=old;throw error;}
            },
            snapshot:()=>({changes:changes.map(c=>({...c})),gestures:gestures.map(g=>({id:g.id,points:g.points.map(p=>({...p}))}))})
        };
    }
    return {valueAt,reducePoints,mergeGesture,sections,flatten,createTake};
});
