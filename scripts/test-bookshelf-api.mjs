import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const window = {};
vm.runInNewContext(fs.readFileSync(new URL('../ui/js/desktop/apps/bookshelf/api.js', import.meta.url), 'utf8'), { window, URLSearchParams });
const requests = [];
const api = window.BookshelfAPI.create({ api: (path, options) => {
    requests.push(new Request('http://bookshelf.test' + path, options));
    return Promise.resolve({});
} });

const source = { name: 'Books', provider: 'local', path: 'Books' };
await api.addSource(source);
await api.updateBook('book', { title: 'Updated' }, 7);
await api.saveProgress('book', { fraction: 0.5 }, 3);
for (const [body, revision] of [[{ source }, null], [{ title: 'Updated' }, '7'], [{ fraction: 0.5 }, '3']]) {
    const request = requests.shift();
    assert.equal(request.headers.get('Content-Type'), 'application/json');
    assert.equal(request.headers.get('If-Match'), revision);
    assert.deepEqual(await request.json(), body);
}

await api.scanSource('source');
await api.prepare('book');
await api.convert('book');
await api.addToCollection('collection', 'book');
for (const request of requests.splice(0)) {
    assert.equal(request.headers.get('Content-Type'), 'application/json');
    assert.deepEqual(await request.json(), {});
}

const upload = new FormData();
upload.append('file', new Blob(['book bytes']), 'book.pdf');
await api.upload(upload);
await api.updateCover('book', upload, 9);
for (const [request, revision] of requests.map((request, i) => [request, i ? '9' : null])) {
    assert.match(request.headers.get('Content-Type'), /^multipart\/form-data; boundary=/);
    assert.equal(request.headers.get('If-Match'), revision);
    const file = (await request.formData()).get('file');
    assert.equal(file.name, 'book.pdf');
    assert.equal(await file.text(), 'book bytes');
}
console.log('Bookshelf JSON request bodies, revisions and multipart uploads passed.');
