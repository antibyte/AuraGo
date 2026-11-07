import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import http from 'node:http';
import { dirname, extname, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..'), ui = resolve(root, 'ui');
const browser = [process.env.CHROME_BIN, process.env.PROGRAMFILES && resolve(process.env.PROGRAMFILES, 'Google/Chrome/Application/chrome.exe'),
    process.env['PROGRAMFILES(X86)'] && resolve(process.env['PROGRAMFILES(X86)'], 'Microsoft/Edge/Application/msedge.exe'),
    '/usr/bin/google-chrome', '/usr/bin/chromium', '/usr/bin/chromium-browser'].filter(Boolean).find(existsSync);
assert.ok(browser, 'Chrome/Edge is required for the real DOM/save-boundary regressions');
// The renderer is mocked below, so save.js and renderer.js stay out of the page.
const names = ['content', 'content-act1', 'content-act2', 'content-act3', 'content-act4', 'state', 'commands', 'interactions', 'speech', 'panel', 'app'];
const labels = JSON.parse(readFileSync(resolve(ui, 'lang/desktop/de.json'), 'utf8'));
const interfaceText = JSON.parse(readFileSync(resolve(ui, 'lang/goldfish-interface/de.json'), 'utf8'));
const fixture = `
window.labels = ${JSON.stringify(labels)};
window.ui = ${JSON.stringify(interfaceText)};
window.fixtureErrors=[];window.addEventListener('error',event=>fixtureErrors.push(event.message));
window.until = async (predicate,message='',ms=1500) => { for (let i=0;i<ms/10;i++) { if(predicate()) return; await new Promise(r=>setTimeout(r,10)); } throw new Error('Fixture wait expired: '+message+' '+document.body.innerText); };
// Reading preferences live in localStorage. Tests run with manual advance so no line is ever read by a timer by accident.
window.speechPrefs = {autoAdvance:false,readSpeed:'normal'};
window.talking = () => !document.querySelector('.agfish-speech').hidden;
window.lineText = () => document.querySelector('.agfish-line').textContent;
window.press = (key,code=key) => document.querySelector('.agfish-app').dispatchEvent(new KeyboardEvent('keydown',{key,code,bubbles:true}));
// Esc skips the whole speech queue without marking unread lines; Space skips (and reads) the current line.
window.escape = async () => { if(!talking())return; press('Escape'); await until(()=>!talking(),'Escape must end the speech queue'); };
window.skipLine = () => press(' ','Space');
window.menu = name => { if(!document.querySelector('dialog .agfish-menu'))document.querySelector('.agfish-menu-button').click(); return [...document.querySelectorAll('dialog .agfish-menu button')].find(value=>value.textContent===name); };
// A hotspot click walks first, then performs; several actions or a puzzle open a list in the bar.
window.clickAction = async id => {
    if(talking())throw new Error('clickAction needs a quiet scene; dismiss the speech first');
    const record=GoldfishContent.actions.find(value=>value.id===id);
    document.querySelector('[data-agfish-hotspot="'+record.hotspot+'"]').click();
    await until(()=>document.querySelector('[data-agfish-action="'+id+'"]')||document.querySelector('.agfish-busy')||document.querySelector('dialog')||document.querySelector('.agfish-choice-line')||talking(),'hotspot click for '+id+' did nothing');
    const line=document.querySelector('[data-agfish-action="'+id+'"]');if(line)line.click();
};
window.pickOption = id => document.querySelector('[data-agfish-option="'+id+'"]').click();
window.options = () => [...document.querySelectorAll('[data-agfish-option]')].map(value=>value.dataset.agfishOption);
window.fastTimers = on => { // Speech timers (1.6 s and longer) fire after 5 ms; the requested delays are recorded.
    if(!on){window.setTimeout=window.realSetTimeout;return;}
    window.realSetTimeout=window.setTimeout;window.timerRequests=[];
    window.setTimeout=(callback,ms,...rest)=>{if(ms>=1500){timerRequests.push(ms);ms=5;}return realSetTimeout(callback,ms,...rest);};
};
window.testSlots = ['manual-1','manual-2','manual-3','auto','chapter-1','chapter-2','chapter-3','chapter-4','branch-customs','branch-garden','branch-hall','prefinal'];
window.GoldfishRenderer = {create:(canvas,options)=>{
    const data=storageFixture,finish=()=>{data.moments.splice(0).forEach(resolve=>resolve(false));options.onMoment?.(false);};
    return {load:async room=>{window.lastRenderedRoom=room;options.onStatus('ready');return true;},update(){},setPaused(){},cancelMotion:finish,speak(){},unlock(){},dispose:finish,
        skipMoment:()=>{if(!data.moments.length)return false;data.moments.splice(0).forEach(resolve=>resolve(true));options.onMoment?.(false);return true;},walk:async()=>true,anchor(){return null;},
        act:async()=>{if(!data.holdMoment)return true;options.onMoment?.(true);return new Promise(resolve=>data.moments.push(resolve));}};
}};
window.startFixture = async (state=null,seed=[]) => {
    window.GoldfishAdventureApp.dispose('fixture');
    localStorage.setItem('aurago.goldfish.speech',JSON.stringify(speechPrefs));
    const clone=value=>JSON.parse(JSON.stringify(value));
    const data=window.storageFixture={records:new Map(seed.map(value=>[value.id,clone(value)])),calls:[],pending:[],hold:false,failAuto:false,failSlots:new Set(),active:0,maxActive:0,holdMoment:false,moments:[]};
    if(state) data.records.set('auto',{id:'auto',revision:1,current:{state:clone(state),label:'',updatedAt:1},prior:null});
    const store={list:async()=>testSlots.map(id=>{const value=data.records.get(id);return value?{id,revision:value.revision,empty:false,valid:true,room:value.current.state.room,updatedAt:1,hasPrior:!!value.prior}:{id,revision:0,empty:true};}),
        read:async(id,{prior=false}={})=>{const value=data.records.get(id);if(!value)return null;const selected=prior?value.prior:value.current;return selected?{id,revision:value.revision,state:clone(selected.state),prior}:null;},
        write:(id,state,{expectedRevision})=>new Promise((resolve,reject)=>{
            data.calls.push({id,state:clone(state),expectedRevision});data.active++;data.maxActive=Math.max(data.maxActive,data.active);
            const finish=error=>{data.active--;if(error){reject(Object.assign(new Error(error),{code:error}));return;}const old=data.records.get(id);if((old?.revision||0)!==expectedRevision){reject(Object.assign(new Error('conflict'),{code:'conflict'}));return;}const value={id,revision:expectedRevision+1,current:{state:clone(state),label:'',updatedAt:1},prior:old?.current||null};data.records.set(id,value);resolve({id,revision:value.revision});};
            if(data.hold)data.pending.push(finish);else finish((data.failAuto&&id==='auto')||data.failSlots.has(id)?'storage':null);
        }),close(){}};
    window.GoldfishSave={SLOTS:testSlots,open:async()=>store};
    window.remountFixture=async()=>{
        GoldfishAdventureApp.dispose('fixture');
        GoldfishAdventureApp.render(document.getElementById('host'),'fixture',{lang:'de',t:key=>labels[key]||key,isActive:()=>true,setWindowBeforeClose:(id,handler)=>window.closeGuard=handler});
        await until(()=>document.querySelector('.agfish-start')&&(!data.records.has('auto')||!document.querySelector('.agfish-start .agfish-primary').hidden));
        await until(()=>document.querySelector('.agfish-art-status').hidden);
    };
    await remountFixture();
};
window.enterFixture = async state => {await startFixture(state);[...document.querySelectorAll('.agfish-start button')].find(value=>value.textContent===(state?'Reise fortsetzen':'Neue Reise')).click();await until(()=>document.querySelector('.agfish-start').hidden&&!document.querySelector('dialog')&&talking());await escape();};
window.loadSlot=async(id,prior=false)=>{
    menu('Spielstände').click();await until(()=>document.querySelectorAll('.agfish-slot').length===12);
    const row=[...document.querySelectorAll('.agfish-slot')][testSlots.indexOf(id)];
    const label=prior?'Vorherige Version laden':id.startsWith('branch-')||id.startsWith('chapter-')||id==='prefinal'?'Von hier neu reisen':'Laden';
    [...row.querySelectorAll('button')].find(value=>value.textContent===label).click();await until(()=>document.querySelectorAll('dialog').length===2);
    [...document.querySelectorAll('dialog')].at(-1).querySelector('.agfish-primary').click();
    await until(()=>!document.querySelector('dialog')&&talking());await escape();
};
window.pathTo = (state,to) => {const queue=[[state.room]],seen=new Set([state.room]);while(queue.length){const route=queue.shift(),id=route.at(-1);if(id===to)return route.slice(1);for(const exit of GoldfishContent.rooms.find(value=>value.id===id).exits){if(!seen.has(exit.to)&&GoldfishState.conditions(state,exit)){seen.add(exit.to);queue.push(route.concat(exit.to));}}}return null;};
window.stateAt = to => {let state=GoldfishState.create();for(let pass=0;pass<150;pass++){let route=pathTo(state,to);if(route!==null){for(const room of route)state=GoldfishState.move(state,room).state;state.seenLines=GoldfishState.pendingResults(state).map(value=>value.text);return state;}let changed=false;for(const record of GoldfishContent.actions){if(record.repeatable||state.applied.includes(record.id)||['r07_maintenance','r09_new','r12_diagnosis'].includes(record.id)||record.effects?.ending)continue;route=pathTo(state,record.room);if(route===null)continue;const candidate={...state,room:record.room};if(!GoldfishState.available(candidate,record))continue;for(const room of route)state=GoldfishState.move(state,room).state;state=GoldfishState.apply(state,record.id,{item:record.item,answer:record.challenge?.answer}).state;changed=true;if(pathTo(state,to)!==null)break;}if(!changed)throw new Error('Unreachable fixture '+to);}throw new Error('Fixture loop');};
`;
const html = '<!doctype html><html lang="de"><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-app-goldfish-adventure.css"><style>body{margin:0}#host{width:1100px;height:760px}</style><div class="vd-window vd-window-active"><div id="host"></div></div>' +
    names.map(name => '<script src="/js/desktop/apps/goldfish-adventure/' + name + '.js"></script>').join('') + '<script>' + fixture + '</script>';
const server = http.createServer((request, response) => {
    try {
        const path = new URL(request.url, 'http://localhost').pathname;
        if (path === '/') { response.setHeader('Content-Type', 'text/html; charset=utf-8'); response.end(html); return; }
        const file = resolve(ui, '.' + decodeURIComponent(path));
        if (!file.startsWith(ui + sep)) throw new Error('path');
        response.setHeader('Content-Type', { '.json': 'application/json', '.js': 'text/javascript', '.css': 'text/css', '.png': 'image/png' }[extname(file)] || 'application/octet-stream');
        response.end(readFileSync(file));
    } catch (_) { response.statusCode = 404; response.end(); }
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const scratch = resolve(root, 'disposable/_goldfish-app'); mkdirSync(scratch, { recursive: true });
const profile = mkdtempSync(resolve(scratch, 'profile-'));
const child = spawn(browser, ['--headless=new', '--no-sandbox', '--no-first-run', '--disable-gpu', '--remote-debugging-port=0', '--user-data-dir=' + profile, 'about:blank'],
    { windowsHide: true, stdio: ['ignore', 'ignore', 'pipe'] });
let socket;
try {
    const debug = await new Promise((resolve, reject) => {
        let text = ''; const timer = setTimeout(() => reject(new Error('Browser startup timed out')), 20000);
        child.stderr.on('data', value => { text += value; const match = text.match(/DevTools listening on (ws:\/\/\S+)/); if (match) { clearTimeout(timer); resolve(match[1]); } });
        child.once('error', reject);
    });
    const target = await fetch('http://' + new URL(debug).host + '/json/new?about:blank', { method: 'PUT' }).then(value => value.json());
    socket = new WebSocket(target.webSocketDebuggerUrl);
    await new Promise((resolve, reject) => { socket.addEventListener('open', resolve, { once: true }); socket.addEventListener('error', reject, { once: true }); });
    let id = 0; const pending = new Map(), listeners = new Map(), errors = [];
    socket.addEventListener('message', event => {
        const message = JSON.parse(event.data);
        if (message.method === 'Runtime.exceptionThrown') errors.push(message.params);
        if (message.method) listeners.get(message.method)?.(message.params);
        const request = pending.get(message.id); if (!request) return;
        pending.delete(message.id); message.error ? request.reject(new Error(JSON.stringify(message.error))) : request.resolve(message.result);
    });
    const call = (method, params = {}) => new Promise((resolve, reject) => { const next = ++id; pending.set(next, { resolve, reject }); socket.send(JSON.stringify({ id: next, method, params })); });
    const evaluate = async expression => {
        const value = await call('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
        assert.equal(value.exceptionDetails, undefined, JSON.stringify(value.exceptionDetails)); return value.result.value;
    };
    await call('Runtime.enable'); await call('Page.enable');
    const loaded = new Promise(resolve => listeners.set('Page.loadEventFired', resolve));
    await call('Page.navigate', { url: 'http://127.0.0.1:' + server.address().port }); await loaded;
    const result = await evaluate(`(async()=>{
        const check=(value,message)=>{if(!value)throw new Error(message);};
        await enterFixture();
        storageFixture.failAuto=true;await clickAction('r01_stop');await until(()=>document.querySelector('dialog'));
        document.querySelector('dialog .agfish-close').click();await until(()=>!document.querySelector('.agfish-busy'));
        check(!talking(),'A declined action shows no result');
        await clickAction('r01_stop');await until(()=>document.querySelector('dialog'),'Declined failed action must restore prior rewards/actions: the action is offered and attempted again');
        document.querySelector('dialog .agfish-close').click();await until(()=>!document.querySelector('.agfish-busy'));
        document.querySelector('[data-agfish-hotspot="grabber"]').click();await until(()=>talking(),'The grabber falls back to its look lines');
        check(lineText()===GoldfishContent.texts['goldfish.R01.grabber.look.1'],'A declined action must not leak its flag: the clamp stays unavailable');
        await escape();
        check(!storageFixture.records.has('auto'),'Failed boundary cannot create an autosave');
        await enterFixture(stateAt('R02'));storageFixture.failAuto=true;
        document.querySelector('[data-agfish-exit="R01"]').click();await until(()=>document.querySelector('dialog'));
        document.querySelector('dialog').close();await until(()=>!document.querySelector('.agfish-busy'));
        check(document.querySelector('.agfish-app').dataset.room==='R02'&&lastRenderedRoom==='R02','Declined failed travel restores room and artwork together');

        const old=stateAt('R02');await startFixture(null,[{id:'chapter-1',revision:7,current:{state:old,label:'',updatedAt:1},prior:null}]);
        [...document.querySelectorAll('.agfish-start button')].find(value=>value.textContent==='Neue Reise').click();await until(()=>storageFixture.records.get('chapter-1')?.revision===8);
        check(storageFixture.records.get('chapter-1').prior.state.flags.includes('p01_done'),'New-run chapter checkpoint retains prior valid run');
        const branch=stateAt('R07'),older=JSON.parse(JSON.stringify(branch));branch.settings.sound=true;
        await startFixture(branch,[{id:'branch-customs',revision:3,current:{state:older,label:'',updatedAt:1},prior:null}]);
        [...document.querySelectorAll('.agfish-start button')].find(value=>value.textContent==='Reise fortsetzen').click();await escape();await clickAction('r07_public');
        await until(()=>!document.querySelector('.agfish-busy')&&storageFixture.records.get('branch-customs')?.revision===4);
        const named=storageFixture.records.get('branch-customs');
        check(named.current.state.settings.sound&&!named.prior.state.settings.sound,'New branch commitment refreshes current and preserves previous source');
        check(!named.current.state.flags.includes('r07_public'),'Branch checkpoint is before the new choice');
        for(const [room,puzzle]of[['R07','P06'],['R09','P10'],['R12','P14']]){
            await enterFixture(stateAt(room));menu('Ein kleiner Hinweis').click();
            const select=document.querySelector('.agfish-hint-picker select');check(!!select,'Branch hints require a puzzle picker');
            check([...select.options].some(value=>value.value===puzzle),'Alternative puzzle must be selectable: '+puzzle);
            select.value=puzzle;select.dispatchEvent(new Event('change',{bubbles:true}));
            const spec=GoldfishContent.puzzles.find(value=>value.id===puzzle);
            for(let level=0;level<3;level++){
                check(document.querySelector('.agfish-hint').textContent===GoldfishContent.texts[spec.hints[level]],'Every alternative has all three hints');
                if(level<2)document.querySelector('.agfish-hint-content button').click();
            }
            document.querySelector('dialog').close();
        }

        await enterFixture();menu('Einstellungen').click();
        const input=document.querySelector('dialog input');input.checked=true;input.dispatchEvent(new Event('change',{bubbles:true}));
        storageFixture.hold=true;document.querySelector('dialog').close();await until(()=>storageFixture.pending.length===1);
        await clickAction('r01_stop');await until(()=>document.querySelector('.agfish-busy'));
        check(storageFixture.calls.filter(value=>value.id==='auto').length===1,'Action save waits for older settings write');
        storageFixture.pending.shift()();await until(()=>storageFixture.pending.length===1);
        check(document.querySelector('.agfish-save-status').textContent!=='Reise gespeichert','An earlier acknowledgement cannot mark later state saved');
        storageFixture.pending.shift()('storage');await until(()=>document.querySelector('dialog'));
        window.guardSettled=false;window.guardResult=null;closeGuard().then(value=>{guardSettled=true;guardResult=value;});
        await until(()=>document.querySelectorAll('dialog').length===2||guardSettled);
        check(!guardSettled,'Failed latest progress must not silently pass the close guard');
        [...document.querySelectorAll('dialog')].at(-1).close();await until(()=>guardSettled);
        check(guardResult===false,'Cancelling the close guard keeps the app open');
        document.querySelector('dialog').close();await until(()=>!document.querySelector('.agfish-busy'));
        check(storageFixture.maxActive===1,'App serializes snapshots before reaching IndexedDB');
        await enterFixture();storageFixture.hold=true;menu('Spielstände').click();
        await until(()=>document.querySelectorAll('.agfish-slot').length===12);
        const manual=[...document.querySelectorAll('.agfish-slot button')].filter(value=>value.textContent==='Hier speichern');
        manual[0].click();manual[1].click();await until(()=>storageFixture.pending.length===1);
        check(storageFixture.calls.filter(value=>value.id.startsWith('manual-')).length===1,'Concurrent manual clicks are serialized');
        storageFixture.pending.shift()();await until(()=>storageFixture.pending.length===1);
        storageFixture.pending.shift()();await until(()=>storageFixture.records.has('manual-1')&&storageFixture.records.has('manual-2'));
        check(storageFixture.maxActive===1,'Manual writes share the same snapshot queue');

        await enterFixture();storageFixture.holdMoment=true;await clickAction('r01_stop');
        await until(()=>storageFixture.moments.length===1&&storageFixture.records.has('auto'));
        const resultLines=GoldfishContent.actions.find(value=>value.id==='r01_stop').lines;
        check(!storageFixture.records.get('auto').current.state.seenLines.includes(resultLines[0].text),'An unpresented result is never acknowledged');
        check(await closeGuard()===true,'A committed scene can safely close before its result');
        await remountFixture();storageFixture.holdMoment=false;
        [...document.querySelectorAll('.agfish-start button')].find(value=>value.textContent==='Reise fortsetzen').click();
        check(lineText()===GoldfishContent.texts[resultLines[0].text],'Reopen resumes result text, not just the room entry');
        check(storageFixture.records.get('auto').current.state.applied.filter(value=>value==='r01_stop').length===1,'Resume never reapplies the reward');
        check(!storageFixture.records.get('auto').current.state.seenLines.includes(resultLines[0].text),'Displaying alone cannot acknowledge a not-yet-readable line');
        skipLine();
        await until(()=>storageFixture.records.get('auto').current.state.seenLines.includes(resultLines[0].text));
        check(await closeGuard()===true,'Explicit line acknowledgements are durable before closing');
        await remountFixture();[...document.querySelectorAll('.agfish-start button')].find(value=>value.textContent==='Reise fortsetzen').click();
        check(lineText()===GoldfishContent.texts[resultLines[1].text],'Partial result resumes its next unread line');
        press('Escape');
        check(!talking(),'Escape dismisses the current result without acknowledging it');
        await remountFixture();[...document.querySelectorAll('.agfish-start button')].find(value=>value.textContent==='Reise fortsetzen').click();
        check(lineText()===GoldfishContent.texts[resultLines[1].text],'Dismissed results remain recoverable');

        await enterFixture();await clickAction('r01_stop');await until(()=>!document.querySelector('.agfish-busy')&&talking());
        storageFixture.hold=true;skipLine();await until(()=>storageFixture.pending.length===1);await escape();
        await clickAction('r01_clamp');await until(()=>document.querySelector('.agfish-busy'));
        storageFixture.pending.shift()();await until(()=>storageFixture.pending.length===1);
        check(document.querySelector('.agfish-save-status').textContent!=='Reise gespeichert','A line acknowledgement cannot clear a later action');
        storageFixture.pending.shift()();await until(()=>!document.querySelector('.agfish-busy'));
        check(storageFixture.records.get('auto').current.state.applied.includes('r01_clamp'),'Later action and earlier acknowledgement both survive');

        for(const source of ['auto','manual-1','auto-prior']){
            const clean=GoldfishState.create();
            await enterFixture(clean);
            const remote=GoldfishState.apply(clean,'r01_stop').state;
            storageFixture.records.set('auto',{id:'auto',revision:2,current:{state:remote,label:'',updatedAt:1},prior:{state:clean,label:'',updatedAt:1}});
            storageFixture.records.set('manual-1',{id:'manual-1',revision:1,current:{state:clean,label:'',updatedAt:1},prior:null});
            await clickAction('r01_stop');await until(()=>document.querySelector('dialog'));
            document.querySelector('dialog').close();await until(()=>!document.querySelector('.agfish-busy'));
            const attempts=storageFixture.calls.filter(value=>value.id==='auto').length;
            await loadSlot(source==='auto-prior'?'auto':source,source==='auto-prior');
            if(source==='auto'){
                await clickAction('r01_clamp');await until(()=>!document.querySelector('.agfish-busy')&&!document.querySelector('dialog'));
                const write=storageFixture.calls.filter(value=>value.id==='auto').at(-1);
                check(write.expectedRevision===2&&storageFixture.records.get('auto').revision===3,'Current auto reload rebases its precise read revision and re-enables autosave');
            }else{
                await clickAction('r01_stop');await until(()=>document.querySelector('dialog'));
                check(storageFixture.calls.filter(value=>value.id==='auto').length===attempts,'Other/prior loads cannot silently resolve an auto conflict');
                document.querySelector('dialog').close();await until(()=>!document.querySelector('.agfish-busy'));
            }
        }

        const beforeChoice=stateAt('R07');
        await enterFixture(beforeChoice);
        storageFixture.records.set('branch-customs',{id:'branch-customs',revision:4,current:{state:beforeChoice,label:'',updatedAt:1},prior:null});
        await clickAction('r07_public');await until(()=>document.querySelector('dialog'));
        document.querySelector('dialog').close();await until(()=>!document.querySelector('.agfish-busy'));
        await loadSlot('branch-customs');await clickAction('r07_public');
        await until(()=>!document.querySelector('.agfish-busy')&&storageFixture.records.get('branch-customs').revision===5);
        check(storageFixture.calls.filter(value=>value.id==='branch-customs').at(-1).expectedRevision===4,'Explicit checkpoint reload recovers that slot CAS revision');

        await startFixture();menu('Spielstände').click();await until(()=>document.querySelectorAll('.agfish-slot').length===12);
        [...document.querySelectorAll('dialog button')].find(value=>value.textContent==='Spielstand importieren').click();
        await until(()=>document.querySelector('input[type=file]'));
        const transfer=new DataTransfer(),imported=GoldfishState.apply(GoldfishState.create(),'r01_stop').state;
        transfer.items.add(new File([JSON.stringify(imported)],'legacy.json',{type:'application/json'}));
        const file=document.querySelector('input[type=file]');file.files=transfer.files;file.dispatchEvent(new Event('change'));
        await until(()=>document.querySelector('dialog .agfish-primary'));document.querySelector('dialog .agfish-primary').click();
        await until(()=>!document.querySelector('dialog')&&talking());
        check(lineText()===GoldfishContent.texts[resultLines[0].text],'Schema-one import resumes unread results without new persisted fields');

        const previousRun=stateAt('R02');await startFixture(previousRun);
        const oldAuto=JSON.stringify(storageFixture.records.get('auto'));
        storageFixture.failSlots.add('chapter-1');
        [...document.querySelectorAll('.agfish-start button')].find(value=>value.textContent==='Neue Reise').click();await until(()=>document.querySelector('dialog'));
        document.querySelector('dialog .agfish-close').click();
        await until(()=>!document.querySelector('.agfish-start').hidden&&!document.querySelector('.agfish-busy'));
        check(document.querySelector('.agfish-app').dataset.room==='R02'&&lastRenderedRoom==='R02','Declined new game restores its previous room and title screen');
        check(JSON.stringify(storageFixture.records.get('auto'))===oldAuto,'Declined new game preserves the previous autosave');
        check(await closeGuard()===true,'Declined new game restores prior dirty/started state');

        // ---- SCUMM-style controls: speech queues, verbs, items, choice lists and puzzle menus -----------------
        const sentence=()=>document.querySelector('.agfish-sentence').textContent;
        const bar=()=>document.querySelector('.agfish-panel');
        check(GoldfishSpeech.duration('x','normal')===1600&&GoldfishSpeech.duration('x'.repeat(100),'slow')===900+100*70&&GoldfishSpeech.duration('x'.repeat(100),'normal')===900+100*48&&GoldfishSpeech.duration('x'.repeat(100),'fast')===900+100*32&&GoldfishSpeech.duration('x'.repeat(9999),'slow')===9000,'Line duration is clamp(900 + chars x rate, 1600, 9000)');

        await startFixture();[...document.querySelectorAll('.agfish-start button')].find(value=>value.textContent==='Neue Reise').click();await until(()=>document.querySelector('.agfish-start').hidden&&talking());
        const firstLine=lineText();
        check(bar().classList.contains('agfish-dim'),'A story queue dims the bar');
        document.querySelector('[data-agfish-hotspot="conveyor"]').click();
        check(lineText()!==firstLine&&talking()&&!document.querySelector('.agfish-busy'),'During a story queue a hotspot click only skips the current line');
        await escape();
        check(!bar().classList.contains('agfish-dim'),'Skipping the queue frees the bar');

        const scrap=document.querySelector('[data-agfish-hotspot="scrap"]');
        scrap.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}));await until(()=>talking(),'Right click looks at the hotspot');
        check(lineText()===GoldfishContent.texts['goldfish.R01.scrap.look.1']&&document.querySelector('.agfish-speech').dataset.speaker==='narrator'&&!bar().classList.contains('agfish-dim'),'Right click speaks the look lines as an ambient queue');
        scrap.click();
        check(!talking(),'Any click cancels ambient speech at once');
        await until(()=>document.querySelectorAll('[data-agfish-action]').length===2,'Two actions open a choice list');
        check([...document.querySelectorAll('[data-agfish-action]')].map(value=>value.dataset.agfishAction).join()==='r01_colleague,r01_horizon','The choice list offers both actions of the hotspot');
        press('Escape');
        check(!document.querySelector('[data-agfish-action]')&&document.querySelector('.agfish-verb').offsetParent,'Escape closes the choice list and shows the verbs again');

        const take=document.querySelector('[data-agfish-verb="take"]');
        take.click();
        check(take.getAttribute('aria-pressed')==='true'&&sentence()===ui['goldfish.ui.verb_take']+' …','A verb arms and the sentence line shows it');
        document.querySelector('[data-agfish-hotspot="conveyor"]').click();await until(()=>talking(),'A refused verb is answered');
        check(lineText()===ui['goldfish.ui.refuse_take_1']&&document.querySelector('.agfish-speech').dataset.speaker==='r0st'&&take.getAttribute('aria-pressed')==='false','A verb without a matching action gets an R0ST refusal and resets');
        take.click();document.querySelector('[data-agfish-hotspot="conveyor"]').click();await until(()=>talking()&&lineText()===ui['goldfish.ui.refuse_take_2'],'Refusals alternate');
        await escape();

        const armedState=GoldfishState.apply(GoldfishState.apply(GoldfishState.create(),'r01_stop').state,'r01_clamp').state;
        armedState.seenLines=GoldfishState.pendingResults(armedState).map(value=>value.text);
        await enterFixture(armedState);
        const clamp=document.querySelector('[data-agfish-item="tool_clamp"]');check(!!clamp,'A taken item sits in the inventory grid');
        clamp.click();
        check(clamp.getAttribute('aria-pressed')==='true'&&sentence()===ui['goldfish.ui.sentence_use_item_open'].replace('{item}',clamp.getAttribute('aria-label')),'Clicking an item arms it: "Benutze ... mit ..."');
        press('Escape');
        check(clamp.getAttribute('aria-pressed')==='false'&&!document.querySelector('dialog'),'Escape disarms an item instead of opening the menu');
        clamp.click();document.querySelector('[data-agfish-hotspot="conveyor"]').click();await until(()=>talking(),'A wrong item is refused');
        check(lineText()===ui['goldfish.ui.refuse_item_1']&&clamp.getAttribute('aria-pressed')==='false','An item that fits nothing gets an R0ST refusal');
        await escape();

        document.querySelector('[data-agfish-hotspot="magnet"]').click();await until(()=>options().length===3,'The puzzle opens as a menu in the bar');
        check(options().join()==='release,hold,stop'&&!document.querySelector('dialog'),'Sequence steps are listed reversed in the bar, not in a dialog');
        for(const step of ['hold','stop','release'])pickOption(step);
        await until(()=>talking()&&options().length===3,'A wrong sequence is answered and the menu comes back');
        check(lineText()===GoldfishContent.texts['goldfish.r01_brake.wrong.1']&&!storageFixture.records.get('auto').current.state.flags.includes('r01_bolt'),'A wrong sequence speaks the wrong lines and changes nothing');
        pickOption('stop');
        check(options().join()==='release,hold'&&sentence()===GoldfishContent.texts['goldfish.r01_brake.option.stop']+' → …','A picked step leaves the list and shows in the sentence line');
        document.querySelector('[data-agfish-back]').click();
        check(options().join()==='release,hold,stop','The undo line takes one step back');
        for(const step of ['stop','hold','release'])pickOption(step);
        await until(()=>storageFixture.records.get('auto').current.state.flags.includes('r01_bolt'),'The last step checks automatically');
        await until(()=>talking()&&!document.querySelector('.agfish-busy'));await escape();

        press('Escape');check(!!document.querySelector('dialog .agfish-menu'),'Escape with nothing active opens the game menu');
        document.querySelector('dialog').close();
        menu('Einstellungen').click();
        const toggles=[...document.querySelectorAll('dialog .agfish-setting')];
        const reading=toggles.find(value=>value.textContent.includes(ui['goldfish.ui.auto_advance'])).querySelector('input');
        reading.checked=true;reading.dispatchEvent(new Event('change',{bubbles:true}));
        const speed=document.querySelector('dialog select');speed.value='fast';speed.dispatchEvent(new Event('change',{bubbles:true}));
        const stored=JSON.parse(localStorage.getItem('aurago.goldfish.speech'));
        check(stored.autoAdvance===true&&stored.readSpeed==='fast','Reading preferences are kept in localStorage');
        document.querySelector('dialog').close();await until(()=>!document.querySelector('.agfish-busy'));
        check(!JSON.stringify(storageFixture.records.get('auto').current.state).includes('readSpeed'),'Reading preferences never enter the save');

        // Auto-advance: a line counts as read once its timer elapsed, and result queues persist each read line.
        speechPrefs.autoAdvance=true;fastTimers(true);
        await startFixture();[...document.querySelectorAll('.agfish-start button')].find(value=>value.textContent==='Neue Reise').click();
        await until(()=>timerRequests.length===4&&!talking(),'The entry queue advances and ends by itself');
        check(timerRequests.join()===GoldfishState.enter(GoldfishState.create()).map(line=>GoldfishSpeech.duration(GoldfishContent.texts[line.text],'normal')).join(),'Every line timer follows the duration formula');
        document.querySelector('[data-agfish-hotspot="conveyor"]').click();
        await until(()=>resultLines.every(line=>storageFixture.records.get('auto')?.current.state.seenLines.includes(line.text)),'Elapsed result lines are persisted as read');
        fastTimers(false);speechPrefs.autoAdvance=false;

        // Layout: a visible line stays inside the scene while the window narrows and widens again.
        await startFixture();[...document.querySelectorAll('.agfish-start button')].find(value=>value.textContent==='Neue Reise').click();await until(()=>talking());
        for(const [width,height] of [[700,660],[1100,760]]){
            document.getElementById('host').style.width=width+'px';document.getElementById('host').style.height=height+'px';
            await new Promise(resolve=>setTimeout(resolve,120));
            const scene=document.querySelector('.agfish-scene').getBoundingClientRect(),box=document.querySelector('.agfish-speech').getBoundingClientRect();
            check(box.width>0&&box.left>=scene.left-1&&box.right<=scene.right+1&&box.top>=scene.top-1&&box.bottom<=scene.bottom+1,'A spoken line stays inside the scene at '+width+' px');
        }
        document.getElementById('host').style.width='700px';document.getElementById('host').style.height='660px';
        await new Promise(resolve=>setTimeout(resolve,80));
        check(!fixtureErrors.length,'ResizeObservers must not report layout feedback errors: '+fixtureErrors.join());
        GoldfishAdventureApp.dispose('fixture');
        return 'Goldfish app: unread/partial/dismissed/imported result recovery, exact CAS reloads, refused new games, prior regressions, speech queues, verbs, items, puzzle menus and resize observers passed.';
    })()`);
    await evaluate(`(async()=>{await startFixture(stateAt('R02'));window.oldAutoBeforeEscape=JSON.stringify(storageFixture.records.get('auto'));storageFixture.failSlots.add('chapter-1');[...document.querySelectorAll('.agfish-start button')].find(value=>value.textContent==='Neue Reise').click();await until(()=>document.querySelector('dialog'));})()`);
    await call('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27, nativeVirtualKeyCode: 27 });
    await call('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27, nativeVirtualKeyCode: 27 });
    await evaluate(`(async()=>{await until(()=>!document.querySelector('.agfish-start').hidden&&!document.querySelector('.agfish-busy'));if(document.querySelector('.agfish-app').dataset.room!=='R02'||lastRenderedRoom!=='R02'||JSON.stringify(storageFixture.records.get('auto'))!==oldAutoBeforeEscape)throw new Error('Escape must roll back the complete new-game boundary');GoldfishAdventureApp.dispose('fixture');})()`);
    assert.deepEqual(errors, [], JSON.stringify(errors));
    console.log(result);
} finally {
    socket?.close(); child.kill(); await new Promise(resolve => server.close(resolve));
    if (child.exitCode === null) await new Promise(resolve => child.once('exit', resolve));
    try { rmSync(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 }); } catch (_) { /* Ignored browser scratch. */ }
}
