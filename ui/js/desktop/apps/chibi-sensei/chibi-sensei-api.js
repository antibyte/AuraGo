// Chibi Sensei API client: /api/desktop/chibi-sensei, one method per contract route. Every failure is thrown as
// {csn, status, code, message, issues, network} (P.errors.normalize); the server's answers are flat {error, code, message,
// issues?}. Project and row writes carry If-Match. The project event stream is an EventSource that comes back after an error
// answer closed it. One AbortController per window cancels whatever is in flight when the window closes. This is the only file of
// the app that calls fetch or creates an EventSource. Classic script, publishes P.createApi.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
    const BASE = '/api/desktop/chibi-sensei';
    const EVENTS_WAIT_MIN_MS = 1000;
    const EVENTS_WAIT_MAX_MS = 30000;
    const EVENT_SOURCE_CLOSED = 2;

    // An event stream that comes back. The browser retries a dropped connection by itself, but an error answer such as a 502 from a
    // proxy or a 401 closes an EventSource for good. This then opens a new one after a wait of 1 s, doubling up to 30 s, and an open
    // resets the wait. A throw from the first open() goes to the caller.
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

    // qs builds a query string from an object and drops undefined, null and empty values. 0 and false stay.
    function qs(obj) {
        const parts = [];
        Object.keys(obj || {}).forEach(key => {
            const value = obj[key];
            if (value === undefined || value === null || value === '') return;
            parts.push(encodeURIComponent(key) + '=' + encodeURIComponent(String(value)));
        });
        return parts.length ? '?' + parts.join('&') : '';
    }

    const enc = encodeURIComponent;
    const list = (body, name) => (body && Array.isArray(body[name]) ? body[name] : []);

    function createApi(options) {
        const o = options || {};
        const lifetime = new AbortController();
        const doFetch = o.fetch || ((url, init) => window.fetch(url, init));
        const makeEventSource = o.EventSource || (typeof window.EventSource === 'function' ? url => new window.EventSource(url) : null);

        async function request(method, path, init) {
            const spec = init || {};
            const headers = Object.assign({}, spec.headers || {});
            if (spec.ifMatch !== undefined && spec.ifMatch !== null && spec.ifMatch !== '') headers['If-Match'] = String(spec.ifMatch);
            const opts = { method, headers, credentials: 'same-origin', cache: 'no-store', signal: spec.signal || lifetime.signal };
            if (spec.body !== undefined) opts.body = spec.body;
            let response;
            try {
                response = await doFetch(BASE + path, opts);
            } catch (err) {
                if (err && err.name === 'AbortError') throw Object.assign(P.errors.normalize({ message: 'aborted' }), { aborted: true });
                throw P.errors.normalize({ message: String(err && err.message || err) });
            }
            if (!response.ok) {
                let body = null;
                try { body = await response.json(); } catch (_) { body = null; }
                throw P.errors.fromResponse(response.status, body, response.headers.get('retry-after'));
            }
            return response;
        }

        async function json(method, path, init) {
            const response = await request(method, path, init);
            if (response.status === 204) return {};
            try {
                return await response.json();
            } catch (_) {
                throw P.errors.normalize({ code: 'bad_response' });
            }
        }

        const jsonHeaders = () => ({ 'Content-Type': 'application/json' });
        const send = (method, path, payload, extra) => json(method, path, Object.assign({ headers: jsonHeaders(), body: JSON.stringify(payload === undefined ? {} : payload) }, extra || {}));
        const project = id => '/projects/' + enc(id);

        const api = {
            abort() { lifetime.abort(); },

            meta() { return json('GET', '/meta'); },

            async listProjects(archived) { return list(await json('GET', '/projects' + qs({ archived: archived ? 1 : 0 })), 'projects'); },
            async createProject(spec) { return (await send('POST', '/projects', spec)).project; },
            async getProject(id) { return (await json('GET', project(id))).project; },
            async patchProject(id, rev, patch) { return (await send('PATCH', project(id), patch, { ifMatch: rev })).project; },
            async archiveProject(id, archived) { return (await send('POST', project(id) + '/archive', { archived: !!archived })).project; },
            deleteProject(id, confirmName) { return send('POST', project(id) + '/delete', { confirm_name: confirmName }); },

            // {rows, total, offset, limit, data_rev}
            listRows(id, query) { return json('GET', project(id) + '/rows' + qs(query)); },
            // {rows, data_rev}
            addRows(id, rows) { return send('POST', project(id) + '/rows', { rows }); },
            async getRow(id, rowId) { return (await json('GET', project(id) + '/rows/' + enc(rowId))).row; },
            // {row, data_rev}
            putRow(id, rowId, rev, input) { return send('PUT', project(id) + '/rows/' + enc(rowId), input, { ifMatch: rev }); },
            // {affected, batch_id, data_rev}
            bulk(id, select, action, value, token) {
                const body = { select, action };
                if (value !== undefined && value !== null && value !== '') body.value = value;
                if (token) body.token = token;
                return send('POST', project(id) + '/rows/bulk', body);
            },
            // {count, token, expires_in_s}
            deletePreview(id, select) { return send('POST', project(id) + '/rows/delete-preview', { select }); },
            // {removed, batch_id, data_rev}
            dedupe(id) { return send('POST', project(id) + '/rows/dedupe', {}); },
            async history(id, limit) { return list(await json('GET', project(id) + '/history' + qs({ limit })), 'batches'); },
            // {restored, data_rev}
            undo(id, batchId) { return send('POST', project(id) + '/history/' + enc(batchId) + '/undo', {}); },

            runChecks(id) { return send('POST', project(id) + '/checks/run', {}); },
            // {result, stale}
            getChecks(id) { return json('GET', project(id) + '/checks'); },
            getSplit(id) { return json('GET', project(id) + '/split'); },
            reseed(id) { return send('POST', project(id) + '/split/reseed', {}); },

            async listDocuments(id) { return list(await json('GET', project(id) + '/documents'), 'documents'); },
            async addDocument(id, path) { return (await send('POST', project(id) + '/documents', { workspace_path: path })).document; },
            removeDocument(id, docId) { return json('DELETE', project(id) + '/documents/' + enc(docId)); },

            // {format, columns, mapping, sample, issues}
            importPreview(id, spec) { return send('POST', project(id) + '/imports/preview', spec); },
            async startImport(id, spec) { return (await send('POST', project(id) + '/imports', spec)).job; },
            async startExport(id, spec) { return (await send('POST', project(id) + '/exports', spec)).job; },
            async generate(id, spec) { return (await send('POST', project(id) + '/generate', spec)).job; },
            async clean(id, spec) { return (await send('POST', project(id) + '/clean', spec)).job; },

            // {suggestions, total}
            suggestions(jobId, offset, limit) { return json('GET', '/jobs/' + enc(jobId) + '/suggestions' + qs({ offset, limit })); },
            // ids is 'all' or an array of row ids; the answer is {applied, batch_id, data_rev}
            applySuggestions(jobId, ids) { return send('POST', '/jobs/' + enc(jobId) + '/apply', { accept: ids }); },

            async listJobs(id, activeOnly) { return list(await json('GET', project(id) + '/jobs' + qs({ active: activeOnly ? 1 : '' })), 'jobs'); },
            async getJob(jobId) { return (await json('GET', '/jobs/' + enc(jobId))).job; },
            async cancelJob(jobId) { return (await send('POST', '/jobs/' + enc(jobId) + '/cancel', {})).job; },

            plan(id, preset) { return json('GET', project(id) + '/plan' + qs({ preset })); },

            // {ready, admin, preset, checked_at, items, fix}
            preflight(id, preset) { return json('GET', project(id) + '/preflight' + qs({ preset })); },
            // {applied}; the server maps each flag name to its value
            enablePreflight(flags) { return send('POST', '/preflight/enable', { flags }); },
            async consent(profileId) { return (await send('POST', '/consents', { profile_id: profileId })).consent; },
            // {plan_token, expires_in_s, run_id, plan, destinations, data, license, time, cleanup}
            prepareRun(id, preset) { return send('POST', project(id) + '/runs/prepare', { preset }); },
            async startRun(id, planToken) { return (await send('POST', project(id) + '/runs', { plan_token: planToken })).run; },
            async listRuns(id, limit) { return list(await json('GET', project(id) + '/runs' + qs({ limit })), 'runs'); },
            async getRun(runId) { return (await json('GET', '/runs/' + enc(runId))).run; },
            async cancelRun(runId) { return (await send('POST', '/runs/' + enc(runId) + '/cancel', {})).run; },
            // {metrics, max_steps, live}
            runMetrics(runId, sinceStep) { return json('GET', '/runs/' + enc(runId) + '/metrics' + qs({ since_step: sinceStep || 0 })); },
            async runEvents(runId, after, limit) { return list(await json('GET', '/runs/' + enc(runId) + '/events' + qs({ after: after || 0, limit })), 'events'); },
            async runArtifacts(runId) { return list(await json('GET', '/runs/' + enc(runId) + '/artifacts'), 'artifacts'); },
            artifactURL(runId, artifactId) { return BASE + '/runs/' + enc(runId) + '/artifacts/' + enc(artifactId) + '/file'; },
            async runReport(runId) { return (await json('GET', '/runs/' + enc(runId) + '/report')).report || null; },
            // {tail, bytes}
            runLog(runId) { return json('GET', '/runs/' + enc(runId) + '/log'); },
            deleteRun(runId) { return send('POST', '/runs/' + enc(runId) + '/delete', {}); },

            // events(projectId, {onJob, onRows, onChecks, onRun, onMetric, onStage, onState}) -> {close()}; onState('open' | 'error')
            events(projectId, handlers) {
                const h = handlers || {};
                if (!makeEventSource) return { close() {} };
                let source = null;
                try { source = reopeningSource(() => makeEventSource(BASE + project(projectId) + '/events')); } catch (_) { return { close() {} }; }
                const parse = event => {
                    try { return JSON.parse(event.data); } catch (_) { return null; }
                };
                source.addEventListener('job', event => {
                    const body = parse(event);
                    const job = body && body.job ? body.job : body;
                    if (h.onJob && job && typeof job === 'object' && typeof job.id === 'string') h.onJob(job);
                });
                source.addEventListener('rows', event => {
                    const body = parse(event);
                    if (h.onRows && body && typeof body === 'object') h.onRows(body);
                });
                source.addEventListener('checks', event => {
                    const body = parse(event);
                    if (h.onChecks) h.onChecks(body && typeof body === 'object' ? body : {});
                });
                source.addEventListener('run', event => {
                    const body = parse(event);
                    const run = body && body.run ? body.run : body;
                    if (h.onRun && run && typeof run === 'object' && typeof run.id === 'string') h.onRun(run);
                });
                source.addEventListener('metric', event => {
                    const body = parse(event);
                    if (h.onMetric && body && typeof body.run_id === 'string' && Array.isArray(body.points)) h.onMetric(body);
                });
                source.addEventListener('stage', event => {
                    const body = parse(event);
                    if (h.onStage && body && typeof body.run_id === 'string') h.onStage(body);
                });
                source.addEventListener('open', () => { if (h.onState) h.onState('open'); });
                source.addEventListener('error', () => { if (h.onState) h.onState('error'); });
                return { close() { try { source.close(); } catch (_) { /* gone */ } } };
            }
        };
        return api;
    }

    P.createApi = createApi;
    P.API_BASE = BASE;
})();
