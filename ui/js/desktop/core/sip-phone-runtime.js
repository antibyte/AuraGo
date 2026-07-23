    const sipPhoneShellState = {
        initialized: false,
        appState: null,
        phase: 'loading',
        error: '',
        eventSource: null,
        peerConnection: null,
        localStream: null,
        localSender: null,
        remoteAudio: null,
        browserSessionID: '',
        callID: '',
        muted: false,
        devices: { inputs: [], outputs: [] },
        lease: null,
        incomingNoticeCallID: '',
        informationalNoticeCallID: '',
        reconnectTimer: null,
        durationTimer: null,
        ringTimer: null,
        ringContext: null,
        ringOscillator: null,
        listeners: new Set()
    };

    const sipPhonePreferenceKey = 'aurago.sip-phone.preferences.v1';
    const sipPhoneClientKey = 'aurago.sip-phone.client-id.v1';

    function sipPhoneText(key, fallback, params) {
        const translationKey = 'desktop.sip_phone_' + key;
        const value = t(translationKey, params || {});
        return value === translationKey ? fallback : value;
    }

    function sipPhonePreferences() {
        const defaults = { input_device: '', output_device: '', volume: 0.82, ringtone_enabled: true, favorites: [] };
        try {
            const stored = JSON.parse(localStorage.getItem(sipPhonePreferenceKey) || '{}');
            const result = Object.assign({}, defaults, stored || {});
            result.volume = Math.max(0, Math.min(1, Number(result.volume)));
            result.favorites = Array.isArray(result.favorites) ? result.favorites.slice(0, 24) : [];
            return result;
        } catch (_) {
            return defaults;
        }
    }

    function saveSIPPhonePreferences(patch) {
        const next = Object.assign({}, sipPhonePreferences(), patch || {});
        next.favorites = Array.isArray(next.favorites) ? next.favorites.slice(0, 24) : [];
        localStorage.setItem(sipPhonePreferenceKey, JSON.stringify(next));
        sipPhoneEmit();
        return next;
    }

    function sipPhoneClientID() {
        let value = '';
        try { value = sessionStorage.getItem(sipPhoneClientKey) || ''; } catch (_) {}
        if (!value) {
            if (window.crypto && typeof window.crypto.randomUUID === 'function') value = window.crypto.randomUUID();
            else {
                const bytes = new Uint8Array(16);
                window.crypto.getRandomValues(bytes);
                value = Array.from(bytes, item => item.toString(16).padStart(2, '0')).join('');
            }
            try { sessionStorage.setItem(sipPhoneClientKey, value); } catch (_) {}
        }
        return value;
    }

    function sipPhoneSnapshot() {
        const appState = sipPhoneShellState.appState || {};
        return {
            appState,
            phase: sipPhoneShellState.phase,
            error: sipPhoneShellState.error,
            call: appState.active_call || null,
            muted: sipPhoneShellState.muted,
            mediaOwned: !!sipPhoneShellState.peerConnection,
            observer: !!(appState.active_call && !sipPhoneShellState.peerConnection),
            devices: {
                inputs: sipPhoneShellState.devices.inputs.slice(),
                outputs: sipPhoneShellState.devices.outputs.slice()
            },
            preferences: sipPhonePreferences(),
            secureContext: !!(window.isSecureContext || ['localhost', '127.0.0.1', '::1'].includes(location.hostname)),
            sinkSelectionSupported: !!(HTMLMediaElement.prototype && HTMLMediaElement.prototype.setSinkId)
        };
    }

    function sipPhoneEmit() {
        const snapshot = sipPhoneSnapshot();
        sipPhoneShellState.listeners.forEach(listener => {
            try { listener(snapshot); } catch (_) {}
        });
        window.dispatchEvent(new CustomEvent('aurago:sip-phone-state', { detail: snapshot }));
        renderSIPPhoneShellChrome(snapshot);
    }

    async function sipPhoneRequest(path, options) {
        const response = await fetch(path, Object.assign({
            credentials: 'same-origin',
            cache: 'no-store'
        }, options || {}));
        const body = await response.json().catch(() => ({}));
        if (!response.ok) {
            const error = new Error(body.error || body.message || ('HTTP ' + response.status));
            error.status = response.status;
            throw error;
        }
        return body;
    }

    async function refreshSIPPhoneState() {
        try {
            const appState = await sipPhoneRequest('/api/sip/app/state');
            const previousCall = sipPhoneShellState.appState && sipPhoneShellState.appState.active_call;
            sipPhoneShellState.appState = appState;
            const call = appState.active_call;
            if (!call) {
                sipPhoneShellState.phase = 'idle';
                sipPhoneShellState.callID = '';
                stopSIPPhoneRinging();
                removeSIPPhoneIncomingNotice();
                if (previousCall && sipPhoneShellState.peerConnection) teardownSIPPhoneMedia(false);
            } else {
                sipPhoneShellState.callID = call.id;
                sipPhoneShellState.phase = call.state || 'connecting';
                if (call.state === 'ringing') {
                    handleSIPPhoneIncomingCall(call, appState.inbound_route);
                } else {
                    stopSIPPhoneRinging();
                    removeSIPPhoneIncomingNotice();
                }
            }
            syncSIPPhoneMicrophoneTrack();
            sipPhoneShellState.error = '';
        } catch (error) {
            sipPhoneShellState.error = error.message || String(error);
            sipPhoneShellState.phase = 'unavailable';
        }
        sipPhoneEmit();
        return sipPhoneSnapshot();
    }

    function connectSIPPhoneEvents() {
        if (sipPhoneShellState.eventSource) sipPhoneShellState.eventSource.close();
        const source = new EventSource('/api/sip/events', { withCredentials: true });
        sipPhoneShellState.eventSource = source;
        source.addEventListener('open', () => {
            refreshSIPPhoneState();
        });
        source.addEventListener('sip', () => {
            refreshSIPPhoneState();
        });
        source.addEventListener('error', () => {
            if (sipPhoneShellState.eventSource !== source) return;
            sipPhoneShellState.phase = sipPhoneShellState.appState ? sipPhoneShellState.phase : 'reconnecting';
            sipPhoneEmit();
        });
    }

    function canonicalSIPPhoneTarget(rawTarget, domain) {
        let target = String(rawTarget || '').trim();
        const dialDomain = String(domain || '').trim().toLowerCase();
        if (!target) throw new Error(sipPhoneText('error_target_required', 'Enter a SIP number or name.'));
        if (/[\r\n\x00]/.test(target)) throw new Error(sipPhoneText('error_target_invalid', 'This destination is not valid.'));
        if (/^sips:/i.test(target) || /[;?]/.test(target)) throw new Error(sipPhoneText('error_target_invalid', 'This destination is not valid.'));
        if (/^sip:/i.test(target)) target = target.slice(4);
        if (target.includes('@')) {
            const match = target.match(/^([A-Za-z0-9_.!~*'()%+\-]+)@([A-Za-z0-9.\-\[\]:]+)$/);
            if (!match) throw new Error(sipPhoneText('error_target_invalid', 'This destination is not valid.'));
            return 'sip:' + match[1] + '@' + match[2].toLowerCase();
        }
        if (!dialDomain) throw new Error(sipPhoneText('error_domain_missing', 'No SIP dial domain is configured.'));
        if (/^[+\d\s().-]+$/.test(target)) target = target.replace(/[\s().-]/g, '');
        if (!/^[A-Za-z0-9_.!~*'()%+\-]+$/.test(target)) {
            throw new Error(sipPhoneText('error_target_invalid', 'This destination is not valid.'));
        }
        return 'sip:' + target + '@' + dialDomain;
    }

    function secureSIPPhoneContext() {
        return !!(window.isSecureContext || ['localhost', '127.0.0.1', '::1'].includes(location.hostname));
    }

    async function waitForSIPPhoneICE(peerConnection) {
        if (peerConnection.iceGatheringState === 'complete') return;
        await new Promise((resolve, reject) => {
            const timeout = setTimeout(() => {
                peerConnection.removeEventListener('icegatheringstatechange', changed);
                reject(new Error(sipPhoneText('error_ice_timeout', 'Browser media negotiation timed out.')));
            }, 8000);
            function changed() {
                if (peerConnection.iceGatheringState !== 'complete') return;
                clearTimeout(timeout);
                peerConnection.removeEventListener('icegatheringstatechange', changed);
                resolve();
            }
            peerConnection.addEventListener('icegatheringstatechange', changed);
        });
    }

    async function refreshSIPPhoneDevices() {
        if (!navigator.mediaDevices || !navigator.mediaDevices.enumerateDevices) return;
        const devices = await navigator.mediaDevices.enumerateDevices();
        sipPhoneShellState.devices.inputs = devices.filter(device => device.kind === 'audioinput');
        sipPhoneShellState.devices.outputs = devices.filter(device => device.kind === 'audiooutput');
        sipPhoneEmit();
    }

    async function prepareSIPPhoneMedia() {
        if (!secureSIPPhoneContext()) throw new Error(sipPhoneText('error_insecure_context', 'Microphone access requires HTTPS or localhost.'));
        if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia || !window.RTCPeerConnection) {
            throw new Error(sipPhoneText('error_media_unsupported', 'This browser cannot provide telephone audio.'));
        }
        if (sipPhoneShellState.peerConnection) return;
        const audioLease = window.AuraBrowserAudioLease;
        if (!audioLease) throw new Error(sipPhoneText('error_audio_lease', 'The shared audio session is unavailable.'));
        try {
            sipPhoneShellState.lease = await audioLease.acquire('sip-phone', 'sip-phone:' + sipPhoneClientID());
        } catch (error) {
            if (error && error.code === 'audio_session_busy') {
                throw new Error(sipPhoneText('error_audio_busy', 'Live Speech or another telephone tab is using the microphone.'));
            }
            throw error;
        }

        const preferences = sipPhonePreferences();
        const constraints = {
            audio: preferences.input_device
                ? { deviceId: { exact: preferences.input_device }, channelCount: 1, echoCancellation: true, noiseSuppression: true }
                : { channelCount: 1, echoCancellation: true, noiseSuppression: true },
            video: false
        };
        let pendingLocalStream = null;
        let pendingPeerConnection = null;
        let pendingRemoteAudio = null;
        try {
            try {
                pendingLocalStream = await navigator.mediaDevices.getUserMedia(constraints);
            } catch (error) {
                if (error && ['NotAllowedError', 'NotFoundError', 'SecurityError'].includes(error.name)) {
                    throw new Error(sipPhoneText('error_microphone', 'Microphone access was denied or no input device is available.'));
                }
                throw error;
            }
            pendingPeerConnection = new RTCPeerConnection({ iceServers: [] });
            const track = pendingLocalStream.getAudioTracks()[0];
            if (!track) throw new Error(sipPhoneText('error_microphone', 'Microphone access was denied or no input device is available.'));
            // The SDP offer needs a microphone track, but no microphone audio
            // may leave the browser before the SIP dialog is active.
            track.enabled = false;
            const sender = pendingPeerConnection.addTrack(track, pendingLocalStream);
            const transceiver = pendingPeerConnection.getTransceivers().find(item => item.sender === sender);
            if (transceiver && typeof transceiver.setCodecPreferences === 'function' && window.RTCRtpReceiver) {
                const capabilities = RTCRtpReceiver.getCapabilities('audio');
                const pcmu = capabilities && capabilities.codecs
                    ? capabilities.codecs.filter(codec => String(codec.mimeType || '').toLowerCase() === 'audio/pcmu' && Number(codec.clockRate) === 8000)
                    : [];
                if (pcmu.length) transceiver.setCodecPreferences(pcmu);
            }
            pendingRemoteAudio = new Audio();
            pendingRemoteAudio.autoplay = true;
            pendingRemoteAudio.playsInline = true;
            pendingRemoteAudio.volume = preferences.volume;
            pendingPeerConnection.addEventListener('track', event => {
                pendingRemoteAudio.srcObject = event.streams && event.streams[0]
                    ? event.streams[0]
                    : new MediaStream([event.track]);
                pendingRemoteAudio.play().catch(() => {});
            });
            pendingPeerConnection.addEventListener('connectionstatechange', () => {
                if (['failed', 'closed'].includes(pendingPeerConnection.connectionState) && sipPhoneShellState.peerConnection === pendingPeerConnection) {
                    sipPhoneShellState.error = sipPhoneText('error_media_disconnected', 'The browser audio connection ended.');
                    sipPhoneEmit();
                }
            });
            if (preferences.output_device && typeof pendingRemoteAudio.setSinkId === 'function') {
                await pendingRemoteAudio.setSinkId(preferences.output_device).catch(() => {});
            }
            await refreshSIPPhoneDevices();

            const offer = await pendingPeerConnection.createOffer();
            await pendingPeerConnection.setLocalDescription(offer);
            await waitForSIPPhoneICE(pendingPeerConnection);
            const session = await sipPhoneRequest('/api/sip/browser-media/sessions', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    client_id: sipPhoneClientID(),
                    offer_sdp: pendingPeerConnection.localDescription.sdp
                })
            });
            sipPhoneShellState.browserSessionID = session.session_id;
            await pendingPeerConnection.setRemoteDescription({ type: 'answer', sdp: session.answer_sdp });
            sipPhoneShellState.peerConnection = pendingPeerConnection;
            sipPhoneShellState.localStream = pendingLocalStream;
            sipPhoneShellState.localSender = sender;
            sipPhoneShellState.remoteAudio = pendingRemoteAudio;
            syncSIPPhoneMicrophoneTrack();
        } catch (error) {
            disposeSIPPhoneMediaResources(pendingPeerConnection, pendingLocalStream, pendingRemoteAudio);
            teardownSIPPhoneMedia(true);
            throw error;
        }
    }

    async function dialSIPPhone(target) {
        const appState = sipPhoneShellState.appState || await refreshSIPPhoneState().then(value => value.appState);
        const canonicalTarget = canonicalSIPPhoneTarget(target, appState.dial_domain);
        sipPhoneShellState.phase = 'preparing';
        sipPhoneShellState.error = '';
        sipPhoneEmit();
        try {
            await prepareSIPPhoneMedia();
            const call = await sipPhoneRequest('/api/sip/calls', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-SIP-Client-ID': sipPhoneClientID()
                },
                body: JSON.stringify({
                    target: canonicalTarget,
                    media_mode: 'browser',
                    browser_session_id: sipPhoneShellState.browserSessionID
                })
            });
            sipPhoneShellState.callID = call.id;
            sipPhoneShellState.phase = call.state || 'connecting';
            await refreshSIPPhoneState();
            return call;
        } catch (error) {
            sipPhoneShellState.error = error.message || String(error);
            sipPhoneShellState.phase = 'idle';
            teardownSIPPhoneMedia(true);
            sipPhoneEmit();
            throw error;
        }
    }

    async function answerSIPPhone(callID) {
        const id = String(callID || (sipPhoneShellState.appState && sipPhoneShellState.appState.active_call && sipPhoneShellState.appState.active_call.id) || '');
        if (!id) throw new Error(sipPhoneText('error_call_missing', 'The incoming call is no longer available.'));
        sipPhoneShellState.phase = 'preparing';
        sipPhoneShellState.error = '';
        stopSIPPhoneRinging();
        sipPhoneEmit();
        try {
            await prepareSIPPhoneMedia();
            await sipPhoneRequest('/api/sip/calls/' + encodeURIComponent(id) + '/answer', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-SIP-Client-ID': sipPhoneClientID()
                },
                body: JSON.stringify({ browser_session_id: sipPhoneShellState.browserSessionID })
            });
            sipPhoneShellState.callID = id;
            removeSIPPhoneIncomingNotice();
            openApp('sip-phone');
            await refreshSIPPhoneState();
        } catch (error) {
            const message = error.message || String(error);
            teardownSIPPhoneMedia(true);
            await refreshSIPPhoneState();
            const currentCall = sipPhoneShellState.appState && sipPhoneShellState.appState.active_call;
            if (currentCall && currentCall.id === id && currentCall.state === 'ringing') startSIPPhoneRinging();
            sipPhoneShellState.error = message;
            sipPhoneEmit();
            throw error;
        }
    }

    async function rejectSIPPhone(callID) {
        const id = String(callID || (sipPhoneShellState.appState && sipPhoneShellState.appState.active_call && sipPhoneShellState.appState.active_call.id) || '');
        if (!id) return;
        await sipPhoneRequest('/api/sip/calls/' + encodeURIComponent(id) + '/reject', { method: 'POST' });
        stopSIPPhoneRinging();
        removeSIPPhoneIncomingNotice();
        await refreshSIPPhoneState();
    }

    async function hangupSIPPhone() {
        const id = sipPhoneShellState.callID || (sipPhoneShellState.appState && sipPhoneShellState.appState.active_call && sipPhoneShellState.appState.active_call.id);
        if (id) {
            await sipPhoneRequest('/api/sip/calls/' + encodeURIComponent(id) + '/hangup', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: '{}'
            }).catch(error => {
                if (error.status !== 404) throw error;
            });
        }
        teardownSIPPhoneMedia(true);
        await refreshSIPPhoneState();
    }

    async function sendSIPPhoneDTMF(digit) {
        const id = sipPhoneShellState.callID || (sipPhoneShellState.appState && sipPhoneShellState.appState.active_call && sipPhoneShellState.appState.active_call.id);
        if (!id || !/^[0-9*#ABCD]$/.test(String(digit || ''))) return;
        await sipPhoneRequest('/api/sip/calls/' + encodeURIComponent(id) + '/dtmf', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ digit: String(digit) })
        });
    }

    function setSIPPhoneMuted(muted) {
        sipPhoneShellState.muted = !!muted;
        syncSIPPhoneMicrophoneTrack();
        sipPhoneEmit();
    }

    function syncSIPPhoneMicrophoneTrack() {
        const call = sipPhoneShellState.appState && sipPhoneShellState.appState.active_call;
        const microphoneEnabled = !!(
            sipPhoneShellState.peerConnection &&
            call &&
            call.id === sipPhoneShellState.callID &&
            call.state === 'active' &&
            !sipPhoneShellState.muted
        );
        if (sipPhoneShellState.localStream) {
            sipPhoneShellState.localStream.getAudioTracks().forEach(track => { track.enabled = microphoneEnabled; });
        }
    }

    async function setSIPPhoneInputDevice(deviceID) {
        const id = String(deviceID || '');
        if (!sipPhoneShellState.peerConnection || !sipPhoneShellState.localSender) {
            saveSIPPhonePreferences({ input_device: id });
            return;
        }
        let replacement = null;
        let replaced = false;
        try {
            replacement = await navigator.mediaDevices.getUserMedia({
                audio: id ? { deviceId: { exact: id }, channelCount: 1, echoCancellation: true, noiseSuppression: true } : true,
                video: false
            });
            const nextTrack = replacement.getAudioTracks()[0];
            if (!nextTrack) throw new Error(sipPhoneText('error_microphone', 'Microphone access was denied or no input device is available.'));
            nextTrack.enabled = false;
            const previous = sipPhoneShellState.localStream;
            await sipPhoneShellState.localSender.replaceTrack(nextTrack);
            replaced = true;
            sipPhoneShellState.localStream = replacement;
            syncSIPPhoneMicrophoneTrack();
            if (previous) previous.getTracks().forEach(track => track.stop());
            saveSIPPhonePreferences({ input_device: id });
            await refreshSIPPhoneDevices();
        } catch (error) {
            if (!replaced && replacement) replacement.getTracks().forEach(track => track.stop());
            throw error;
        }
    }

    async function setSIPPhoneOutputDevice(deviceID) {
        const id = String(deviceID || '');
        if (sipPhoneShellState.remoteAudio && typeof sipPhoneShellState.remoteAudio.setSinkId === 'function') {
            await sipPhoneShellState.remoteAudio.setSinkId(id);
        }
        saveSIPPhonePreferences({ output_device: id });
    }

    function setSIPPhoneVolume(value) {
        const volume = Math.max(0, Math.min(1, Number(value)));
        saveSIPPhonePreferences({ volume });
        if (sipPhoneShellState.remoteAudio) sipPhoneShellState.remoteAudio.volume = volume;
    }

    function teardownSIPPhoneMedia(deleteSession) {
        const sessionID = sipPhoneShellState.browserSessionID;
        const peerConnection = sipPhoneShellState.peerConnection;
        const localStream = sipPhoneShellState.localStream;
        const remoteAudio = sipPhoneShellState.remoteAudio;
        if (deleteSession && sessionID) {
            fetch('/api/sip/browser-media/sessions/' + encodeURIComponent(sessionID), {
                method: 'DELETE',
                credentials: 'same-origin',
                cache: 'no-store',
                keepalive: true,
                headers: { 'X-SIP-Client-ID': sipPhoneClientID() }
            }).catch(() => {});
        }
        sipPhoneShellState.browserSessionID = '';
        sipPhoneShellState.peerConnection = null;
        sipPhoneShellState.localStream = null;
        sipPhoneShellState.localSender = null;
        sipPhoneShellState.remoteAudio = null;
        disposeSIPPhoneMediaResources(peerConnection, localStream, remoteAudio);
        sipPhoneShellState.muted = false;
        if (sipPhoneShellState.lease && window.AuraBrowserAudioLease) {
            window.AuraBrowserAudioLease.release(sipPhoneShellState.lease.token);
        }
        sipPhoneShellState.lease = null;
        sipPhoneEmit();
    }

    function disposeSIPPhoneMediaResources(peerConnection, localStream, remoteAudio) {
        if (peerConnection) {
            try { peerConnection.close(); } catch (_) {}
        }
        if (localStream) {
            localStream.getTracks().forEach(track => track.stop());
        }
        if (remoteAudio) {
            try { remoteAudio.pause(); } catch (_) {}
            remoteAudio.srcObject = null;
        }
    }

    function handleSIPPhoneIncomingCall(call, route) {
        if (!call || !call.id) return;
        if (route !== 'manual') {
            if (sipPhoneShellState.informationalNoticeCallID !== call.id) {
                sipPhoneShellState.informationalNoticeCallID = call.id;
                showDesktopNotification({
                    title: sipPhoneText('incoming_title', 'Incoming call'),
                    message: sipPhoneText('managed_incoming', 'This call is handled by the configured SIP route.'),
                    duration: 7000
                });
            }
            return;
        }
        if (sipPhoneShellState.incomingNoticeCallID === call.id) return;
        sipPhoneShellState.incomingNoticeCallID = call.id;
        showSIPPhoneIncomingNotice(call);
        startSIPPhoneRinging();
    }

    function showSIPPhoneIncomingNotice(call) {
        const previous = document.getElementById('vd-sip-incoming');
        if (previous) previous.remove();
        sipPhoneShellState.incomingNoticeCallID = call.id;
        const notice = document.createElement('section');
        notice.id = 'vd-sip-incoming';
        notice.className = 'vd-sip-incoming';
        notice.setAttribute('role', 'dialog');
        notice.setAttribute('aria-live', 'assertive');
        notice.innerHTML = `
            <div class="vd-sip-incoming-icon">${iconMarkup('phone', 'P', 'vd-sip-glyph', 24)}</div>
            <div class="vd-sip-incoming-copy">
                <strong>${esc(sipPhoneText('incoming_title', 'Incoming call'))}</strong>
                <span>${esc(call.remote_party || sipPhoneText('unknown_party', 'Unknown caller'))}</span>
            </div>
            <div class="vd-sip-incoming-actions">
                <button type="button" class="is-answer" data-sip-notice="answer">${esc(sipPhoneText('answer', 'Answer'))}</button>
                <button type="button" class="is-reject" data-sip-notice="reject">${esc(sipPhoneText('reject', 'Reject'))}</button>
                <button type="button" class="is-open" data-sip-notice="open">${esc(sipPhoneText('open_phone', 'Open Phone'))}</button>
            </div>`;
        document.body.appendChild(notice);
        notice.querySelector('[data-sip-notice="answer"]').addEventListener('click', () => {
            answerSIPPhone(call.id).catch(error => {
                showDesktopNotification({ title: sipPhoneText('app_name', 'Phone'), message: error.message });
            });
        });
        notice.querySelector('[data-sip-notice="reject"]').addEventListener('click', () => {
            rejectSIPPhone(call.id).catch(error => {
                showDesktopNotification({ title: sipPhoneText('app_name', 'Phone'), message: error.message });
            });
        });
        notice.querySelector('[data-sip-notice="open"]').addEventListener('click', () => openApp('sip-phone'));
    }

    function removeSIPPhoneIncomingNotice() {
        const notice = document.getElementById('vd-sip-incoming');
        if (notice) notice.remove();
        sipPhoneShellState.incomingNoticeCallID = '';
    }

    function startSIPPhoneRinging() {
        if (!sipPhonePreferences().ringtone_enabled || sipPhoneShellState.ringTimer) return;
        const AudioContextClass = window.AudioContext || window.webkitAudioContext;
        if (!AudioContextClass) return;
        try {
            const context = new AudioContextClass();
            const oscillator = context.createOscillator();
            const gain = context.createGain();
            oscillator.type = 'sine';
            oscillator.frequency.value = 440;
            gain.gain.value = 0;
            oscillator.connect(gain);
            gain.connect(context.destination);
            oscillator.start();
            sipPhoneShellState.ringContext = context;
            sipPhoneShellState.ringOscillator = oscillator;
            const pulse = () => {
                const now = context.currentTime;
                gain.gain.cancelScheduledValues(now);
                gain.gain.setValueAtTime(0, now);
                gain.gain.linearRampToValueAtTime(0.04, now + 0.03);
                gain.gain.setValueAtTime(0.04, now + 0.22);
                gain.gain.linearRampToValueAtTime(0, now + 0.3);
            };
            pulse();
            sipPhoneShellState.ringTimer = setInterval(pulse, 1800);
        } catch (_) {}
    }

    function stopSIPPhoneRinging() {
        if (sipPhoneShellState.ringTimer) clearInterval(sipPhoneShellState.ringTimer);
        sipPhoneShellState.ringTimer = null;
        if (sipPhoneShellState.ringOscillator) {
            try { sipPhoneShellState.ringOscillator.stop(); } catch (_) {}
        }
        if (sipPhoneShellState.ringContext) sipPhoneShellState.ringContext.close().catch(() => {});
        sipPhoneShellState.ringOscillator = null;
        sipPhoneShellState.ringContext = null;
    }

    function renderSIPPhoneShellChrome(snapshot) {
        const taskbarSystem = document.querySelector('.vd-taskbar-system');
        if (!taskbarSystem) return;
        let pill = document.getElementById('vd-sip-call-pill');
        const call = snapshot.call;
        if (!call) {
            if (pill) pill.remove();
            return;
        }
        if (!pill) {
            pill = document.createElement('button');
            pill.id = 'vd-sip-call-pill';
            pill.type = 'button';
            pill.className = 'vd-sip-call-pill';
            pill.addEventListener('click', () => openApp('sip-phone'));
            taskbarSystem.insertBefore(pill, taskbarSystem.firstChild);
        }
        const stateLabel = call.state === 'ringing'
            ? sipPhoneText('ringing', 'Ringing')
            : sipPhoneText('call_active', 'Call active');
        pill.innerHTML = `<span class="vd-sip-call-pill-icon">${iconMarkup('phone', 'P', 'vd-sip-glyph', 15)}</span><span><strong>${esc(stateLabel)}</strong><small>${esc(call.remote_party || '')}</small></span>`;
        pill.classList.toggle('is-ringing', call.state === 'ringing');
    }

    function closeSIPPhoneShellRuntime() {
        stopSIPPhoneRinging();
        removeSIPPhoneIncomingNotice();
        const callID = sipPhoneShellState.callID || (sipPhoneShellState.appState && sipPhoneShellState.appState.active_call && sipPhoneShellState.appState.active_call.id);
        if (callID && sipPhoneShellState.peerConnection) {
            fetch('/api/sip/calls/' + encodeURIComponent(callID) + '/hangup', {
                method: 'POST',
                credentials: 'same-origin',
                keepalive: true,
                headers: { 'Content-Type': 'application/json' },
                body: '{}'
            }).catch(() => {});
        }
        teardownSIPPhoneMedia(true);
        if (sipPhoneShellState.eventSource) sipPhoneShellState.eventSource.close();
        sipPhoneShellState.eventSource = null;
    }

    function initSIPPhoneShellRuntime() {
        if (sipPhoneShellState.initialized) return;
        sipPhoneShellState.initialized = true;
        const modules = window.AuraDesktopModules;
        const translations = modules && typeof modules.loadAppI18nSections === 'function'
            ? modules.loadAppI18nSections('sip-phone')
            : Promise.resolve();
        translations.finally(() => {
            connectSIPPhoneEvents();
            refreshSIPPhoneState();
        });
        if (navigator.mediaDevices && navigator.mediaDevices.addEventListener) {
            navigator.mediaDevices.addEventListener('devicechange', refreshSIPPhoneDevices);
        }
    }

    window.SipPhoneRuntime = {
        initialize: initSIPPhoneShellRuntime,
        refresh: refreshSIPPhoneState,
        subscribe(listener) {
            if (typeof listener !== 'function') return function () {};
            sipPhoneShellState.listeners.add(listener);
            listener(sipPhoneSnapshot());
            return function () { sipPhoneShellState.listeners.delete(listener); };
        },
        getState: sipPhoneSnapshot,
        canonicalizeTarget: canonicalSIPPhoneTarget,
        dial: dialSIPPhone,
        answer: answerSIPPhone,
        reject: rejectSIPPhone,
        hangup: hangupSIPPhone,
        sendDTMF: sendSIPPhoneDTMF,
        setMuted: setSIPPhoneMuted,
        setInputDevice: setSIPPhoneInputDevice,
        setOutputDevice: setSIPPhoneOutputDevice,
        setVolume: setSIPPhoneVolume,
        setPreferences: saveSIPPhonePreferences,
        refreshDevices: refreshSIPPhoneDevices,
        disposeMedia: teardownSIPPhoneMedia
    };
