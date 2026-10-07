window.fixtureErrors=[];
window.addEventListener('error',e=>fixtureErrors.push(e.message));
window.addEventListener('unhandledrejection',e=>fixtureErrors.push(String(e.reason)));
window.synthFiles=new Map();window.synthRevision=0;window.synthFailWrite=false;
const synthFetch=window.fetch.bind(window);
window.fetch=async(url,opts={})=>{
    const path=new URL(String(url),location.href),body=opts.body&&typeof opts.body==='string'?JSON.parse(opts.body):{};
    if(!path.pathname.startsWith('/api/'))return synthFetch(url,opts);
    if(path.pathname==='/api/desktop/bootstrap')return Response.json(synthTest.state.bootstrap);
    if(path.pathname==='/api/desktop/file'){
        const name=body.path||path.searchParams.get('path');
        if(opts.method==='PUT'){
            if(synthFailWrite)return new Response(JSON.stringify({error:'fixture failed write'}),{status:503});
            const result={path:name,content:body.content,version:'"synth-'+(++synthRevision)+'"'};synthFiles.set(name,result);return Response.json(result);
        }
        if(!synthFiles.has(name))return new Response('{}',{status:404});
        return Response.json(synthFiles.get(name));
    }
    if(path.pathname==='/api/desktop/files')return Response.json({entries:[],path:path.searchParams.get('path')||''});
    return Response.json({});
};
window.synthFixtureReady=(async()=>{
    const words=await(await synthFetch('/lang/desktop/en.json')).json();window.t=key=>words[key]||key;window.i18n={t:window.t};
    synthTest.state.bootstrap={enabled:true,readonly:false,builtin_apps:[{id:'synth-studio',name:'Synth Studio',icon:'synth-studio',category:'creative',metadata:{open_maximized:'true',logo_path:'/img/desktop-icons/synth-studio.svg'}}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'appearance.theme':'standard','windows.restore_session':false}};
    document.body.dataset.theme='standard';document.body.dataset.animations='false';document.getElementById('vd-disabled').hidden=true;
    await synthTest.loadIconManifest();synthTest.openApp('synth-studio');
})();
window.synthInstance=()=>[...SynthStudioApp.instances.values()][0];
