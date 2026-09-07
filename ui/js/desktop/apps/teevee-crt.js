(function () {
    'use strict';

    const VERTEX = `attribute vec2 position;
varying vec2 uv;
void main(){uv=position*.5+.5;gl_Position=vec4(position,0.,1.);}`;
    const SIGNAL = `precision highp float;
varying vec2 uv;
uniform sampler2D source, previous;
uniform vec2 resolution, sourceSize;
uniform float elapsed, motion, time;
vec3 linearize(vec3 c){return pow(max(c,vec3(0.)),vec3(2.2));}
float luma(vec3 c){return dot(c,vec3(.2126,.7152,.0722));}
vec3 sampleBeam(vec2 p,vec2 dx){
    vec3 c=linearize(texture2D(source,p).rgb);
    vec3 soft=(linearize(texture2D(source,p-dx).rgb)+linearize(texture2D(source,p+dx).rgb))*.5;
    // A colour CRT resolves luminance more sharply than chrominance.
    vec3 chroma=mix(c,soft,.28);
    return max(chroma+vec3(luma(c)-luma(chroma)),vec3(0.));
}
void main(){
    vec2 q=uv*2.-1.;
    vec2 curved=q*(1.+.055*dot(q,q))/1.11;
    vec2 p=curved*.5+.5;
    float target=resolution.x/resolution.y, aspect=sourceSize.x/sourceSize.y;
    vec2 fit=vec2(min(1.,aspect/target),min(1.,target/aspect))*.97;
    p=(p-.5)/fit+.5;
    if(any(lessThan(p,vec2(0.)))||any(greaterThan(p,vec2(1.)))){
        gl_FragColor=vec4(0.,0.,0.,1.);return;
    }
    vec2 pixel=1./sourceSize;
    vec2 dx=vec2(max(pixel.x,1./resolution.x)*.8,0.);
    vec3 c=sampleBeam(p,dx);
    float edge=dot(q,q)*.22;
    c.r=mix(c.r,linearize(texture2D(source,p+dx*edge).rgb).r,.6);
    c.b=mix(c.b,linearize(texture2D(source,p-dx*edge).rgb).b,.6);
    vec3 halo=(linearize(texture2D(source,p+dx*2.).rgb)+linearize(texture2D(source,p-dx*2.).rgb)
        +linearize(texture2D(source,p+vec2(0.,pixel.y*2.)).rgb)+linearize(texture2D(source,p-vec2(0.,pixel.y*2.)).rgb))*.25;
    c+=max(halo-.38,vec3(0.))*.065;
    // Integrate the line pattern as density falls, rather than aliasing it.
    float lines=min(288.,resolution.y*.38);
    float beam=.5+.5*cos(p.y*lines*6.283185);
    float strength=.15*smoothstep(200.,650.,resolution.y)*(1.-.55*clamp(luma(c),0.,1.));
    c*=1.-strength*beam;
    float maskStrength=.10*smoothstep(500.,1000.,resolution.x);
    float cell=mod(floor(gl_FragCoord.x),3.);
    vec3 mask=cell<1.?vec3(1.,.8,.8):(cell<2.?vec3(.8,1.,.8):vec3(.8,.8,1.));
    c*=mix(vec3(1.),mask,maskStrength/.2);
    c*=1.-.065*dot(q,q);
    float noise=fract(sin(dot(gl_FragCoord.xy,vec2(12.9898,78.233))+floor(time*25.))*43758.5453)-.5;
    c+=noise*.0015*motion;
    vec3 old=linearize(texture2D(previous,uv).rgb);
    c=max(c,old*exp(-elapsed/.012)*motion*.22);
    gl_FragColor=vec4(pow(clamp(c,0.,1.),vec3(1./2.2)),1.);
}`;
    const PRESENT = `precision mediump float;varying vec2 uv;uniform sampler2D source;
void main(){gl_FragColor=texture2D(source,uv);}`;

    function create({ video, mount, onStatus }) {
        const canvas = document.createElement('canvas');
        canvas.className = 'teevee-crt-canvas';
        canvas.setAttribute('aria-hidden', 'true');
        canvas.hidden = true;
        mount.appendChild(canvas);
        const windowEl = mount.closest('.vd-window');
        const reduced = window.matchMedia('(prefers-reduced-motion: reduce)');
        let enabled = false, disposed = false, failed = false, lost = false;
        let mode = '', callback = 0, callbackKind = '', lastTime = -1, lastDraw = 0;
        let gl, signal, present, buffer, input, frames = [], write = 0, restored = false;
        const listeners = [];

        function listen(target, name, fn) {
            target.addEventListener(name, fn);
            listeners.push(() => target.removeEventListener(name, fn));
        }
        function status(value) {
            canvas.hidden = value !== 'webgl';
            mount.dataset.crtMode = value;
            if (value === mode) return;
            mode = value;
            if (onStatus) onStatus(value);
        }
        function visible() {
            return !document.hidden && mount.isConnected && mount.getClientRects().length > 0
                && (!windowEl || (!windowEl.classList.contains('minimized')
                    && !windowEl.classList.contains('vd-space-hidden') && windowEl.dataset.spaceHidden !== 'true'));
        }
        function cancel() {
            if (callback) {
                if (callbackKind === 'video') video.cancelVideoFrameCallback(callback);
                else cancelAnimationFrame(callback);
            }
            callback = 0;
        }
        function compile(type, source) {
            const shader = gl.createShader(type);
            gl.shaderSource(shader, source);
            gl.compileShader(shader);
            if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
                gl.deleteShader(shader);
                throw new Error('CRT shader');
            }
            return shader;
        }
        function program(fragment) {
            const p = gl.createProgram();
            const shaders = [];
            try {
                shaders.push(compile(gl.VERTEX_SHADER, VERTEX));
                shaders.push(compile(gl.FRAGMENT_SHADER, fragment));
                shaders.forEach(s => gl.attachShader(p, s));
                gl.bindAttribLocation(p, 0, 'position');
                gl.linkProgram(p);
                if (!gl.getProgramParameter(p, gl.LINK_STATUS)) throw new Error('CRT program');
                const uniforms = {};
                ['source', 'previous', 'resolution', 'sourceSize', 'elapsed', 'motion', 'time'].forEach(k => {
                    uniforms[k] = gl.getUniformLocation(p, k);
                });
                return { p, uniforms };
            } catch (err) {
                gl.deleteProgram(p);
                throw err;
            } finally { shaders.forEach(s => gl.deleteShader(s)); }
        }
        function texture() {
            const tex = gl.createTexture();
            gl.bindTexture(gl.TEXTURE_2D, tex);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
            return tex;
        }
        function release() {
            if (gl && !lost) {
                frames.forEach(f => { gl.deleteFramebuffer(f.fbo); gl.deleteTexture(f.tex); });
                if (input) gl.deleteTexture(input);
                if (buffer) gl.deleteBuffer(buffer);
                if (signal) gl.deleteProgram(signal.p);
                if (present) gl.deleteProgram(present.p);
            }
            frames = []; input = buffer = signal = present = null;
        }
        function init() {
            gl = canvas.getContext('webgl', { alpha: false, antialias: false, depth: false, stencil: false, powerPreference: 'low-power' });
            if (!gl) throw new Error('CRT unavailable');
            signal = program(SIGNAL);
            present = program(PRESENT);
            buffer = gl.createBuffer();
            gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
            gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.STATIC_DRAW);
            gl.enableVertexAttribArray(0);
            gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
            input = texture();
            gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, true);
        }
        function resize() {
            const rect = mount.getBoundingClientRect();
            const dpr = Math.min(devicePixelRatio || 1, 2);
            const scale = Math.min(dpr, 1920 / Math.max(1, rect.width), 1440 / Math.max(1, rect.height));
            const width = Math.max(1, Math.round(rect.width * scale));
            const height = Math.max(1, Math.round(rect.height * scale));
            if (canvas.width === width && canvas.height === height && frames.length) return;
            canvas.width = width; canvas.height = height;
            frames.forEach(f => { gl.deleteFramebuffer(f.fbo); gl.deleteTexture(f.tex); });
            frames = [];
            for (let i = 0; i < 2; i++) {
                const tex = texture(), fbo = gl.createFramebuffer();
                frames.push({ tex, fbo });
                gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, width, height, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
                gl.bindFramebuffer(gl.FRAMEBUFFER, fbo);
                gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0);
                if (gl.checkFramebufferStatus(gl.FRAMEBUFFER) !== gl.FRAMEBUFFER_COMPLETE) throw new Error('CRT framebuffer');
                gl.clearColor(0, 0, 0, 1); gl.clear(gl.COLOR_BUFFER_BIT);
            }
            lastDraw = 0;
        }
        function fallback() {
            failed = true;
            cancel(); release();
            status(enabled ? 'basic' : 'off');
        }
        function draw(now, force) {
            if (disposed || !enabled || failed || lost || !visible()) return;
            if (video.readyState < 2 || !video.videoWidth) { status('waiting'); return; }
            if (!force && lastTime === video.currentTime) return;
            try {
                if (!signal) init();
                resize();
                gl.activeTexture(gl.TEXTURE0);
                gl.bindTexture(gl.TEXTURE_2D, input);
                // Playback stays origin-policy compatible. A blocked texture gets the native fallback.
                gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, video);
                if (gl.getError() !== gl.NO_ERROR) throw new Error('CRT video upload');
                const target = frames[write], previous = frames[1 - write];
                gl.viewport(0, 0, canvas.width, canvas.height);
                gl.useProgram(signal.p);
                gl.uniform1i(signal.uniforms.source, 0);
                gl.activeTexture(gl.TEXTURE1); gl.bindTexture(gl.TEXTURE_2D, previous.tex);
                gl.uniform1i(signal.uniforms.previous, 1);
                gl.uniform2f(signal.uniforms.resolution, canvas.width, canvas.height);
                gl.uniform2f(signal.uniforms.sourceSize, video.videoWidth, video.videoHeight);
                gl.uniform1f(signal.uniforms.elapsed, lastDraw ? Math.min((now - lastDraw) / 1000, 1) : 1);
                gl.uniform1f(signal.uniforms.time, now / 1000);
                gl.uniform1f(signal.uniforms.motion, reduced.matches || document.body.dataset.animations === 'false' ? 0 : 1);
                gl.bindFramebuffer(gl.FRAMEBUFFER, target.fbo);
                gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
                gl.bindFramebuffer(gl.FRAMEBUFFER, null);
                gl.useProgram(present.p);
                gl.activeTexture(gl.TEXTURE0); gl.bindTexture(gl.TEXTURE_2D, target.tex);
                gl.uniform1i(present.uniforms.source, 0);
                gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
                write = 1 - write; lastTime = video.currentTime; lastDraw = now;
                status('webgl');
            } catch (_) { fallback(); }
        }
        function schedule() {
            if (callback || disposed || !enabled || failed || lost || video.paused || !visible()) return;
            if (typeof video.requestVideoFrameCallback === 'function') {
                callbackKind = 'video';
                callback = video.requestVideoFrameCallback(now => { callback = 0; draw(now, true); schedule(); });
            } else {
                callbackKind = 'raf';
                callback = requestAnimationFrame(now => {
                    callback = 0;
                    if (now - lastDraw >= 1000 / 60) draw(now, false);
                    schedule();
                });
            }
        }
        function refresh() {
            cancel();
            if (!enabled || disposed) return;
            if (!visible()) { lastDraw = 0; return; }
            draw(performance.now(), true); schedule();
        }
        function reset() {
            if (disposed) return;
            cancel(); lastDraw = 0; lastTime = -1; failed = false;
            // Reallocate the tiny history surfaces once, never carry a previous station into a new one.
            if (gl && !lost) frames.forEach(f => { gl.deleteFramebuffer(f.fbo); gl.deleteTexture(f.tex); });
            frames = [];
            status(enabled ? 'waiting' : 'off');
        }
        listen(video, 'playing', refresh);
        listen(video, 'loadeddata', refresh);
        listen(video, 'pause', cancel);
        listen(video, 'seeking', () => { lastDraw = 0; lastTime = -1; });
        listen(video, 'seeked', refresh);
        listen(video, 'emptied', reset);
        listen(video, 'ended', cancel);
        listen(document, 'visibilitychange', refresh);
        listen(reduced, 'change', refresh);
        listen(canvas, 'webglcontextlost', event => {
            event.preventDefault(); lost = true; fallback();
        });
        listen(canvas, 'webglcontextrestored', () => {
            lost = false;
            if (disposed || restored) return;
            restored = true; failed = false; refresh();
        });
        const resizeObserver = new ResizeObserver(refresh);
        resizeObserver.observe(mount);
        const visibilityObserver = new MutationObserver(refresh);
        if (windowEl) visibilityObserver.observe(windowEl, { attributes: true, attributeFilter: ['class', 'style', 'data-space-hidden'] });
        visibilityObserver.observe(document.body, { attributes: true, attributeFilter: ['data-animations'] });
        return {
            setEnabled(value) {
                enabled = !!value;
                cancel();
                if (!enabled) { status('off'); lastDraw = 0; }
                else if (failed || lost) status('basic');
                else refresh();
            },
            reset,
            dispose() {
                if (disposed) return;
                disposed = true; enabled = false;
                cancel(); listeners.forEach(remove => remove());
                resizeObserver.disconnect(); visibilityObserver.disconnect();
                release(); status('off');
                if (gl && !lost) { const extension = gl.getExtension('WEBGL_lose_context'); if (extension) extension.loseContext(); }
                canvas.remove(); gl = null;
            }
        };
    }
    window.TeeVeeCrt = { create };
})();
