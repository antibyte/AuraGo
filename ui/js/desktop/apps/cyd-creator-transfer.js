// CYD Creator transfer (plan 5): "Export" saves the project as a ZIP (GET /projects/{id}/export: the
// full PlatformIO project with the generated code, cyd.project.json, assets and a README, never tokens or
// Wi-Fi credentials) after flushing unsaved edits; "Import ZIP" on the start screen picks a CYD Creator
// export, uploads it (POST /import) and returns the new project, which the start screen opens.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const enc = value => encodeURIComponent(String(value));

    Object.assign(P.api, {
        exportUrl: projectId => P.api.BASE + '/projects/' + enc(projectId) + '/export',
        importZip: file => {
            const form = new FormData();
            form.append('file', file, (file && file.name) || 'project.zip');
            return P.api.request('POST', '/import', { body: form }).then(r => r.data.project);
        }
    });

    // fileName reads the file name of a Content-Disposition header ("attachment; filename=x.zip").
    function fileName(header, fallback) {
        const m = /filename\*?=(?:UTF-8'')?"?([^";]+)"?/i.exec(String(header || ''));
        if (!m) return fallback;
        try { return decodeURIComponent(m[1]); } catch (_) { return m[1]; }
    }

    // saveBlob hands a blob to the browser's download through a temporary object URL.
    function saveBlob(blob, name) {
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = name;
        a.hidden = true;
        document.body.appendChild(a);
        a.click();
        a.remove();
        setTimeout(() => URL.revokeObjectURL(url), 30000);
    }

    // exportProject flushes unsaved edits, then downloads the ZIP. Refusals (CYD_BUSY export or
    // publication_review, CYD_FILE_LIMIT export_bytes) get their sentences from P.errors.message.
    async function exportProject(inst) {
        const t = inst.t;
        if (!inst.projectId) return false;
        if (typeof inst.flushAll === 'function' && !(await inst.flushAll())) {
            inst.notify('error', t('cyd_creator.transfer.export_unsaved'));
            return false;
        }
        try {
            const resp = await fetch(P.api.exportUrl(inst.projectId), { credentials: 'same-origin', cache: 'no-store' });
            if (!resp.ok) {
                let data = null;
                try { data = await resp.json(); } catch (_) { data = null; }
                throw P.api.toError(resp.status, data);
            }
            const blob = await resp.blob();
            const name = fileName(resp.headers.get('Content-Disposition'), ((inst.project && inst.project.slug) || 'cyd-project') + '.zip');
            saveBlob(blob, name);
            inst.notify('info', t('cyd_creator.transfer.exported', { name }));
            return true;
        } catch (err) {
            inst.notify('error', P.errors.message(t, err));
            return false;
        }
    }

    // importErrorText explains a refused archive (CYD_IMPORT_INVALID, details.reason not_zip, too_large,
    // too_many_entries, unsafe_path, layout_missing, file_type, layout, board, app_files or asset). The
    // size limits have their own sentences: images over the total image budget (asset with limit
    // asset_bytes, {{size}} = details.max) and too many or too large source files (app_files with a
    // numeric limit). Every other reason has its own sentence, transfer.import_<reason> ({{path}} for
    // unsafe_path/file_type, {{board}} for board, {{size}} = details.limit bytes for too_large, {{max}} =
    // details.limit entries for too_many_entries); app_files and asset name a path only sometimes, so it
    // follows their sentence in brackets. A reason without a sentence shows the "not a CYD Creator export"
    // sentence plus the path or board the server names.
    function importErrorText(t, err) {
        const details = (err && err.details) || {};
        const reason = P.errors.reason(err);
        const has = key => typeof t.has === 'function' && t.has(key);
        if (reason === 'asset' && details.limit === 'asset_bytes' && has('cyd_creator.transfer.limit_asset_bytes')) {
            return t('cyd_creator.transfer.limit_asset_bytes', { size: P.format.bytes(details.max) });
        }
        if (reason === 'app_files' && typeof details.limit === 'number' && has('cyd_creator.transfer.limit_app_files')) {
            return t('cyd_creator.transfer.limit_app_files');
        }
        const where = [details.path, details.board].find(v => typeof v === 'string' && v);
        if (/^[a-z_]+$/.test(reason) && has('cyd_creator.transfer.import_' + reason)) {
            const limit = Number(details.limit) || 0;
            const text = t('cyd_creator.transfer.import_' + reason, {
                path: typeof details.path === 'string' ? details.path : '',
                board: typeof details.board === 'string' ? details.board : '',
                size: P.format.bytes(limit),
                max: String(limit)
            });
            const tail = (reason === 'app_files' || reason === 'asset') && typeof details.path === 'string' && details.path;
            return tail ? text + ' (' + details.path + ')' : text;
        }
        const text = P.errors.message(t, err);
        return where ? text + ' (' + where + ')' : text;
    }

    // pickFile opens the file chooser; it resolves with the file, or null when the chooser was
    // cancelled (the input's cancel event, Chrome 113+).
    function pickFile(accept) {
        return new Promise(resolve => {
            const input = document.createElement('input');
            input.type = 'file';
            input.accept = accept;
            input.hidden = true;
            const done = file => { input.remove(); resolve(file); };
            input.addEventListener('change', () => done(input.files && input.files[0] ? input.files[0] : null), { once: true });
            input.addEventListener('cancel', () => done(null), { once: true });
            document.body.appendChild(input);
            input.click();
        });
    }

    // importZip lets the user pick a ZIP and imports it. It resolves with the new ProjectSummary, or
    // null when nothing was picked or the archive was refused (CYD_IMPORT_INVALID: the reason is already
    // shown); it throws ApiError otherwise (CYD_READONLY, CYD_TEMPLATE_TOO_NEW, …).
    async function importZip(inst) {
        if (inst.readonly) throw new P.api.ApiError(403, 'CYD_READONLY', 'read-only', {});
        const file = await (P.transfer.pickFile || pickFile)('.zip,application/zip');
        if (!file) return null;
        inst.notify('info', inst.t('cyd_creator.transfer.importing', { name: file.name || '' }));
        try {
            return await P.api.importZip(file);
        } catch (err) {
            if (P.errors.code(err) !== 'CYD_IMPORT_INVALID') throw err;
            inst.notify('error', importErrorText(inst.t, err));
            return null;
        }
    }

    P.transfer = { exportProject, importZip, pickFile, fileName };
})();
