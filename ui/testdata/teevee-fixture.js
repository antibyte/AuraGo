/* Real shell and video decoder. Only catalog/bootstrap networking is local. */
window.fixtureErrors = [];
document.documentElement.lang = 'de';
window.addEventListener('error', e => fixtureErrors.push(e.message));
window.addEventListener('unhandledrejection', e => fixtureErrors.push(String(e.reason)));
window.fixtureRequests = [];
window.heldCatalog = [];
const nativeFetch = window.fetch.bind(window);
const firstNames = ['16 Anni e Incinta','3sat','90210','Adria Music Television','Adventure Earth','Adventure Earth','Adventure Earth'];
const categories = ['entertainment','general','series','music','documentary','documentary','documentary'];
window.fixtureChannels = Array.from({length:568}, (_,i) => ({
    id:'fixture-'+i, name:firstNames[i] || 'Z Test '+String(i).padStart(3,'0'),
    country:i===567?'US':'DE', languages:['de'], categories:[categories[i] || 'news']
}));
window.fixtureStreams = fixtureChannels.map((c,i) => ({channel:c.id,url:location.origin+'/testdata/teevee-test.mp4?station='+i,quality:i===1||i===5?'720p':i>=3?'1080p':''}));
const media=new URLSearchParams(location.search).get('media');
if(media==='hls') fixtureStreams[0].url=location.origin+'/testdata/teevee-test.m3u8';
if(media==='encrypted') fixtureStreams[0].url=location.origin+'/testdata/teevee-encrypted.m3u8';
if(media==='cors') fixtureStreams[0].url='__CROSS_ORIGIN__/testdata/teevee-test.mp4';
if(media==='nogl') {
    const getContext=HTMLCanvasElement.prototype.getContext;
    HTMLCanvasElement.prototype.getContext=function(type,...args){return type==='webgl'?null:getContext.call(this,type,...args);};
}
window.fetch = async (url,opts={}) => {
    const path=String(url);
    if(path.startsWith('https://iptv-org.github.io/api/')) {
        fixtureRequests.push(path);
        if(window.holdCatalog) await new Promise(resolve=>heldCatalog.push(resolve));
        if(window.rejectCatalog) return new Response('{}',{status:503});
        const data=path.endsWith('channels.json')?fixtureChannels:path.endsWith('streams.json')?fixtureStreams:
            [...new Set(categories.concat('news'))].map(id=>({id,name:id==='general'?'General':id==='series'?'Series':id==='music'?'Music':id==='documentary'?'Documentary':'Entertainment'}));
        return new Response(JSON.stringify(data));
    }
    if(path.startsWith('/api/')) return new Response('{}');
    return nativeFetch(url,opts);
};
window.fixtureReady=(async()=>{
    const words=await (await nativeFetch('/lang/desktop/de.json')).json();
    window.i18n={t:key=>words[key]||key,getLanguage:()=> 'de'};
    window.t=key=>words[key]||key;
    if(!new URLSearchParams(location.search).has('keep')) {
        localStorage.removeItem('aurago.teevee.favorites.v2');
        localStorage.removeItem('aurago.teevee.recent.v1');
        localStorage.removeItem('aurago.teevee.appearance.v1');
        localStorage.setItem('aurago.teevee.favorites.v1',JSON.stringify(fixtureStreams.slice(12,15).map(s=>s.url)));
    }
    teeveeTest.state.bootstrap={enabled:true,builtin_apps:[{id:'teevee',name:'TeeVee',icon:'teevee'},{id:'calculator',name:'Calculator',icon:'calculator'}],apps:[],widgets:[],shortcuts:[],desktop_files:[],settings:{'appearance.theme':'standard','windows.restore_session':false}};
    document.body.dataset.theme='standard';document.body.dataset.animations='false';
    document.getElementById('vd-disabled').hidden=true;
    await teeveeTest.loadIconManifest();
    teeveeTest.openApp('teevee');
    const win=document.querySelector('[data-app-id="teevee"]');
    const size=new URLSearchParams(location.search);
    Object.assign(win.style,{width:(size.get('width')||Math.min(1672,innerWidth))+'px',height:(size.get('height')||Math.min(941,innerHeight-64))+'px',left:'0px',top:'0px'});
    document.addEventListener('playing',e=>{if(e.target.tagName==='VIDEO')e.target.loop=true;},true);
})();
