// Storymaker app API client: /api/desktop/storymaker (contract section 5) plus the static catalog files. Every failure is
// thrown as {stm, status, code, message, issues, network} (S.errors.normalize); the server's answers are flat
// {error, code, message, issues?}. Story saves carry If-Match, a new code scene If-None-Match: *, a render If-Match and an
// Idempotency-Key. The project event stream is an EventSource that hands every `job` event on. One AbortController per window
// cancels whatever is in flight when the window closes. Classic script, publishes S.createApi.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const BASE = '/api/desktop/storymaker';
    const CATALOG_BASE = '/img/storymaker/';
    const MAX_JSON_BYTES = 16 * 1048576;
    const AUDIO_KEY_RE = /^[0-9a-f]{64}$/;
    const AUDIO_EXTS = ['wav', 'mp3'];
    const EVENTS_WAIT_MIN_MS = 1000;
    const EVENTS_WAIT_MAX_MS = 30000;
    const EVENT_SOURCE_CLOSED = 2;

    // The only place that builds an audio address. '' means refused: a key that is not 64 lower-case hex digits, or an ext other
    // than wav and mp3.
    function audioUrl(id, folder, key, ext) {
        if (typeof key !== 'string' || !AUDIO_KEY_RE.test(key) || AUDIO_EXTS.indexOf(ext) < 0) return '';
        return BASE + '/projects/' + encodeURIComponent(id) + '/audio/' + folder + '/' + key + '.' + ext;
    }

    // An event stream that comes back. The browser retries a dropped connection by itself, but an error answer such as a 502 from a
    // proxy or a 401 closes an EventSource for good. This then opens a new one after a wait of 1 s, doubling up to 30 s, and an open
    // resets the wait. Every new connection starts with the project's unfinished and latest jobs (initialJobs on the server), so the
    // panels learn what ended meanwhile and need no poll of their own. A throw from the first open() goes to the caller.
    function reopeningSource(open) {
        const listeners = [];
        let source = null, timer = null, closed = false, wait = EVENTS_WAIT_MIN_MS;

        function retryLater() {
            timer = setTimeout(() => {
                timer = null;
                try { attach(open()); } catch (_) { retryLater(); }
            }, wait);
            wait = Math.min(wait * 2, EVENTS_WAIT_MAX_MS);
        }

        function attach(next) {
            source = next;
            listeners.forEach(([name, fn]) => next.addEventListener(name, fn));
            next.addEventListener('open', () => { wait = EVENTS_WAIT_MIN_MS; });
            next.addEventListener('error', () => {
                if (!closed && timer === null && next === source && next.readyState === EVENT_SOURCE_CLOSED) retryLater();
            });
        }

        attach(open());
        return {
            addEventListener(name, fn) {
                listeners.push([name, fn]);
                source.addEventListener(name, fn);
            },
            close() {
                closed = true;
                clearTimeout(timer);
                timer = null;
                source.close();
            }
        };
    }

    function createApi(options) {
        const o = options || {};
        const lifetime = new AbortController();
        const doFetch = o.fetch || ((url, init) => window.fetch(url, init));
        const makeEventSource = o.EventSource || (typeof window.EventSource === 'function' ? url => new window.EventSource(url) : null);

        async function request(method, path, init) {
            const spec = init || {};
            const headers = Object.assign({}, spec.headers || {});
            const options = { method, headers, credentials: 'same-origin', cache: 'no-store', signal: spec.signal || lifetime.signal };
            if (spec.body !== undefined) options.body = spec.body;
            let response;
            try {
                response = await doFetch(spec.raw ? path : BASE + path, options);
            } catch (err) {
                if (err && err.name === 'AbortError') throw Object.assign(S.errors.normalize({ message: 'aborted' }), { aborted: true });
                throw S.errors.normalize({ message: String(err && err.message || err) });
            }
            if (!response.ok) {
                let body = null;
                try { body = await response.json(); } catch (_) { body = null; }
                throw S.errors.fromResponse(response.status, body, response.headers.get('retry-after'));
            }
            return response;
        }

        async function json(method, path, init) {
            const response = await request(method, path, init);
            if (response.status === 204) return {};
            try {
                return await response.json();
            } catch (_) {
                throw S.errors.normalize({ message: 'The server answered with something that is not JSON.' });
            }
        }

        const jsonHeaders = extra => Object.assign({ 'Content-Type': 'application/json' }, extra || {});

        const api = {
            abort() { lifetime.abort(); },
            playerUrl() { return BASE + '/player'; },

            status() { return json('GET', '/status'); },
            async listProjects() {
                const body = await json('GET', '/projects');
                return Array.isArray(body.projects) ? body.projects : [];
            },
            createProject(spec) { return json('POST', '/projects', { headers: jsonHeaders(), body: JSON.stringify(spec) }); },
            getProject(id) { return json('GET', '/projects/' + encodeURIComponent(id)); },
            deleteProject(id) { return json('DELETE', '/projects/' + encodeURIComponent(id)); },

            // text is the story JSON; the answer is {etag, issues, defaults_applied, story}
            saveStory(id, text, etag) {
                return json('PUT', '/projects/' + encodeURIComponent(id) + '/story', { headers: jsonHeaders({ 'If-Match': etag }), body: text });
            },

            async getCode(id, scene) {
                const response = await request('GET', '/projects/' + encodeURIComponent(id) + '/scenes/' + encodeURIComponent(scene) + '/code');
                return { text: await response.text(), etag: response.headers.get('ETag') || '' };
            },
            // etag null creates the file (If-None-Match: *); the answer is {etag, ok, errors:[{line, column, message}]}
            putCode(id, scene, source, etag) {
                const headers = { 'Content-Type': 'text/javascript' };
                if (etag) headers['If-Match'] = etag; else headers['If-None-Match'] = '*';
                return json('PUT', '/projects/' + encodeURIComponent(id) + '/scenes/' + encodeURIComponent(scene) + '/code', { headers, body: source });
            },
            // the bundled scene for the player; null when the last bundle failed
            async getBundle(id, scene) {
                try {
                    const response = await request('GET', '/projects/' + encodeURIComponent(id) + '/scenes/' + encodeURIComponent(scene) + '/bundle');
                    return await response.text();
                } catch (err) {
                    if (err && err.status === 404) return null;
                    throw err;
                }
            },

            // generate is true only from the music panel's Generate button. Without it the server starts no paid generation.
            music(id, durationMs, generate) {
                const body = { duration_ms: durationMs };
                if (generate) body.generate = true;
                return json('POST', '/projects/' + encodeURIComponent(id) + '/music', { headers: jsonHeaders(), body: JSON.stringify(body) });
            },
            noisemakerTracks() { return json('GET', '/api/desktop/noisemaker/tracks', { raw: true }); },

            async startRender(id, etag, key, options) {
                const spec = { headers: { 'If-Match': etag, 'Idempotency-Key': key || S.uuid() } };
                if (options) {
                    spec.headers['Content-Type'] = 'application/json';
                    spec.body = JSON.stringify(options);
                }
                const body = await json('POST', '/projects/' + encodeURIComponent(id) + '/render', spec);
                return body.job || body;
            },
            async getJob(id) {
                const body = await json('GET', '/jobs/' + encodeURIComponent(id));
                return body.job || body;
            },
            async cancelJob(id) {
                const body = await json('POST', '/jobs/' + encodeURIComponent(id) + '/cancel');
                return body.job || body;
            },

            // {providers}; works read-only
            voices() { return json('GET', '/voices'); },
            // the AudioAnswer of the stored story: {narration, missing, chars, music, music_issue?}
            narration(id) { return json('GET', '/projects/' + encodeURIComponent(id) + '/narration'); },
            // {job}: 202 a new job, 200 the job already running, 200 {job: null} when nothing is missing
            narrate(id) { return json('POST', '/projects/' + encodeURIComponent(id) + '/narrate'); },
            async listJobs(id) {
                const body = await json('GET', '/projects/' + encodeURIComponent(id) + '/jobs');
                return Array.isArray(body.jobs) ? body.jobs : [];
            },
            // audio addresses, only from values of the parent's own /narration answer; '' means refused
            narrationUrl(id, hash, ext) { return audioUrl(id, 'narration', hash, ext); },
            musicUrl(id, key, ext) { return audioUrl(id, 'music', key, ext); },

            // events(projectId, onJob, onState) -> {close()}; onState('open' | 'error')
            events(projectId, onJob, onState) {
                if (!makeEventSource) return { close() {} };
                let source = null;
                try { source = reopeningSource(() => makeEventSource(BASE + '/projects/' + encodeURIComponent(projectId) + '/events')); } catch (_) { return { close() {} }; }
                source.addEventListener('job', event => {
                    let body = null;
                    try { body = JSON.parse(event.data); } catch (_) { return; }
                    const job = body && body.job ? body.job : body;
                    if (job && typeof job === 'object' && typeof job.id === 'string') onJob(job);
                });
                source.addEventListener('open', () => onState && onState('open'));
                source.addEventListener('error', () => onState && onState('error'));
                return { close() { try { source.close(); } catch (_) { /* gone */ } } };
            },

            // static files below /img/storymaker and /fonts: the catalog, its part files and the fonts
            async catalogJSON(path) {
                const response = await request('GET', CATALOG_BASE + String(path).replace(/^\/+/, ''), { raw: true });
                const text = await response.text();
                if (text.length > MAX_JSON_BYTES) throw S.errors.normalize({ message: 'The catalog file is too large.' });
                return JSON.parse(text);
            },
            // maxBytes is the byte limit of a fallback face; the default stays 8 MiB
            async fontBuffer(file, maxBytes) {
                const response = await request('GET', String(file), { raw: true });
                const buffer = await response.arrayBuffer();
                if (buffer.byteLength > (maxBytes > 0 ? maxBytes : 8 * 1048576)) throw S.errors.normalize({ message: 'The font is too large.' });
                return buffer;
            }
        };
        return api;
    }

    S.createApi = createApi;
    S.API_BASE = BASE;
})();
