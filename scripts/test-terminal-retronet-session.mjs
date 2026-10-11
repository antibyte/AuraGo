#!/usr/bin/env node
// Runs ui/js/desktop/apps/terminal-retronet-session.js (after terminal-text.js) in Node with a fake WebSocket and a
// fake xterm: entry-ID URL, binary and JSON frames, the three echo modes of Telnet world entries (line editing with
// echo, hidden line editing without echo or history, character mode) and their transitions, cell-accurate erasing
// through TerminalText (CJK two cells, emoji one), history, multi-line paste, raw control keys, host-key frames (the
// grid goes out in the URL only, never as a resize frame), hang-up, remote close, bbs entries, that server text never
// reaches the screen through the module and the host-key answer parser (localized letters of all 16 locales, ASCII
// y/n, case-insensitive).
// Usage: node scripts/test-terminal-retronet-session.mjs
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const ui = path.join(here, '..', 'ui');
const apps = path.join(ui, 'js', 'desktop', 'apps');

const sockets = [];
class FakeSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static CLOSING = 2;
    static CLOSED = 3;
    static initialState = 1;
    constructor(url) {
        this.url = url;
        this.readyState = FakeSocket.initialState;
        this.binaryType = 'blob';
        this.sent = [];
        this.closeCode = 0;
        sockets.push(this);
    }
    send(value) {
        this.sent.push(typeof value === 'string' ? 'txt:' + value : 'bin:' + new TextDecoder().decode(value));
        this.sizes = (this.sizes || []).concat(typeof value === 'string' ? [] : [value.byteLength]);
    }
    close(code) { this.readyState = 3; this.closeCode = code; }
}
const window = {};
const context = vm.createContext({
    window, WebSocket: FakeSocket, location: { protocol: 'https:', host: 'desk.example' },
    TextEncoder, Uint8Array, ArrayBuffer, JSON, Array, String, Number, Math
});
for (const file of ['terminal-text.js', 'terminal-retronet-session.js']) {
    vm.runInContext(fs.readFileSync(path.join(apps, file), 'utf8'), context, { filename: file });
}
const Session = window.TerminalRetroNetSession;

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
const screen = { out: '', writes: 0, write(text) { this.out += text; this.writes += 1; } };
const control = (socket, message) => socket.onmessage({ data: JSON.stringify(message) });
const frame = (text) => {
    const bytes = new TextEncoder().encode(text);
    return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
};
const closes = [];
const controls = [];
const received = [];

// 1. Socket, URL and incoming frames; server text reaches only onData/onControl, never the screen.
const session = Session.open({
    term: screen,
    entry: { id: 'discworld', protocol: 'telnet', kind: 'world' },
    cols: 100,
    rows: 30,
    onControl: (message) => controls.push(message.type),
    onData: (bytes) => received.push(new TextDecoder().decode(bytes)),
    onClose: () => closes.push('local')
});
const ws = sockets[0];
same('dials by entry ID only', ws.url, 'wss://desk.example/api/desktop/retronet/connect?entry=discworld&cols=100&rows=30');
same('binary frames arrive as array buffers', ws.binaryType, 'arraybuffer');
ws.onmessage({ data: frame('Welcome\x1b]0;evil\x07\r\n') });
same('binary frames reach onData', received, ['Welcome\x1b]0;evil\x07\r\n']);
control(ws, { type: 'connected', protocol: 'telnet', kind: 'world', charset: 'utf8' });
control(ws, { type: 'hostkey_prompt', key_type: 'ssh-ed25519\x1b[2J', fingerprint: 'SHA256:\x9b31m' });
ws.onmessage({ data: 'not json' });
same('controls reach onControl and junk is ignored', controls, ['connected', 'hostkey_prompt']);
same('the session never writes server text itself', screen.out, '');

// 2. remote:false, hidden:false -> local line editing with echo.
control(ws, { type: 'echo', remote: false, hidden: false });
session.send('look');
same('line mode echoes locally', screen.out, 'look');
same('line mode sends nothing before Enter', ws.sent, []);
session.send('\x7f');
session.send('k\r');
same('Enter sends the edited line with CR', ws.sent, ['bin:look\r']);
session.send('say one\r\nsay two\r\n');
session.send('\x03');
same('paste goes out line by line, Ctrl+C raw', ws.sent.slice(1), ['bin:say one\r', 'bin:say two\r', 'bin:\x03']);
screen.out = '';
session.send('a\x9bb');
same('a typed C1 control goes out raw and is never echoed', [screen.out, ws.sent.at(-1)], ['ab', 'bin:\x9b']);
session.send('\x15');

// 3. Erasing follows the shared TerminalText widths.
screen.out = '';
session.send('漢');
session.send('\x7f');
same('a CJK character erases two cells', screen.out, '漢\b\b  \b\b');
screen.out = '';
session.send('\u{1f600}');
session.send('\x7f');
same('an emoji erases one cell (Unicode 6, as xterm)', screen.out, '\u{1f600}\b \b');

// 4. History.
screen.out = '';
session.send('\x1b[A');
same('Up recalls the latest line', screen.out, 'say two');
session.send('\x1b[A');
session.send('\x1b[B');
session.send('\x1b[B');
session.send('\r');
same('Down past the newest line restores the empty draft', ws.sent.at(-1), 'bin:\r');

// 5. remote:false, hidden:true -> invisible line editing without history.
screen.out = '';
control(ws, { type: 'echo', remote: false, hidden: true });
session.send('s3crex');
session.send('\x7f');
session.send('\x1b[A');
session.send('t\r');
same('hidden echo writes only the line break', screen.out, '\r\n');
same('the hidden line is sent with CR', ws.sent.at(-1), 'bin:s3cret\r');
control(ws, { type: 'echo', remote: false, hidden: false });
screen.out = '';
session.send('\x1b[A');
same('the hidden line never enters the history', screen.out, 'say two');
session.send('\x15');

// 6. Mode transitions.
screen.out = '';
session.send('ab');
control(ws, { type: 'echo', remote: false, hidden: true });
same('going hidden mid-line erases the visible part', screen.out, 'ab\b\b  \b\b');
session.send('c\r');
same('the buffer survives the switch', ws.sent.at(-1), 'bin:abc\r');
session.send('zz');
control(ws, { type: 'echo', remote: false, hidden: false });
session.send('\r');
same('an unfinished hidden buffer is discarded, never revealed', ws.sent.at(-1), 'bin:\r');
screen.out = '';
session.send('north');
control(ws, { type: 'echo', remote: true, hidden: false });
same('character mode receives the unfinished line', ws.sent.at(-1), 'bin:north');
same('and its local echo is erased', screen.out, 'north\b\b\b\b\b     \b\b\b\b\b');

// 7. remote:true -> character mode.
screen.out = '';
const before = ws.sent.length;
session.send('Q');
session.send('Z');
same('keys go out at once', ws.sent.slice(before), ['bin:Q', 'bin:Z']);
same('no local echo in character mode', screen.out, '');

// 8. Host key, hang-up; the session has no resize (every Retro-Net grid is pinned).
same('the session offers no resize', typeof session.resize, 'undefined');
session.hostKeyDecision(true);
same('host-key decision frame', ws.sent.at(-1), 'txt:{"type":"hostkey_decision","accept":true}');
session.hangup();
same('hang-up closes with 1000', [ws.readyState, ws.closeCode], [3, 1000]);
session.send('x');
same('nothing is sent after hang-up', ws.sent.at(-1), 'txt:{"type":"hostkey_decision","accept":true}');
same('a local hang-up detaches the socket handlers', [ws.onmessage, ws.onclose], [null, null]);
same('no close callback after a local hang-up', closes, []);

// 9. A remote close reaches onClose once.
const remote = Session.open({ term: screen, entry: { id: 'telehack', protocol: 'telnet', kind: 'world' }, cols: 80, rows: 24, onClose: () => closes.push('remote') });
const remoteSocket = sockets[1];
remoteSocket.readyState = 3;
remoteSocket.onclose({ code: 1006 });
same('a remote close is reported once', closes, ['remote']);
remote.send('look\r');
same('nothing is sent after a remote close', remoteSocket.sent, []);

// 10. bbs entries stay in character mode regardless of the echo state.
const bbs = Session.open({ term: screen, entry: { id: 'vertrauen', protocol: 'telnet', kind: 'bbs' }, cols: 80, rows: 25 });
const bbsSocket = sockets[2];
control(bbsSocket, { type: 'echo', remote: false, hidden: true });
screen.out = '';
bbs.send('guest\r');
same('bbs input is character mode without local echo', [bbsSocket.sent, screen.out], [['bin:guest\r'], '']);
bbs.dispose();

// 11. Large input stays below the server's 64 KiB read limit: frames of at most 16 KiB, cut between code points.
const FRAME_LIMIT = 16 * 1024;
const big = 'aä漢\u{1f600}'.repeat(20000);
const frames = (socket, from) => socket.sent.slice(from).filter((item) => item.startsWith('bin:')).map((item) => item.slice(4));
const charMode = Session.open({ term: screen, entry: { id: 'vertrauen', protocol: 'telnet', kind: 'bbs' }, cols: 80, rows: 25 });
const charSocket = sockets.at(-1);
charMode.send(big);
check('a 200 KB paste in character mode goes out in several frames', charSocket.sizes.length > 10, String(charSocket.sizes.length));
check('every frame is at most 16 KiB', charSocket.sizes.every((size) => size <= FRAME_LIMIT), JSON.stringify(charSocket.sizes));
check('the frames join to the paste (no code point is cut)', frames(charSocket, 0).join('') === big);
charMode.dispose();
const lineMode = Session.open({ term: screen, entry: { id: 'discworld', protocol: 'telnet', kind: 'world' }, cols: 80, rows: 25 });
const lineSocket = sockets.at(-1);
screen.out = '';
screen.writes = 0;
lineMode.send('look');
same('typed text is echoed in one write per input chunk', [screen.writes, screen.out], [1, 'look']);
lineMode.send('\x15');
screen.writes = 0;
lineMode.send(big);
same('a long pasted line is echoed in one write', screen.writes, 1);
lineMode.send('\r');
check('the long line goes out in frames of at most 16 KiB', lineSocket.sizes.length > 10 && lineSocket.sizes.every((size) => size <= FRAME_LIMIT), JSON.stringify(lineSocket.sizes));
check('the frames join to the line and its CR', frames(lineSocket, 0).join('') === big + '\r');
screen.writes = 0;
lineMode.send('say one\r\nsay two\r\n');
same('a multi-line paste writes each line and line break once', screen.writes, 4);
lineMode.dispose();

// 12. Line editor details: Tab, escape sequences, whole characters.
const editor = Session.open({ term: screen, entry: { id: 'discworld', protocol: 'telnet', kind: 'world' }, cols: 80, rows: 25 });
const edSocket = sockets.at(-1);
const lastSent = () => edSocket.sent.at(-1);
screen.out = '';
editor.send('a\tb');
same('Tab stays in the line and shows as one cell', [screen.out, edSocket.sent.length], ['a b', 0]);
editor.send('\x7f');
editor.send('\x7f');
same('Backspace removes the Tab as one cell', screen.out, 'a b\b \b\b \b');
editor.send('\t\r');
same('the line goes out with its Tab', lastSent(), 'bin:a\t\r');
control(edSocket, { type: 'echo', remote: false, hidden: true });
screen.out = '';
editor.send('x\ty\r');
same('hidden line mode keeps Tab in the line too', [lastSent(), screen.out], ['bin:x\ty\r', '\r\n']);
control(edSocket, { type: 'echo', remote: false, hidden: false });
const sentBefore = edSocket.sent.length;
screen.out = '';
for (const key of ['\x1b[C', '\x1b[D', '\x1b[H', '\x1b[F', '\x1b[1~', '\x1b[4~', '\x1b[3~', '\x1b[5~', '\x1b[6~', '\x1bOC', '\x1bOD']) editor.send(key);
same('cursor and editing keys are swallowed', [edSocket.sent.length - sentBefore, screen.out], [0, '']);
editor.send('\x1b[31mred\x1b[0m and \x1b]0;title\x07plain');
same('pasted coloured text keeps only its text', screen.out, 'red and plain');
editor.send('\x1b[200~ pasted\x1b[201~');
editor.send('\x1bx');
editor.send('\r');
same('paste markers and Alt+key leave only the pasted text', lastSent(), 'bin:red and plain pasted\r');
editor.send('\x1b');
same('a lone Escape still goes out at once', lastSent(), 'bin:\x1b');
screen.out = '';
editor.send('ae\u{301}');
editor.send('\x7f');
same('Backspace removes a base with its combining mark', screen.out, 'ae\u{301}\b \b');
editor.send('\x7f');
editor.send('\u{1f468}\u{200d}\u{1f469}\u{200d}\u{1f467}');
screen.out = '';
editor.send('\x7f');
same('a ZWJ sequence goes at once (three cells in xterm)', screen.out, '\b\b\b   \b\b\b');
editor.send('\u{1f1e9}\u{1f1ea}\u{1f44d}\u{1f3fd}');
screen.out = '';
editor.send('\x7f');
editor.send('\x7f');
same('a skin tone and a flag go with their base', screen.out, '\b\b  \b\b\b\b  \b\b');
editor.send('\r');
same('buffer and screen stay in step', lastSent(), 'bin:\r');
editor.dispose();

// 13. Before the socket opens: input is dropped (the coordinator forwards keys only after the connected control,
// i.e. on an open socket), opening sends nothing (the grid went out in the URL), dispose closes quietly.
FakeSocket.initialState = 0;
const early = Session.open({ term: screen, entry: { id: 'discworld', protocol: 'telnet', kind: 'world' }, cols: 80, rows: 25, onClose: () => closes.push('early') });
const earlySocket = sockets.at(-1);
screen.out = '';
early.send('look\r');
same('nothing is sent or echoed while connecting', [earlySocket.sent, screen.out], [[], '']);
earlySocket.readyState = 1;
if (typeof earlySocket.onopen === 'function') earlySocket.onopen();
same('opening sends nothing: the grid went out in the URL', [earlySocket.sent, /[?&]cols=80&rows=25$/.test(earlySocket.url)], [[], true]);
early.send('look\r');
same('input flows once the socket is open', earlySocket.sent.at(-1), 'bin:look\r');
early.dispose();
const connecting = Session.open({ term: screen, entry: { id: 'telehack', protocol: 'telnet', kind: 'world' }, cols: 80, rows: 25, onClose: () => closes.push('connecting') });
const connectingSocket = sockets.at(-1);
connecting.dispose();
same('dispose while connecting closes the socket quietly', [connectingSocket.readyState, connectingSocket.closeCode, connectingSocket.onopen, connectingSocket.onclose, closes.includes('connecting')], [3, 1000, null, null, false]);
FakeSocket.initialState = 1;

// 14. Host-key answers: localized letters first, then ASCII y/n, case-insensitive; anything else is no answer.
const answer = Session.hostKeyAnswer;
same('English letters in both cases', [answer('y', 'Y', 'N'), answer('Y', 'Y', 'N'), answer('n', 'Y', 'N'), answer('N\r', 'Y', 'N')], [true, true, false, false]);
same('German J/N with the ASCII fallback', [answer('j', 'J', 'N'), answer('J', 'J', 'N'), answer('y', 'J', 'N'), answer('n', 'J', 'N')], [true, true, true, false]);
same('Czech A/N and Polish T/N', [answer('a', 'A', 'N'), answer('T', 'T', 'N'), answer('Y', 'T', 'N')], [true, true, true]);
same('localized letters win a collision with y/n', [answer('n', 'N', 'J'), answer('y', 'N', 'J'), answer('j', 'N', 'J')], [true, true, false]);
same('other keys are no answer', [answer('x', 'Y', 'N'), answer('', 'Y', 'N'), answer('\x1b[A', 'Y', 'N'), answer('yes', 'Y', 'N'), answer('\r', 'Y', 'N')], [null, null, null, null, null]);
const langDir = path.join(ui, 'lang', 'desktop');
for (const file of fs.readdirSync(langDir).filter((name) => name.endsWith('.json')).sort()) {
    const dict = JSON.parse(fs.readFileSync(path.join(langDir, file), 'utf8'));
    const yes = dict['desktop.terminal_retronet_hostkey_yes'];
    const no = dict['desktop.terminal_retronet_hostkey_no'];
    same(file + ' host-key letters are answers in both cases', [answer(yes.toLowerCase(), yes, no), answer(yes.toUpperCase(), yes, no), answer(no.toLowerCase(), yes, no), answer(no.toUpperCase(), yes, no)], [true, true, false, false]);
    same(file + ' accepts ASCII y and n', [answer('Y', yes, no), answer('n', yes, no)], [no.toLowerCase() === 'y' ? false : true, yes.toLowerCase() === 'n' ? true : false]);
}

// 15. C64 colour keys (Contract C): event.code with exact modifiers becomes ESC [ 6 4 ; <PETSCII byte> ~.
{
    const colorKey = Session.c64ColorKey;
    const press = (code, modifiers) => colorKey(Object.assign({ type: 'keydown', code, key: '', ctrlKey: false, altKey: false, shiftKey: false, metaKey: false }, modifiers));
    const CTRL_SHIFT = { ctrlKey: true, shiftKey: true };
    const ALT_SHIFT = { altKey: true, shiftKey: true };
    const digits = ['Digit1', 'Digit2', 'Digit3', 'Digit4', 'Digit5', 'Digit6', 'Digit7', 'Digit8'];
    const ctrl = digits.map((code) => press(code, CTRL_SHIFT));
    const alt = digits.map((code) => press(code, ALT_SHIFT));
    const reverse = [press('Digit9', CTRL_SHIFT), press('Digit0', CTRL_SHIFT)];
    same('Ctrl+Shift+1-8: black, white, red, cyan, purple, green, blue, yellow', ctrl,
        ['\x1b[64;144~', '\x1b[64;5~', '\x1b[64;28~', '\x1b[64;159~', '\x1b[64;156~', '\x1b[64;30~', '\x1b[64;31~', '\x1b[64;158~']);
    same('Alt+Shift+1-8: orange, brown, light red, dark grey, grey, light green, light blue, light grey', alt,
        ['\x1b[64;129~', '\x1b[64;149~', '\x1b[64;150~', '\x1b[64;151~', '\x1b[64;152~', '\x1b[64;153~', '\x1b[64;154~', '\x1b[64;155~']);
    same('Ctrl+Shift+9/0: reverse on/off', reverse, ['\x1b[64;18~', '\x1b[64;146~']);
    const allowList = [0x90, 0x05, 0x1c, 0x9f, 0x9c, 0x1e, 0x1f, 0x9e, 0x81, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0x9b, 0x12, 0x92];
    same('the keys produce exactly the server allow-list (Contract A)', ctrl.concat(alt, reverse).map((seq) => Number(/^\x1b\[64;(\d+)~$/.exec(seq)[1])).sort((a, b) => a - b), allowList.slice().sort((a, b) => a - b));
    same('event.code counts, not the layout key', colorKey({ type: 'keydown', code: 'Digit3', key: '§', ctrlKey: true, shiftKey: true, altKey: false, metaKey: false }), '\x1b[64;28~');
    same('keyup of a colour key maps too (the coordinator sends only on keydown)', colorKey({ type: 'keyup', code: 'Digit1', ctrlKey: true, shiftKey: true, altKey: false, metaKey: false }), '\x1b[64;144~');
    same('other keys and modifier sets are no colour keys', [
        press('Digit1', { ctrlKey: true }), press('Digit1', { shiftKey: true }), press('Digit1', { ctrlKey: true, altKey: true, shiftKey: true }),
        press('Digit1', { ctrlKey: true, shiftKey: true, metaKey: true }), press('Digit9', ALT_SHIFT), press('Digit0', ALT_SHIFT),
        press('KeyA', CTRL_SHIFT), press('Numpad1', CTRL_SHIFT), press('toString', CTRL_SHIFT), colorKey(null)
    ], ['', '', '', '', '', '', '', '', '', '']);
}

// 16. C64 entries are character mode at 40x25: keys go out at once and unchanged, nothing is echoed locally.
{
    screen.out = '';
    const c64 = Session.open({ term: screen, entry: { id: 'the-cabin', protocol: 'telnet', kind: 'c64' }, cols: 40, rows: 25 });
    const socket = sockets.at(-1);
    same('a C64 call asks for 40x25', socket.url, 'wss://desk.example/api/desktop/retronet/connect?entry=the-cabin&cols=40&rows=25');
    c64.send('A');
    c64.send('\x7f');
    c64.send('\x1b[64;144~');
    same('keys and colour sequences go out unchanged', socket.sent, ['bin:A', 'bin:\x7f', 'bin:\x1b[64;144~']);
    same('no local echo', screen.out, '');
    c64.dispose();
}

if (failures) {
    console.log(failures + ' check(s) failed');
    process.exit(1);
}
console.log('terminal retronet session ok');
