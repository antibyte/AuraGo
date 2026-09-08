# xterm canvas renderer

- Package: `xterm-addon-canvas@0.5.0` (MIT; see `xterm-addon-canvas.LICENSE`).
- Source: https://registry.npmjs.org/xterm-addon-canvas/-/xterm-addon-canvas-0.5.0.tgz
- Vendored file: unmodified `package/lib/xterm-addon-canvas.js` as `xterm-addon-canvas.min.js`.
- SHA-256: `0de8c8c4685f21c06f6022a1ab0c48673ecba2330d3238bf39f3d0ec7607c053`.
- Compatibility: embedded `xterm.min.js` is the exact `xterm@5.3.0` distribution
  (SHA-256 `f0aea0f75f48559013ae6643c2479dd737d26da42d5524e6d2b70915ae6523c7`).
  The [upstream 5.3.0 release](https://github.com/xtermjs/xterm.js/blob/5.3.0/addons/xterm-addon-canvas/package.json)
  pairs this core with canvas addon 0.5.0. Do not replace it with an independently
  versioned `@xterm/addon-canvas` release; its private core services differ.
- Consumer: Desktop Terminal only. No external downloads at runtime.
- Verification: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopTerminalRetroBrowser -count=1`.
