#!/usr/bin/env node
// Runs ui/js/desktop/apps/terminal-retronet-entries.js in Node with a minimal fake DOM (no npm packages) and checks the
// own-entry editor: client-side validation mirroring internal/retronet (name and description runes and invisible
// categories, RFC 1123 host names, IP literals and restricted ranges, ports, SSH users), the exact keys of the stored
// document, host keys kept only while the SSH target is unchanged, the 64-entry limit, server errors shown in the
// dialog, text set only as text, delete, dismissal while a save is pending and focus return.
// Usage: node scripts/test-terminal-retronet-entries.mjs
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const ui = path.join(here, '..', 'ui');
const english = JSON.parse(fs.readFileSync(path.join(ui, 'lang', 'desktop', 'en.json'), 'utf8'));

// --- Minimal DOM: elements, attributes, text, events, <dialog> ------------------------------------------------
const doc = { activeElement: null };
class FakeElement {
    constructor(tag) {
        this.tagName = tag.toUpperCase();
        this.attributes = new Map();
        this.children = [];
        this.parent = null;
        this.listeners = {};
        this.text = '';
        this.value = '';
        this.hidden = false;
        this.disabled = false;
        this.open = false;
        this.className = '';
    }
    get isConnected() {
        let node = this;
        while (node.parent) node = node.parent;
        return node === body;
    }
    get textContent() { return this.text + this.children.map((child) => child.textContent).join(''); }
    set textContent(value) {
        this.text = String(value);
        this.children = [];
    }
    setAttribute(name, value) { this.attributes.set(name, String(value)); }
    getAttribute(name) { return this.attributes.has(name) ? this.attributes.get(name) : null; }
    hasAttribute(name) { return this.attributes.has(name); }
    removeAttribute(name) { this.attributes.delete(name); }
    appendChild(child) {
        if (!(child instanceof FakeElement)) throw new Error('appendChild expects an element, got ' + typeof child);
        if (child.parent) child.remove();
        child.parent = this;
        this.children.push(child);
        return child;
    }
    remove() {
        if (!this.parent) return;
        this.parent.children = this.parent.children.filter((node) => node !== this);
        this.parent = null;
    }
    addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
    dispatch(type) {
        const event = { type, target: this, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } };
        (this.listeners[type] || []).slice().forEach((fn) => fn(event));
        return event;
    }
    focus() { doc.activeElement = this; }
    showModal() {
        if (!this.isConnected) throw new Error('showModal on a detached dialog');
        if (this.tagName !== 'DIALOG') throw new Error('showModal on ' + this.tagName);
        this.open = true;
    }
    close() {
        if (!this.open) return;
        this.open = false;
        this.dispatch('close');
    }
}
const body = new FakeElement('body');
doc.createElement = (tag) => new FakeElement(tag);
doc.body = body;
const walk = (node, out = []) => {
    out.push(node);
    node.children.forEach((child) => walk(child, out));
    return out;
};
const find = (root, attr, value) => walk(root).find((node) => node.hasAttribute(attr) && (value === undefined || node.getAttribute(attr) === value));

const window = { crypto: webcrypto };
const sandbox = { window, document: doc, TextEncoder, Uint8Array, Promise, JSON };
vm.createContext(sandbox);
vm.runInContext(fs.readFileSync(path.join(ui, 'js', 'desktop', 'apps', 'terminal-retronet-entries.js'), 'utf8'), sandbox, { filename: 'terminal-retronet-entries.js' });
const Entries = window.TerminalRetroNetEntries;

let failures = 0;
function check(name, cond, detail) {
    if (cond) {
        console.log('ok   ' + name);
        return;
    }
    failures += 1;
    console.log('FAIL ' + name + (detail ? ' - ' + detail : ''));
}
const same = (name, actual, expected) => check(name, JSON.stringify(actual) === JSON.stringify(expected), JSON.stringify(actual) + ' !== ' + JSON.stringify(expected));
const t = (key, params) => {
    let text = Object.prototype.hasOwnProperty.call(english, key) ? english[key] : key;
    Object.keys(params || {}).forEach((name) => { text = text.split('{{' + name + '}}').join(String(params[name])); });
    return text;
};
const flush = () => new Promise((resolve) => setTimeout(resolve, 0));
const ENTRY_KEYS = ['id', 'name', 'description', 'protocol', 'host', 'port', 'kind', 'charset', 'user', 'host_key'];
const HOME = {
    id: 'own-homeboard001', name: 'Home Board', description: 'My own test board', category: 'own', protocol: 'ssh',
    host: 'bbs.example.org', port: 2222, user: 'guest', host_key: 'SHA256:' + 'A'.repeat(43), own: true
};
const MUD = { id: 'own-mudworld0001', name: 'Mud', category: 'own', protocol: 'telnet', host: 'mud.example.net', port: 4000, kind: 'world', charset: 'latin1', own: true, description_key: '' };
const CATALOG = { id: 'telehack', name: 'Telehack', description_key: 'desktop.terminal_retronet_entry_telehack', category: 'classics', protocol: 'telnet', host: 'telehack.com', port: 23, kind: 'world', charset: 'utf8', own: false };

// Opens a dialog with a recording API; `respond` decides each PUT's outcome.
function openEditor(options) {
    const calls = [];
    const saved = [];
    const host = body.appendChild(new FakeElement('div'));
    const opener = host.appendChild(new FakeElement('textarea'));
    opener.focus();
    const gets = [];
    const respond = options.respond || (() => Promise.resolve({ ok: true }));
    // GET directory answers with the stored own entries (`server`, default: the dialog's snapshot) and a catalog entry.
    const api = (url, init) => {
        if (url === '/api/desktop/retronet/directory') {
            gets.push(url);
            if (options.directory) return options.directory();
            return Promise.resolve({ entries: [CATALOG].concat(options.server || options.own || []), status: {}, stale: false, can_edit: true });
        }
        calls.push({ url, init, doc: JSON.parse(JSON.parse(init.body).value) });
        return respond(calls.length);
    };
    const method = options.remove ? 'confirmDelete' : 'open';
    const dialog = Entries[method]({
        host, t, api, entry: options.entry || null, ownEntries: options.own || [], onSaved: (entries, entry) => saved.push({ entries, entry })
    });
    const field = (name) => find(dialog, 'name', name);
    const set = (values) => Object.keys(values).forEach((name) => {
        field(name).value = values[name];
        field(name).dispatch('change');
    });
    const form = walk(dialog).find((node) => node.tagName === 'FORM');
    const error = find(dialog, 'data-retronet-error');
    return { dialog, host, opener, calls, gets, saved, field, set, form, error, submit: () => form.dispatch('submit') };
}

async function editWith(options, values) {
    const editor = openEditor(options);
    editor.set(values);
    editor.submit();
    await flush();
    return editor;
}

async function attempt(values, own) {
    const editor = openEditor({ own: own || [] });
    editor.set(values);
    const event = editor.submit();
    await flush();
    return { editor, event, problem: editor.error.hidden ? '' : editor.error.textContent };
}

// 1. A new Telnet entry: dialog, defaults, exact document keys, own- ID, onSaved, close and focus return.
{
    const editor = openEditor({ own: [HOME, CATALOG, MUD] });
    check('the dialog is a native modal dialog in the host', editor.dialog.tagName === 'DIALOG' && editor.dialog.open && editor.dialog.parent === editor.host);
    same('it is marked as the new-entry dialog', editor.dialog.getAttribute('data-terminal-retronet-dialog'), 'new');
    check('focus starts in the name field', doc.activeElement === editor.field('name'));
    same('defaults: Telnet mailbox on port 23 in CP437', ['protocol', 'kind', 'charset', 'port'].map((name) => editor.field(name).value), ['telnet', 'bbs', 'cp437', '23']);
    same('Telnet hides the SSH user row', [editor.field('user').parent.hidden, editor.field('kind').parent.hidden, editor.field('charset').parent.hidden], [true, false, false]);
    editor.set({ name: '  New Board  ', host: ' BBS.Example.NET ', port: '2323', user: 'leftover' });
    const event = editor.submit();
    check('submit is handled in the page', event.defaultPrevented);
    await flush();
    same('one PUT to the desktop settings', editor.calls.map((call) => [call.url, call.init.method, JSON.parse(call.init.body).key]), [['/api/desktop/settings', 'PUT', 'retronet.entries']]);
    const stored = editor.calls[0].doc;
    same('the document has exactly version and entries', Object.keys(stored), ['version', 'entries']);
    same('catalog entries are never stored', stored.entries.map((entry) => entry.id).slice(0, 2), ['own-homeboard001', 'own-mudworld0001']);
    check('every entry has only stored keys', stored.entries.every((entry) => Object.keys(entry).every((key) => ENTRY_KEYS.includes(key))), JSON.stringify(stored.entries));
    same('the other own entries keep every stored field', stored.entries.slice(0, 2), [
        { id: HOME.id, name: HOME.name, description: HOME.description, protocol: 'ssh', host: HOME.host, port: 2222, user: 'guest', host_key: HOME.host_key },
        { id: MUD.id, name: 'Mud', protocol: 'telnet', host: MUD.host, port: 4000, kind: 'world', charset: 'latin1' }
    ]);
    const added = stored.entries[2];
    check('a new ID is own- plus 12 [a-z0-9]', /^own-[a-z0-9]{12}$/.test(added.id), added.id);
    same('the new Telnet entry: trimmed, lower-case host, kind and charset, no user', { ...added, id: '' }, { id: '', name: 'New Board', protocol: 'telnet', host: 'bbs.example.net', port: 2323, kind: 'bbs', charset: 'cp437' });
    same('onSaved receives the list and the saved entry', [editor.saved.length, editor.saved[0].entries.length, editor.saved[0].entry.id], [1, 3, added.id]);
    check('the dialog closes and leaves the DOM', !editor.dialog.open && !editor.dialog.parent);
    check('focus returns to the opener', doc.activeElement === editor.opener);
}

// 2. Names: 1-40 runes, something visible besides white space and U+200D, no Cc/Cf/Co/Zl/Zp/U+FFFD (U+200D allowed).
{
    const nameError = english['desktop.terminal_retronet_error_name'];
    const base = { host: 'bbs.example.net' };
    for (const [label, name] of [
        ['empty', ''], ['blank', '   '], ['only a zero-width joiner', '\u200d'], ['zero-width space (Cf)', 'a\u200bb'],
        ['right-to-left override (Cf)', 'evil\u202eexe'], ['inner BOM (Cf)', 'Na\ufeffme'], ['tag character (Cf)', 'a\u{e0041}'],
        ['bell (Cc)', 'a\u0007'], ['C1 control (Cc)', 'a\u0085b'], ['private use (Co)', '\ue000x'], ['line separator (Zl)', 'a\u2028b'],
        ['paragraph separator (Zp)', 'a\u2029b'], ['replacement character', 'a\ufffd'], ['lone surrogate', 'a\ud800'], ['41 runes', 'x'.repeat(41)]
    ]) {
        same('name rejected: ' + label, (await attempt({ ...base, name })).problem, nameError);
    }
    for (const [label, name] of [
        ['one letter', 'x'], ['40 runes', 'x'.repeat(40)], ['40 emoji (80 UTF-16 units)', '\u{1f600}'.repeat(40)],
        ['emoji ZWJ sequence', '\u{1f468}\u200d\u{1f469}\u200d\u{1f467}'], ['CJK and accents', '漢字 Café'], ['inner no-break space', 'a\u00a0b']
    ]) {
        const result = await attempt({ ...base, name });
        same('name accepted: ' + label, [result.problem, result.editor.calls.length], ['', 1]);
    }
}

// 3. Descriptions: at most 80 runes, same invisible categories, optional.
{
    const descriptionError = english['desktop.terminal_retronet_error_description'];
    const base = { name: 'Board', host: 'bbs.example.net' };
    same('description rejected: 81 runes', (await attempt({ ...base, description: 'd'.repeat(81) })).problem, descriptionError);
    same('description rejected: left-to-right mark (Cf)', (await attempt({ ...base, description: 'a\u200eb' })).problem, descriptionError);
    same('description rejected: tab (Cc)', (await attempt({ ...base, description: 'a\tb' })).problem, descriptionError);
    const long = await attempt({ ...base, description: '\u{1f600}'.repeat(80) });
    same('description accepted: 80 emoji', [long.problem, long.editor.calls[0].doc.entries[0].description.length], ['', 160]);
    const zwj = await attempt({ ...base, description: 'a\u200db' });
    same('description accepted: U+200D', zwj.problem, '');
    const none = await attempt({ ...base, description: '   ' });
    check('an empty description is left out', !('description' in none.editor.calls[0].doc.entries[0]));
}

// 4. Hosts: RFC 1123 names or IP literals; private, loopback, link-local and CGNAT literals rejected.
{
    const hostError = english['desktop.terminal_retronet_error_host'];
    const privateError = english['desktop.terminal_retronet_error_private_host'];
    const base = { name: 'Board' };
    const invalid = ['', 'exa mple.com', '-a.com', 'a-.com', 'a..com', 'a.com.', '.a.com', 'host_name.com', 'exämple.com',
        'a.123', '1.2.3', '1.2.3.4.5', '0x7f000001', 'foo.0x1f', 'foo.0X', '01.2.3.4', '256.1.1.1', 'a'.repeat(64) + '.com',
        ('a'.repeat(63) + '.').repeat(4).slice(0, 254), '[::1]', '1::2::3', '1:2:3:4:5:6:7:8:9', 'fe80::1%eth0', '12345::1', 'g::1', '::ffff:1.2.3'];
    for (const host of invalid) same('host rejected: ' + JSON.stringify(host).slice(0, 40), (await attempt({ ...base, host })).problem, hostError);
    // internal/security nonPublicURLPrefixes plus the non-global-unicast addresses, mapped IPv4 unwrapped first.
    const restricted = ['10.0.0.1', '127.0.0.1', '169.254.169.254', '100.64.0.1', '100.127.255.255', '172.16.5.4', '172.31.0.1', '192.168.1.1',
        '0.0.0.0', '192.0.0.8', '192.0.2.1', '198.18.0.1', '198.19.255.255', '198.51.100.7', '203.0.113.9', '224.0.0.1', '240.0.0.1', '255.255.255.255',
        '::1', '::', '::1.2.3.4', 'fe80::1', 'FEBF::1', 'fd00::1', 'fc00::1', 'ff02::1', '64:ff9b::8.8.8.8', '64:ff9b:1::1', '2002::1', '2001::1',
        '2001:db8::1', '2001:0db8:0:0:0:0:0:1', '::ffff:10.0.0.1', '::ffff:127.0.0.1', '::ffff:0a00:0001', 'localhost', 'foo.localhost'];
    for (const host of restricted) same('host rejected as private: ' + host, (await attempt({ ...base, host })).problem, privateError);
    const valid = ['bbs.example.net', 'Example.COM', 'a', 'a-b.c0', 'xn--bcher-kva.example', '1.example', 'a.b1', '0x.example', '8.8.8.8', '1.1.1.1',
        '100.128.0.1', '172.32.0.1', '198.20.0.1', '2001:4860:4860::8888', '2a00:1450:4001:0829:0000:0000:0000:200e', '2a00:1450::', '::ffff:8.8.8.8',
        'a'.repeat(63) + '.com'];
    for (const host of valid) {
        const result = await attempt({ ...base, host });
        same('host accepted: ' + host.slice(0, 40), [result.problem, result.editor.calls.length && result.editor.calls[0].doc.entries[0].host], ['', host.toLowerCase()]);
    }
}

// 5. Ports: 1-65535 without the mail ports.
{
    const portError = english['desktop.terminal_retronet_error_port'];
    const mailError = english['desktop.terminal_retronet_error_mail_port'];
    const base = { name: 'Board', host: 'bbs.example.net' };
    for (const port of ['0', '65536', '', 'abc', '22.5', '-1', '1e3', '0x17']) same('port rejected: ' + JSON.stringify(port), (await attempt({ ...base, port })).problem, portError);
    for (const port of ['25', '465', '587']) same('mail port rejected: ' + port, (await attempt({ ...base, port })).problem, mailError);
    for (const port of ['1', '65535', '2323']) same('port accepted: ' + port, (await attempt({ ...base, port })).editor.calls[0].doc.entries[0].port, Number(port));
}

// 6. SSH: user ^[a-z0-9._-]{1,32}$, no kind or charset; Telnet never carries a user.
{
    const userError = english['desktop.terminal_retronet_error_user'];
    const base = { name: 'Shell', host: 'ssh.example.net', protocol: 'ssh' };
    for (const user of ['', 'Guest', 'bad user', 'a'.repeat(33), 'gäst', 'root@x']) same('SSH user rejected: ' + JSON.stringify(user), (await attempt({ ...base, user })).problem, userError);
    const ok = await attempt({ ...base, user: ' bbs.user_1-x ' });
    same('an SSH entry stores user but no kind or charset', { ...ok.editor.calls[0].doc.entries[0], id: '' }, { id: '', name: 'Shell', protocol: 'ssh', host: 'ssh.example.net', port: 22, user: 'bbs.user_1-x' });
    const editor = openEditor({});
    editor.set({ protocol: 'ssh' });
    same('SSH shows the user row and hides kind and charset', [editor.field('user').parent.hidden, editor.field('kind').parent.hidden, editor.field('charset').parent.hidden], [false, true, true]);
    same('switching to SSH moves the default port to 22', editor.field('port').value, '22');
    editor.set({ protocol: 'telnet' });
    same('and back to 23', editor.field('port').value, '23');
    editor.set({ port: '2323', protocol: 'ssh' });
    same('a custom port stays', editor.field('port').value, '2323');
    editor.dialog.close();
}

// 7. Charset follows the type until it is chosen by hand.
{
    const editor = openEditor({});
    editor.set({ kind: 'world' });
    same('a text world defaults to UTF-8', editor.field('charset').value, 'utf8');
    editor.set({ charset: 'latin1' });
    editor.set({ kind: 'bbs' });
    same('a charset chosen by hand stays', editor.field('charset').value, 'latin1');
    editor.dialog.close();
}

// 8. Editing keeps the ID and the pinned host key only while protocol, host, port and user are unchanged.
async function edit(values) {
    const editor = openEditor({ entry: HOME, own: [HOME, MUD] });
    editor.set(values);
    editor.submit();
    await flush();
    return editor;
}
{
    const editor = openEditor({ entry: HOME, own: [HOME, MUD] });
    same('it is marked as the edit dialog', editor.dialog.getAttribute('data-terminal-retronet-dialog'), 'edit');
    same('the form shows the stored values', ['name', 'description', 'protocol', 'host', 'port', 'user'].map((name) => editor.field(name).value), ['Home Board', 'My own test board', 'ssh', 'bbs.example.org', '2222', 'guest']);
    editor.dialog.close();
    const renamed = await edit({ name: 'Home Board II', host: 'BBS.example.org' });
    same('a rename keeps ID, position and host key', renamed.calls[0].doc.entries[0], { id: HOME.id, name: 'Home Board II', description: HOME.description, protocol: 'ssh', host: 'bbs.example.org', port: 2222, user: 'guest', host_key: HOME.host_key });
    for (const [label, values] of [['port', { port: '2223' }], ['host', { host: 'other.example.org' }], ['user', { user: 'sysop' }]]) {
        const changed = await edit(values);
        check('changing the ' + label + ' drops the host key', !('host_key' in changed.calls[0].doc.entries[0]), JSON.stringify(changed.calls[0].doc.entries[0]));
    }
    const telnet = await edit({ protocol: 'telnet', port: '23' });
    same('changing the protocol to Telnet drops user and host key', { ...telnet.calls[0].doc.entries[0] }, { id: HOME.id, name: 'Home Board', description: HOME.description, protocol: 'telnet', host: 'bbs.example.org', port: 23, kind: 'bbs', charset: 'cp437' });
}

// 9. Limits and server errors.
{
    const many = Array.from({ length: 64 }, (_, i) => ({ id: 'own-' + String(i).padStart(12, '0'), name: 'E' + i, protocol: 'telnet', host: 'bbs.example.net', port: 23, kind: 'bbs', charset: 'cp437', own: true }));
    const full = await attempt({ name: 'One too many', host: 'bbs.example.net' }, many);
    same('the 65th entry is refused locally', [full.problem, full.editor.calls.length], [english['desktop.terminal_retronet_error_limit'], 0]);
    const failing = openEditor({ own: [HOME], respond: () => Promise.reject(Object.assign(new Error('invalid desktop setting value for retronet.entries'), { status: 400 })) });
    failing.set({ name: 'Board', host: 'bbs.example.net' });
    failing.submit();
    const save = find(failing.dialog, 'data-retronet-save');
    check('the save button is disabled while saving', save.disabled);
    await flush();
    same('the server error text is shown in the dialog', failing.error.textContent, 'Saving failed: invalid desktop setting value for retronet.entries');
    check('the dialog stays open, nothing is reported as saved', failing.dialog.open && failing.saved.length === 0 && !save.disabled);
    check('the error is announced', failing.error.getAttribute('role') === 'alert' && !failing.error.hidden);
    failing.dialog.close();
    const silent = openEditor({ respond: () => Promise.reject(new Error('')) });
    silent.set({ name: 'Board', host: 'bbs.example.net' });
    silent.submit();
    await flush();
    same('an error without text falls back to the generic message', silent.error.textContent, 'Saving failed: ' + english['desktop.request_failed']);
    silent.dialog.close();
}

// 10. Dismissal is blocked while a save is pending; Cancel closes otherwise.
{
    let release;
    const pending = openEditor({ respond: () => new Promise((resolve) => { release = resolve; }) });
    pending.set({ name: 'Board', host: 'bbs.example.net' });
    pending.submit();
    check('Escape is ignored while saving', pending.dialog.dispatch('cancel').defaultPrevented);
    find(pending.dialog, 'data-retronet-cancel').dispatch('click');
    check('Cancel waits for the save', pending.dialog.open);
    await flush();
    check('the request is still pending', pending.dialog.open && pending.calls.length === 1);
    release({ ok: true });
    await flush();
    check('the finished save closes the dialog', !pending.dialog.open && pending.saved.length === 1);
    const idle = openEditor({});
    find(idle.dialog, 'data-retronet-close').dispatch('click');
    check('the close button closes an idle dialog', !idle.dialog.open && !idle.dialog.parent && idle.calls.length === 0);
}

// 11. Delete: confirmation with the name as text, the entry removed from the stored list, server errors shown.
{
    const hostile = { ...MUD, name: '<img src=x onerror=alert(1)>' };
    const remove = openEditor({ remove: true, entry: hostile, own: [HOME, hostile] });
    same('it is marked as the delete dialog', remove.dialog.getAttribute('data-terminal-retronet-dialog'), 'delete');
    check('the confirmation names the entry as plain text', walk(remove.dialog).some((node) => node.text === 'Delete “<img src=x onerror=alert(1)>” from the directory?'));
    check('focus starts on Cancel', doc.activeElement === find(remove.dialog, 'data-retronet-cancel'));
    remove.submit();
    await flush();
    same('the stored list loses only that entry', remove.calls[0].doc.entries.map((entry) => entry.id), [HOME.id]);
    same('onSaved receives the list and null', [remove.saved[0].entries.length, remove.saved[0].entry], [1, null]);
    const broken = openEditor({ remove: true, entry: MUD, own: [MUD], respond: () => Promise.reject(new Error('Retro-Net is disabled.')) });
    broken.submit();
    await flush();
    same('a failed delete shows the server text', broken.error.textContent, 'Deleting failed: Retro-Net is disabled.');
    broken.dialog.close();
}

// 13. Saves start from the stored entries read right before the PUT, not from the dialog's snapshot.
{
    const OTHER = { id: 'own-otherwindow01', name: 'Other', protocol: 'telnet', host: 'other.example.net', port: 23, kind: 'bbs', charset: 'cp437', own: true };
    const created = await editWith({ own: [HOME], server: [HOME, OTHER] }, { name: 'New Board', host: 'bbs.example.net' });
    same('the directory is read again before saving', created.gets.length, 1);
    same('an entry created in another window is kept', created.calls[0].doc.entries.map((entry) => entry.id).slice(0, 2), [HOME.id, OTHER.id]);
    same('the new entry is appended to the stored list', [created.calls[0].doc.entries.length, created.calls[0].doc.entries[2].name], [3, 'New Board']);
    same('onSaved receives the merged list', created.saved[0].entries.length, 3);
    const unpinned = { ...HOME };
    delete unpinned.host_key;
    const pinned = await editWith({ entry: unpinned, own: [unpinned], server: [HOME] }, { name: 'Renamed' });
    same('a host key the server pinned meanwhile survives an unrelated edit', pinned.calls[0].doc.entries[0].host_key, HOME.host_key);
    const moved = await editWith({ entry: unpinned, own: [unpinned], server: [HOME] }, { port: '2223' });
    check('but not a change of the SSH target', !('host_key' in moved.calls[0].doc.entries[0]), JSON.stringify(moved.calls[0].doc.entries[0]));
    const elsewhere = await editWith({ entry: HOME, own: [HOME], server: [{ ...HOME, host: 'new.example.org' }] }, { name: 'Renamed' });
    check('a key pinned for a target changed elsewhere is dropped', !('host_key' in elsewhere.calls[0].doc.entries[0]), JSON.stringify(elsewhere.calls[0].doc.entries[0]));
    const gone = await editWith({ entry: HOME, own: [HOME, MUD], server: [MUD] }, { name: 'Renamed' });
    same('editing an entry deleted elsewhere shows an error', gone.error.textContent, english['desktop.terminal_retronet_error_gone']);
    check('and saves nothing', gone.calls.length === 0 && gone.saved.length === 0 && gone.dialog.open);
    gone.dialog.close();
    const goneDelete = openEditor({ remove: true, entry: HOME, own: [HOME], server: [MUD] });
    goneDelete.submit();
    await flush();
    same('deleting an entry deleted elsewhere shows the same error', [goneDelete.error.textContent, goneDelete.calls.length], [english['desktop.terminal_retronet_error_gone'], 0]);
    goneDelete.dialog.close();
    const deleted = openEditor({ remove: true, entry: MUD, own: [MUD], server: [OTHER, MUD] });
    deleted.submit();
    await flush();
    same('a delete keeps entries created elsewhere', deleted.calls[0].doc.entries.map((entry) => entry.id), [OTHER.id]);
    const offline = await editWith({ own: [HOME], directory: () => Promise.reject(new Error('Retro-Net is disabled.')) }, { name: 'Board', host: 'bbs.example.net' });
    same('a failed re-read shows the server text', offline.error.textContent, 'Saving failed: Retro-Net is disabled.');
    check('and saves nothing', offline.calls.length === 0 && offline.saved.length === 0 && offline.dialog.open);
    offline.dialog.close();
    const malformed = await editWith({ own: [HOME], directory: () => Promise.resolve({ status: {} }) }, { name: 'Board', host: 'bbs.example.net' });
    same('a reply without entries saves nothing', [malformed.error.textContent, malformed.calls.length], ['Saving failed: ' + english['desktop.request_failed'], 0]);
    malformed.dialog.close();
    const stored = Array.from({ length: 64 }, (_, i) => ({ id: 'own-' + String(i).padStart(12, '0'), name: 'E' + i, protocol: 'telnet', host: 'bbs.example.net', port: 23, kind: 'bbs', charset: 'cp437', own: true }));
    const full = await editWith({ own: stored.slice(0, 63), server: stored }, { name: 'One too many', host: 'bbs.example.net' });
    same('the limit counts the stored entries', [full.error.textContent, full.calls.length], [english['desktop.terminal_retronet_error_limit'], 0]);
    full.dialog.close();
}

// 12. Every visible text is plain text: no element carries markup from user data.
check('no dialog text was parsed as markup', walk(body).every((node) => node.tagName !== 'IMG'));

if (failures) {
    console.log(failures + ' check(s) failed');
    process.exit(1);
}
console.log('terminal retronet entries ok');
