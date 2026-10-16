import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('../ui/js/desktop/apps/editor-filemenu.js', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
const callback = source.match(/openFile: \(entry\) => \{([\s\S]*?)\n\s*\},\n\s*openMedia:/)?.[1];
assert.ok(callback, 'Find the production File Manager open callback');
const opened = [];
const saved = {};
const current = { context: { storage: { provider: 'koofr', mount_id: 'library' } } };
const context = {
    id: 'files', initialLocation: { provider: 'local' },
    state: { windows: new Map([['files', current]]) },
    fileExtension: name => name.split('.').at(-1).toLowerCase(),
    defaultAppForExtension: extension => saved[extension],
    openApp: (app, args) => opened.push({ app, ...args }),
    isWriterFile: () => false, isSheetsFile: () => false,
    is3DFile: () => false, isPixelImageFile: () => false,
    isViewerFile: entry => entry.name.endsWith('.pdf'),
    entryLooksPlayableMedia: () => false,
    openEditorFile: path => opened.push({ app: 'editor', path })
};
const open = vm.runInNewContext(`(entry) => {${callback}}`, context);
function check(name, app, extra = {}) {
    open({ name, path: `Books/${name}` });
    assert.deepEqual(opened.pop(), { app, path: `Books/${name}`, ...extra });
}
check('novel.epub', 'bookshelf', { provider: 'koofr', mount_id: 'library' });
saved.zip = 'zipper';
check('novel.fb2.zip', 'bookshelf', { provider: 'koofr', mount_id: 'library' });
saved['fb2.zip'] = 'viewer';
check('novel.fb2.zip', 'viewer', { provider: 'koofr', mount_id: 'library' });
current.context.storage = { provider: 'webdav', mount_id: '' };
check('novel.mobi', 'bookshelf', { provider: 'webdav', mount_id: '' });
check('archive.zip', 'zipper');
check('document.pdf', 'viewer');
console.log('Bookshelf File Manager routing and current-source checks passed.');
