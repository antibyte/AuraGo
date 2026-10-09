#!/usr/bin/env node
// Runs ui/js/desktop/apps/terminal-retronet-session.js (after terminal-text.js) in Node with a fake WebSocket and a
// fake xterm: entry-ID URL, binary and JSON frames, the three echo modes of Telnet world entries (line editing with
// echo, hidden line editing without echo or history, character mode) and their transitions, cell-accurate erasing
// through TerminalText (CJK two cells, emoji one), history, multi-line paste, raw control keys, resize and host-key
// frames, hang-up, remote close, bbs entries, that server text never reaches the screen through the module and
// the host-key answer parser (localized letters of all 16 locales, ASCII y/n, case-insensitive).
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
    constructor(url) {
        this.url = url;
        this.readyState = 1;
        this.binaryType = 'blob';
        this.sent = [];
        this.closeCode = 0;
        sockets.push(this);
    }
    send(value) { this.sent.push(typeof value === 'string' ? 'txt:' + value : 'bin:' + new TextDecoder().decode(value)); }
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
const screen = { out: '', write(text) { this.out += text; } };
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

// 8. Resize, host key, hang-up.
session.resize(100, 30);
session.resize(120, 40);
same('resize sends one JSON frame per change', ws.sent.at(-1), 'txt:{"type":"resize","cols":120,"rows":40}');
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

// 11. Host-key answers: localized letters first, then ASCII y/n, case-insensitive; anything else is no answer.
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

if (failures) {
    console.log(failures + ' check(s) failed');
    process.exit(1);
}
console.log('terminal retronet session ok');
