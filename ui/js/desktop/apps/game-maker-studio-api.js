(function () {
    'use strict';

    const base = '/api/game-maker';

    function create(api) {
        async function request(path, options) {
            const opts = Object.assign({}, options || {});
            if (opts.body && typeof opts.body !== 'string') {
                opts.headers = Object.assign({ 'Content-Type': 'application/json' }, opts.headers || {});
                opts.body = JSON.stringify(opts.body);
            }
            return api(base + path, opts);
        }

        return {
            capabilities: () => request('/capabilities'),
            listProjects: () => request('/projects'),
            createProject: body => request('/projects', { method: 'POST', body }),
            getProject: id => request('/projects/' + encodeURIComponent(id)),
            renameProject: (id, name) => request('/projects/' + encodeURIComponent(id), {
                method: 'PATCH',
                body: { name }
            }),
            deleteProject: id => request('/projects/' + encodeURIComponent(id), { method: 'DELETE' }),
            startJob: (id, body) => request('/projects/' + encodeURIComponent(id) + '/jobs', {
                method: 'POST',
                body
            }),
            cancelJob: id => request('/jobs/' + encodeURIComponent(id) + '/cancel', { method: 'POST' }),
            revisions: id => request('/projects/' + encodeURIComponent(id) + '/revisions'),
            restore: (id, revision) => request('/projects/' + encodeURIComponent(id) + '/revisions/' +
                encodeURIComponent(revision) + '/restore', { method: 'POST' }),
            previewGrant: id => request('/projects/' + encodeURIComponent(id) + '/preview-token', {
                method: 'POST'
            }),
            exportURL: id => base + '/projects/' + encodeURIComponent(id) + '/export',
            eventURL: (id, after) => base + '/projects/' + encodeURIComponent(id) +
                '/events' + (after ? '?after=' + encodeURIComponent(after) : '')
        };
    }

    window.GameMakerStudioAPI = { create };
})();
