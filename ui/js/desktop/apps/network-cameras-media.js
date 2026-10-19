// Network Cameras media settings, telemetry and deliberate push-to-talk lifecycle.
(function () {
    'use strict';

    const prefix = 'desktop.network_cameras.';
    const choices = {
        preview_source: [['auto', 'automatic'], ['main', 'main_stream'], ['sub', 'sub_stream']],
        detection_source: [['auto', 'automatic'], ['main', 'main_stream'], ['sub', 'sub_stream']],
        snapshot_mode: [['auto', 'automatic'], ['camera', 'snapshot_source'], ['stream', 'stream_image']],
        codec_profile: [['original', 'original'], ['h264_opus', 'H.264 / Opus'], ['h264_aac', 'H.264 / AAC']],
        transport: [['inherit', 'automatic'], ['tcp', 'TCP'], ['udp', 'UDP']],
        playback_mode: [['auto', 'automatic'], ['webrtc_tcp', 'WebRTC / TCP'], ['mse', 'MSE'], ['hls', 'HLS'], ['mp4', 'MP4'], ['mjpeg', 'MJPEG']]
    };
    function text(state, key, values) { return state.ctx.t(prefix + key, values || {}); }
    function setText(node, value) { if (node && node.textContent !== value) node.textContent = value; }
    function badge(node, online, label) {
        if (!node) return;
        node.classList.toggle('is-online', online);
        if (node.dataset.label !== label) { node.replaceChildren(document.createElement('span'), document.createTextNode(label)); node.dataset.label = label; }
    }
    function optionText(state, value) { return /[A-Z]/.test(value) ? value : text(state, value); }
    function defaults(stream) {
        const saved = stream.configured_media || stream.media || stream;
        const result = { sub_source: '', snapshot_source: '', clear_sub_source: false, clear_snapshot_source: false };
        for (const key of Object.keys(choices)) result[key] = saved[key] || choices[key][0][0];
        result.preload = !!saved.preload;
        result.talk_enabled = !!saved.talk_enabled;
        return result;
    }
    function patch(modal) {
        const result = {};
        for (const key of Object.keys(choices)) result[key] = modal[key] || choices[key][0][0];
        for (const key of ['preload', 'talk_enabled', 'clear_sub_source', 'clear_snapshot_source']) result[key] = !!modal[key];
        result.sub_source = modal.sub_source || '';
        result.snapshot_source = modal.snapshot_source || '';
        return result;
    }
    function fields(state, modal) {
        const esc = state.ctx.esc;
        let html = '<details class="nc-media-options"><summary>' + esc(text(state, 'media_settings')) + '</summary><div class="nc-form">';
        for (const key of ['sub_source', 'snapshot_source']) {
            html += '<label>' + esc(text(state, key)) + '<input type="password" autocomplete="off" data-field="' + key + '" value="' + esc(modal[key] || '') + '" placeholder="' + esc(text(state, 'keep_source')) + '"></label>';
            html += '<label class="nc-check"><input type="checkbox" data-field="clear_' + key + '" ' + (modal['clear_' + key] ? 'checked' : '') + '><span>' + esc(text(state, 'clear_' + key)) + '</span></label>';
        }
        for (const [key, options] of Object.entries(choices)) {
            html += '<label>' + esc(text(state, key)) + '<select data-field="' + key + '">' + options.map(([value, label]) => '<option value="' + value + '" ' + ((modal[key] || options[0][0]) === value ? 'selected' : '') + '>' + esc(optionText(state, label)) + '</option>').join('') + '</select></label>';
        }
        for (const key of ['preload', 'talk_enabled']) html += '<label class="nc-check"><input type="checkbox" data-field="' + key + '" ' + (modal[key] ? 'checked' : '') + '><span>' + esc(text(state, key)) + '</span></label>';
        return html + '</div></details>';
    }
    function sourceRole(state, stream) { return state.sourceRoles && state.sourceRoles[stream.id] === 'sub' && stream.has_substream ? 'sub' : 'main'; }
    function viewerURL(state, stream, role) { return stream.media_ready === false ? 'about:blank' : '/api/desktop/integrations/go2rtc/viewer/' + encodeURIComponent(stream.id) + '?role=' + encodeURIComponent(role || sourceRole(state, stream)); }
    function toolbar(state, stream) {
        const esc = state.ctx.esc;
        let html = '';
        if (stream.has_substream) html += '<label class="nc-source-choice"><span>' + esc(text(state, 'media_source')) + '</span><select data-role="media-source"><option value="main" ' + (sourceRole(state, stream) === 'main' ? 'selected' : '') + '>' + esc(text(state, 'main_stream')) + '</option><option value="sub" ' + (sourceRole(state, stream) === 'sub' ? 'selected' : '') + '>' + esc(text(state, 'sub_stream')) + '</option></select></label>';
        if (state.data.can_manage) html += '<button type="button" data-media-action="diagnose">' + esc(text(state, 'diagnose')) + '</button>';
        if (stream.media && stream.media.talk_enabled) {
            const reason = !state.data.can_talk ? 'talk_permission' : (stream.talk_busy && (!state.mediaTalk || state.mediaTalk.camera !== stream.id)) ? 'talk_busy' : !state.data.webrtc_enabled ? 'talk_requires_webrtc' : stream.media_ready === false ? 'connecting' : (!window.isSecureContext || !navigator.mediaDevices ? 'talk_https' : '');
            html += '<button class="nc-talk" type="button" data-role="talk" aria-pressed="false" title="' + esc(text(state, reason || 'talk_hold')) + '" ' + (reason ? 'disabled' : '') + '>' + esc(text(state, reason === 'talk_busy' ? 'talk_busy' : 'talk_hold')) + '</button>';
        }
        return html;
    }
    function update(state) {
        const streams = (state.data && state.data.streams) || [];
        const runtime = state.host.querySelector('.nc-runtime');
        if (runtime) { const online = !!(state.data.integration && state.data.integration.api_usable); runtime.classList.toggle('is-offline', !online); badge(runtime, online, text(state, online ? 'online' : 'offline')); }
        if (!state.mediaRevisions) state.mediaRevisions = new Map();
        for (const stream of streams) {
            if (stream.media_ready === false && state.mediaTalk && state.mediaTalk.camera === stream.id) stop(state);
            const revision = String(stream.revision || 0);
            const previous = state.mediaRevisions.get(stream.id);
            state.mediaRevisions.set(stream.id, revision);
            if (previous !== undefined && previous !== revision) {
                if (state.selected === stream.id) stop(state);
                const oldURL = state.objectURLs.get(stream.id);
                if (oldURL) { URL.revokeObjectURL(oldURL); state.objectURLs.delete(stream.id); }
                for (const image of state.host.querySelectorAll('img[data-thumbnail]')) if (image.dataset.thumbnail === stream.id) { image.removeAttribute('src'); image.classList.remove('is-loaded'); }
            }
            const card = Array.from(state.host.querySelectorAll('[data-stream-card]')).find(el => el.dataset.streamCard === stream.id);
            if (card) {
                const status = card.querySelector('.nc-card-state');
                const code = stream.state || (stream.reachable ? 'online' : 'idle');
                badge(status, code === 'online', text(state, ['online', 'connecting', 'idle'].includes(code) ? code : 'offline'));
                const viewers = card.querySelector('.nc-card-meta > span');
                setText(viewers, text(state, 'viewers', { count: Number(stream.consumers || 0) }));
                setText(card.querySelector('.nc-card-copy small'), (stream.codecs || []).join(' · ') || text(state, 'waiting_codec'));
            }
            for (const frame of state.host.querySelectorAll('iframe[data-camera]')) {
                if (frame.dataset.camera !== stream.id) continue;
                if (stream.media_ready === false) {
                    if (frame.getAttribute('src') !== 'about:blank') frame.src = 'about:blank';
                    frame.dataset.pending = 'true';
                    continue;
                }
                if (String(stream.revision || 0) === frame.dataset.revision && !frame.dataset.pending) continue;
                if (state.selected === stream.id) stop(state);
                delete frame.dataset.pending;
                frame.dataset.revision = String(stream.revision || 0);
                frame.src = viewerURL(state, stream, frame.dataset.sourceRole === 'preview' ? 'preview' : sourceRole(state, stream));
            }
        }
        const stream = streams.find(item => item.id === state.selected);
        if (!stream) return;
        const target = state.host.querySelector('[data-role="media-toolbar"]');
        const signature = JSON.stringify([stream.has_substream, stream.media_ready !== false, !!state.data.can_talk, !!state.data.can_manage, !!state.data.webrtc_enabled, stream.media && stream.media.talk_enabled, !!stream.talk_busy && (!state.mediaTalk || state.mediaTalk.camera !== stream.id)]);
        if (target && target.dataset.signature !== signature) { stop(state); target.innerHTML = toolbar(state, stream); target.dataset.signature = signature; bind(state); }
        const stats = state.host.querySelector('[data-role="media-stats"]');
        if (!state.mediaCounters) state.mediaCounters = new Map();
        const previous = state.mediaCounters.get(stream.id);
        const counters = { revision: stream.revision, video: stream.video || {}, audio: stream.audio || {} };
        const traffic = kind => {
            const now = counters[kind], before = previous && previous.revision === stream.revision ? previous[kind] : now;
            const difference = key => Math.max(0, Number(now[key] || 0) - Number(before[key] || 0));
            return '↓' + Number(now.received_bytes || 0) + ' B / ' + Number(now.received_packets || 0) + ' pkt (+' + difference('received_bytes') + ' B / +' + difference('received_packets') + ' pkt) ↑' + Number(now.sent_bytes || 0) + ' B / ' + Number(now.sent_packets || 0) + ' pkt';
        };
        setText(stats, text(state, 'last_image', { time: stream.last_snapshot_at ? new Date(stream.last_snapshot_at).toLocaleString() : '—' }) + ' · V ' + (stream.video_codecs || []).join(', ') + ' ' + traffic('video') + ' · A ' + (stream.audio_codecs || []).join(', ') + ' ' + traffic('audio'));
        state.mediaCounters.set(stream.id, counters);
        setText(state.host.querySelector('[data-role="media-error"]'), stream.last_error ? text(state, stream.last_error === 'camera_snapshot_fallback' ? 'snapshot_fallback' : 'camera_error') : '');
    }
    async function diagnose(state) {
        const id = state.selected;
        const output = state.host.querySelector('[data-role="media-diagnosis"]');
        if (!output) return;
        output.textContent = text(state, 'loading');
        try {
            const result = await state.ctx.api('/api/desktop/integrations/go2rtc/diagnostics/' + encodeURIComponent(id), { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
            if (state.disposed || id !== state.selected || !output.isConnected) return;
            output.textContent = result.status === 'ok' ? text(state, 'diagnostics_success', { width: result.width, height: result.height, ms: result.duration_ms }) : text(state, 'diagnostics_failed');
            if (result.error_code) output.textContent += ' (' + result.error_code + ')';
            if (result.effective_transcoding_backend) output.textContent += ' · ' + (result.effective_transcoding_backend === 'software' ? 'CPU' : result.effective_transcoding_backend);
            if (result.stream) output.textContent += ' · ' + (result.stream.video_codecs || []).join(', ') + ' / ' + (result.stream.audio_codecs || []).join(', ');
        } catch (_) { if (output.isConnected && id === state.selected) output.textContent = text(state, 'diagnostics_failed'); }
    }
    function buttonState(state, active) {
        const button = state.host.querySelector('[data-role="talk"]');
        if (button) { button.setAttribute('aria-pressed', active ? 'true' : 'false'); button.classList.toggle('is-active', active); button.textContent = text(state, active ? 'talk_active' : 'talk_hold'); }
    }
    function stop(state, dispose) {
        const activity = state.mediaTalk;
        state.mediaTalk = null;
        if (activity) {
            activity.held = false;
            if (activity.stream) activity.stream.getTracks().forEach(track => track.stop());
            if (activity.peer) activity.peer.close();
            if (activity.socket) { activity.socket.onopen = null; activity.socket.onclose = null; activity.socket.onmessage = null; activity.socket.onerror = null; activity.socket.close(); }
            if (activity.heartbeat) clearInterval(activity.heartbeat);
            if (activity.session) fetch('/api/go2rtc/talk/' + encodeURIComponent(activity.camera) + '?session=' + encodeURIComponent(activity.session), { method: 'DELETE', credentials: 'same-origin', keepalive: true }).catch(() => {});
        }
        buttonState(state, false);
        if (dispose && state.mediaBlur) { window.removeEventListener('blur', state.mediaBlur); state.mediaBlur = null; }
    }
    async function beginTalk(state) {
        if (state.mediaTalk || state.disposed || !state.visible) return;
        const stream = state.data.streams.find(item => item.id === state.selected);
        if (!stream) return;
        const activity = { held: true, camera: stream.id, revision: stream.revision, stream: null, socket: null, peer: null, session: '', heartbeat: null };
        state.mediaTalk = activity;
        const current = () => state.mediaTalk === activity && activity.held && !state.disposed && state.visible;
        const failed = () => { if (current()) { stop(state); const output = state.host.querySelector('[data-role="media-diagnosis"]'); if (output) output.textContent = text(state, 'talk_failed'); } };
        try {
            activity.stream = await navigator.mediaDevices.getUserMedia({ audio: true });
            if (!current()) { activity.stream.getTracks().forEach(track => track.stop()); return; }
            const created = await state.ctx.api('/api/desktop/integrations/go2rtc/talk/' + encodeURIComponent(activity.camera), { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
            activity.session = created.session;
            if (!current()) { fetch('/api/go2rtc/talk/' + encodeURIComponent(activity.camera) + '?session=' + encodeURIComponent(activity.session), { method: 'DELETE', credentials: 'same-origin', keepalive: true }).catch(() => {}); return; }
            const peer = activity.peer = new RTCPeerConnection({ bundlePolicy: 'max-bundle', iceServers: [] });
            peer.onconnectionstatechange = () => { if (['failed', 'disconnected', 'closed'].includes(peer.connectionState)) failed(); };
            activity.stream.getAudioTracks().forEach(track => peer.addTransceiver(track, { direction: 'sendonly' }));
            const target = new URL(created.ws_url, location.href); target.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
            const socket = activity.socket = new WebSocket(target.href);
            const send = message => { if (current() && socket.readyState === 1) socket.send(JSON.stringify(message)); };
            const candidates = [];
            peer.onicecandidate = event => send({ type: 'webrtc/candidate', value: event.candidate ? event.candidate.candidate : '' });
            socket.onopen = async () => {
                if (!current()) return;
                try {
                    send({ type: 'talk/keepalive' });
                    activity.heartbeat = setInterval(() => send({ type: 'talk/keepalive' }), 1000);
                    const offer = await peer.createOffer(); await peer.setLocalDescription(offer);
                    if (current()) send({ type: 'webrtc/offer', value: offer.sdp });
                }
                catch (_) { failed(); }
            };
            socket.onmessage = async event => {
                if (!current()) return;
                try {
                    const message = JSON.parse(event.data);
                    if (message.type === 'webrtc/answer') { await peer.setRemoteDescription({ type: 'answer', sdp: message.value }); for (const candidate of candidates.splice(0)) await peer.addIceCandidate(candidate); if (current()) buttonState(state, true); }
                    else if (message.type === 'webrtc/candidate' && message.value) { const candidate = { candidate: message.value, sdpMid: '0' }; if (peer.remoteDescription) await peer.addIceCandidate(candidate); else candidates.push(candidate); }
                    else if (message.type === 'error') failed();
                } catch (_) { failed(); }
            };
            socket.onclose = failed;
            socket.onerror = failed;
        } catch (_) {
            failed();
        }
    }
    function bind(state) {
        const controls = state.host.querySelector('[data-role="media-toolbar"]');
        if (!controls) return;
        const select = controls.querySelector('[data-role="media-source"]');
        if (select) select.addEventListener('change', () => { stop(state); state.sourceRoles[state.selected] = select.value; if (state.savePreferences) state.savePreferences(); const stream = state.data.streams.find(item => item.id === state.selected); const frame = state.host.querySelector('.nc-detail-video iframe'); if (frame && stream) frame.src = viewerURL(state, stream); });
        const diagnoseButton = controls.querySelector('[data-media-action="diagnose"]');
        if (diagnoseButton) diagnoseButton.addEventListener('click', () => diagnose(state));
        const button = controls.querySelector('[data-role="talk"]');
        if (button) {
            button.addEventListener('pointerdown', event => { if (event.button !== 0) return; event.preventDefault(); button.setPointerCapture(event.pointerId); beginTalk(state); });
            for (const kind of ['pointerup', 'pointercancel', 'lostpointercapture', 'blur']) button.addEventListener(kind, () => stop(state));
            button.addEventListener('keydown', event => { if (![' ', 'Enter'].includes(event.key)) return; event.preventDefault(); if (!event.repeat) beginTalk(state); });
            button.addEventListener('keyup', event => { if ([' ', 'Enter'].includes(event.key)) { event.preventDefault(); stop(state); } });
        }
        if (!state.mediaBlur) { state.mediaBlur = () => stop(state); window.addEventListener('blur', state.mediaBlur); }
    }
    window.NetworkCamerasMedia = { choices, defaults, patch, fields, toolbar, sourceRole, viewerURL, bind, stop, update, diagnose };
})();
