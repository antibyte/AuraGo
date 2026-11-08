# Mesh2Motion desktop pet animation inputs

Official source: https://github.com/Mesh2Motion/mesh2motion-app
Pinned revision: `fd9fdeb2ba214d210e8e4f28015c2a3c06cd8f5b`.

The thirty selected human animation GLBs and skeleton in `assets/desktop-pets/mesh2motion/` are covered by the upstream CC0 dedication, retained in `LICENSE-CC0.txt`. Selection names, original file paths and metadata are in `source.json`; SHA-256 hashes of the consumed files are recorded in `ui/3d/pets/punk/motions-manifest.json` and the runtime motion pack.

The TypeScript retargeting math and Rigify name mapping in `assets/desktop-pets/mesh2motion/retarget/` are upstream MIT code, with trailing whitespace normalized. `LICENSE-MIT.txt` retains its notice. The offline builder uses this code to adapt the human motions to AuraGo's connected T-pose deform skeleton; the TypeScript library itself is not loaded by the browser.

The original AuraGo Punk artwork, ImageGen reference, NC AI reconstruction and locally authored Rigify character are separate inputs. The Mesh2Motion licenses apply to the upstream movements/code and do not relicense the character artwork.
