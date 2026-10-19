// Adapt the pinned upstream player to AuraGo's revision-bound signaling lease.
export function mountGo2RTCViewer({ stream, role = 'main', mode }) {
    const video = document.createElement('video-stream');
    const splitMedia = mode.includes('webrtc') && ['mse', 'hls', 'mp4', 'mjpeg'].some(value => mode.includes(value));
    let mediaSocket = null;
    let queuedMedia = [];
    const send = video.send.bind(video);
    video.send = function (message) {
        if (!splitMedia || message.type.startsWith('webrtc/')) return send(message);
        if (!mediaSocket) return;
        if (mediaSocket.readyState === WebSocket.OPEN) mediaSocket.send(JSON.stringify(message));
        else if (mediaSocket.readyState === WebSocket.CONNECTING) queuedMedia.push(message);
    };
    const opened = video.onopen.bind(video);
    video.onopen = function () {
        if (splitMedia) {
            const socket = new WebSocket(this.wsURL);
            socket.binaryType = 'arraybuffer';
            mediaSocket = socket;
            queuedMedia = [];
            socket.addEventListener('open', () => {
                if (mediaSocket !== socket) return;
                for (const message of queuedMedia) socket.send(JSON.stringify(message));
                queuedMedia = [];
            });
            socket.addEventListener('message', event => {
                if (mediaSocket !== socket) return;
                if (typeof event.data === 'string') {
                    const message = JSON.parse(event.data);
                    for (const key in this.onmessage) if (key !== 'webrtc') this.onmessage[key](message);
                } else if (this.ondata) this.ondata(event.data);
            });
            socket.addEventListener('close', () => {
                if (mediaSocket !== socket) return;
                mediaSocket = null;
                queuedMedia = [];
                if (this.ws) this.ws.close();
            });
        }
        return opened();
    };
    const rtcVideo = video.onpcvideo.bind(video);
    video.onpcvideo = function (otherVideo) {
        // Let upstream close the losing media connection while retaining the
        // separate signaling lease needed for server-side RTC revocation.
        const signaling = this.ws;
        const media = mediaSocket;
        mediaSocket = null;
        this.ws = media;
        try { rtcVideo(otherVideo); }
        finally { this.ws = signaling; }
        if (this.pcState === WebSocket.OPEN) {
            queuedMedia = [];
            this.ondata = null;
        } else mediaSocket = media;
        if (signaling && signaling.readyState === WebSocket.OPEN) this.wsState = WebSocket.OPEN;
    };
    const disconnected = video.ondisconnect.bind(video);
    video.ondisconnect = function () {
        if (this.reconnectTID) {
            clearTimeout(this.reconnectTID);
            this.reconnectTID = 0;
        }
        const media = mediaSocket;
        mediaSocket = null;
        queuedMedia = [];
        if (media) media.close();
        return disconnected();
    };
    const socketClosed = video.onclose.bind(video);
    video.onclose = function () {
        const reconnect = this.wsState !== WebSocket.CLOSED;
        this.ondisconnect();
        if (!reconnect) return false;
        this.wsState = WebSocket.CONNECTING;
        return socketClosed();
    };
    const mp4 = video.onmp4.bind(video);
    video.onmp4 = function () {
        // v1.9.14 passes this native method as a callback without its receiver.
        this.video.canPlayType = this.video.canPlayType.bind(this.video);
        return mp4();
    };
    video.pcConfig = { bundlePolicy: 'max-bundle', iceServers: [] };
    video.src = '/api/go2rtc/proxy/api/ws?src=' + encodeURIComponent(stream) + '&role=' + encodeURIComponent(role);
    video.mode = mode;
    document.body.appendChild(video);
    return video;
}
