// Adapt the pinned upstream player to AuraGo's revision-bound signaling lease.
export function mountGo2RTCViewer({ stream, role = 'main', mode }) {
    const video = document.createElement('video-stream');
    const rtcVideo = video.onpcvideo.bind(video);
    video.onpcvideo = function (otherVideo) {
        // Upstream closes signaling after selecting RTC. Keep it alive so the
        // sidecar's lifecycle patch can revoke the media peer with this lease.
        const socket = this.ws;
        this.ws = null;
        rtcVideo(otherVideo);
        this.ws = socket;
        if (socket && socket.readyState === WebSocket.OPEN) this.wsState = WebSocket.OPEN;
    };
    const socketClosed = video.onclose.bind(video);
    video.onclose = function () {
        this.ondisconnect();
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
