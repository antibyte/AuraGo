(function () {
    'use strict';

    const Common = window.AuraRealtimeProviderCommon;

    function tokenizedURL(baseURL, token) {
        const separator = String(baseURL).includes('?') ? '&' : '?';
        return String(baseURL) + separator + 'access_token=' + encodeURIComponent(token);
    }

    async function decodeGeminiMessage(data) {
        let json = data;
        if (typeof Blob !== 'undefined' && data instanceof Blob) {
            json = await data.text();
        } else if (data instanceof ArrayBuffer) {
            json = new TextDecoder().decode(data);
        } else if (ArrayBuffer.isView(data)) {
            json = new TextDecoder().decode(data);
        }
        return Common.safeJSON(json);
    }

    class GeminiRealtimeAdapter extends Common.ProviderAdapter {
        constructor(options) {
            super(options);
            this.socket = null;
            this.player = new Common.PCMPlayer(24000);
            this.player.addEventListener('playing', () => this.emit('audio', { active: true }));
            this.player.addEventListener('idle', () => this.emit('audio', { active: false }));
            this.resumptionHandle = '';
            this.activityActive = false;
            this.userTranscript = '';
            this.assistantTranscript = '';
            this.turnId = '';
            this.turnCompleteSeen = false;
            this.turnFinalizeTimer = null;
        }

        async connect(connectOptions) {
            this.closed = false;
            this.setState('connecting');
            const resumeHandle = connectOptions.resumptionHandle || this.resumptionHandle;
            const resuming = !!resumeHandle;
            const response = await connectOptions.createSession({
                resumption_handle: resumeHandle
            });
            this.session = response;
            await this.openSocket(response);
            if (!resuming) await this.syncContext(response.context || []);
            this.setState('listening');
            return response;
        }

        openSocket(response) {
            return new Promise((resolve, reject) => {
                const socket = new WebSocket(tokenizedURL(response.websocket_url, response.access_token));
                socket.binaryType = 'arraybuffer';
                this.socket = socket;
                let setupComplete = false;
                let promiseSettled = false;
                let timeout = 0;
                const rejectBeforeSetup = error => {
                    if (setupComplete || promiseSettled) return;
                    promiseSettled = true;
                    window.clearTimeout(timeout);
                    reject(error);
                };
                const resolveSetup = () => {
                    if (promiseSettled) return;
                    setupComplete = true;
                    promiseSettled = true;
                    window.clearTimeout(timeout);
                    resolve();
                };
                timeout = window.setTimeout(() => {
                    rejectBeforeSetup(new Error('Gemini Live connection timed out'));
                    try { socket.close(); } catch (_) { }
                }, 15000);
                socket.addEventListener('open', () => {
                    this.connected = true;
                    this.send({ setup: response.setup || {} });
                }, { once: true });
                socket.addEventListener('message', event => {
                    void decodeGeminiMessage(event.data).then(payload => {
                        if (!payload) {
                            rejectBeforeSetup(new Error('Gemini Live returned an invalid setup response'));
                            return;
                        }
                        if (!setupComplete && payload.error) {
                            const message = String(payload.error.message || payload.error.status || 'session setup was rejected').trim();
                            rejectBeforeSetup(new Error('Gemini Live rejected session setup: ' + message));
                            try { socket.close(); } catch (_) { }
                            return;
                        }
                        if (!setupComplete && (payload.setupComplete || payload.setup_complete)) {
                            resolveSetup();
                        }
                        this.handleEvent(payload);
                    }).catch(error => {
                        const detail = error instanceof Error ? error.message : String(error || 'invalid response');
                        rejectBeforeSetup(new Error('Gemini Live response could not be decoded: ' + detail));
                    });
                });
                socket.addEventListener('error', () => {
                    if (!setupComplete) rejectBeforeSetup(new Error('Gemini Live connection failed'));
                    else this.fail(new Error('Gemini Live connection failed'));
                });
                socket.addEventListener('close', event => {
                    this.connected = false;
                    if (!setupComplete) {
                        const reason = String(event.reason || '').trim().slice(0, 500);
                        const detail = reason ? ': ' + reason : (event.code ? ' (code ' + event.code + ')' : '');
                        rejectBeforeSetup(new Error('Gemini Live closed before setup completed' + detail));
                    }
                    if (!this.closed && this.options.state !== 'parked') this.setState('disconnected');
                });
            });
        }

        send(payload) {
            if (!this.socket || this.socket.readyState !== WebSocket.OPEN) return false;
            this.socket.send(JSON.stringify(payload));
            return true;
        }

        startActivity() {
            if (this.activityActive) return;
            this.activityActive = true;
            this.send({ realtimeInput: { activityStart: {} } });
        }

        sendAudio(samples) {
            if (!this.connected || !samples || !samples.length) return;
            this.startActivity();
            this.send({
                realtimeInput: {
                    audio: {
                        data: Common.floatToBase64PCM16(samples, 16000, 16000),
                        mimeType: 'audio/pcm;rate=16000'
                    }
                }
            });
        }

        endTurn() {
            if (!this.activityActive) return;
            this.activityActive = false;
            this.send({ realtimeInput: { activityEnd: {} } });
        }

        sendToolResult(call, result) {
            this.send({
                toolResponse: {
                    functionResponses: [{
                        id: call.callId,
                        name: call.name,
                        response: result
                    }]
                }
            });
        }

        interruptOutput() {
            this.player.stop();
            this.startActivity();
            this.responseActive = false;
        }

        async park() {
            this.options.state = 'parked';
            this.player.stop();
            if (this.socket) {
                try { this.socket.close(1000, 'parked'); } catch (_) { }
            }
            this.connected = false;
            this.setState('parked', { resumptionHandle: this.resumptionHandle });
            return { resumptionHandle: this.resumptionHandle };
        }

        async resume(connectOptions) {
            this.options.state = 'connecting';
            return this.connect(Object.assign({}, connectOptions, { resumptionHandle: this.resumptionHandle }));
        }

        async sendContextMessage(message) {
            if (!message || !message.content) return;
            const role = message.role === 'assistant' ? 'model' : 'user';
            this.send({
                clientContent: {
                    turns: [{
                        role,
                        parts: [{ text: String(message.content) }]
                    }],
                    turnComplete: false
                }
            });
        }

        scheduleTurnFinalize() {
            window.clearTimeout(this.turnFinalizeTimer);
            this.turnFinalizeTimer = window.setTimeout(() => this.finalizeTurnTranscripts(), 500);
        }

        finalizeTurnTranscripts() {
            window.clearTimeout(this.turnFinalizeTimer);
            this.turnFinalizeTimer = null;
            if (this.userTranscript) this.transcript('user', this.userTranscript, true, this.turnId || 'user');
            if (this.assistantTranscript) this.transcript('assistant', this.assistantTranscript, true, this.turnId || 'assistant');
            this.userTranscript = '';
            this.assistantTranscript = '';
            this.turnId = Common.randomID('turn');
            this.turnCompleteSeen = false;
            this.responseActive = false;
        }

        handleEvent(event) {
            if (!event || typeof event !== 'object') return;
            if (event.error) {
                this.fail(new Error(event.error.message || 'Gemini Live error'));
                return;
            }
            const update = event.sessionResumptionUpdate || event.session_resumption_update;
            const handle = update && (update.newHandle || update.new_handle);
            if (handle) {
                this.resumptionHandle = handle;
                this.emit('state', { state: 'metadata', resumptionHandle: handle });
            }

            const content = event.serverContent || event.server_content;
            if (content) {
                const input = content.inputTranscription || content.input_transcription;
                const output = content.outputTranscription || content.output_transcription;
                if (input && input.text) {
                    this.userTranscript += String(input.text);
                    this.transcript('user', this.userTranscript, false, this.turnId || 'user');
                }
                if (output && output.text) {
                    this.assistantTranscript += String(output.text);
                    this.transcript('assistant', this.assistantTranscript, false, this.turnId || 'assistant');
                }
                const parts = (content.modelTurn && content.modelTurn.parts) ||
                    (content.model_turn && content.model_turn.parts) || [];
                parts.forEach(part => {
                    const inline = part.inlineData || part.inline_data;
                    if (inline && inline.data && String(inline.mimeType || inline.mime_type || '').startsWith('audio/')) {
                        this.responseActive = true;
                        void this.player.appendBase64PCM16(inline.data, 24000).catch(error => this.fail(error));
                    }
                    if (part.text && !output) {
                        this.assistantTranscript += String(part.text);
                        this.transcript('assistant', this.assistantTranscript, false, this.turnId || 'assistant');
                    }
                });
                if (content.interrupted) {
                    this.player.stop();
                    this.responseActive = false;
                }
                if (content.turnComplete || content.turn_complete) {
                    this.turnCompleteSeen = true;
                }
                if (this.turnCompleteSeen) this.scheduleTurnFinalize();
            }

            const toolCall = event.toolCall || event.tool_call;
            const calls = (toolCall && (toolCall.functionCalls || toolCall.function_calls)) || [];
            calls.forEach(call => {
                this.emit('toolCall', {
                    name: call.name,
                    arguments: call.args || call.arguments || {},
                    callId: call.id || Common.randomID('call')
                });
            });
            const usage = event.usageMetadata || event.usage_metadata;
            if (usage) this.emit('usage', { usage });
        }

        async close() {
            this.closed = true;
            this.connected = false;
            window.clearTimeout(this.turnFinalizeTimer);
            this.turnFinalizeTimer = null;
            this.player.stop();
            if (this.socket) {
                try { this.socket.close(); } catch (_) { }
            }
            this.socket = null;
            await this.player.close();
            this.setState('closed');
        }
    }

    window.AuraRealtimeProviders = window.AuraRealtimeProviders || {};
    window.AuraRealtimeProviders.gemini = GeminiRealtimeAdapter;
})();
