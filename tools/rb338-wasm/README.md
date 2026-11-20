# aurago-rb338

The [OpenRB-338](https://github.com/pingw33n/OpenRB-338) ReBirth RB-338 engine as a WebAssembly
module for AuraGo's Synth Studio. GPL-3.0-only (see `LICENSE`).

- Upstream: <https://github.com/pingw33n/OpenRB-338>, commit
  `0bebfcdfd544b3cdea14536f19fb28a8de314df5`. `orb-engine`, `rbs` and `rbm` are git dependencies
  pinned to that rev.
- A raw `extern "C"` ABI, no wasm-bindgen, no imports. The exports are listed in `src/abi.rs`; the
  wire messages (the original panel's) in `src/wire.rs`; the 192-byte snapshot in `src/snapshot.rs`.
- The kit (`Standard.rbm`) is not part of the wasm: the host passes its bytes to `orb_create`. The
  library never names `rbm::standard::EMBEDDED`, `builtin_samples` or `builtin_file`; only the
  tests use `EMBEDDED`.
- Engine calls are 1024 frames, as the original's MME buffer and OpenRB's player.

## Build and test

The toolchain is pinned by `rust-toolchain.toml` (1.99.1, target `wasm32-unknown-unknown`).

```
cargo +1.99.1 test --release --manifest-path tools/rb338-wasm/Cargo.toml
cargo +1.99.1 build --release --locked --target wasm32-unknown-unknown --manifest-path tools/rb338-wasm/Cargo.toml
```

The wasm is `target/wasm32-unknown-unknown/release/aurago_rb338.wasm`.

The tests check, among others, that the song-mode export of `tests/data/909_bass.rbs` and
`303_slide.rbs` equals the SHA-256 of the OpenRB-338 corpus WAVs byte for byte (see
`tests/data/README.md`).
