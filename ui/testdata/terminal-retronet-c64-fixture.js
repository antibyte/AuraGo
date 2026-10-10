// C64 additions to the Retro-Net terminal fixture (TestDesktopTerminalRetroNetC64Browser), served right after
// terminal-retronet-fixture.js in the same script: one C64 catalog board, numbered 36 under its own category between
// the games and the own entries (the status answers never list it), and a record of the backgrounds the CRT is told
// to paint behind the grid.

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
