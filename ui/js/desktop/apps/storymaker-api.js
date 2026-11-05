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

            async startRender(id, etag, key) {
                const body = await json('POST', '/projects/' + encodeURIComponent(id) + '/render', { headers: { 'If-Match': etag, 'Idempotency-Key': key || S.uuid() } });
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

            // events(projectId, onJob, onState) -> {close()}; onState('open' | 'error')
            events(projectId, onJob, onState) {
                if (!makeEventSource) return { close() {} };
                let source = null;
                try { source = makeEventSource(BASE + '/projects/' + encodeURIComponent(projectId) + '/events'); } catch (_) { return { close() {} }; }
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
