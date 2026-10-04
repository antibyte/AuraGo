    // Versions belong to the editor/SDK client that read the bytes, never to a
    // global path cache (another window may have read a newer revision).
    function prepareDesktopFileMutation(url, options) {
        const endpoint = String(url).split('?')[0];
        const method = String(options.method || 'GET').toUpperCase();
        let field = '';
        if (endpoint === '/api/desktop/file' && method === 'PUT') field = 'path';
        if (endpoint === '/api/desktop/file' && method === 'PATCH') field = 'new_path';
        if (endpoint === '/api/desktop/copy' && method === 'POST') field = 'dest_path';
        if (endpoint === '/api/pixel/save' && method === 'POST') field = 'path';
        const trash = endpoint === '/api/desktop/trash' && method === 'POST';
        const upload = endpoint === '/api/desktop/upload' && method === 'POST' && options.body instanceof FormData;
        if (!field && !upload && !trash) return null;
        const headers = new Headers(options.headers || {});
        if (!headers.has('If-Match') && !headers.has('If-None-Match')) headers.set('If-None-Match', '*');
        options.headers = headers;
        const body = upload ? options.body : JSON.parse(options.body || '{}');
        const file = upload ? body.get('file') : null;
        const originalPath = upload ? workspaceJoinPath(body.get('path') || '', file.name) : body[field];
        return { body, file, field, upload, trash, originalPath, copy: 0, copying: false };
    }

    async function resolveDesktopFileConflict(mutation, options, body) {
        if (!mutation || !['file_conflict', 'directory_conflict'].includes(body.code)) return false;
        if (options.signal?.aborted) throw new DOMException('Aborted', 'AbortError');
        const conflict = body.conflict || {};
        const decision = mutation.copying ? 'copy' : await modalDialog({
            title: t('desktop.file_conflict_title'),
            message: t('desktop.file_conflict_message').replace('{{path}}', conflict.path || mutation.originalPath),
            signal: options.signal,
            choices: [
                { value: 'replace', label: t('desktop.file_conflict_replace'), disabled: !conflict.version },
                { value: 'copy', label: t('desktop.file_conflict_copy') }
            ]
        });
        if (!decision) throw new DOMException('Cancelled', 'AbortError');
        if (mutation.trash) {
            mutation.body.resolutions ||= {};
            mutation.body.resolutions[conflict.source] = decision === 'replace'
                ? { version: String(conflict.version) }
                : { copy: true };
            options.body = JSON.stringify(mutation.body);
            return true;
        }
        options.headers.delete('If-Match');
        options.headers.delete('If-None-Match');
        if (decision === 'replace' && conflict.version) {
            options.headers.set('If-Match', conflict.version);
        } else {
            // Only retry a create-only copy, never an uncertain mutation or an
            // overwrite. Each occupied candidate is rejected by the server.
            if (++mutation.copy > 999) return false;
            mutation.copying = true;
            const path = mutation.originalPath;
            const name = pathBaseName(path);
            const dot = name.lastIndexOf('.');
            const copyName = dot > 0 ? name.slice(0, dot) + ' (' + mutation.copy + ')' + name.slice(dot) : name + ' (' + mutation.copy + ')';
            if (mutation.upload) mutation.body.set('file', mutation.file, copyName);
            else mutation.body[mutation.field] = workspaceJoinPath(pathDir(path), copyName);
            options.headers.set('If-None-Match', '*');
        }
        options.body = mutation.upload ? mutation.body : JSON.stringify(mutation.body);
        return true;
    }
