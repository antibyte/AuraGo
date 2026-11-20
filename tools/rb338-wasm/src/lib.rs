// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors

//! The OpenRB-338 ReBirth engine as a WebAssembly module for AuraGo's Synth Studio.
//!
//! The crate wraps `orb-engine`, `rbs` and `rbm` (pinned git dependencies) behind a raw
//! `extern "C"` ABI (no wasm-bindgen, no imports). The host (an AudioWorklet or a Worker)
//! creates a context from the kit bytes (`Standard.rbm`), pushes the original panel's wire
//! messages, renders 1024-frame calls and reads a fixed 192-byte snapshot back.
//!
//! The kit is never embedded here: nothing in the library may name `rbm::standard::EMBEDDED`,
//! `builtin_samples` or `builtin_file`, so that the 2.5 MB kit stays out of the wasm.

pub mod abi;
pub mod compose;
pub mod export;
pub mod kit;
pub mod session;
pub mod snapshot;
pub mod wire;

pub use abi::*;
