// CYD Creator API client: request() with the typed ApiError envelope {error, code, details} and
// one wrapper per plan-2/3 endpoint under /api/desktop/cyd-creator. Wrappers return the payload.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const BASE = '/api/desktop/cyd-creator';

    class ApiError extends Error {
        constructor(status, code, message, details) {
            super(message || code || ('HTTP ' + status));
            this.name = 'ApiError';
            this.status = Number(status) || 0;
            this.code = code || '';
            this.details = details && typeof details === 'object' ? details : {};
        }
    }

    // AuraGo's desktop auth and policy layer answers {"error": "<code>", "message"?} without a code
    // field (desktop_scope_required, unauthorized, desktop_readonly): such an error is the code.
    const AURAGO_ERROR_CODE = /^[A-Za-z][A-Za-z0-9_]{0,63}$/;

    // toError builds the ApiError of a failed response. body is the parsed JSON envelope or the raw
    // text; status 0 means the request never reached the server.
    function toError(status, body) {
        if (body && typeof body === 'object') {
            if (!body.code && typeof body.error === 'string' && AURAGO_ERROR_CODE.test(body.error)) {
                return new ApiError(status, body.error, String(body.message || body.error), body.details);
            }
            return new ApiError(status, String(body.code || ''), String(body.error || body.message || ''), body.details);
        }
        const text = typeof body === 'string' ? body.trim().slice(0, 300) : '';
        return new ApiError(status, '', text || ('HTTP ' + status), {});
    }

    function enc(value) { return encodeURIComponent(String(value)); }
    function encPath(path) { return String(path).split('/').map(encodeURIComponent).join('/'); }
    function url(path) { return BASE + path; }

    // request(method, path, {json, body, headers, signal, raw}) → {status, data, headers}. path is
    // relative to BASE. JSON answers are parsed unless raw is set (then data is the text); errors
    // always parse the JSON envelope and throw ApiError.
    async function request(method, path, opts) {
        const o = opts || {};
        const headers = Object.assign({}, o.headers || {});
        let body = o.body;
        if (o.json !== undefined) {
            headers['Content-Type'] = 'application/json';
            body = JSON.stringify(o.json);
        }
        let resp;
        try {
            resp = await fetch(url(path), { method, headers, body, signal: o.signal, credentials: 'same-origin', cache: 'no-store' });
        } catch (err) {
            if (err && err.name === 'AbortError') throw err;
            throw new ApiError(0, '', (err && err.message) || 'network error', {});
        }
        const type = (resp.headers && resp.headers.get('content-type')) || '';
        const text = resp.status === 204 ? '' : await resp.text();
        let data = text;
        if ((!o.raw || !resp.ok) && text && type.includes('json')) {
            try { data = JSON.parse(text); } catch (_) { data = text; }
        }
        if (resp.status === 204) data = null;
        if (!resp.ok) throw toError(resp.status, data);
        return { status: resp.status, data, headers: resp.headers };
    }

    const get = path => request('GET', path).then(r => r.data);
    const post = (path, json) => request('POST', path, { json: json || {} }).then(r => r.data);

    function projectPath(id, rest) { return '/projects/' + enc(id) + (rest || ''); }

    P.api = {
        BASE,
        ApiError,
        toError,
        request,

        capabilities: () => get('/capabilities'),
        icons: () => get('/icons'),
        importIcon: (id, selection, version) => request('POST', projectPath(id, '/icons'), {
            json: selection, headers: { 'If-Match': String(version) }
        }).then(r => r.data),
        pullToolchain: () => post('/toolchain/pull').then(d => d.toolchain),

        listProjects: () => get('/projects').then(d => d.projects || []),
        createProject: body => post('/projects', body).then(d => d.project),
        getProject: id => get(projectPath(id)).then(d => d.project),
        renameProject: (id, name) => request('PATCH', projectPath(id), { json: { name } }).then(r => r.data.project),
        deleteProject: id => request('DELETE', projectPath(id)).then(() => true),

        getLayout: id => get(projectPath(id, '/layout')),
        putLayout: (id, project, version) => request('PUT', projectPath(id, '/layout'), {
            json: { project },
            headers: { 'If-Match': String(version) }
        }).then(r => r.data),

        listFiles: id => get(projectPath(id, '/files')).then(d => d.files || []),
        readFile: (id, path) => request('GET', projectPath(id, '/files/' + encPath(path)), { raw: true })
            .then(r => ({ text: String(r.data || ''), sha: r.headers.get('X-CYD-SHA256') || '' })),
        writeFile: (id, path, text, ifMatch) => request('PUT', projectPath(id, '/files/' + encPath(path)), {
            body: text,
            headers: { 'Content-Type': 'text/plain; charset=utf-8', 'If-Match': ifMatch || '*' }
        }).then(r => r.data.sha256),
        deleteFile: (id, path, ifMatch) => request('DELETE', projectPath(id, '/files/' + encPath(path)), {
            headers: { 'If-Match': ifMatch || '*' }
        }).then(() => true),

        uploadAsset: (id, file) => {
            const form = new FormData();
            form.append('file', file, file && file.name ? file.name : 'image.png');
            return request('POST', projectPath(id, '/assets'), { body: form }).then(r => r.data);
        },
        deleteAsset: (id, assetId) => request('DELETE', projectPath(id, '/assets/' + enc(assetId))).then(r => r.data),
        assetUrl: (id, assetId) => url(projectPath(id, '/assets/' + enc(assetId))),

        startBuild: id => post(projectPath(id, '/builds')).then(d => d.build),
        listBuilds: id => get(projectPath(id, '/builds')).then(d => d.builds || []),
        getBuild: buildId => get('/builds/' + enc(buildId)).then(d => d.build),
        cancelBuild: buildId => post('/builds/' + enc(buildId) + '/cancel').then(() => true),
        buildLog: buildId => request('GET', '/builds/' + enc(buildId) + '/log', { raw: true }).then(r => String(r.data || '')),
        buildLogUrl: buildId => url('/builds/' + enc(buildId) + '/log'),
        artifactUrl: (buildId, name) => url('/builds/' + enc(buildId) + '/artifacts/' + enc(name)),
        decodeBacktrace: (buildId, addresses) => post('/builds/' + enc(buildId) + '/decode-backtrace', { addresses }).then(d => d.frames || []),

        eventsUrl: (id, after) => url(projectPath(id, '/events')) + (Number(after) > 0 ? '?after=' + enc(Number(after)) : ''),

        startJob: (id, body) => post(projectPath(id, '/jobs'), body).then(d => d.job),
        listJobs: id => get(projectPath(id, '/jobs')).then(d => d.jobs || []),
        getJob: jobId => get('/jobs/' + enc(jobId)).then(d => d.job),
        confirmJob: (jobId, prompt) => post('/jobs/' + enc(jobId) + '/confirm', prompt ? { prompt } : {}).then(d => d.job),
        cancelJob: jobId => post('/jobs/' + enc(jobId) + '/cancel').then(() => true),
        discardDraft: id => post(projectPath(id, '/draft/discard')).then(() => true),
        listMessages: id => get(projectPath(id, '/messages')).then(d => d.messages || []),

        listRevisions: id => get(projectPath(id, '/revisions')).then(d => d.revisions || []),
        revisionDiff: revisionId => get('/revisions/' + enc(revisionId) + '/diff'),
        restoreRevision: revisionId => post('/revisions/' + enc(revisionId) + '/restore')
    };
})();
