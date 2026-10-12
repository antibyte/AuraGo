// C64 additions to the Retro-Net terminal fixture (TestDesktopTerminalRetroNetC64Browser), served right after
// terminal-retronet-fixture.js in the same script: one C64 catalog board, numbered 36 under its own category between
// the games and the own entries (the status answers never list it), a record of the backgrounds the CRT is told to
// paint behind the grid, and helpers for the keys, the hint hold after CONNECT (kept for manual release or timed)
// and xterm's font switches.

fixtureCatalog.push(catalogEntry('c64-test', 'Test C64 Board', 'commodore', 'telnet', 'c64.example.org', 6400, 'c64', 'petscii'));

window.fixtureCrtBackgrounds = [];
const fixtureCreateCrt = TerminalCrt.create;
TerminalCrt.create = function (options) {
    const crt = fixtureCreateCrt(options);
    const setProfile = crt.setProfile;
    crt.setProfile = function (profile) {
        fixtureCrtBackgrounds.push(profile && profile.theme ? profile.theme.background : '');
        return setProfile.call(crt, profile);
    };
    return crt;
};

// Keys that reached the xterm textarea, as modifiers (C/A/S) plus the key code, e.g. 'CS+Digit1'.
window.fixtureKeys = [];
document.addEventListener('keydown', event => {
    const term = fixtureTerm();
    if (!term || event.target !== term.textarea) return;
    fixtureKeys.push((event.ctrlKey ? 'C' : '') + (event.altKey ? 'A' : '') + (event.shiftKey ? 'S' : '') + '+' + event.code);
}, true);

// The hint hold after CONNECT. terminal.js arms it as window.setTimeout(function () { endHold(run); }, C64_HINT_HOLD_MS)
// (2000 ms, both pinned by TestDesktopTerminalAppWiresStyles); only a timer with that delay and that callback counts,
// every other timer passes through untouched. Each hold is recorded in fixtureHolds with its state: 'pending',
// 'cleared' (terminal.js ended it: a key, a full queue or the end of the call) or 'fired' (its timer, or
// fixtureReleaseHold()). With fixtureExpectHold(true) the callback is kept instead of scheduled, so the hold lasts
// until terminal.js ends it or the test releases it: checks need no wall-clock deadline. With false the real timer
// runs and fixtureHolds still records when it was armed and fired.
window.fixtureHoldManual = false;
window.fixtureHolds = [];
const fixtureSetTimeout = window.setTimeout;
const fixtureClearTimeout = window.clearTimeout;
let fixtureHoldId = 0x7fff0000;
window.setTimeout = function (callback, delay) {
    if (delay !== 2000 || typeof callback !== 'function' || !String(callback).includes('endHold(run)')) {
        return fixtureSetTimeout.apply(window, arguments);
    }
    const hold = { manual: fixtureHoldManual, state: 'pending', armedAt: performance.now(), endedAt: 0 };
    hold.fire = late => {
        if (hold.state === 'pending') {
            hold.state = 'fired';
            hold.endedAt = performance.now();
        } else if (!late) {
            return false;
        }
        callback();
        return true;
    };
    hold.id = hold.manual ? ++fixtureHoldId : fixtureSetTimeout.call(window, () => hold.fire(false), delay);
    fixtureHolds.push(hold);
    return hold.id;
};
window.clearTimeout = function (id) {
    const hold = fixtureHolds.find(h => h.id === id);
    if (hold && hold.state === 'pending') {
        hold.state = 'cleared';
        hold.endedAt = performance.now();
    }
    return fixtureClearTimeout.apply(window, arguments);
};
// Before a CONNECT: forget earlier holds and choose whether the next one is kept (manual) or runs its real timer.
window.fixtureExpectHold = manual => {
    fixtureHoldManual = !!manual;
    fixtureHolds = [];
};
// The hold of the current call, or null before CONNECT.
window.fixtureHold = () => fixtureHolds.at(-1) || null;
// Fires the kept callback of a pending manual hold as its timer would and returns true; false when there is none.
// `late` fires it even after terminal.js cleared it, like a timer that went off anyway.
window.fixtureReleaseHold = late => {
    const hold = fixtureHold();
    return !!hold && hold.manual && hold.fire(!!late);
};
// A translated dim line as the coordinator wraps it: at spaces, one cell short of the screen width.
window.fixtureWrapped = key => TerminalText.wrapWords(t(key), fixtureTerm().cols - 1);
// The Telnet notice and the packed key hint are on screen, the board output `marker` is not.
window.fixtureHintShown = async marker => {
    await fixtureFlush();
    const lines = fixtureLines();
    const own = fixtureWrapped('desktop.terminal_retronet_telnet_notice')
        .concat(TerminalText.wrapHints(t('desktop.terminal_retronet_c64_keys'), fixtureTerm().cols - 1));
    return own.every(line => lines.includes(line)) && !lines.some(line => line.includes(marker));
};

// Every font family xterm switches to after fixtureWatchFonts() (test-only access to xterm's options service).
window.fixtureFonts = [];
window.fixtureWatchFonts = () => {
    fixtureFonts = [];
    if (window.fixtureFontWatch) fixtureFontWatch.dispose();
    window.fixtureFontWatch = fixtureTerm()._core.optionsService.onSpecificOptionChange('fontFamily', family => fixtureFonts.push(family));
};
