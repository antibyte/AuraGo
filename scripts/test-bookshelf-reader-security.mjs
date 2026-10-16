import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { build } from 'esbuild';

const bundle = await build({ entryPoints: ['ui/js/desktop/apps/bookshelf/reader-security.js'], bundle: true, write: false, format: 'esm', platform: 'node' });
const security = await import('data:text/javascript;base64,' + Buffer.from(bundle.outputFiles[0].contents).toString('base64'));
for (const name of ['../book.epub', '/root/book', 'a\\book', 'a/../book', 'C:book', 'a\0b']) assert.throws(() => security.safeEntry(name));
const entry = { filename: 'chapter.xhtml', compressedSize: 100, uncompressedSize: 100, externalFileAttributes: 0 };
assert.doesNotThrow(() => security.checkEntries([entry]));
for (const entries of [[entry, { ...entry, filename: 'CHAPTER.XHTML' }], [{ ...entry, encrypted: true }],
    [{ ...entry, uncompressedSize: 1e8 }], [{ ...entry, externalFileAttributes: 0xa000 << 16 }]]) assert.throws(() => security.checkEntries(entries));
assert.equal(security.sanitizeCSS('@import "https://tracker.invalid/a";p{background:url(https://tracker.invalid/b)}'), 'p{background:none}');

const mobiSource = await readFile('ui/js/vendor/foliate/source/mobi.js', 'utf8');
const { MOBI } = await import('data:text/javascript;base64,' + Buffer.from(mobiSource).toString('base64'));
function fixture({ records = 1, offset = 86, encryption = 0 } = {}) {
    const bytes = new Uint8Array(350), view = new DataView(bytes.buffer);
    bytes.set(new TextEncoder().encode('BOOKMOBI'), 60); view.setUint16(76, records); view.setUint32(78, offset);
    view.setUint16(86 + 12, encryption);
    return new File([bytes], 'malformed.mobi');
}
for (const options of [{ records: 0 }, { offset: 1 }, { records: 65000 }, { encryption: 1 }]) {
    await assert.rejects(new MOBI({ unzlib: () => { throw new Error('unexpected decompression'); } }).open(fixture(options)), /bookshelf_/);
}
console.log('Bookshelf traversal, archive budgets, CSS network stripping and MOBI rejection checks passed');
