(function () {
    'use strict';

    // Preview helpers for Game Maker Studio: loading overlay, stale badge,
    // fullscreen, and opening the sandboxed preview in a new tab.

    function validationActive(state, grant) {
        return grant && state.previewGrant === grant && Date.parse(grant.expires_at) > Date.now()
            && !['ready', 'failed', 'cancelled'].includes(state.job?.status);
    }

    function handleMessage(state, event) {
        if (!state.frame || event.source !== state.frame.contentWindow) return;
        const data = event.data;
        if (!data || typeof data !== 'object' || data.channel !== state.channelID || data.source !== 'aurago-game') return;
        const allowed = new Set(['ready', 'runtime_error', 'resource_error', 'diagnostic', 'gameplay', 'capture']);
        if (!allowed.has(data.type)) return;
        // Game-authored ready calls may precede rendering or describe a canvas
        // outside the viewport. Only the server boot's layout check qualifies.
        if (data.type === 'ready' && (data.boot !== true || data.visible !== true)) return;
        if (!state.project || state.previewProjectID !== state.project.id) return;
        if (state.previewGrant?.validation_id && !validationActive(state, state.previewGrant)) return;
        if(data.type==='capture'){
            const pending=state.visualCapture;
            if(!pending||data.request_id!==pending.id)return;
            clearTimeout(pending.timer);state.visualCapture=null;
            const captures=boundedCaptures(data.captures);
            showCaptures(state,captures);
            if(pending.manual) reviewCurrent(state,captures,pending);
            else state.api.reportPreview(state.previewProjectID,{token:state.previewGrant.token,type:'capture',captures}).catch(()=>{});
            return;
        }
        if (data.type === 'gameplay') {
            if (!state.previewGrant?.validation_id || state.previewReported.has('gameplay')) return;
            if (!Array.isArray(data.observations) || data.observations.length > 16) return;
            const images = Array.isArray(data.images) ? data.images.filter(image => typeof image === 'string' && image.length <= 700000).slice(0, 2) : [];
            const payload = { token: state.previewGrant.token, type: 'gameplay', observations: data.observations, images, captures:boundedCaptures(data.captures) };
            if (JSON.stringify(payload).length > 1500000) return;
            state.previewReported.add('gameplay');
            showCaptures(state,payload.captures);
            const grant = state.previewGrant;
            state.api.reportPreview(state.previewProjectID, payload).catch(error => {
                if (!state.disposed && validationActive(state, grant)) state.addDiagnostic({ level: 'error', message: error.message || String(error) });
            });
            return;
        }
        const message = data.type === 'ready' ? '' : String(data.message || data.type).slice(0, 1000);
        const key = data.type + ':' + message;
        if (state.previewReported.has(key) || state.previewReported.size >= 21) return;
        state.previewReported.add(key);
        if (state.previewGrant && state.previewGrant.validation_id) {
            const frame = state.frame;
            const grant = state.previewGrant;
            state.api.reportPreview(state.previewProjectID, {
                token: state.previewGrant.token, type: data.type, message, canvas_visible: data.type === 'ready'
            }).catch(error => {
                if (!state.disposed && validationActive(state, grant) && state.frame === frame && state.previewProjectID === state.project.id) {
                    state.addDiagnostic({ level: 'error', message: error.message || String(error) });
                }
            });
        }
        if (data.type === 'ready') {
            if (state.previewGrant?.scenarios?.length) {
                state.frame.contentWindow.postMessage({ source: 'aurago-studio', type: 'run-tests', channel: state.channelID, scenarios: state.previewGrant.scenarios }, '*');
            }
            if(state.previewGrant?.validation_id&&!state.previewGrant?.scenarios?.length){
                const grant=state.previewGrant;
                state.visualStartTimer=setTimeout(()=>{if(!state.disposed&&state.previewGrant===grant)requestCapture(state,false)},3100);
            }
            setSceneDebug(state, state.sceneDebug);
            clearLoading(state);
            return;
        }
        state.previewDiagnostics.push({ level: 'runtime', message });
        state.addDiagnostic({
            level: 'runtime',
            message
        });
    }

    function boundedCaptures(input) {
        return Array.isArray(input)?input.slice(0,2).filter(c=>c&&typeof c.image==='string'&&c.image.startsWith('data:image/png;base64,')&&c.image.length<=700000).map(c=>({
            image:c.image,controlled:c.controlled===true,scenario:String(c.scenario||'').slice(0,96),at:String(c.at||'').slice(0,40),
            width:Number(c.width),height:Number(c.height),hud:String(c.hud||'').slice(0,4000)
        })):[];
    }
    function showCaptures(state,captures){
        const panel=state.container.querySelector('[data-gm-visual]');if(!panel)return;
        panel.querySelectorAll('img').forEach(img=>img.remove());
        for(const c of captures){const img=document.createElement('img');img.src=c.image;img.alt=c.scenario;img.style.cssText='max-width:140px;max-height:90px;margin:4px;object-fit:contain';panel.appendChild(img);}
        panel.hidden=false;
    }
    function visualStatus(state,status){
        const el=state.container.querySelector('[data-gm-visual-status]');
        if(el)el.textContent=state.context.t('game_maker.visual_'+status);
    }
    function showReview(state,result){
        if(state.disposed)return;
        const panel=state.container.querySelector('[data-gm-visual]');if(panel)panel.hidden=false;
        const status=state.container.querySelector('[data-gm-visual-status]');
        if(status)status.textContent=state.context.t('game_maker.visual_checks')+': '+state.context.t('game_maker.check_'+result.status)+(result.model?' · '+result.model:'')+(result.reason?' · '+state.context.t('game_maker.visual_'+result.reason):'');
        for(const f of (result.findings||[]).slice(0,6))state.addDiagnostic({level:'info',message:String(f.observation||'').slice(0,600)+' — '+String(f.region||'').slice(0,160)+' — '+String(f.suggestion||'').slice(0,600)});
    }
    function cancelVisual(state){
        if(state.visualCapture)clearTimeout(state.visualCapture.timer);
        clearTimeout(state.visualStartTimer);state.visualStartTimer=null;state.visualCapture=null;
        state.visualAbort?.abort();state.visualAbort=null;state.visualBusy=false;
        const panel=state.container.querySelector('[data-gm-visual]');if(panel){panel.querySelectorAll('img').forEach(img=>img.remove());panel.hidden=true;}
    }
    function requestCapture(state,manual=true){
        if(state.disposed||!state.frame||!state.previewGrant||state.visualBusy||state.visualCapture)return;
        if(manual&&(state.jobActive||state.previewGrant.validation_id))return;
        const pending={id:Array.from(crypto.getRandomValues(new Uint32Array(4))).join('-'),manual,grant:state.previewGrant,project:state.project.id};
        state.visualCapture=pending;
        const panel=state.container.querySelector('[data-gm-visual]');if(panel)panel.hidden=false;
        visualStatus(state,'capturing');
        pending.timer=setTimeout(()=>{if(state.visualCapture!==pending)return;state.visualCapture=null;visualStatus(state,'capture_unavailable')},2500);
        state.frame.contentWindow.postMessage({source:'aurago-studio',channel:state.channelID,type:'capture',request_id:pending.id},'*');
    }
    async function reviewCurrent(state,captures,pending){
        if(!captures.length){visualStatus(state,'capture_unavailable');return;}
        state.visualBusy=true;const controller=new AbortController();state.visualAbort=controller;
        const timer=setTimeout(()=>{controller.abort();if(!state.disposed&&state.previewGrant===pending.grant)visualStatus(state,'analysis_failed')},55000);visualStatus(state,'analyzing');
        try{
            const result=await state.api.reviewVisual(pending.project,{token:pending.grant.token,captures,provider_id:state.job?.provider_id||'',model:state.job?.model||''},controller.signal);
            if(!state.disposed&&state.previewGrant===pending.grant&&!controller.signal.aborted)showReview(state,result);
        }catch(_){if(!state.disposed&&state.previewGrant===pending.grant&&!controller.signal.aborted)visualStatus(state,'analysis_failed')}
        finally{clearTimeout(timer);if(state.visualAbort===controller){state.visualAbort=null;state.visualBusy=false}}
    }

    function setSceneDebug(state, enabled) {
        state.sceneDebug = enabled === true;
        state.container.querySelector('[data-gm-action="scene_debug"]')?.setAttribute('aria-pressed', String(state.sceneDebug));
        if (!state.disposed && state.frame && state.channelID) state.frame.contentWindow.postMessage({ source: 'aurago-studio', type: 'scene-debug', channel: state.channelID, enabled: state.sceneDebug }, '*');
    }

    function showLoading(state, shellEl, frame) {
        cancelVisual(state);
        clearLoading(state);
        const overlay = document.createElement('div');
        overlay.className = 'gm-preview-loading';
        overlay.setAttribute('data-gm-preview-loading', 'true');
        overlay.innerHTML = `<span class="gm-job-spinner" aria-hidden="true"></span>
            <span>${state.context.esc(state.context.t('game_maker.preview_loading'))}</span>`;
        shellEl.appendChild(overlay);
        let timer = null;
        let settled = false;
        const clear = () => {
            if (settled) return;
            settled = true;
            overlay.remove();
            if (timer) clearTimeout(timer);
            if (state.previewLoadClear === clear) {
                state.previewLoadTimer = null;
                state.previewLoadClear = null;
            }
        };
        // Prefer the game's ready diagnostic; fall back to iframe load so a
        // hung/missing canvas does not leave the studio overlay forever.
        frame.addEventListener('load', () => setTimeout(clear, 600), { once: true });
        state.previewLoadClear = clear;
        timer = setTimeout(() => {
            const current = state.previewLoadClear === clear;
            clear();
            if (current) {
                state.addDiagnostic({ level: 'info', message: state.context.t('game_maker.preview_timeout') });
            }
        }, 12000);
        state.previewLoadTimer = timer;
    }

    function clearLoading(state) {
        if (state.previewLoadClear) {
            state.previewLoadClear();
            return;
        }
        if (state.previewLoadTimer) clearTimeout(state.previewLoadTimer);
        state.previewLoadTimer = null;
    }

    function updateStaleBadge(state) {
        const shellEl = state.container.querySelector('[data-gm-preview]');
        if (!shellEl) return;
        let badge = shellEl.querySelector('.gm-preview-stale');
        const stale = Boolean(state.jobActive && state.previewStale && state.frame);
        if (stale && !badge) {
            badge = document.createElement('span');
            badge.className = 'gm-preview-stale';
            shellEl.appendChild(badge);
        }
        if (badge) {
            badge.hidden = !stale;
            badge.textContent = state.context.t('game_maker.preview_stale');
        }
    }

    function toggleFullscreen(state) {
        const shellEl = state.container.querySelector('[data-gm-preview]');
        if (!shellEl || !shellEl.requestFullscreen) return;
        if (document.fullscreenElement) document.exitFullscreen();
        else shellEl.requestFullscreen();
    }

    async function openTab(state) {
        if (!state.project) return;
        try {
            const grant = await state.api.previewGrant(state.project.id);
            if (state.disposed) return;
            window.open(grant.url, '_blank', 'noopener');
        } catch (error) {
            state.fail(error);
        }
    }

    window.GameMakerStudioPreview = { requestCapture, cancelVisual, showReview, visualStatus, handleMessage, setSceneDebug, showLoading, clearLoading, updateStaleBadge, toggleFullscreen, openTab };
})();
