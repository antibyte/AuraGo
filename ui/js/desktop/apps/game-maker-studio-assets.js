(function () {
    'use strict';

    const packName = (state, id) => state.context.t('game_maker.pack_' + id.replaceAll('-', '_'));
    function selectionText(state) {
        const ids = state.selectedAssetPackIDs || [];
        const models = [...(state.selectedModelAssetIDs || []),...(state.selectedPresentation?.effects||[]),...(state.selectedPresentation?.sounds||[]).map(s=>s.sound),...(state.selectedPresentation?.environment?[state.selectedPresentation.environment]:[])];
        return ids.length || models.length ? state.context.t('game_maker.assets_selected') + ': ' + [...ids.map(id => packName(state, id)), ...models].join(', ')
            : state.context.t('game_maker.assets_automatic');
    }
    function updateSelection(state) {
        state.container.querySelectorAll('[data-gm-asset-selection]').forEach(node => { node.textContent = selectionText(state); });
    }
    function selectionMarkup(state) {
        return `<p class="gm-asset-selection" data-gm-asset-selection>${state.context.esc(selectionText(state))}</p>`;
    }
    function clearSelection(state) {
        state.selectedAssetPackIDs = [];
        state.selectedModelAssetIDs = [];
        state.selectedPresentation = null;
        updateSelection(state);
    }
    function show(state, helpers) {
        const { esc, t } = state.context;
        helpers.showModal(state, `<section class="gm-modal gm-assets-modal" role="dialog" aria-modal="true" aria-label="${esc(t('game_maker.assets'))}">
            <header><div><span>${esc(t('game_maker.assets_offline'))}</span><h2>${esc(t('game_maker.assets'))}</h2></div>
                <button type="button" data-modal-close aria-label="${esc(t('game_maker.close'))}">×</button></header>
            <div class="gm-assets-toolbar"><input type="search" data-asset-search placeholder="${esc(t('game_maker.assets_search'))}" aria-label="${esc(t('game_maker.assets_search'))}">
                <select data-asset-kind aria-label="${esc(t('game_maker.assets_category'))}">${['all','sprite2d','model3d','effect','audio'].map(k=>`<option value="${k}">${esc(t('game_maker.asset_kind_'+k))}</option>`).join('')}</select><select data-asset-category aria-label="${esc(t('game_maker.assets_category'))}"><option value="">${esc(t('game_maker.assets_all'))}</option></select></div>
            <div class="gm-assets-workspace"><div class="gm-asset-cards" data-asset-cards role="list"></div>
                <div class="gm-asset-detail" data-asset-detail><p role="status">${esc(t('game_maker.loading'))}</p></div></div>
            ${selectionMarkup(state)}<footer><button type="button" data-clear-selection>${esc(t('game_maker.assets_auto_button'))}</button>
                <button type="button" data-modal-close>${esc(t('game_maker.close'))}</button></footer></section>`, layer => {
            const abort = new AbortController();
            let timer = null, detailID = 0, packs = [], modelPack = null, presentationPacks = [], viewerCleanup = null;
            const cleanup = () => {
                abort.abort(); clearInterval(timer);
                viewerCleanup?.(); viewerCleanup = null;
                if (state.assetBrowserCleanup === cleanup) state.assetBrowserCleanup = null;
            };
            state.assetBrowserCleanup = cleanup;
            const current = () => !state.disposed && !abort.signal.aborted;
            const cards = layer.querySelector('[data-asset-cards]'), detail = layer.querySelector('[data-asset-detail]');
            const search = layer.querySelector('[data-asset-search]'), category = layer.querySelector('[data-asset-category]');
            const kind=layer.querySelector('[data-asset-kind]');
            const selected = () => state.selectedAssetPackIDs || [];
            function renderCards() {
                const query = search.value.trim().toLowerCase();
                const visible = packs.filter(pack => pack.kind === 'sprite2d' && ['all','sprite2d'].includes(kind.value) && (!category.value || category.value === pack.id) &&
                    [packName(state, pack.id), pack.description, ...pack.tags].join(' ').toLowerCase().includes(query));
                cards.innerHTML = visible.map(pack => `<article class="gm-asset-card" role="listitem">
                    <button type="button" data-pack="${esc(pack.id)}"><img src="${esc(state.api.assetPackImageURL(pack.id))}" alt="" loading="lazy">
                    <strong>${esc(packName(state, pack.id))}</strong></button><label><input type="checkbox" aria-label="${esc(packName(state, pack.id) + ': ' + t('game_maker.assets_use_next'))}" data-select-pack="${esc(pack.id)}" ${selected().includes(pack.id) ? 'checked' : ''}>
                        ${esc(t('game_maker.assets_use_next'))}</label></article>`).join('');
                const models = (modelPack?.assets || []).filter(model => ['all','model3d'].includes(kind.value) &&
                    (!category.value || category.value === modelPack.id || category.value === '3d:' + model.category) &&
                    [model.name, model.description, ...model.tags].join(' ').toLowerCase().includes(query));
                cards.innerHTML += models.map(model => `<article class="gm-asset-card gm-model-card" role="listitem">
                    <button type="button" data-model="${esc(model.id)}"><img src="${esc(state.api.assetPackFileURL(modelPack.id, model.preview))}" alt="" loading="lazy">
                    <strong>${esc(model.name)}</strong></button><label><input type="checkbox" data-select-model="${esc(model.id)}"
                    aria-label="${esc(model.name + ': ' + t('game_maker.assets_use_next'))}" ${(state.selectedModelAssetIDs || []).includes(model.id) ? 'checked' : ''}>
                    ${esc(t('game_maker.assets_use_next'))}</label></article>`).join('');
                const presentation = presentationPacks.flatMap(pack=>pack.assets.map(asset=>({pack,asset}))).filter(({pack,asset})=>['all',pack.kind].includes(kind.value)&&(!category.value||category.value===pack.id)&&[asset.name,asset.description,...asset.tags].join(' ').toLowerCase().includes(query));
                cards.innerHTML += presentation.map(({pack,asset})=>`<article class="gm-asset-card" role="listitem"><button type="button" data-presentation="${esc(asset.id)}" data-presentation-pack="${esc(pack.id)}"><span style="font:48px system-ui;color:${pack.kind==='audio'?'#d6a7ff':'#6bddd5'};height:80px;display:grid;place-items:center" aria-hidden="true">${pack.kind==='audio'?'♪':'≋'}</span><strong>${esc(asset.name)}</strong></button><label><input type="checkbox" aria-label="${esc(asset.name + ": " + t('game_maker.assets_use_next'))}" data-select-presentation="${esc(asset.id)}" data-presentation-pack="${esc(pack.id)}" ${(state.selectedPresentation?.effects||[]).includes(asset.id)||state.selectedPresentation?.environment===asset.id||(state.selectedPresentation?.sounds||[]).some(s=>s.sound===asset.id)?'checked':''}>${esc(t('game_maker.assets_use_next'))}</label></article>`).join('');
                if (!visible.length && !models.length && !presentation.length) cards.innerHTML = `<p>${esc(t('game_maker.assets_no_results'))}</p>`;
            }
            async function showPack(id) {
                clearInterval(timer);
                viewerCleanup?.(); viewerCleanup = null;
                const requestID = ++detailID;
                detail.innerHTML = `<p role="status">${esc(t('game_maker.loading'))}</p>`;
                try {
                    const pack = await state.api.assetPack(id, { signal: abort.signal });
                    if (!current() || detailID !== requestID) return;
                    detail.innerHTML = `<h3>${esc(packName(state, id))}</h3><p>${esc(pack.description)}</p>
                        <div class="gm-asset-stage is-checker" data-asset-stage><img class="gm-asset-sheet" src="${esc(state.api.assetPackImageURL(id))}" alt="${esc(packName(state, id))}">
                            <canvas width="256" height="256" data-asset-canvas aria-label="${esc(t('game_maker.assets_preview'))}"></canvas></div>
                        <label>${esc(t('game_maker.assets_background'))}<select data-asset-background>
                            <option value="checker">${esc(t('game_maker.assets_checker'))}</option><option value="white">${esc(t('game_maker.assets_white'))}</option>
                            <option value="dark">${esc(t('game_maker.assets_dark'))}</option></select></label>
                        ${(pack.assemblies || []).length ? `<label>${esc(t('game_maker.assets_assembly'))}<select data-asset-assembly><option value="">${esc(t('game_maker.assets_individual'))}</option>${pack.assemblies.map(a => `<option value="${esc(a.id)}">${esc(a.name)}</option>`).join('')}</select></label>` : ''}
                        <label>${esc(t('game_maker.assets_sprite'))}<select data-asset-sprite>${pack.assets.map(asset => `<option value="${esc(asset.id)}">${esc(asset.name)}</option>`).join('')}</select></label>
                        <p data-asset-description></p><label>${esc(t('game_maker.assets_animation'))}<select data-asset-animation><option value="">${esc(t('game_maker.assets_still'))}</option>
                            ${pack.animations.map(a => `<option value="${esc(a.id)}">${esc(a.id)}</option>`).join('')}</select></label>
                        <div class="gm-asset-playback"><button type="button" data-asset-play>${esc(t('game_maker.assets_play'))}</button>
                            <button type="button" data-asset-stop>${esc(t('game_maker.assets_pause'))}</button><span data-asset-frame></span></div>`;
                    const ctx = detail.querySelector('[data-asset-canvas]').getContext('2d');
                    ctx.imageSmoothingEnabled = false;
                    const image = detail.querySelector('.gm-asset-sheet'), spriteSelect = detail.querySelector('[data-asset-sprite]');
                    const animationSelect = detail.querySelector('[data-asset-animation]');
                    const assemblySelect = detail.querySelector('[data-asset-assembly]');
                    let frameIndex = 0;
                    function drawAssembly(elapsed = 0) {
                        const assembly = pack.assemblies?.find(a => a.id === assemblySelect?.value);
                        if (!assembly || !current() || detailID !== requestID || !image.complete || !image.naturalWidth) return;
                        ctx.clearRect(0, 0, 256, 256);
                        const scale = 256 / Math.max(assembly.width, assembly.height);
                        const left = (256 - assembly.width * scale) / 2, top = (256 - assembly.height * scale) / 2;
                        for (const part of assembly.parts) {
                            const animation = pack.animations.find(a => a.id === part.animation_id);
                            let index = part.frame;
                            if (animation) {
                                const sequence = animation.frames.slice();
                                if (animation.yoyo && sequence.length > 2) sequence.push(...sequence.slice(1, -1).reverse());
                                const step = Math.floor(elapsed * animation.frame_rate / 1000);
                                index = sequence[animation.repeat === -1 || step < sequence.length * (animation.repeat + 1)
                                    ? step % sequence.length : sequence.length - 1];
                            }
                            const frame = pack.frames[index];
                            ctx.drawImage(image, frame.x, frame.y, frame.w, frame.h,
                                left + part.x * scale, top + part.y * scale, frame.w * scale, frame.h * scale);
                        }
                        detail.querySelector('[data-asset-frame]').textContent = assembly.width + ' × ' + assembly.height;
                    }
                    function draw(index) {
                        if (assemblySelect?.value) { drawAssembly(); return; }
                        if (!current() || detailID !== requestID || !image.complete || !image.naturalWidth) return;
                        const frame = pack.frames[index];
                        ctx.clearRect(0, 0, 256, 256);
                        ctx.drawImage(image, frame.x, frame.y, frame.w, frame.h, 0, 0, 256, 256);
                        detail.querySelector('[data-asset-frame]').textContent = t('game_maker.assets_frame') + ' ' + index;
                    }
                    function chooseSprite() {
                        clearInterval(timer);
                        if (assemblySelect) assemblySelect.value = '';
                        const asset = pack.assets.find(a => a.id === spriteSelect.value);
                        frameIndex = asset.frames[0];
                        detail.querySelector('[data-asset-description]').textContent = asset.description;
                        animationSelect.value = pack.animations.find(a => a.asset_id === asset.id)?.id || '';
                        draw(frameIndex);
                    }
                    function play() {
                        clearInterval(timer);
                        if (assemblySelect?.value) {
                            drawAssembly();
                            const assembly = pack.assemblies.find(a => a.id === assemblySelect.value);
                            if (!assembly.parts.some(p => p.animation_id)) return;
                            const started = performance.now();
                            timer = setInterval(() => drawAssembly(performance.now() - started), 1000 / 30);
                            return;
                        }
                        const animation = pack.animations.find(a => a.id === animationSelect.value);
                        if (!animation) { draw(frameIndex); return; }
                        spriteSelect.value = animation.asset_id;
                        detail.querySelector('[data-asset-description]').textContent = pack.assets.find(a => a.id === animation.asset_id).description;
                        const sequence = animation.frames.slice();
                        if (animation.yoyo && sequence.length > 2) sequence.push(...sequence.slice(1, -1).reverse());
                        let position = 0, loops = 0;
                        draw(sequence[0]);
                        timer = setInterval(() => {
                            if (!current()) { clearInterval(timer); return; }
                            if (++position >= sequence.length) {
                                if (animation.repeat !== -1 && loops >= animation.repeat) { clearInterval(timer); return; }
                                position = 0; loops++;
                            }
                            draw(sequence[position]);
                        }, 1000 / animation.frame_rate);
                    }
                    image.addEventListener('load', () => draw(frameIndex), { once: true });
                    image.addEventListener('error', () => { if (current() && detailID === requestID) helpers.modalError(layer, t('game_maker.assets_load_failed')); }, { once: true });
                    detail.querySelector('[data-asset-background]').addEventListener('change', event => { detail.querySelector('[data-asset-stage]').className = 'gm-asset-stage is-' + event.target.value; });
                    spriteSelect.addEventListener('change', chooseSprite);
                    animationSelect.addEventListener('change', () => { if (assemblySelect) assemblySelect.value = ''; play(); });
                    assemblySelect?.addEventListener('change', () => {
                        clearInterval(timer);
                        if (!assemblySelect.value) { chooseSprite(); return; }
                        animationSelect.value = '';
                        detail.querySelector('[data-asset-description]').textContent = pack.assemblies.find(a => a.id === assemblySelect.value).description;
                        play();
                    });
                    detail.querySelector('[data-asset-play]').addEventListener('click', play);
                    detail.querySelector('[data-asset-stop]').addEventListener('click', () => clearInterval(timer));
                    chooseSprite();
                    if (assemblySelect) {
                        assemblySelect.value = pack.assemblies[0].id;
                        animationSelect.value = '';
                        detail.querySelector('[data-asset-description]').textContent = pack.assemblies[0].description;
                        drawAssembly();
                    }
                } catch (error) { if (current() && detailID === requestID) helpers.modalError(layer, error.message || t('game_maker.assets_load_failed')); }
            }
            function showModel(id) {
                const model = modelPack?.assets.find(a => a.id === id);
                if (!model) return;
                ++detailID; clearInterval(timer); viewerCleanup?.();
                viewerCleanup = window.GameMakerStudioModels.mount(state, detail, modelPack, model);
            }
            cards.addEventListener('click', event => {
                const effect=event.target.closest('[data-presentation]');
                if(effect){++detailID;clearInterval(timer);viewerCleanup?.();const pack=presentationPacks.find(p=>p.id===effect.dataset.presentationPack);viewerCleanup=mountPresentation(state,detail,pack,pack.assets.find(a=>a.id===effect.dataset.presentation),presentationPacks);return}
                const model = event.target.closest('[data-model]');
                if (model) { showModel(model.dataset.model); return; }
                const button = event.target.closest('[data-pack]'); if (button) showPack(button.dataset.pack);
            });
            cards.addEventListener('change', event => {
                const presentationID=event.target.dataset.selectPresentation;
                if(presentationID){
                    const pack=presentationPacks.find(p=>p.id===event.target.dataset.presentationPack),asset=pack?.assets.find(a=>a.id===presentationID);if(!asset)return;
                    const p=state.selectedPresentation||={effects:[],sounds:[],quality:'auto'};
                    if(pack.kind==='audio') {p.sounds=p.sounds.filter(s=>s.sound!==asset.id);if(event.target.checked)p.sounds.push({event:soundEvent(asset),sound:asset.id})}
                    else if(asset.category==='environment'){p.environment=event.target.checked?asset.id:''}
                    else {p.effects=p.effects.filter(id=>id!==asset.id);if(event.target.checked)p.effects.push(asset.id)}
                    if(p.effects.length>32){p.effects.pop();helpers.modalError(layer,t('game_maker.effect_limit'))}
                    if(!p.environment&&!p.effects.length&&!p.sounds.length)state.selectedPresentation=null;
                    renderCards();updateSelection(state);return;
                }
                const modelID = event.target.dataset.selectModel;
                if (modelID) {
                    if (!modelPack?.assets.some(a => a.id === modelID)) return;
                    const chosen = state.selectedModelAssetIDs || [];
                    if (event.target.checked && chosen.length >= 64 && !chosen.includes(modelID)) {
                        event.target.checked = false; helpers.modalError(layer, t('game_maker.model_limit')); return;
                    }
                    state.selectedModelAssetIDs = event.target.checked ? [...new Set([...chosen, modelID])] : chosen.filter(id => id !== modelID);
                    updateSelection(state); return;
                }
                const id = event.target.dataset.selectPack;
                if (!id || !packs.some(p => p.id === id)) return;
                state.selectedAssetPackIDs = event.target.checked ? [...new Set([...selected(), id])] : selected().filter(value => value !== id);
                updateSelection(state);
            });
            layer.querySelector('[data-clear-selection]').addEventListener('click', () => { clearSelection(state); renderCards(); });
            kind.addEventListener('change',()=>{category.value='';renderCards()});
            search.addEventListener('input', renderCards); category.addEventListener('change', renderCards);
            state.api.assetPacks({ signal: abort.signal }).then(async body => {
                if (!current()) return;
                packs = body.packs;
                const models = packs.find(p => p.kind === 'model3d');
                category.innerHTML += packs.filter(p => p.kind === 'sprite2d').map(p => `<option value="${esc(p.id)}">${esc(packName(state, p.id))}</option>`).join('');
                renderCards();
                if (models) {
                    try { modelPack = await state.api.modelPack(models.id, { signal: abort.signal }); }
                    catch (error) { if (current()) helpers.modalError(layer, error.message || t('game_maker.assets_load_failed')); }
                    if (!current()) return;
                }
                for(const p of packs.filter(p=>['effect','audio'].includes(p.kind))){
                    try {const manifest=await state.api.modelPack(p.id,{signal:abort.signal});if(!current())return;presentationPacks.push(manifest);category.innerHTML+=`<option value="${esc(p.id)}">${esc(t('game_maker.asset_kind_'+p.kind))}</option>`}
                    catch(error){if(current())helpers.modalError(layer,error.message)}
                }
                if (modelPack) {
                    category.innerHTML += `<option value="${esc(models.id)}">${esc(t('game_maker.models'))}</option>` +
                        modelPack.categories.map(c => `<option value="3d:${esc(c.id)}">3D · ${esc(t('game_maker.model_category_' + c.id))}</option>`).join('');
                }
                if (state.project?.dimension === '3d' && modelPack) category.value = models.id;
                renderCards();
                if (category.value === models?.id && modelPack?.assets.length) showModel(modelPack.assets[0].id);
                else if (packs.some(p=>p.kind==='sprite2d')) showPack(packs.find(p => p.kind === 'sprite2d').id);
            }).catch(error => { if (current()) helpers.modalError(layer, error.message || t('game_maker.assets_load_failed')); });
        });
    }
    function soundEvent(a) {
        if(a.loop)return a.id==='engine'?'engine':'ambient';
        if(a.id.startsWith('step-'))return 'step';
        if(['pistol','rifle','shotgun','laser'].includes(a.id))return 'shot';
        if(a.id.startsWith('impact-')||a.id.startsWith('explosion-'))return 'hit';
        return {jump:'jump',land:'land',reload:'reload','water-splash':'splash','ui-click':'ui',pickup:'pickup',victory:'win',defeat:'lose'}[a.id]||'interact';
    }
    function mountPresentation(state,host,pack,asset,packs) {
        const {esc,t}=state.context,abort=new AbortController();let disposed=false,fx,engine,renderer,frame=0,last=0,playing=true,visible=true,invalidate=()=>{},observer,visibility,mesh,owned=[],cleanupEngine=()=>{};
        host.innerHTML=`<h3>${esc(asset.name)}</h3><p>${esc(asset.description)}</p><div class="gm-model-stage" data-fx-stage tabindex="0" aria-label="${esc(asset.name)}"></div><div class="gm-model-controls"><label>${esc(t('game_maker.effect_dimension'))}<select data-fx-dimension><option>2d</option><option selected>3d</option></select></label><label>${esc(t('game_maker.effect_quality'))}<select data-fx-quality>${['auto','low','medium','high'].map(v=>`<option value="${v}">${esc(t('game_maker.quality_'+v))}</option>`).join('')}</select></label><button type="button" data-fx-play>${esc(t('game_maker.assets_play'))}</button><button type="button" data-fx-pause>${esc(t('game_maker.assets_pause'))}</button></div><div data-fx-parameters class="gm-model-controls"></div><p data-fx-status role="status"></p>`;
        const stage=host.querySelector('[data-fx-stage]'),status=host.querySelector('[data-fx-status]'),dimension=host.querySelector('[data-fx-dimension]');stage.style.position='relative';
        const options={...asset.defaults};for(const [key,val] of Object.entries(options)){
            if(!['number','boolean','string'].includes(typeof val))continue;
            const label=document.createElement('label');label.textContent=t('game_maker.effect_param_'+key);const input=document.createElement('input');input.type=typeof val==='boolean'?'checkbox':typeof val==='number'?'number':'color';input.value=String(val);input.checked=!!val;input.step='0.1';input.style.width='90px';label.append(input);host.querySelector('[data-fx-parameters]').append(label);input.addEventListener('change',()=>{options[key]=typeof val==='boolean'?input.checked:typeof val==='number'?Number(input.value):input.value;try{fx?.set(asset.id,options)}catch(e){status.textContent=e.message}},{signal:abort.signal});
        }
        if(pack.kind==='audio'){
            dimension.closest('label').hidden=true;host.querySelector('[data-fx-quality]').closest('label').hidden=true;stage.style.cssText+=';display:grid;place-items:center';
            const audio=document.createElement('audio');audio.controls=true;audio.preload='none';audio.loop=asset.loop;audio.src=state.api.assetPackFileURL(pack.id,asset.files[0].file);stage.append(audio);
            audio.onerror=()=>{if(!disposed)status.textContent=t('game_maker.assets_load_failed')};
            const label=document.createElement('label');label.textContent=t('game_maker.sound_event');const select=document.createElement('select');for(const id of ['step','jump','land','shot','reload','hit','pickup','win','lose','splash','interact','engine','ui','ambient'])select.add(new Option(t('game_maker.sound_event_'+id),id));select.value=state.selectedPresentation?.sounds.find(s=>s.sound===asset.id)?.event||soundEvent(asset);label.append(select);host.querySelector('[data-fx-parameters]').append(label);select.onchange=()=>{const binding=state.selectedPresentation?.sounds.find(s=>s.sound===asset.id);if(binding)binding.event=select.value};
            host.querySelector('[data-fx-play]').onclick=()=>audio.play().catch(e=>status.textContent=e.message);host.querySelector('[data-fx-pause]').onclick=()=>audio.pause();
            status.textContent=`WAV · 48 kHz · PCM 16 · ${asset.duration.toFixed(1)} s · ${asset.provenance?.license||'MIT'}`;
            document.addEventListener('visibilitychange',()=>{if(document.hidden)audio.pause()},{signal:abort.signal});visibility=new IntersectionObserver(entries=>{if(!entries[0].isIntersecting)audio.pause()});visibility.observe(stage);
            return()=>{disposed=true;abort.abort();visibility.disconnect();audio.pause();audio.removeAttribute('src');audio.load()};
        }
        const effectPack=packs.find(p=>p.kind==='effect'),sounds=packs.find(p=>p.kind==='audio');
        const ids=new Set([asset.id,...(asset.effects||[])]),config={effects:effectPack.assets.filter(a=>ids.has(a.id)),sounds:(sounds?.assets||[]).filter(a=>(asset.sounds||[]).includes(a.id)),environment:asset.category==='environment'?asset.id:'',quality:'auto',base:state.api.assetPackFileURL('aurago-sounds','')};
        function stop(){cancelAnimationFrame(frame);frame=0;last=0;observer?.disconnect();fx?.dispose();fx=null;cleanupEngine();engine=null;renderer=null;mesh=null;invalidate=()=>{};cleanupEngine=()=>{};stage.replaceChildren()}
        let revision=0;
        async function start(){
            const rev=++revision;stop();status.textContent=t('game_maker.loading');
            try{
                const dim=dimension.value,runtime=await import(state.api.assetPackFileURL('runtime','aurago-effects-'+dim+'-1.js'));if(disposed||revision!==rev)return;
                const ready=adapter=>{if(disposed||revision!==rev){adapter.dispose();return}fx=runtime.createPresentation({adapter,config,root:stage,controls:false,report:e=>status.textContent=String(e)});status.textContent=t('game_maker.effect_trigger');trigger();};
                if(dim==='3d'){
                    const A=await import(state.api.assetPackFileURL('runtime','aurago-three-assets-1.js'));if(disposed||revision!==rev)return;const T=A.THREE,scene=new T.Scene(),camera=new T.PerspectiveCamera(48,1,.05,450),sun=new T.DirectionalLight(0xffecd0,3),ambient=new T.HemisphereLight(0xccefff,0x324337,2);scene.add(sun,ambient);sun.position.set(5,15,8);camera.position.set(8,6,12);camera.lookAt(0,1,0);renderer=new T.WebGLRenderer({antialias:true});renderer.setPixelRatio(Math.min(devicePixelRatio,1.5));renderer.toneMapping=T.ACESFilmicToneMapping;stage.append(renderer.domElement);
                    const ground=new T.Mesh(asset.id.startsWith('water-')||asset.id==='coast'?new T.CircleGeometry(3,32):new T.PlaneGeometry(80,80),new T.MeshStandardMaterial({color:0x465d50}));ground.rotation.x=-Math.PI/2;scene.add(ground);mesh=new T.Mesh(new T.BoxGeometry(2,2,2),new T.MeshStandardMaterial({color:0x789fbe}));mesh.position.y=1;scene.add(mesh);owned=[ground,mesh];
                    const adapter=runtime.createThreeAdapter({scene,camera,renderer,sun,ambient});adapter.registerSurface(ground,{kind:'ground'});ready(adapter);
                    observer=new ResizeObserver(()=>{const w=stage.clientWidth,h=stage.clientHeight;if(w&&h){renderer.setSize(w,h);camera.aspect=w/h;camera.updateProjectionMatrix();fx?.resize();invalidate()}});observer.observe(stage);
                    cleanupEngine=()=>{owned.forEach(o=>{o.geometry.dispose();o.material.dispose()});renderer.dispose();renderer.forceContextLoss()};
                    const draw=now=>{frame=0;if(disposed||revision!==rev||!visible||document.hidden)return;const dt=last?Math.min(.05,(now-last)/1000):0;last=now;if(playing&&!document.hidden)fx?.update(dt);fx?.render();if(playing)frame=requestAnimationFrame(draw)};invalidate=()=>{if(!frame&&!disposed&&visible&&!document.hidden)frame=requestAnimationFrame(draw)};invalidate();
                }else{
                    if(!window.Phaser){window.__gmPhaserLoad||=new Promise((resolve,reject)=>{const script=document.createElement('script');script.src=state.api.assetPackFileURL('runtime','phaser-4.2.1.min.js');script.onload=resolve;script.onerror=()=>{window.__gmPhaserLoad=null;reject(Error(t('game_maker.assets_load_failed')))};document.head.append(script)});await window.__gmPhaserLoad}
                    if(disposed||revision!==rev)return;
                    engine=new Phaser.Game({type:Phaser.AUTO,parent:stage,width:720,height:420,backgroundColor:'#15222f',scale:{mode:Phaser.Scale.FIT,autoCenter:Phaser.Scale.CENTER_BOTH},scene:{create(){const ground=this.add.rectangle(360,390,720,60,0x46624c),roof=this.add.rectangle(210,220,230,16,0x9d7552);mesh=this.add.rectangle(480,345,60,60,0x789fbe);const adapter=runtime.createPhaserAdapter({scene:this,view:'side',report:e=>status.textContent=String(e)});adapter.registerSurface(ground,{kind:'ground'});adapter.registerSurface(roof,{kind:'roof'});ready(adapter)},update(_,dt){if(playing)fx?.update(Math.min(.05,dt/1000))}}});cleanupEngine=()=>engine.destroy(true);
                }
            }catch(e){if(!disposed&&revision===rev)status.textContent=e.message}
        }
        let releaseObject;
        function trigger(){if(!fx)return;playing=true;last=0;fx.setPaused(false);engine?.loop.wake();invalidate();const position=dimension.value==='3d'?asset.id==='blood-pool'||asset.id==='water-ripple'?[2,.01,1]:[0,1.2,1.01]:[480,320,0];if(['hologram','dissolve','hit-flash'].includes(asset.id)&&mesh){releaseObject?.();releaseObject=fx.applyObject(mesh,asset.id,options)}else if(asset.category!=='environment'){if(['fire','smoke','embers','engine-trail'].includes(asset.id))fx.set(asset.id,{...options,position});fx.emit(asset.id,{...options,position,normal:dimension.value==='3d'?(asset.id==='blood-decal'?[0,0,1]:[0,1,0]):[0,-1,0]})}}
        host.querySelector('[data-fx-play]').onclick=trigger;stage.addEventListener('pointerdown',trigger,{signal:abort.signal});host.querySelector('[data-fx-pause]').onclick=()=>{playing=false;fx?.setPaused(true);engine?.loop.sleep()};dimension.onchange=start;host.querySelector('[data-fx-quality]').onchange=e=>{fx?.setQuality(e.target.value);invalidate()};
        const active=()=>{fx?.setActive(visible&&!document.hidden);last=0;if(visible&&!document.hidden){if(playing)engine?.loop.wake();invalidate()}else{cancelAnimationFrame(frame);frame=0;engine?.loop.sleep()}};visibility=new IntersectionObserver(entries=>{visible=entries[0].isIntersecting;active()});visibility.observe(stage);document.addEventListener('visibilitychange',active,{signal:abort.signal});start();
        return()=>{if(disposed)return;disposed=true;revision++;abort.abort();visibility.disconnect();stop()};
    }

    window.GameMakerStudioAssets = { show, selectionMarkup, updateSelection, clearSelection };
})();
