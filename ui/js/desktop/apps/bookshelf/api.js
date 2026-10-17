(function () {
    'use strict';

    const base = '/api/desktop/bookshelf';
    const idPath = id => encodeURIComponent(String(id || ''));

    function create(context, signal) {
        const api = context.api;
        const call = (path, options = {}) => api(base + path, Object.assign({}, options, { signal }));
        const json = (method, body, headers) => ({ method, body: JSON.stringify(body), headers: { 'Content-Type': 'application/json', ...headers } });
        return {
            capabilities: () => call('/capabilities'),
            cache: () => call('/cache'),
            sources: () => call('/sources'),
            browse: (provider, mountId, path) => {
                const query = new URLSearchParams({ provider, path: path || '' });
                if (mountId) query.set('mount_id', mountId);
                return call('/sources/browse?' + query.toString());
            },
            browseSource: (id, path) => call('/sources/' + idPath(id) + '/browse?' + new URLSearchParams({ path: path || '' }).toString()),
            addSource: (source, credential) => call('/sources', json('POST', { source, ...(credential ? { credential } : {}) })),
            updateSource: (id, patch, generation) => call('/sources/' + idPath(id), json('PATCH', patch, generation ? { 'If-Match': String(generation) } : undefined)),
            removeSource: id => call('/sources/' + idPath(id), { method: 'DELETE' }),
            scanSource: id => call('/sources/' + idPath(id) + '/scan', json('POST', {})),
            job: id => call('/jobs/' + idPath(id)),
            books: params => {
                const query = new URLSearchParams();
                Object.entries(params || {}).forEach(([key, value]) => {
                    if (value !== '' && value != null) query.set(key, String(value));
                });
                return call('/books' + (query.size ? '?' + query.toString() : ''));
            },
            book: id => call('/books/' + idPath(id)),
            resolve: async location => {
                const query = '/books/resolve?' + new URLSearchParams({ path: location?.path || '' }).toString();
                try { return await call(query); }
                catch (error) {
                    if (error.status !== 404) throw error;
                    return call('/books/resolve', json('POST', { path: location?.path || '' }));
                }
            },
            prepare: (id, format) => call('/books/' + idPath(id) + '/prepare' + (format ? '?' + new URLSearchParams({ format }).toString() : ''), json('POST', {})),
            convert: id => call('/books/' + idPath(id) + '/convert', json('POST', {})),
            updateReadingState: (id, state, revision) => call('/books/' + idPath(id) + '/state', json('PATCH', { state }, revision ? { 'If-Match': String(revision) } : undefined)),
            setPinned: (id, pinned, revision) => call('/books/' + idPath(id) + '/cache', json('PATCH', { pinned }, revision ? { 'If-Match': String(revision) } : undefined)),
            saveProgress: (id, progress, revision) => call('/books/' + idPath(id) + '/progress', json('PATCH', progress, revision !== undefined && revision !== null ? { 'If-Match': String(revision) } : undefined)),
            collections: () => call('/collections'),
            createCollection: collection => call('/collections', json('POST', collection)),
            updateCollection: (id, patch, revision) => call('/collections/' + idPath(id), json('PATCH', patch, revision ? { 'If-Match': String(revision) } : undefined)),
            reorderCollectionBooks: (id, bookIds, revision) => call('/collections/' + idPath(id) + '/books', json('PATCH', { book_ids: bookIds }, revision ? { 'If-Match': String(revision) } : undefined)),
            addToCollection: (collectionId, bookId) => call('/collections/' + idPath(collectionId) + '/books/' + idPath(bookId), json('PUT', {})),
            removeFromCollection: (collectionId, bookId) => call('/collections/' + idPath(collectionId) + '/books/' + idPath(bookId), { method: 'DELETE' }),
            annotations: id => call('/books/' + idPath(id) + '/annotations'),
            createAnnotation: (id, annotation) => call('/books/' + idPath(id) + '/annotations', json('POST', annotation)),
            updateAnnotation: (id, annotationId, patch, revision) => call('/annotations/' + idPath(annotationId), json('PATCH', patch, revision ? { 'If-Match': String(revision) } : undefined)),
            removeAnnotation: (annotationId, revision) => call('/annotations/' + idPath(annotationId), { method: 'DELETE', headers: revision ? { 'If-Match': String(revision) } : undefined }),
            updateBook: (id, metadata, revision) => call('/books/' + idPath(id), json('PATCH', metadata, revision ? { 'If-Match': String(revision) } : undefined)),
            removeBook: id => call('/books/' + idPath(id), { method: 'DELETE' }),
            updateCover: (id, form, revision) => call('/books/' + idPath(id) + '/cover', { method: 'PUT', body: form, headers: revision ? { 'If-Match': String(revision) } : undefined }),
            upload: form => call('/imports', { method: 'POST', body: form })
        };
    }

    window.BookshelfAPI = { create };
})();
