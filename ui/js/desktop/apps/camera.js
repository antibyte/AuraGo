(function () {
    'use strict';

    const disposers = new Map();

    function render(host, windowId, context) {
        if (!host) return;
        const ctx = context || {};
        const esc = ctx.esc || escapeHTML;
        const t = ctx.t || ((key, fallback) => fallback || key);
        const iconMarkup = ctx.iconMarkup || ((key, fallback) => '<span>' + esc(fallback || key || '') + '</span>');
        const notify = typeof ctx.notify === 'function' ? ctx.notify : function () {};

        const state = {
            stream: null,
            capturedDataURL: null,
            facingMode: 'user',
            sending: false,
            saving: false,
            error: '',
            hasMultipleCameras: false
        };

        host.innerHTML = '<div class="camera-app" data-camera-app="' + esc(windowId) + '">' +
            '<div class="camera-error" data-error hidden></div>' +
            '<div class="camera-viewport" data-viewport>' +
                '<video class="camera-video" data-video autoplay playsinline muted></video>' +
                '<img class="camera-preview" data-preview hidden alt="">' +
                '<div class="camera-overlay" data-overlay>' +
                    '<div class="camera-overlay-ring"></div>' +
                '</div>' +
            '</div>' +
            '<div class="camera-toolbar" data-toolbar>' +
                '<button class="camera-btn camera-btn-switch" type="button" data-action="switch" hidden aria-label="' + esc(t('desktop.camera_switch', 'Switch Camera')) + '">' +
                    iconMarkup('refresh', 'S', 'camera-btn-icon', 18) +
                '</button>' +
                '<button class="camera-btn camera-btn-capture" type="button" data-action="capture" aria-label="' + esc(t('desktop.camera_capture', 'Capture')) + '">' +
                    '<span class="camera-capture-ring"></span>' +
                '</button>' +
                '<div class="camera-actions" data-actions hidden>' +
                    '<button class="camera-btn camera-btn-action" type="button" data-action="retake">' +
                        iconMarkup('refresh', 'R', 'camera-btn-icon', 16) +
                        '<span>' + esc(t('desktop.camera_retake', 'Retake')) + '</span>' +
                    '</button>' +
                    '<button class="camera-btn camera-btn-action" type="button" data-action="save">' +
                        iconMarkup('save', 'S', 'camera-btn-icon', 16) +
                        '<span>' + esc(t('desktop.camera_save', 'Save')) + '</span>' +
                    '</button>' +
                    '<button class="camera-btn camera-btn-action camera-btn-send" type="button" data-action="send">' +
                        iconMarkup('chat', 'A', 'camera-btn-icon', 16) +
                        '<span>' + esc(t('desktop.camera_send_agent', 'Send to Agent')) + '</span>' +
                    '</button>' +
                '</div>' +
            '</div>' +
        '</div>';

        var video = host.querySelector('[data-video]');
        var preview = host.querySelector('[data-preview]');
        var overlay = host.querySelector('[data-overlay]');
        var errorEl = host.querySelector('[data-error]');
        var toolbar = host.querySelector('[data-toolbar]');
        var actions = host.querySelector('[data-actions]');
        var captureBtn = host.querySelector('[data-action="capture"]');
        var switchBtn = host.querySelector('[data-action="switch"]');
        var retakeBtn = host.querySelector('[data-action="retake"]');
        var saveBtn = host.querySelector('[data-action="save"]');
        var sendBtn = host.querySelector('[data-action="send"]');

        function showError(msg) {
            state.error = msg;
            errorEl.textContent = msg;
            errorEl.hidden = !msg;
        }

        function setWindowMenus() {
            if (typeof ctx.setWindowMenus !== 'function') return;
            var captured = !!state.capturedDataURL;
            ctx.setWindowMenus(windowId, [
                {
                    id: 'file',
                    labelKey: 'desktop.menu_file',
                    items: [
                        { id: 'save', labelKey: 'desktop.camera_save', icon: 'save', disabled: !captured || state.saving, action: function () { saveBtn.click(); } },
                        { id: 'send', labelKey: 'desktop.camera_send_agent', icon: 'chat', disabled: !captured || state.sending, action: function () { sendBtn.click(); } }
                    ]
                },
                {
                    id: 'view',
                    labelKey: 'desktop.menu_view',
                    items: [
                        { id: 'switch', labelKey: 'desktop.camera_switch', icon: 'refresh', disabled: captured, action: function () { switchBtn.click(); } }
                    ]
                }
            ]);
        }

        function updateUI() {
            var captured = !!state.capturedDataURL;
            video.hidden = captured;
            preview.hidden = !captured;
            overlay.hidden = captured;
            captureBtn.hidden = captured;
            actions.hidden = !captured;
            if (captured) {
                preview.src = state.capturedDataURL;
            }
            setWindowMenus();
        }

        function stopStream() {
            if (state.stream) {
                state.stream.getTracks().forEach(function (t) { t.stop(); });
                state.stream = null;
            }
        }

        function startCamera() {
            stopStream();
            state.capturedDataURL = null;
            state.error = '';
            errorEl.hidden = true;

            if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
                var isSecure = location.protocol === 'https:' || location.hostname === 'localhost' || location.hostname === '127.0.0.1';
                showError(isSecure
                    ? t('desktop.camera_no_camera', 'No camera found')
                    : t('desktop.camera_insecure', 'Camera requires HTTPS'));
                updateUI();
                return;
            }

            var constraints = {
                video: {
                    facingMode: state.facingMode,
                    width: { ideal: 1280 },
                    height: { ideal: 720 }
                },
                audio: false
            };

            navigator.mediaDevices.getUserMedia(constraints).then(function (stream) {
                state.stream = stream;
                video.srcObject = stream;
                video.play().catch(function () {});
                detectCameras();
                updateUI();
            }).catch(function (err) {
                var name = (err && err.name) || '';
                if (name === 'NotAllowedError' || name === 'PermissionDeniedError') {
                    showError(t('desktop.camera_permission_denied', 'Camera access denied'));
                } else {
                    showError(t('desktop.camera_no_camera', 'No camera found'));
                }
                updateUI();
            });
        }

        function detectCameras() {
            if (!navigator.mediaDevices || !navigator.mediaDevices.enumerateDevices) return;
            navigator.mediaDevices.enumerateDevices().then(function (devices) {
                var videoDevices = devices.filter(function (d) { return d.kind === 'videoinput'; });
                state.hasMultipleCameras = videoDevices.length > 1;
                switchBtn.hidden = !state.hasMultipleCameras;
            }).catch(function () {});
        }

        function capture() {
            if (!state.stream || !video.videoWidth) return;
            var canvas = document.createElement('canvas');
            canvas.width = video.videoWidth;
            canvas.height = video.videoHeight;
            var ctx2 = canvas.getContext('2d');
            ctx2.drawImage(video, 0, 0);
            state.capturedDataURL = canvas.toDataURL('image/jpeg', 0.9);
            stopStream();
            updateUI();
        }

        function retake() {
            state.capturedDataURL = null;
            startCamera();
        }

        function savePhoto() {
            if (!state.capturedDataURL || state.saving) return;
            state.saving = true;
            saveBtn.disabled = true;
            var blob = dataURLtoBlob(state.capturedDataURL);
            var ts = new Date();
            var filename = 'photo_' + ts.getFullYear() +
                String(ts.getMonth() + 1).padStart(2, '0') +
                String(ts.getDate()).padStart(2, '0') + '_' +
                String(ts.getHours()).padStart(2, '0') +
                String(ts.getMinutes()).padStart(2, '0') +
                String(ts.getSeconds()).padStart(2, '0') + '.jpg';

            var form = new FormData();
            form.append('file', blob, filename);
            form.append('path', 'Pictures');

            var xhr = new XMLHttpRequest();
            xhr.open('POST', '/api/desktop/upload');
            xhr.onload = function () {
                state.saving = false;
                saveBtn.disabled = false;
                if (xhr.status >= 200 && xhr.status < 300) {
                    notify(t('desktop.camera_saved', 'Photo saved'));
                } else {
                    notify(t('desktop.camera_save_error', 'Failed to save photo'));
                }
            };
            xhr.onerror = function () {
                state.saving = false;
                saveBtn.disabled = false;
                notify(t('desktop.camera_save_error', 'Failed to save photo'));
            };
            xhr.send(form);
        }

        function sendToAgent() {
            if (!state.capturedDataURL || state.sending) return;
            state.sending = true;
            sendBtn.disabled = true;

            var base64 = state.capturedDataURL.split(',')[1] || state.capturedDataURL;
            var body = JSON.stringify({
                message: t('desktop.camera_analyze_prompt', 'Please analyze this photo.'),
                context: {
                    source: 'camera',
                    image_base64: base64
                }
            });

            var xhr = new XMLHttpRequest();
            xhr.open('POST', '/api/desktop/chat/stream');
            xhr.setRequestHeader('Content-Type', 'application/json');

            var responseText = '';
            xhr.onreadystate = function () {};

            xhr.onreadystatechange = function () {
                if (xhr.readyState >= 3) {
                    var newData = xhr.responseText.substring(responseText.length);
                    responseText += newData;
                }
                if (xhr.readyState === 4) {
                    state.sending = false;
                    sendBtn.disabled = false;
                    if (xhr.status >= 200 && xhr.status < 300) {
                        notify(t('desktop.camera_sent', 'Sent to agent'));
                    } else {
                        notify(t('desktop.camera_send_error', 'Failed to send to agent'));
                    }
                }
            };

            xhr.send(body);
        }

        function switchCamera() {
            state.facingMode = state.facingMode === 'user' ? 'environment' : 'user';
            if (!state.capturedDataURL) {
                startCamera();
            }
        }

        function dataURLtoBlob(dataURL) {
            var parts = dataURL.split(',');
            var mime = parts[0].match(/:(.*?);/)[1];
            var raw = atob(parts[1]);
            var arr = new Uint8Array(raw.length);
            for (var i = 0; i < raw.length; i++) {
                arr[i] = raw.charCodeAt(i);
            }
            return new Blob([arr], { type: mime });
        }

        function handleKeydown(e) {
            if (e.target && (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA')) return;
            if (e.key === ' ' || e.code === 'Space') {
                e.preventDefault();
                if (!state.capturedDataURL) capture();
            } else if ((e.key === 'r' || e.key === 'R') && state.capturedDataURL) {
                e.preventDefault();
                retake();
            }
        }

        captureBtn.addEventListener('click', capture);
        switchBtn.addEventListener('click', switchCamera);
        retakeBtn.addEventListener('click', retake);
        saveBtn.addEventListener('click', savePhoto);
        sendBtn.addEventListener('click', sendToAgent);
        host.addEventListener('keydown', handleKeydown);

        disposers.set(windowId, function () {
            stopStream();
            host.removeEventListener('keydown', handleKeydown);
        });

        setWindowMenus();
        startCamera();
    }

    function dispose(windowId) {
        var cleanup = disposers.get(windowId);
        if (!cleanup) return;
        cleanup();
        disposers.delete(windowId);
    }

    function escapeHTML(value) {
        return String(value == null ? '' : value)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
    }

    window.CameraApp = { render: render, dispose: dispose };
})();
