// C64 additions to the Retro-Net terminal fixture (TestDesktopTerminalRetroNetC64Browser), served right after
// terminal-retronet-fixture.js in the same script: one C64 catalog board, numbered 36 under its own category between
// the games and the own entries (the status answers never list it), a record of the backgrounds the CRT is told to
// paint behind the grid, and helpers for the keys, the hint hold after CONNECT and xterm's font switches.

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

// The hint hold: fixtureConnectAt is when the coordinator switched to 'retro' (CONNECT) after fixtureWatchConnect().
window.fixtureConnectAt = 0;
window.fixtureWatchConnect = () => {
    fixtureConnectAt = 0;
    const root = fixtureRoot();
    const observer = new MutationObserver(() => {
        if (root.dataset.terminalMode !== 'retro') return;
        fixtureConnectAt = performance.now();
        observer.disconnect();
    });
    observer.observe(root, { attributes: true, attributeFilter: ['data-terminal-mode'] });
};
// Resolves `ms` after CONNECT.
window.fixtureAfterConnect = ms => new Promise(r => setTimeout(r, Math.max(0, fixtureConnectAt + ms - performance.now())));
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
