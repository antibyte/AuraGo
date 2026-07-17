(function () {
    'use strict';

    const Common = window.AuraRealtimeProviderCommon;

    class XAIRealtimeAdapter extends Common.ProviderAdapter {
        constructor(options) {
            super(options);
            this.socket = null;
            this.player = new Common.PCMPlayer(24000);
            this.player.addEventListener('playing', () => this.emit('audio', { active: true }));
            this.player.addEventListener('idle', () => this.emit('audio', { active: false }));
            this.conversationId = '';
            this.userTranscripts = new Map();
            this.assistantTranscripts = new Map();
        }

        async connect(connectOptions) {
            this.closed = false;
            this.setState('connecting');
            const resumeConversationId = connectOptions.conversationId || this.conversationId;
            const resuming = !!resumeConversationId;
            const response = await connectOptions.createSession({
                conversation_id: resumeConversationId
            });
            this.session = response;
            await this.openSocket(response);
            if (!resuming) await this.syncContext(response.context || []);
            this.setState('listening');
            return response;
        }

        openSocket(response) {
            return new Promise((resolve, reject) => {
                const socket = new WebSocket(response.websocket_url, response.websocket_protocol);
                this.socket = socket;
                const timeout = window.setTimeout(() => {
                    try { socket.close(); } catch (_) { }
                    reject(new Error('xAI realtime connection timed out'));
                }, 15000);
                socket.addEventListener('open', () => {
                    window.clearTimeout(timeout);
                    this.connected = true;
                    this.send({ type: 'session.update', session: response.session_config || {} });
                    resolve();
                }, { once: true });
                socket.addEventListener('message', event => this.handleEvent(Common.safeJSON(event.data)));
                socket.addEventListener('error', () => {
                    window.clearTimeout(timeout);
                    if (!this.connected) reject(new Error('xAI realtime connection failed'));
                    else this.fail(new Error('xAI realtime connection failed'));
                });
                socket.addEventListener('close', () => {
                    this.connected = false;
                    if (!this.closed && this.options.state !== 'parked') this.setState('disconnected');
                });
            });
        }

        send(payload) {
            if (!this.socket || this.socket.readyState !== WebSocket.OPEN) return false;
            this.socket.send(JSON.stringify(payload));
            return true;
        }

        sendAudio(samples) {
            if (!this.connected || !samples || !samples.length) return;
            this.send({
                type: 'input_audio_buffer.append',
                audio: Common.floatToBase64PCM16(samples, 16000, 16000)
            });
        }

        endTurn() {
            this.send({ type: 'input_audio_buffer.commit' });
            this.send({ type: 'response.create' });
        }

        sendToolResult(call, result) {
            this.send({
                type: 'conversation.item.create',
                item: {
                    type: 'function_call_output',
                    call_id: call.callId,
                    output: JSON.stringify(result)
                }
            });
            void this.player.whenIdle().then(() => {
                if (this.connected) this.send({ type: 'response.create' });
            });
        }

        interruptOutput() {
            this.send({ type: 'response.cancel' });
            this.player.stop();
            this.responseActive = false;
        }

        async park() {
            this.options.state = 'parked';
            this.player.stop();
            if (this.socket) {
                try { this.socket.close(1000, 'parked'); } catch (_) { }
            }
            this.connected = false;
            this.setState('parked', { conversationId: this.conversationId });
            return { conversationId: this.conversationId };
        }

        async resume(connectOptions) {
            this.options.state = 'connecting';
            return this.connect(Object.assign({}, connectOptions, { conversationId: this.conversationId }));
        }

        async sendContextMessage(message) {
            if (!message || !message.content) return;
            const role = message.role === 'assistant' ? 'assistant' : 'user';
            this.send({
                type: 'conversation.item.create',
                item: {
                    type: 'message',
                    role,
                    content: [{ type: role === 'assistant' ? 'output_text' : 'input_text', text: String(message.content) }]
                }
            });
        }

        appendTranscript(store, id, delta) {
            const next = (store.get(id) || '') + String(delta || '');
            store.set(id, next);
            return next;
        }

        handleEvent(event) {
            if (!event || typeof event !== 'object') return;
            const type = String(event.type || '');
            if (type === 'error') {
                this.fail(new Error((event.error && event.error.message) || 'xAI realtime error'));
                return;
            }
            const conversationId = event.conversation_id ||
                (event.conversation && event.conversation.id) ||
                (event.session && event.session.conversation_id);
            if (conversationId) {
                this.conversationId = conversationId;
                this.emit('state', { state: 'metadata', conversationId });
            }
            if (type === 'response.created') {
                this.responseActive = true;
                this.emit('audio', { active: true, pending: true });
            }
            if (type === 'response.done' || type === 'response.cancelled') {
                this.responseActive = false;
                this.emit('usage', { usage: event.response && event.response.usage });
            }
            if (type === 'response.output_audio.delta' || type === 'response.audio.delta') {
                const audio = event.delta || event.audio;
                if (audio) void this.player.appendBase64PCM16(audio, 24000).catch(error => this.fail(error));
            }

            if (type.includes('input_audio_transcription.updated')) {
                const id = event.item_id || 'user';
                const text = String(event.transcript || event.text || event.delta || '');
                this.userTranscripts.set(id, text);
                this.transcript('user', text, false, id);
            } else if (type.includes('input_audio_transcription.delta')) {
                const id = event.item_id || 'user';
                this.transcript('user', this.appendTranscript(this.userTranscripts, id, event.delta), false, id);
            } else if (type.includes('input_audio_transcription.completed')) {
                const id = event.item_id || 'user';
                const text = String(event.transcript || this.userTranscripts.get(id) || '');
                this.userTranscripts.delete(id);
                this.transcript('user', text, true, id);
            } else if (type.includes('output_audio_transcript.delta') || type.includes('output_audio_transcription.delta')) {
                const id = event.item_id || event.response_id || 'assistant';
                this.transcript('assistant', this.appendTranscript(this.assistantTranscripts, id, event.delta), false, id);
            } else if (type.includes('output_audio_transcript.done') || type.includes('output_audio_transcription.completed')) {
                const id = event.item_id || event.response_id || 'assistant';
                const text = String(event.transcript || this.assistantTranscripts.get(id) || '');
                this.assistantTranscripts.delete(id);
                this.transcript('assistant', text, true, id);
            }

            if (type === 'response.function_call_arguments.done') {
                this.emit('toolCall', {
                    name: event.name,
                    arguments: Common.safeJSON(event.arguments) || {},
                    callId: event.call_id || event.item_id || Common.randomID('call')
                });
            }
        }

        async close() {
            this.closed = true;
            this.connected = false;
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
    window.AuraRealtimeProviders.xai = XAIRealtimeAdapter;
})();
