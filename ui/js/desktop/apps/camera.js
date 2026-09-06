(function () {
    'use strict';

    const disposers = new Map();
    const TIMER_OPTIONS = [0, 3, 5, 10];
    const MAX_RECENTS = 12;
    const VIDEO_MIME_CANDIDATES = ['video/webm;codecs=vp9', 'video/webm;codecs=vp8', 'video/webm'];

    function render(host, windowId, context) {
        if (!host) return;
        const ctx = context || {};
        const esc = ctx.esc || escapeHTML;
        const t = ctx.t || ((key, fallback) => (fallback && typeof fallback === 'string' ? fallback : key));
        const iconMarkup = ctx.iconMarkup || ((key, fallback) => '<span>' + esc(fallback || key || '') + '</span>');
        const notify = typeof ctx.notify === 'function' ? ctx.notify : function () {};

        const state = {
            stream: null,
            devices: [],
            selectedDeviceId: '',
            facingMode: 'user',
            capturedDataURL: null,
            videoBlob: null,
            videoURL: null,
            mode: 'photo',
            recording: false,
            recorder: null,
            recordChunks: [],
            recordStart: 0,
            recordClock: null,
            discardClip: false,
            timerSeconds: 0,
            countdownLeft: 0,
            countdownInterval: null,
            mirror: true,
            grid: false,
            sending: false,
            saving: false,
            copying: false,
            recents: [],
            disposed: false
        };

        host.innerHTML = '<div class="camera-app" data-camera-app="' + esc(windowId) + '" data-state="live" data-mode="photo">' +
            '<div class="camera-error" data-error hidden></div>' +
            '<div class="camera-viewport" data-viewport>' +
                '<video class="camera-video" data-video autoplay playsinline muted></video>' +
                '<img class="camera-preview" data-preview alt="" hidden>' +
                '<video class="camera-preview-video" data-clip controls playsinline hidden></video>' +
                '<div class="camera-grid" data-grid hidden aria-hidden="true"></div>' +
                '<div class="camera-overlay" data-overlay aria-hidden="true"></div>' +
                '<div class="camera-flash" data-flash aria-hidden="true"></div>' +
                '<div class="camera-status" data-status hidden><span class="camera-status-dot"></span><span data-status-text></span></div>' +
                '<div class="camera-rec-badge" data-rec hidden role="status"><span class="camera-rec-dot"></span><span data-rec-time>00:00</span><span class="camera-rec-label">' + esc(t('desktop.camera_recording')) + '</span></div>' +
                '<div class="camera-countdown" data-countdown hidden><span data-countdown-num></span></div>' +
                '<div class="camera-recents" data-recents hidden role="group" aria-label="' + esc(t('desktop.camera_recent')) + '"></div>' +
            '</div>' +
            '<div class="camera-dock" data-dock>' +
                '<div class="camera-dock-live" data-dock-live>' +
                    '<div class="camera-dock-cluster is-left">' +
                        '<button class="camera-chip" type="button" data-action="switch" hidden title="' + esc(t('desktop.camera_switch')) + '" aria-label="' + esc(t('desktop.camera_switch')) + '">' +
                            iconMarkup('refresh', 'S', 'camera-btn-icon', 17) +
                        '</button>' +
                        '<select class="camera-device-select" data-device-select hidden aria-label="' + esc(t('desktop.camera_select_device')) + '"></select>' +
                        '<button class="camera-chip" type="button" data-action="timer" title="' + esc(t('desktop.camera_timer')) + '" aria-label="' + esc(t('desktop.camera_timer')) + '">' +
                            iconMarkup('clock', 'T', 'camera-btn-icon', 16) +
                            '<span data-timer-label></span>' +
                        '</button>' +
                    '</div>' +
                    '<div class="camera-dock-center">' +
                        '<div class="camera-mode" role="group" aria-label="' + esc(t('desktop.camera_mode_photo')) + ' / ' + esc(t('desktop.camera_mode_video')) + '">' +
                            '<button type="button" data-mode="photo" class="is-active">' + esc(t('desktop.camera_mode_photo')) + '</button>' +
                            '<button type="button" data-mode="video">' + esc(t('desktop.camera_mode_video')) + '</button>' +
                        '</div>' +
                        '<button class="camera-btn camera-btn-capture" type="button" data-action="capture" aria-label="' + esc(t('desktop.camera_capture')) + '">' +
                            '<span class="camera-capture-ring"></span>' +
                        '</button>' +
                    '</div>' +
                    '<div class="camera-dock-cluster is-right">' +
                        '<button class="camera-chip" type="button" data-action="mirror" title="' + esc(t('desktop.camera_mirror')) + '" aria-label="' + esc(t('desktop.camera_mirror')) + '">' +
                            iconMarkup('columns', 'M', 'camera-btn-icon', 16) +
                        '</button>' +
                        '<button class="camera-chip" type="button" data-action="grid" title="' + esc(t('desktop.camera_grid')) + '" aria-label="' + esc(t('desktop.camera_grid')) + '">' +
                            iconMarkup('grid', 'G', 'camera-btn-icon', 16) +
                        '</button>' +
                    '</div>' +
                '</div>' +
                '<div class="camera-actions" data-actions hidden>' +
                    '<button class="camera-btn camera-btn-action" type="button" data-action="retake">' +
                        iconMarkup('refresh', 'R', 'camera-btn-icon', 16) +
                        '<span class="camera-btn-spinner" aria-hidden="true"></span>' +
                        '<span>' + esc(t('desktop.camera_retake')) + '</span>' +
                    '</button>' +
                    '<button class="camera-btn camera-btn-action" type="button" data-action="save">' +
                        iconMarkup('save', 'S', 'camera-btn-icon', 16) +
                        '<span class="camera-btn-spinner" aria-hidden="true"></span>' +
                        '<span>' + esc(t('desktop.camera_save')) + '</span>' +
                    '</button>' +
                    '<button class="camera-btn camera-btn-action" type="button" data-action="download">' +
                        iconMarkup('download', 'D', 'camera-btn-icon', 16) +
                        '<span>' + esc(t('desktop.camera_download')) + '</span>' +
                    '</button>' +
                    '<button class="camera-btn camera-btn-action" type="button" data-action="copy">' +
                        iconMarkup('copy', 'C', 'camera-btn-icon', 16) +
                        '<span class="camera-btn-spinner" aria-hidden="true"></span>' +
                        '<span>' + esc(t('desktop.camera_copy')) + '</span>' +
                    '</button>' +
                    '<button class="camera-btn camera-btn-action camera-btn-send" type="button" data-action="send">' +
                        iconMarkup('chat', 'A', 'camera-btn-icon', 16) +
                        '<span class="camera-btn-spinner" aria-hidden="true"></span>' +
                        '<span>' + esc(t('desktop.camera_send_agent')) + '</span>' +
                    '</button>' +
                '</div>' +
                '<div class="camera-toolbar-hint" data-hint aria-hidden="true"></div>' +
            '</div>' +
        '</div>';

        var video = host.querySelector('[data-video]');
        var preview = host.querySelector('[data-preview]');
        var clip = host.querySelector('[data-clip]');
        var overlay = host.querySelector('[data-overlay]');
        var gridEl = host.querySelector('[data-grid]');
        var errorEl = host.querySelector('[data-error]');
        var statusEl = host.querySelector('[data-status]');
        var statusText = host.querySelector('[data-status-text]');
        var recBadge = host.querySelector('[data-rec]');
        var recTime = host.querySelector('[data-rec-time]');
        var countdownEl = host.querySelector('[data-countdown]');
        var countdownNum = host.querySelector('[data-countdown-num]');
        var recentsEl = host.querySelector('[data-recents]');
        var dockLive = host.querySelector('[data-dock-live]');
        var actions = host.querySelector('[data-actions]');
        var captureBtn = host.querySelector('[data-action="capture"]');
        var switchBtn = host.querySelector('[data-action="switch"]');
        var deviceSelect = host.querySelector('[data-device-select]');
        var timerBtn = host.querySelector('[data-action="timer"]');
        var timerLabel = host.querySelector('[data-timer-label]');
        var mirrorBtn = host.querySelector('[data-action="mirror"]');
        var gridBtn = host.querySelector('[data-action="grid"]');
        var modeBtns = host.querySelectorAll('[data-mode]');
        var retakeBtn = host.querySelector('[data-action="retake"]');
        var saveBtn = host.querySelector('[data-action="save"]');
        var downloadBtn = host.querySelector('[data-action="download"]');
        var copyBtn = host.querySelector('[data-action="copy"]');
        var sendBtn = host.querySelector('[data-action="send"]');
        var appRoot = host.querySelector('.camera-app');
        var flashEl = host.querySelector('[data-flash]');
        var hintEl = host.querySelector('[data-hint]');
        var reduceMotion = typeof window.matchMedia === 'function'
            && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

        function isClipPreview() {
            return !!state.videoURL;
        }

        function isPreview() {
            return !!state.capturedDataURL || !!state.videoURL;
        }

        function showError(msg) {
            state.error = msg;
            errorEl.textContent = msg;
            errorEl.hidden = !msg;
        }

        function setWindowMenus() {
            if (typeof ctx.setWindowMenus !== 'function') return;
            var photo = isPreview() && !isClipPreview();
            var anyPreview = isPreview();
            ctx.setWindowMenus(windowId, [
                {
                    id: 'file',
                    labelKey: 'desktop.menu_file',
                    items: [
                        { id: 'save', labelKey: 'desktop.camera_save', icon: 'save', disabled: !anyPreview || state.saving, action: function () { saveBtn.click(); } },
                        { id: 'download', labelKey: 'desktop.camera_download', icon: 'download', disabled: !anyPreview, action: function () { downloadBtn.click(); } },
                        { id: 'copy', labelKey: 'desktop.camera_copy', icon: 'copy', disabled: !photo || state.copying, action: function () { copyBtn.click(); } },
                        { id: 'send', labelKey: 'desktop.camera_send_agent', icon: 'chat', disabled: !photo || state.sending, action: function () { sendBtn.click(); } }
                    ]
                },
                {
                    id: 'view',
                    labelKey: 'desktop.menu_view',
                    items: [
                        { id: 'switch', labelKey: 'desktop.camera_switch', icon: 'refresh', disabled: anyPreview || state.devices.length < 2 && !state.stream, action: function () { switchCamera(); } },
                        { id: 'mirror', labelKey: 'desktop.camera_mirror', icon: 'columns', disabled: anyPreview, action: function () { toggleMirror(); } },
                        { id: 'grid', labelKey: 'desktop.camera_grid', icon: 'grid', disabled: anyPreview, action: function () { toggleGrid(); } },
                        { id: 'timer', labelKey: 'desktop.camera_timer', icon: 'clock', disabled: anyPreview, action: function () { cycleTimer(); } }
                    ]
                }
            ]);
        }

        function renderHints() {
            var hints = [];
            if (isPreview()) {
                hints.push(['R', t('desktop.camera_hint_retake')]);
                if (!isClipPreview()) hints.push(['C', t('desktop.camera_hint_copy')]);
            } else {
                hints.push(['Space', t('desktop.camera_hint_capture')]);
                hints.push(['V', t('desktop.camera_hint_video')]);
                hints.push(['M', t('desktop.camera_hint_mirror')]);
                hints.push(['G', t('desktop.camera_hint_grid')]);
            }
            hintEl.innerHTML = hints.map(function (pair) {
                return '<span><kbd>' + esc(pair[0]) + '</kbd>' + esc(pair[1]) + '</span>';
            }).join('');
        }

        function updateUI() {
            var previewActive = isPreview();
            var clipActive = isClipPreview();
            appRoot.setAttribute('data-state', previewActive ? 'preview' : 'live');
            appRoot.setAttribute('data-mode', state.mode);
            appRoot.classList.toggle('is-recording', state.recording);

            preview.hidden = !(previewActive && !clipActive);
            if (previewActive && !clipActive && preview.src !== state.capturedDataURL) {
                preview.src = state.capturedDataURL;
            }
            clip.hidden = !(previewActive && clipActive);
            if (previewActive && clipActive && clip.src !== state.videoURL) {
                clip.src = state.videoURL;
            }
            gridEl.hidden = !state.grid || previewActive;
            recBadge.hidden = !state.recording;
            countdownEl.hidden = state.countdownLeft <= 0;
            statusEl.hidden = !(state.stream && !previewActive);

            dockLive.hidden = previewActive;
            actions.hidden = !previewActive;
            copyBtn.hidden = clipActive;
            sendBtn.hidden = clipActive;

            modeBtns.forEach(function (btn) {
                btn.classList.toggle('is-active', btn.dataset.mode === state.mode);
                btn.disabled = state.recording || previewActive;
            });

            var captureLabel = state.recording
                ? t('desktop.camera_record_stop')
                : (state.mode === 'video' ? t('desktop.camera_record') : t('desktop.camera_capture'));
            captureBtn.setAttribute('aria-label', captureLabel);
            captureBtn.title = captureLabel;

            var busy = state.recording;
            switchBtn.disabled = busy;
            deviceSelect.disabled = busy;
            timerBtn.disabled = busy;
            timerBtn.hidden = state.mode !== 'photo';
            timerBtn.classList.toggle('is-active', state.timerSeconds > 0);
            timerLabel.textContent = state.timerSeconds > 0 ? t('desktop.camera_timer_seconds', { seconds: state.timerSeconds }) : '';
            mirrorBtn.classList.toggle('is-active', state.mirror);
            gridBtn.classList.toggle('is-active', state.grid);
            video.classList.toggle('is-mirrored', state.mirror);

            recentsEl.hidden = state.recents.length === 0;
            renderHints();
            setWindowMenus();
        }

        function triggerFlash() {
            if (!flashEl || reduceMotion) return;
            appRoot.classList.remove('is-flashing');
            // Force reflow so the animation restarts on rapid captures.
            void appRoot.offsetWidth;
            appRoot.classList.add('is-flashing');
        }

        function setLoading(btn, loading) {
            if (!btn) return;
            btn.classList.toggle('is-loading', !!loading);
            btn.disabled = !!loading;
        }

        function stopStream() {
            if (state.stream) {
                state.stream.getTracks().forEach(function (track) { track.stop(); });
                state.stream = null;
            }
        }

        function startCamera() {
            stopStream();
            state.capturedDataURL = null;
            clearClip();
            state.error = '';
            errorEl.hidden = true;

            if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
                var isSecure = location.protocol === 'https:' || location.hostname === 'localhost' || location.hostname === '127.0.0.1';
                showError(isSecure
                    ? t('desktop.camera_no_camera')
                    : t('desktop.camera_insecure'));
                updateUI();
                return;
            }

            var videoConstraints = {
                width: { ideal: 1280 },
                height: { ideal: 720 }
            };
            if (state.selectedDeviceId) {
                videoConstraints.deviceId = { exact: state.selectedDeviceId };
            } else {
                videoConstraints.facingMode = state.facingMode;
            }

            navigator.mediaDevices.getUserMedia({ video: videoConstraints, audio: false }).then(function (stream) {
                if (state.disposed) {
                    stream.getTracks().forEach(function (track) { track.stop(); });
                    return;
                }
                state.stream = stream;
                video.srcObject = stream;
                video.play().catch(function () {});
                updateStatusChip();
                detectCameras();
                updateUI();
            }).catch(function (err) {
                var name = (err && err.name) || '';
                if (name === 'NotAllowedError' || name === 'PermissionDeniedError') {
                    showError(t('desktop.camera_permission_denied'));
                } else if (name === 'OverconstrainedError' && state.selectedDeviceId) {
                    state.selectedDeviceId = '';
                    startCamera();
                    return;
                } else {
                    showError(t('desktop.camera_no_camera'));
                }
                updateUI();
            });
        }

        function updateStatusChip() {
            var track = state.stream && state.stream.getVideoTracks()[0];
            if (!track) return;
            var settings = typeof track.getSettings === 'function' ? track.getSettings() : {};
            var label = track.label || '';
            var parts = [];
            if (label) parts.push(label);
            if (settings.width && settings.height) parts.push(settings.width + '×' + settings.height);
            statusText.textContent = parts.join(' · ') || t('desktop.app_camera');
            if (settings.deviceId && settings.deviceId !== state.selectedDeviceId) {
                state.selectedDeviceId = settings.deviceId;
                syncDeviceSelect();
            }
        }

        function detectCameras() {
            if (!navigator.mediaDevices || !navigator.mediaDevices.enumerateDevices) return;
            navigator.mediaDevices.enumerateDevices().then(function (devices) {
                if (state.disposed) return;
                state.devices = devices.filter(function (d) { return d.kind === 'videoinput'; });
                syncDeviceSelect();
                updateUI();
            }).catch(function () {});
        }

        function syncDeviceSelect() {
            var devices = state.devices;
            var hasLabels = devices.some(function (d) { return !!d.label; });
            var useSelect = devices.length > 1 && hasLabels;
            deviceSelect.hidden = !useSelect;
            switchBtn.hidden = useSelect || devices.length < 2;
            if (!useSelect) return;
            var options = devices.map(function (device, index) {
                var label = device.label || t('desktop.camera_default_device', { index: index + 1 });
                var selected = device.deviceId === state.selectedDeviceId ? ' selected' : '';
                return '<option value="' + esc(device.deviceId) + '"' + selected + '>' + esc(label) + '</option>';
            }).join('');
            deviceSelect.innerHTML = options;
        }

        function clearClip() {
            if (state.videoURL) URL.revokeObjectURL(state.videoURL);
            state.videoURL = null;
            state.videoBlob = null;
            clip.removeAttribute('src');
            clip.load();
        }

        function addRecent(dataURL) {
            state.recents.unshift({ id: Date.now() + '-' + Math.random().toString(36).slice(2, 8), dataURL: dataURL });
            if (state.recents.length > MAX_RECENTS) state.recents.length = MAX_RECENTS;
            renderRecents();
        }

        function renderRecents() {
            recentsEl.innerHTML = state.recents.map(function (item) {
                return '<button type="button" data-recent="' + esc(item.id) + '"><img src="' + esc(item.dataURL) + '" alt=""></button>';
            }).join('');
        }

        function capture() {
            if (!state.stream || !video.videoWidth) return;
            triggerFlash();
            var canvas = document.createElement('canvas');
            canvas.width = video.videoWidth;
            canvas.height = video.videoHeight;
            var ctx2 = canvas.getContext('2d');
            if (state.mirror) {
                ctx2.translate(canvas.width, 0);
                ctx2.scale(-1, 1);
            }
            ctx2.drawImage(video, 0, 0);
            state.capturedDataURL = canvas.toDataURL('image/jpeg', 0.9);
            addRecent(state.capturedDataURL);
            stopStream();
            updateUI();
        }

        function beginCapture() {
            if (state.recording) { stopRecording(false); return; }
            if (state.mode === 'video') { startRecording(); return; }
            if (state.countdownLeft > 0) { cancelCountdown(); return; }
            if (state.timerSeconds > 0) {
                state.countdownLeft = state.timerSeconds;
                showCountdown();
                state.countdownInterval = setInterval(function () {
                    state.countdownLeft -= 1;
                    if (state.countdownLeft <= 0) {
                        cancelCountdown();
                        capture();
                        return;
                    }
                    showCountdown();
                }, 1000);
                updateUI();
                return;
            }
            capture();
        }

        function showCountdown() {
            countdownEl.hidden = false;
            countdownNum.textContent = String(state.countdownLeft);
            countdownNum.style.animation = 'none';
            void countdownNum.offsetWidth;
            countdownNum.style.animation = '';
        }

        function cancelCountdown() {
            if (state.countdownInterval) clearInterval(state.countdownInterval);
            state.countdownInterval = null;
            state.countdownLeft = 0;
            countdownEl.hidden = true;
        }

        function startRecording() {
            if (state.recording || !state.stream) return;
            if (typeof window.MediaRecorder !== 'function') {
                notify(t('desktop.camera_recording_unsupported'));
                return;
            }
            var mime = '';
            for (var i = 0; i < VIDEO_MIME_CANDIDATES.length; i++) {
                if (MediaRecorder.isTypeSupported(VIDEO_MIME_CANDIDATES[i])) { mime = VIDEO_MIME_CANDIDATES[i]; break; }
            }
            var recorder;
            try {
                recorder = mime ? new MediaRecorder(state.stream, { mimeType: mime }) : new MediaRecorder(state.stream);
            } catch (err) {
                notify(t('desktop.camera_recording_unsupported'));
                return;
            }
            state.recorder = recorder;
            state.recordChunks = [];
            state.discardClip = false;
            recorder.addEventListener('dataavailable', function (event) {
                if (event.data && event.data.size) state.recordChunks.push(event.data);
            });
            recorder.addEventListener('stop', function () {
                var type = recorder.mimeType || mime || 'video/webm';
                state.recorder = null;
                if (!state.discardClip && state.recordChunks.length) {
                    state.videoBlob = new Blob(state.recordChunks, { type: type });
                    state.videoURL = URL.createObjectURL(state.videoBlob);
                    stopStream();
                }
                state.recordChunks = [];
                state.discardClip = false;
                updateUI();
            });
            try {
                recorder.start(1000);
            } catch (err) {
                state.recorder = null;
                notify(t('desktop.camera_recording_unsupported'));
                return;
            }
            state.recording = true;
            state.recordStart = Date.now();
            recTime.textContent = '00:00';
            state.recordClock = setInterval(function () {
                recTime.textContent = formatDuration(Date.now() - state.recordStart);
            }, 500);
            updateUI();
        }

        function stopRecording(discard) {
            if (!state.recording || !state.recorder) return;
            state.discardClip = !!discard;
            state.recording = false;
            if (state.recordClock) clearInterval(state.recordClock);
            state.recordClock = null;
            try { state.recorder.stop(); } catch (err) { state.recorder = null; }
            updateUI();
        }

        function formatDuration(ms) {
            var total = Math.max(0, Math.floor(ms / 1000));
            var minutes = Math.floor(total / 60);
            var seconds = total % 60;
            return String(minutes).padStart(2, '0') + ':' + String(seconds).padStart(2, '0');
        }

        function retake() {
            state.capturedDataURL = null;
            clearClip();
            startCamera();
        }

        function timestampName(prefix, ext) {
            var ts = new Date();
            return prefix + '_' + ts.getFullYear() +
                String(ts.getMonth() + 1).padStart(2, '0') +
                String(ts.getDate()).padStart(2, '0') + '_' +
                String(ts.getHours()).padStart(2, '0') +
                String(ts.getMinutes()).padStart(2, '0') +
                String(ts.getSeconds()).padStart(2, '0') + ext;
        }

        function uploadBlob(blob, filename, folder, btn, doneKey, failKey) {
            var form = new FormData();
            form.append('file', blob, filename);
            form.append('path', folder);

            var xhr = new XMLHttpRequest();
            xhr.open('POST', '/api/desktop/upload');
            xhr.onload = function () {
                setLoading(btn, false);
                state.saving = false;
                notify(t(xhr.status >= 200 && xhr.status < 300 ? doneKey : failKey));
            };
            xhr.onerror = function () {
                setLoading(btn, false);
                state.saving = false;
                notify(t(failKey));
            };
            xhr.send(form);
        }

        function saveCapture() {
            if (state.saving) return;
            if (isClipPreview()) {
                if (!state.videoBlob) return;
                state.saving = true;
                setLoading(saveBtn, true);
                uploadBlob(state.videoBlob, timestampName('video', '.webm'), 'Videos', saveBtn, 'desktop.camera_video_saved', 'desktop.camera_video_save_error');
                return;
            }
            if (!state.capturedDataURL) return;
            state.saving = true;
            setLoading(saveBtn, true);
            uploadBlob(dataURLtoBlob(state.capturedDataURL), timestampName('photo', '.jpg'), 'Pictures', saveBtn, 'desktop.camera_saved', 'desktop.camera_save_error');
        }

        function downloadCapture() {
            var link = document.createElement('a');
            if (isClipPreview()) {
                if (!state.videoURL) return;
                link.href = state.videoURL;
                link.download = timestampName('video', '.webm');
            } else {
                if (!state.capturedDataURL) return;
                link.href = state.capturedDataURL;
                link.download = timestampName('photo', '.jpg');
            }
            document.body.appendChild(link);
            link.click();
            link.remove();
        }

        function copyCapture() {
            if (!state.capturedDataURL || state.copying || isClipPreview()) return;
            if (!navigator.clipboard || typeof ClipboardItem !== 'function') {
                notify(t('desktop.camera_copy_error'));
                return;
            }
            state.copying = true;
            setLoading(copyBtn, true);
            var image = new Image();
            image.onload = function () {
                var canvas = document.createElement('canvas');
                canvas.width = image.naturalWidth;
                canvas.height = image.naturalHeight;
                canvas.getContext('2d').drawImage(image, 0, 0);
                canvas.toBlob(function (blob) {
                    var finish = function (ok) {
                        state.copying = false;
                        setLoading(copyBtn, false);
                        notify(t(ok ? 'desktop.camera_copied' : 'desktop.camera_copy_error'));
                    };
                    if (!blob) { finish(false); return; }
                    navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])
                        .then(function () { finish(true); })
                        .catch(function () { finish(false); });
                }, 'image/png');
            };
            image.onerror = function () {
                state.copying = false;
                setLoading(copyBtn, false);
                notify(t('desktop.camera_copy_error'));
            };
            image.src = state.capturedDataURL;
        }

        function sendToAgent() {
            if (!state.capturedDataURL || state.sending || isClipPreview()) return;
            state.sending = true;
            setLoading(sendBtn, true);

            var base64 = state.capturedDataURL.split(',')[1] || state.capturedDataURL;
            var body = JSON.stringify({
                message: t('desktop.camera_analyze_prompt'),
                context: {
                    source: 'camera',
                    image_base64: base64
                }
            });

            var xhr = new XMLHttpRequest();
            xhr.open('POST', '/api/desktop/chat/stream');
            xhr.setRequestHeader('Content-Type', 'application/json');

            var responseText = '';

            xhr.onreadystatechange = function () {
                if (xhr.readyState >= 3) {
                    var newData = xhr.responseText.substring(responseText.length);
                    responseText += newData;
                }
                if (xhr.readyState === 4) {
                    state.sending = false;
                    setLoading(sendBtn, false);
                    if (xhr.status >= 200 && xhr.status < 300) {
                        notify(t('desktop.camera_sent'));
                    } else {
                        notify(t('desktop.camera_send_error'));
                    }
                }
            };

            xhr.send(body);
        }

        function switchCamera() {
            if (state.recording) return;
            if (state.devices.length > 1) {
                var index = state.devices.findIndex(function (d) { return d.deviceId === state.selectedDeviceId; });
                var next = state.devices[(index + 1) % state.devices.length];
                state.selectedDeviceId = next.deviceId;
            } else {
                state.facingMode = state.facingMode === 'user' ? 'environment' : 'user';
                state.selectedDeviceId = '';
                state.mirror = state.facingMode === 'user';
            }
            if (!isPreview()) startCamera();
        }

        function selectDevice() {
            state.selectedDeviceId = deviceSelect.value || '';
            if (!state.recording && !isPreview()) startCamera();
        }

        function setMode(mode) {
            if (state.recording || isPreview()) return;
            state.mode = mode === 'video' ? 'video' : 'photo';
            cancelCountdown();
            updateUI();
        }

        function cycleTimer() {
            var index = TIMER_OPTIONS.indexOf(state.timerSeconds);
            state.timerSeconds = TIMER_OPTIONS[(index + 1) % TIMER_OPTIONS.length];
            updateUI();
        }

        function toggleMirror() {
            state.mirror = !state.mirror;
            updateUI();
        }

        function toggleGrid() {
            state.grid = !state.grid;
            updateUI();
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
            if (e.target && (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT')) return;
            if (e.key === 'Escape' && state.countdownLeft > 0) {
                e.preventDefault();
                cancelCountdown();
            } else if (e.key === ' ' || e.code === 'Space') {
                e.preventDefault();
                if (!isPreview()) beginCapture();
            } else if ((e.key === 'r' || e.key === 'R') && isPreview()) {
                e.preventDefault();
                retake();
            } else if ((e.key === 'v' || e.key === 'V') && !isPreview()) {
                e.preventDefault();
                if (state.mode !== 'video') setMode('video');
                else beginCapture();
            } else if ((e.key === 'm' || e.key === 'M') && !isPreview()) {
                e.preventDefault();
                toggleMirror();
            } else if ((e.key === 'g' || e.key === 'G') && !isPreview()) {
                e.preventDefault();
                toggleGrid();
            } else if ((e.key === 'c' || e.key === 'C') && isPreview() && !isClipPreview()) {
                e.preventDefault();
                copyCapture();
            }
        }

        function showCameraContextMenu(event) {
            if (typeof ctx.showContextMenu !== 'function') return false;
            const captured = !state.capturedDataURL && !state.videoURL;
            const photo = !!state.capturedDataURL && !state.videoURL;
            event.preventDefault();
            ctx.showContextMenu(event.clientX, event.clientY, [
                { labelKey: state.recording ? 'desktop.camera_record_stop' : (state.mode === 'video' ? 'desktop.camera_record' : 'desktop.camera_capture'), icon: state.mode === 'video' ? 'video' : 'camera', disabled: !captured && !state.recording, action: function () { beginCapture(); } },
                { labelKey: 'desktop.camera_retake', icon: 'refresh', disabled: captured, action: function () { retakeBtn.click(); } },
                { type: 'separator' },
                { labelKey: 'desktop.camera_save', icon: 'save', disabled: captured || state.saving, action: function () { saveBtn.click(); } },
                { labelKey: 'desktop.camera_download', icon: 'download', disabled: captured, action: function () { downloadBtn.click(); } },
                { labelKey: 'desktop.camera_copy', icon: 'copy', disabled: !photo || state.copying, action: function () { copyBtn.click(); } },
                { labelKey: 'desktop.camera_send_agent', icon: 'chat', disabled: !photo || state.sending, action: function () { sendBtn.click(); } },
                { type: 'separator' },
                { labelKey: 'desktop.camera_switch', icon: 'refresh', disabled: !captured || state.recording, action: function () { switchCamera(); } },
                { labelKey: 'desktop.camera_mirror', icon: 'columns', disabled: !captured, action: function () { toggleMirror(); } },
                { labelKey: 'desktop.camera_grid', icon: 'grid', disabled: !captured, action: function () { toggleGrid(); } }
            ]);
            return true;
        }
        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);
        host.addEventListener('contextmenu', function (event) {
            if (showCameraContextMenu(event)) return;
        });

        captureBtn.addEventListener('click', beginCapture);
        switchBtn.addEventListener('click', switchCamera);
        deviceSelect.addEventListener('change', selectDevice);
        timerBtn.addEventListener('click', cycleTimer);
        mirrorBtn.addEventListener('click', toggleMirror);
        gridBtn.addEventListener('click', toggleGrid);
        modeBtns.forEach(function (btn) {
            btn.addEventListener('click', function () { setMode(btn.dataset.mode); });
        });
        retakeBtn.addEventListener('click', retake);
        saveBtn.addEventListener('click', saveCapture);
        downloadBtn.addEventListener('click', downloadCapture);
        copyBtn.addEventListener('click', copyCapture);
        sendBtn.addEventListener('click', sendToAgent);
        countdownEl.addEventListener('click', cancelCountdown);
        recentsEl.addEventListener('click', function (event) {
            var button = event.target.closest('button[data-recent]');
            if (!button || state.recording) return;
            var item = state.recents.find(function (entry) { return entry.id === button.dataset.recent; });
            if (!item) return;
            cancelCountdown();
            clearClip();
            state.capturedDataURL = item.dataURL;
            stopStream();
            updateUI();
        });
        host.addEventListener('keydown', handleKeydown);
        var deviceChangeHandler = function () { detectCameras(); };
        if (navigator.mediaDevices && navigator.mediaDevices.addEventListener) {
            navigator.mediaDevices.addEventListener('devicechange', deviceChangeHandler);
        }

        disposers.set(windowId, function () {
            state.disposed = true;
            cancelCountdown();
            if (state.recording) stopRecording(true);
            stopStream();
            if (state.recordClock) clearInterval(state.recordClock);
            clearClip();
            if (navigator.mediaDevices && navigator.mediaDevices.removeEventListener) {
                navigator.mediaDevices.removeEventListener('devicechange', deviceChangeHandler);
            }
            host.removeEventListener('keydown', handleKeydown);
        });

        updateUI();
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
