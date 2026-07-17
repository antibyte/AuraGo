(function () {
    'use strict';

    const Common = window.AuraRealtimeProviderCommon;

    function waitForDataChannel(channel, timeoutMs) {
        if (channel.readyState === 'open') return Promise.resolve();
        return new Promise((resolve, reject) => {
            const timeout = window.setTimeout(() => reject(new Error('OpenAI realtime data channel timed out')), timeoutMs || 15000);
            channel.addEventListener('open', () => {
                window.clearTimeout(timeout);
                resolve();
            }, { once: true });
            channel.addEventListener('error', () => {
                window.clearTimeout(timeout);
                reject(new Error('OpenAI realtime data channel failed'));
            }, { once: true });
        });
    }

    function waitForICE(peer, timeoutMs) {
        if (peer.iceGatheringState === 'complete') return Promise.resolve();
        return new Promise(resolve => {
            const timeout = window.setTimeout(done, timeoutMs || 3000);
            function done() {
                window.clearTimeout(timeout);
                peer.removeEventListener('icegatheringstatechange', change);
                resolve();
            }
            function change() {
                if (peer.iceGatheringState === 'complete') done();
            }
            peer.addEventListener('icegatheringstatechange', change);
        });
    }

    class OpenAIRealtimeAdapter extends Common.ProviderAdapter {
        constructor(options) {
            super(options);
            this.peer = null;
            this.channel = null;
            this.remoteAudio = null;
            this.userTranscripts = new Map();
            this.assistantTranscripts = new Map();
        }

        async connect(connectOptions) {
            this.closed = false;
            this.setState('connecting');
            this.peer = new RTCPeerConnection();
            this.peer.addTransceiver('audio', { direction: 'recvonly' });
            this.channel = this.peer.createDataChannel('oai-events');
            this.channel.addEventListener('message', event => this.handleEvent(Common.safeJSON(event.data)));
            this.channel.addEventListener('close', () => {
                this.connected = false;
                if (!this.closed) this.setState('disconnected');
            });
            this.peer.addEventListener('connectionstatechange', () => {
                if (this.peer && ['failed', 'disconnected'].includes(this.peer.connectionState) && !this.closed) {
                    this.fail(new Error('OpenAI realtime connection was interrupted'));
                }
            });
            this.peer.addEventListener('track', event => {
                const stream = event.streams && event.streams[0] ? event.streams[0] : new MediaStream([event.track]);
                this.remoteAudio = document.createElement('audio');
                this.remoteAudio.autoplay = true;
                this.remoteAudio.srcObject = stream;
                void this.remoteAudio.play().catch(() => { });
            });

            const offer = await this.peer.createOffer();
            await this.peer.setLocalDescription(offer);
            await waitForICE(this.peer);
            const response = await connectOptions.createSession({
                offer_sdp: this.peer.localDescription && this.peer.localDescription.sdp
            });
            this.session = response;
            await this.peer.setRemoteDescription({ type: 'answer', sdp: response.answer_sdp });
            await waitForDataChannel(this.channel);
            this.connected = true;
            await this.syncContext(response.context || []);
            this.setState('listening');
            return response;
        }

        send(payload) {
            if (!this.channel || this.channel.readyState !== 'open') return false;
            this.channel.send(JSON.stringify(payload));
            return true;
        }

        sendAudio(samples) {
            if (!this.connected || !samples || !samples.length) return;
            this.send({
                type: 'input_audio_buffer.append',
                audio: Common.floatToBase64PCM16(samples, 16000, 24000)
            });
        }

        endTurn() {
            if (!this.connected) return;
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
            this.send({ type: 'response.create' });
        }

        interruptOutput() {
            this.send({ type: 'response.cancel' });
            this.send({ type: 'output_audio_buffer.clear' });
            if (this.remoteAudio) {
                try { this.remoteAudio.pause(); } catch (_) { }
            }
            this.responseActive = false;
            this.emit('audio', { active: false });
        }

        async park() {
            this.setState('parked', { warm: true });
        }

        async resume() {
            if (!this.connected) throw new Error('The OpenAI realtime connection is no longer available');
            if (this.remoteAudio) void this.remoteAudio.play().catch(() => { });
            this.setState('listening');
            return this.session;
        }

        async sendContextMessage(message) {
            if (!message || !message.content) return;
            const role = message.role === 'assistant' ? 'assistant' : 'user';
            this.send({
                type: 'conversation.item.create',
                item: {
                    type: 'message',
                    role,
                    content: [{
                        type: role === 'assistant' ? 'output_text' : 'input_text',
                        text: String(message.content)
                    }]
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
                this.fail(new Error((event.error && event.error.message) || 'OpenAI realtime error'));
                return;
            }
            if (type === 'response.created') {
                this.responseActive = true;
                this.emit('audio', { active: true, pending: true });
            }
            if (type === 'response.done' || type === 'response.cancelled') {
                this.responseActive = false;
                this.emit('audio', { active: false });
                this.emit('usage', { usage: event.response && event.response.usage });
            }

            if (type.includes('input_audio_transcription.delta')) {
                const id = event.item_id || event.content_index || 'user';
                const text = this.appendTranscript(this.userTranscripts, id, event.delta);
                this.transcript('user', text, false, id);
            } else if (type.includes('input_audio_transcription.completed')) {
                const id = event.item_id || event.content_index || 'user';
                const text = String(event.transcript || this.userTranscripts.get(id) || '');
                this.userTranscripts.delete(id);
                this.transcript('user', text, true, id);
            } else if (type.includes('output_audio_transcript.delta') || type.includes('output_audio_transcription.delta')) {
                const id = event.item_id || event.response_id || 'assistant';
                const text = this.appendTranscript(this.assistantTranscripts, id, event.delta);
                this.transcript('assistant', text, false, id);
            } else if (type.includes('output_audio_transcript.done') || type.includes('output_audio_transcription.completed')) {
                const id = event.item_id || event.response_id || 'assistant';
                const text = String(event.transcript || this.assistantTranscripts.get(id) || '');
                this.assistantTranscripts.delete(id);
                this.transcript('assistant', text, true, id);
            }

            if (type === 'response.function_call_arguments.done') {
                const args = Common.safeJSON(event.arguments) || {};
                this.emit('toolCall', {
                    name: event.name,
                    arguments: args,
                    callId: event.call_id || event.item_id || Common.randomID('call')
                });
            }
        }

        async close() {
            this.closed = true;
            this.connected = false;
            if (this.channel) {
                try { this.channel.close(); } catch (_) { }
            }
            if (this.peer) {
                try { this.peer.close(); } catch (_) { }
            }
            if (this.remoteAudio) {
                try { this.remoteAudio.pause(); } catch (_) { }
                this.remoteAudio.srcObject = null;
            }
            this.channel = null;
            this.peer = null;
            this.remoteAudio = null;
            this.setState('closed');
        }
    }

    window.AuraRealtimeProviders = window.AuraRealtimeProviders || {};
    window.AuraRealtimeProviders.openai = OpenAIRealtimeAdapter;
})();
