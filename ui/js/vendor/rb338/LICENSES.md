# RB-338 engine: GPL-3.0-only

The RB-338 engine (`rb338-engine.wasm`) is the OpenRB-338 crates orb-engine, rbs and rbm, compiled unchanged to WebAssembly together with AuraGo's wrapper crate `tools/rb338-wasm`.

Copyright (C) 2026 the OpenRB-338 authors. Upstream: https://github.com/pingw33n/OpenRB-338, commit 0bebfcdfd544b3cdea14536f19fb28a8de314df5.
AuraGo wrapper crate, build script and RB-338 browser module: added 2026-11-20, GPL-3.0-only.

The browser combination containing this module is distributed under the GPL-3.0-only terms; independent MIT-licensed AuraGo sources keep their licence. The full licence text is in [LICENSE.txt](LICENSE.txt).

Complete corresponding source (wrapper crate, every vendored dependency, the kit, the JavaScript of the module, the build script and rebuild steps): [sources.zip](sources.zip). Hashes and toolchain: [manifest.json](manifest.json).

## Standard.rbm

`Standard.rbm` is the standard mod of Propellerhead ReBirth 2.01 (46 drum samples, panel images and splash screen, recrypted standard mod, SHA-256 6355b5f14fc2ebfe7ed9e6e42208f97817f96c5e30ed42fdca4205d3cdab5fbe), as documented in OpenRB-338 `assets/README.md`. It is Propellerhead material, not GPL code. AuraGo ships it by the project owner's decision of 2026-11-20. The panel images are not displayed.
