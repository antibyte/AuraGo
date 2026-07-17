(function () {
    'use strict';

    const Common = window.AuraRealtimeProviderCommon;
    const MAX_WAKE_SAMPLES = (window.AuraRealtimeAudio && window.AuraRealtimeAudio.constants.WAKE_BUFFER_SAMPLES) || 48000;
    const CLIENT_ID_KEY = 'aurago.realtimeSpeech.clientId.v1';
    const CHANNEL_NAME = 'aurago-realtime-speech-v1';
    const PEER_PROBE_TIMEOUT_MS = 500;

    function text(key, fallback, vars) {
        let value = typeof window.t === 'function' ? window.t(key, vars) : '';
        if (!value || value === key) value = fallback || key;
        if (vars && typeof value === 'string') {
            Object.keys(vars).forEach(name => {
                value = value.replaceAll('{{' + name + '}}', String(vars[name]));
            });
        }
        return value;
    }

    function clientID() {
        let value = '';
        try { value = localStorage.getItem(CLIENT_ID_KEY) || ''; } catch (_) { }
        if (!value) {
            value = Common.randomID('browser');
            try { localStorage.setItem(CLIENT_ID_KEY, value); } catch (_) { }
        }
        return value;
    }

    function activeWebSessionID() {
        if (typeof window.activeSessionId === 'function') {
            try {
                const value = window.activeSessionId();
                if (value) return value;
            } catch (_) { }
        }
        try { return localStorage.getItem('aurago-session-id') || 'default'; } catch (_) { return 'default'; }
    }

    async function readError(response) {
        const contentType = response.headers.get('content-type') || '';
        try {
            if (contentType.includes('application/json')) {
                const body = await response.json();
                const error = new Error(body.error || body.message || ('HTTP ' + response.status));
                error.status = response.status;
                error.body = body;
                return error;
            }
            const body = await response.text();
            const error = new Error(body || ('HTTP ' + response.status));
            error.status = response.status;
            return error;
        } catch (_) {
            const error = new Error('HTTP ' + response.status);
            error.status = response.status;
            return error;
        }
    }

    function cleanTranscript(value) {
        return String(value || '')
            .replace(/<\/?(?:thinking|think)>/gi, '')
            .replace(/<done\s*\/?>/gi, '')
            .replace(/\u0000/g, '')
            .trim();
    }

    function eventContent(payload) {
        if (!payload || typeof payload !== 'object') return '';
        const choices = Array.isArray(payload.choices) ? payload.choices : [];
        if (choices[0] && choices[0].delta && typeof choices[0].delta.content === 'string') {
            return choices[0].delta.content;
        }
        if (payload.event === 'llm_stream_delta' && typeof payload.content === 'string') return payload.content;
        return '';
    }

    function finalEventContent(payload) {
        if (!payload || typeof payload !== 'object') return '';
        if (payload.event === 'final_response') return String(payload.detail || payload.content || '');
        if (payload.event === 'done' && typeof payload.detail === 'string') return payload.detail;
        return '';
    }

    class RealtimeSpeechRuntime extends EventTarget {
        constructor() {
            super();
            this.clientId = clientID();
            this.tabId = Common.randomID('tab');
            this.channel = typeof BroadcastChannel === 'function' ? new BroadcastChannel(CHANNEL_NAME) : null;
            this.config = null;
            this.catalog = null;
            this.profile = null;
            this.surface = 'webchat';
            this.chatSessionId = '';
            this.sessionId = '';
            this.adapter = null;
            this.audioGate = null;
            this.state = 'idle';
            this.muted = false;
            this.userSpeaking = false;
            this.providerSpeaking = false;
            this.actionActive = false;
            this.currentAction = null;
            this.lastActivityAt = Date.now();
            this.parkTimer = null;
            this.wakeFrames = [];
            this.wakeSamples = 0;
            this.wakeExpired = false;
            this.wakeDeadline = null;
            this.pendingEndTurn = false;
            this.resumePromise = null;
            this.pendingUserTranscript = '';
            this.pendingUserTurnId = '';
            this.pendingAssistantTranscript = '';
            this.directFinalizeTimer = null;
            this.toolTurn = false;
            this.suppressNextAssistantChat = false;
            this.usage = null;
            this.boundVisibleMessage = event => this.syncVisibleMessage(event);

            if (this.channel) this.channel.addEventListener('message', event => this.handleChannelMessage(event.data));
            window.addEventListener('aurago:chat-visible-message', this.boundVisibleMessage);
            window.addEventListener('beforeunload', () => {
                if (this.sessionId) {
                    const url = '/api/realtime-speech/sessions/' + encodeURIComponent(this.sessionId) +
                        '?client_id=' + encodeURIComponent(this.clientId);
                    try {
                        void fetch(url, {
                            method: 'DELETE',
                            credentials: 'same-origin',
                            keepalive: true,
                            headers: { 'X-Realtime-Speech-Client-ID': this.clientId }
                        });
                    } catch (_) { }
                }
                void this.stop({ notifyServer: false });
            });
        }

        emit(type, detail) {
            this.dispatchEvent(new CustomEvent(type, { detail: detail || {} }));
        }

        setState(state, detail) {
            this.state = state;
            this.emit('state', Object.assign({
                state,
                active: !['idle', 'closed'].includes(state),
                muted: this.muted,
                profile: this.profile
            }, detail || {}));
            this.schedulePark();
            if (this.sessionId) void this.syncServerState(state);
        }

        async syncServerState(state, telemetry) {
            if (!this.sessionId) return;
            const allowed = new Set(['connecting', 'listening', 'speaking', 'executing', 'parked', 'reconnecting', 'error']);
            if (!allowed.has(state)) return;
            try {
                await fetch('/api/realtime-speech/sessions/' + encodeURIComponent(this.sessionId) +
                    '?client_id=' + encodeURIComponent(this.clientId), {
                    method: 'PATCH',
                    credentials: 'same-origin',
                    cache: 'no-store',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-Realtime-Speech-Client-ID': this.clientId
                    },
                    body: JSON.stringify(Object.assign({
                        state,
                        conversation_id: this.adapter && this.adapter.conversationId || '',
                        resumption_handle: this.adapter && this.adapter.resumptionHandle || ''
                    }, telemetry || {}))
                });
            } catch (_) { }
        }

        touch() {
            this.lastActivityAt = Date.now();
            this.schedulePark();
        }

        async initialize(force) {
            if (!force && this.config && this.catalog) return { config: this.config, catalog: this.catalog };
            const [configResponse, catalogResponse] = await Promise.all([
                fetch('/api/realtime-speech/config', { credentials: 'same-origin', cache: 'no-store' }),
                fetch('/api/realtime-speech/catalog', { credentials: 'same-origin', cache: 'no-store' })
            ]);
            if (!configResponse.ok) throw await readError(configResponse);
            if (!catalogResponse.ok) throw await readError(catalogResponse);
            this.config = await configResponse.json();
            this.catalog = await catalogResponse.json();
            this.emit('config', { config: this.config, catalog: this.catalog });
            return { config: this.config, catalog: this.catalog };
        }

        profileByID(id) {
            const profiles = (this.config && this.config.profiles) || [];
            return profiles.find(profile => profile.id === id) || null;
        }

        async start(options) {
            options = options || {};
            await this.initialize();
            if (!this.config.enabled) throw new Error(text('chat.realtime_disabled', 'Realtime Speech is disabled.'));
            if (this.sessionId) {
                if (options.surface) {
                    this.surface = options.surface === 'desktop' ? 'desktop' : 'webchat';
                    this.chatSessionId = this.surface === 'desktop' ? 'virtual-desktop' : (options.chatSessionId || activeWebSessionID());
                }
                this.emit('state', { state: this.state, active: true, muted: this.muted, profile: this.profile });
                return;
            }

            const profileId = options.profileId || this.config.default_profile;
            const profile = this.profileByID(profileId);
            if (!profile || !profile.enabled) throw new Error(text('chat.realtime_profile_unavailable', 'The selected live speech profile is unavailable.'));
            if (!profile.api_key_set) throw new Error(text('chat.realtime_profile_key_missing', 'The selected profile has no API key.'));
            const Adapter = window.AuraRealtimeProviders && window.AuraRealtimeProviders[profile.provider];
            if (!Adapter) throw new Error('Unsupported realtime speech provider: ' + profile.provider);

            this.surface = options.surface === 'desktop' ? 'desktop' : 'webchat';
            this.chatSessionId = this.surface === 'desktop' ? 'virtual-desktop' : (options.chatSessionId || activeWebSessionID());
            this.profile = profile;
            this.setState('connecting');
            this.adapter = new Adapter({ profile, state: 'connecting' });
            this.bindAdapter(this.adapter);
            this.audioGate = new window.AuraRealtimeAudio.RealtimeAudioGate();
            this.bindAudioGate(this.audioGate);

            try {
                await this.audioGate.start();
                await this.connectAdapter(!!options.takeover);
                this.channelPost({ type: 'active', sessionId: this.sessionId });
                this.touch();
            } catch (error) {
                if (error && error.status === 409 && error.body && error.body.takeover_available && !options.takeover) {
                    const conflictSessionId = String(error.body.active_session_id || '');
                    const peerActive = await this.probeActiveSession(conflictSessionId);
                    const takeOver = peerActive === false ? true : await this.confirmTakeover();
                    if (takeOver) {
                        await this.stop({ notifyServer: false });
                        this.channelPost({ type: 'takeover', sessionId: conflictSessionId });
                        return this.start(Object.assign({}, options, { takeover: true }));
                    }
                }
                await this.stop();
                this.setState('error', { error, message: error.message });
                throw error;
            }
        }

        async connectAdapter(takeover) {
            const response = await this.adapter.connect({
                createSession: async extra => {
                    const created = await this.createSession(extra, takeover);
                    this.sessionId = created.session_id;
                    return created;
                },
                conversationId: '',
                resumptionHandle: ''
            });
            this.sessionId = response.session_id;
            this.setState('listening');
            return response;
        }

        sessionPayload(extra, takeover) {
            return Object.assign({
                session_id: this.sessionId,
                client_id: this.clientId,
                profile_id: this.profile && this.profile.id,
                surface: this.surface,
                chat_session_id: this.chatSessionId,
                takeover: !!takeover,
                state: this.state === 'parked' ? 'parked' : 'connecting'
            }, extra || {});
        }

        async createSession(extra, takeover) {
            const response = await fetch('/api/realtime-speech/sessions', {
                method: 'POST',
                credentials: 'same-origin',
                cache: 'no-store',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Realtime-Speech-Client-ID': this.clientId,
                    'X-Session-ID': this.chatSessionId
                },
                body: JSON.stringify(this.sessionPayload(extra, takeover))
            });
            if (!response.ok) throw await readError(response);
            return response.json();
        }

        bindAdapter(adapter) {
            adapter.addEventListener('state', event => {
                const detail = event.detail || {};
                if (detail.state === 'metadata') return;
                if (detail.state === 'disconnected' && this.state !== 'parked' && this.state !== 'closed') {
                    this.setState('error', { message: text('chat.realtime_connection_lost', 'The live speech connection was interrupted.') });
                    return;
                }
                if (detail.state && detail.state !== 'closed') this.setState(detail.state, detail);
            });
            adapter.addEventListener('transcript', event => this.handleTranscript(event.detail || {}));
            adapter.addEventListener('audio', event => {
                const active = !!(event.detail && event.detail.active);
                this.providerSpeaking = active;
                if (active && !this.actionActive) this.setState('speaking');
                else if (!active && !this.userSpeaking && !this.actionActive && this.state !== 'parked') this.setState('listening');
                this.touch();
            });
            adapter.addEventListener('toolCall', event => void this.handleToolCall(event.detail || {}));
            adapter.addEventListener('usage', event => {
                this.usage = event.detail && event.detail.usage;
                this.emit('usage', event.detail || {});
                if (this.sessionId && this.usage && typeof this.usage === 'object') {
                    void this.syncServerState(this.state, { usage: this.usage });
                }
            });
            adapter.addEventListener('error', event => {
                const error = event.detail && event.detail.error ? event.detail.error : new Error('Realtime provider error');
                this.setState('error', { error, message: error.message });
            });
        }

        bindAudioGate(gate) {
            gate.addEventListener('speechstart', event => void this.handleSpeechStart(event.detail.audio));
            gate.addEventListener('audio', event => {
                const detail = event.detail || {};
                if (detail.speech === true) this.handleAudio(detail.audio);
            });
            gate.addEventListener('speechend', () => this.handleSpeechEnd());
            gate.addEventListener('error', event => {
                const error = event.detail && event.detail.error;
                this.emit('error', { error, message: error && error.message });
            });
        }

        async handleSpeechStart(audio) {
            if (this.muted || !this.adapter) return;
            const needsResume = this.state === 'parked' || !this.adapter.connected;
            this.userSpeaking = true;
            this.touch();
            if (this.providerSpeaking) {
                this.adapter.interruptOutput();
                this.providerSpeaking = false;
            }
            if (needsResume) {
                this.appendWakeAudio(audio);
                await this.resumeProvider();
                return;
            }
            this.setState(this.actionActive ? 'executing' : 'listening', { userSpeaking: true });
            this.adapter.sendAudio(audio);
        }

        handleAudio(audio) {
            if (this.muted || !this.adapter || !audio || !audio.length) return;
            this.touch();
            if (this.resumePromise || !this.adapter.connected) {
                this.appendWakeAudio(audio);
                return;
            }
            this.adapter.sendAudio(audio);
        }

        handleSpeechEnd() {
            this.userSpeaking = false;
            this.touch();
            if (this.resumePromise || !this.adapter || !this.adapter.connected) {
                this.pendingEndTurn = true;
                return;
            }
            this.adapter.endTurn();
            if (!this.actionActive) this.setState('listening');
        }

        appendWakeAudio(audio) {
            if (!audio || !audio.length || this.wakeExpired) return;
            const remaining = MAX_WAKE_SAMPLES - this.wakeSamples;
            if (remaining <= 0) return;
            const frame = audio.length > remaining ? audio.slice(0, remaining) : audio.slice();
            this.wakeFrames.push(frame);
            this.wakeSamples += frame.length;
        }

        beginWakeDeadline() {
            window.clearTimeout(this.wakeDeadline);
            this.wakeExpired = false;
            this.wakeDeadline = window.setTimeout(() => {
                if (!this.resumePromise) return;
                this.wakeExpired = true;
                this.wakeFrames = [];
                this.wakeSamples = 0;
                this.emit('repeat', {
                    message: text('chat.realtime_repeat_after_reconnect', 'I could not reconnect in time. Please repeat what you said.')
                });
            }, 3000);
        }

        async resumeProvider() {
            if (this.resumePromise) return this.resumePromise;
            const wakeStartedAt = Date.now();
            this.setState('reconnecting');
            this.beginWakeDeadline();
            this.resumePromise = (async () => {
                try {
                    await this.adapter.resume({
                        createSession: extra => this.createSession(extra, false)
                    });
                    window.clearTimeout(this.wakeDeadline);
                    if (!this.wakeExpired) {
                        this.wakeFrames.forEach(frame => this.adapter.sendAudio(frame));
                        if (this.pendingEndTurn) this.adapter.endTurn();
                    }
                    this.wakeFrames = [];
                    this.wakeSamples = 0;
                    this.pendingEndTurn = false;
                    this.setState(this.userSpeaking ? 'listening' : 'listening');
                    void this.syncServerState('listening', {
                        wake_latency_ms: Math.max(0, Date.now() - wakeStartedAt)
                    });
                    this.touch();
                } catch (error) {
                    this.setState('error', { error, message: error.message });
                    this.emit('repeat', {
                        message: text('chat.realtime_repeat_after_reconnect', 'I could not reconnect in time. Please repeat what you said.')
                    });
                    throw error;
                } finally {
                    this.resumePromise = null;
                }
            })();
            return this.resumePromise;
        }

        schedulePark() {
            window.clearTimeout(this.parkTimer);
            this.parkTimer = null;
            if (!this.sessionId || !this.adapter || this.state !== 'listening') return;
            if (this.userSpeaking || this.providerSpeaking || this.actionActive || this.resumePromise) return;
            const seconds = Math.max(5, Math.min(60, Number((this.config && this.config.park_after_seconds) || 5)));
            const delay = Math.max(0, seconds * 1000 - (Date.now() - this.lastActivityAt));
            this.parkTimer = window.setTimeout(() => void this.park(), delay);
        }

        async park() {
            if (!this.adapter || this.userSpeaking || this.providerSpeaking || this.actionActive || this.state !== 'listening') return;
            try {
                await this.adapter.park();
                this.setState('parked', { microphoneLocal: true });
            } catch (error) {
                this.setState('error', { error, message: error.message });
            }
        }

        setMuted(muted) {
            this.muted = !!muted;
            if (this.audioGate) this.audioGate.setMuted(this.muted);
            this.emit('mute', { muted: this.muted });
            this.emit('state', { state: this.state, active: !!this.sessionId, muted: this.muted, profile: this.profile });
        }

        handleTranscript(detail) {
            const speaker = detail.speaker === 'assistant' ? 'assistant' : 'user';
            const transcript = cleanTranscript(detail.text);
            this.emit('transcript', Object.assign({}, detail, { speaker, text: transcript }));
            if (!detail.final || !transcript) return;
            this.touch();
            if (speaker === 'user') {
                this.pendingUserTranscript = transcript;
                this.pendingUserTurnId = detail.turnId || Common.randomID('turn');
                this.emitDisplay('user', transcript, 'direct');
                return;
            }
            this.pendingAssistantTranscript = transcript;
            if (this.suppressNextAssistantChat || this.toolTurn || this.actionActive) {
                this.suppressNextAssistantChat = false;
                this.pendingAssistantTranscript = '';
                this.toolTurn = false;
                return;
            }
            window.clearTimeout(this.directFinalizeTimer);
            this.directFinalizeTimer = window.setTimeout(() => void this.finalizeDirectTurn(), 350);
        }

        async finalizeDirectTurn() {
            const user = this.pendingUserTranscript;
            const assistant = this.pendingAssistantTranscript;
            if (!user || !assistant || this.toolTurn || this.actionActive || this.suppressNextAssistantChat) return;
            const turnId = this.pendingUserTurnId || Common.randomID('turn');
            this.pendingUserTranscript = '';
            this.pendingUserTurnId = '';
            this.pendingAssistantTranscript = '';
            this.emitDisplay('assistant', assistant, 'direct');
            try {
                const response = await fetch('/api/realtime-speech/turns', {
                    method: 'POST',
                    credentials: 'same-origin',
                    cache: 'no-store',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-Realtime-Speech-Client-ID': this.clientId
                    },
                    body: JSON.stringify({
                        session_id: this.sessionId,
                        client_id: this.clientId,
                        turn_id: turnId,
                        user_transcript: user,
                        assistant_transcript: assistant
                    })
                });
                if (!response.ok) throw await readError(response);
            } catch (error) {
                this.emit('error', { error, message: error.message });
            }
        }

        emitDisplay(role, content, kind) {
            const detail = {
                role,
                content,
                kind: kind || 'direct',
                source: 'realtime-speech',
                surface: this.surface,
                chatSessionId: this.chatSessionId
            };
            this.emit('display', detail);
            window.dispatchEvent(new CustomEvent('aurago:realtime-speech-display', { detail }));
        }

        async handleToolCall(call) {
            window.clearTimeout(this.directFinalizeTimer);
            this.directFinalizeTimer = null;
            this.pendingAssistantTranscript = '';
            this.toolTurn = true;
            if (call.name === 'aurago_cancel_current_task') {
                const result = await this.cancelCurrentAction();
                this.adapter.sendToolResult(call, result);
                return;
            }
            if (call.name !== 'aurago_execute') {
                this.adapter.sendToolResult(call, {
                    status: 'error',
                    error: 'Only AuraGo actions are available in this speech session.'
                });
                return;
            }
            const request = cleanTranscript(call.arguments && call.arguments.request);
            if (!request) {
                this.adapter.sendToolResult(call, { status: 'error', error: 'The AuraGo request was empty.' });
                return;
            }
            const result = await this.executeAction(request);
            this.suppressNextAssistantChat = true;
            this.adapter.sendToolResult(call, result);
        }

        async executeAction(request) {
            const requestId = Common.randomID('voice-action');
            const action = { requestId, request, cancelled: false };
            this.actionActive = true;
            this.currentAction = action;
            this.setState('executing', { requestId });
            this.emit('action', { phase: 'started', requestId, request });
            let resultText = '';
            let finalText = '';
            let status = 'completed';
            try {
                const response = await fetch('/api/realtime-speech/actions', {
                    method: 'POST',
                    credentials: 'same-origin',
                    cache: 'no-store',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-Realtime-Speech-Client-ID': this.clientId,
                        'X-Session-ID': this.chatSessionId
                    },
                    body: JSON.stringify({
                        session_id: this.sessionId,
                        client_id: this.clientId,
                        request_id: requestId,
                        request
                    })
                });
                if (!response.ok) throw await readError(response);
                const reader = response.body && response.body.getReader();
                if (!reader) throw new Error('AuraGo action stream is unavailable');
                const decoder = new TextDecoder();
                let buffer = '';
                while (true) {
                    const chunk = await reader.read();
                    buffer += decoder.decode(chunk.value || new Uint8Array(), { stream: !chunk.done });
                    const blocks = buffer.split(/\r?\n\r?\n/);
                    buffer = blocks.pop() || '';
                    blocks.forEach(block => {
                        const data = block.split(/\r?\n/)
                            .filter(line => line.startsWith('data:'))
                            .map(line => line.slice(5).trim())
                            .join('\n');
                        if (!data || data === '[DONE]') return;
                        const payload = Common.safeJSON(data);
                        if (!payload) return;
                        const delta = eventContent(payload);
                        if (delta) resultText += delta;
                        const final = finalEventContent(payload);
                        if (final) finalText = final;
                        const eventName = String(payload.event || '').toLowerCase();
                        if (eventName.includes('question') || eventName.includes('needs_input')) status = 'needs_input';
                        else if (eventName.includes('cancel') || eventName.includes('interrupt')) status = 'cancelled';
                        else if (eventName.includes('error') || eventName.includes('failed')) status = 'error';
                        this.emit('action', { phase: 'event', requestId, event: payload });
                    });
                    if (chunk.done) break;
                }
                if (action.cancelled) status = 'cancelled';
                const displayText = cleanTranscript(finalText || resultText);
                if (status === 'cancelled' && !displayText) {
                    this.emit('action', { phase: 'cancelled', requestId, status });
                    return {
                        status,
                        request_id: requestId,
                        text: 'The current AuraGo task was cancelled.',
                        artifacts: []
                    };
                }
                if (!displayText) throw new Error('AuraGo completed without a displayable result');
                this.emitDisplay('assistant', displayText, 'action');
                this.emit('action', { phase: 'completed', requestId, status, text: displayText });
                return {
                    status,
                    request_id: requestId,
                    text: displayText.length > 8000 ? displayText.slice(0, 8000) + '\n\nThe complete result is visible in the chat.' : displayText,
                    artifacts: []
                };
            } catch (error) {
                status = action.cancelled || (error && error.name === 'AbortError') ? 'cancelled' : 'error';
                const message = cleanTranscript(error && error.message) || 'AuraGo action failed';
                this.emit('action', { phase: status, requestId, status, error: message });
                return { status, request_id: requestId, error: message, artifacts: [] };
            } finally {
                this.actionActive = false;
                this.currentAction = null;
                if (this.sessionId && this.state !== 'parked') this.setState(this.providerSpeaking ? 'speaking' : 'listening');
                this.touch();
            }
        }

        async cancelCurrentAction() {
            const action = this.currentAction;
            if (!action) return { status: 'cancelled', request_id: '', text: 'There is no active AuraGo task.' };
            action.cancelled = true;
            try {
                const response = await fetch('/api/realtime-speech/actions/' + encodeURIComponent(action.requestId) +
                    '?client_id=' + encodeURIComponent(this.clientId), {
                    method: 'DELETE',
                    credentials: 'same-origin',
                    cache: 'no-store',
                    headers: { 'X-Realtime-Speech-Client-ID': this.clientId }
                });
                if (!response.ok) throw await readError(response);
                return { status: 'cancelled', request_id: action.requestId, text: 'The current task was cancelled.' };
            } catch (error) {
                action.cancelled = false;
                return { status: 'error', request_id: action.requestId, error: error.message };
            }
        }

        async syncVisibleMessage(event) {
            const detail = event && event.detail;
            if (!detail || detail.source === 'realtime-speech' || !this.adapter || !this.adapter.connected) return;
            if (detail.surface && detail.surface !== this.surface) return;
            const role = detail.role === 'assistant' || detail.role === 'agent' || detail.role === 'bot' ? 'assistant' : 'user';
            const content = cleanTranscript(detail.content || detail.text);
            if (!content) return;
            try {
                await this.adapter.sendContextMessage({ role, content });
            } catch (error) {
                this.emit('error', { error, message: error.message });
            }
        }

        channelPost(payload) {
            if (!this.channel) return;
            try { this.channel.postMessage(Object.assign({ tabId: this.tabId }, payload || {})); } catch (_) { }
        }

        probeActiveSession(sessionId) {
            const targetSessionId = String(sessionId || '');
            if (!targetSessionId) return Promise.resolve(false);
            if (!this.channel) return Promise.resolve(null);
            return new Promise(resolve => {
                let settled = false;
                let timer = null;
                const finish = active => {
                    if (settled) return;
                    settled = true;
                    window.clearTimeout(timer);
                    this.channel.removeEventListener('message', onMessage);
                    resolve(active);
                };
                const onMessage = event => {
                    const message = event && event.data;
                    if (!message || message.tabId === this.tabId || message.type !== 'active') return;
                    if (String(message.sessionId || '') !== targetSessionId) return;
                    finish(true);
                };
                this.channel.addEventListener('message', onMessage);
                timer = window.setTimeout(() => finish(false), PEER_PROBE_TIMEOUT_MS);
                this.channelPost({ type: 'probe', sessionId: targetSessionId });
            });
        }

        handleChannelMessage(message) {
            if (!message || message.tabId === this.tabId) return;
            if (message.type === 'takeover' && this.sessionId &&
                (!message.sessionId || message.sessionId === this.sessionId)) {
                void this.stop({ notifyServer: false });
                this.emit('takeover', {
                    message: text('chat.realtime_taken_over', 'The microphone session was moved to another tab.')
                });
            }
            if (message.type === 'probe' && this.sessionId) this.channelPost({ type: 'active', sessionId: this.sessionId });
        }

        confirmTakeover() {
            return new Promise(resolve => {
                const overlay = document.createElement('div');
                overlay.className = 'realtime-speech-modal-backdrop';
                overlay.innerHTML = `<div class="realtime-speech-modal" role="dialog" aria-modal="true" aria-labelledby="realtime-speech-takeover-title">
                    <h3 id="realtime-speech-takeover-title">${text('chat.realtime_takeover_title', 'Microphone already in use')}</h3>
                    <p>${text('chat.realtime_takeover_message', 'Realtime Speech is active in another AuraGo tab. Move the microphone session here?')}</p>
                    <div class="realtime-speech-modal-actions">
                        <button type="button" data-cancel>${text('common.btn_cancel', 'Cancel')}</button>
                        <button type="button" class="primary" data-confirm>${text('chat.realtime_takeover', 'Move here')}</button>
                    </div>
                </div>`;
                document.body.appendChild(overlay);
                const finish = value => {
                    overlay.remove();
                    resolve(value);
                };
                overlay.querySelector('[data-cancel]').addEventListener('click', () => finish(false));
                overlay.querySelector('[data-confirm]').addEventListener('click', () => finish(true));
                overlay.addEventListener('click', event => {
                    if (event.target === overlay) finish(false);
                });
                overlay.querySelector('[data-confirm]').focus();
            });
        }

        async stop(options) {
            options = Object.assign({ notifyServer: true }, options || {});
            window.clearTimeout(this.parkTimer);
            window.clearTimeout(this.wakeDeadline);
            window.clearTimeout(this.directFinalizeTimer);
            const sessionId = this.sessionId;
            const adapter = this.adapter;
            const gate = this.audioGate;
            this.sessionId = '';
            this.adapter = null;
            this.audioGate = null;
            this.actionActive = false;
            this.currentAction = null;
            this.userSpeaking = false;
            this.providerSpeaking = false;
            this.resumePromise = null;
            this.wakeFrames = [];
            this.wakeSamples = 0;
            if (adapter) {
                try { await adapter.close(); } catch (_) { }
            }
            if (gate) {
                try { await gate.stop(); } catch (_) { }
            }
            if (options.notifyServer && sessionId) {
                try {
                    await fetch('/api/realtime-speech/sessions/' + encodeURIComponent(sessionId) +
                        '?client_id=' + encodeURIComponent(this.clientId), {
                        method: 'DELETE',
                        credentials: 'same-origin',
                        cache: 'no-store',
                        headers: { 'X-Realtime-Speech-Client-ID': this.clientId },
                        keepalive: true
                    });
                } catch (_) { }
            }
            this.profile = null;
            this.setState('idle');
            this.channelPost({ type: 'stopped', sessionId });
        }
    }

    const runtime = new RealtimeSpeechRuntime();
    window.AuraRealtimeSpeech = runtime;
    window.AuraRealtimeSpeechRuntime = RealtimeSpeechRuntime;
})();
