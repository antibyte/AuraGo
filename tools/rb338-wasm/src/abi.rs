// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors

//! The raw `extern "C"` ABI. The handle is a `*mut Ctx` from `Box::into_raw`. Every pointer the
//! host passes points into this module's linear memory (`orb_alloc`); every pointer it gets back
//! stays valid until the next call that can allocate on the same handle (create, load, save,
//! push, export): the host re-creates its typed-array views after those calls.
//!
//! | export | purpose |
//! |---|---|
//! | `orb_abi_version() -> u32` | 1 |
//! | `orb_alloc(len) -> *mut u8`, `orb_free(ptr, len)` | linear-memory buffers |
//! | `orb_create(kit_ptr, kit_len) -> *mut Ctx`, `orb_last_error() -> u32` | context (null on error) |
//! | `orb_destroy(h)` | |
//! | `orb_new_song(h) -> i32` | File > New |
//! | `orb_load_song(h, ptr, len) -> i32` | 0 ok, 1 not a song, 2 cannot convert, 3 `LoadRefused` |
//! | `orb_save_song(h) -> i32` | RBS bytes into `io` |
//! | `orb_song_info(h) -> i32` | JSON into `io` |
//! | `orb_patterns(h) -> *const u8` | 18688 bytes |
//! | `orb_defaults(h) -> *const u8` | `snapshot::DEFAULTS_LEN` bytes |
//! | `orb_push(h, ptr, len, seq) -> i32` | 0 ok, 1 queue full |
//! | `orb_apply(h)` | drain the queue now |
//! | `orb_render(h) -> *const f32` | one 1024-frame call, planar L then R |
//! | `orb_snapshot(h) -> *const u8` | 192 bytes |
//! | `orb_startup_sound(h)` | the logo click |
//! | `orb_io_ptr(h) -> *const u8`, `orb_io_len(h) -> u32` | result bytes |
//! | `orb_export_wav(h, song_ptr, song_len, mode, bars, idle_blocks) -> i32` | WAV into `io` |

use std::sync::atomic::{AtomicU32, Ordering};

use crate::session::Ctx;
use crate::{export, snapshot};

/// Guards the host against stale cached assets.
pub const ABI_VERSION: u32 = 1;

/// The patterns buffer: 303#1 32x98, 303#2 32x98, 808 32x194, 909 32x194.
pub const PATTERNS_LEN: usize = 2 * 32 * 98 + 2 * 32 * 194;

static LAST_ERROR: AtomicU32 = AtomicU32::new(0);

/// The context behind a handle.
///
/// # Safety
/// `h` must come from `orb_create` and not be destroyed.
unsafe fn cx<'a>(h: *mut Ctx) -> &'a mut Ctx {
    unsafe { &mut *h }
}

/// The bytes the host wrote at `ptr`.
///
/// # Safety
/// `ptr..ptr+len` must be readable.
unsafe fn bytes<'a>(ptr: *const u8, len: u32) -> &'a [u8] {
    if len == 0 {
        &[]
    } else {
        unsafe { std::slice::from_raw_parts(ptr, len as usize) }
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn orb_abi_version() -> u32 {
    ABI_VERSION
}

#[unsafe(no_mangle)]
pub extern "C" fn orb_alloc(len: u32) -> *mut u8 {
    let b = vec![0u8; len.max(1) as usize].into_boxed_slice();
    Box::into_raw(b) as *mut u8
}

/// # Safety
/// `ptr` and `len` must be a pair from `orb_alloc`, freed once.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_free(ptr: *mut u8, len: u32) {
    if ptr.is_null() {
        return;
    }
    let n = len.max(1) as usize;
    drop(unsafe { Box::from_raw(std::ptr::slice_from_raw_parts_mut(ptr, n)) });
}

/// # Safety
/// `kit_ptr..kit_ptr+kit_len` must be readable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_create(kit_ptr: *const u8, kit_len: u32) -> *mut Ctx {
    match Ctx::create(unsafe { bytes(kit_ptr, kit_len) }) {
        Ok(c) => {
            LAST_ERROR.store(0, Ordering::Relaxed);
            Box::into_raw(c)
        }
        Err(code) => {
            LAST_ERROR.store(code, Ordering::Relaxed);
            std::ptr::null_mut()
        }
    }
}

/// 0 none, 1 the kit does not parse, 2 it is not recrypted for orb, 3 its samples do not load.
#[unsafe(no_mangle)]
pub extern "C" fn orb_last_error() -> u32 {
    LAST_ERROR.load(Ordering::Relaxed)
}

/// # Safety
/// `h` from `orb_create`, destroyed once.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_destroy(h: *mut Ctx) {
    if !h.is_null() {
        drop(unsafe { Box::from_raw(h) });
    }
}

/// # Safety
/// `h` from `orb_create`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_new_song(h: *mut Ctx) -> i32 {
    unsafe { cx(h) }.new_song();
    0
}

/// # Safety
/// `h` from `orb_create`; `ptr..ptr+len` readable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_load_song(h: *mut Ctx, ptr: *const u8, len: u32) -> i32 {
    unsafe { cx(h) }.load(unsafe { bytes(ptr, len) })
}

/// # Safety
/// `h` from `orb_create`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_save_song(h: *mut Ctx) -> i32 {
    unsafe { cx(h) }.save()
}

/// # Safety
/// `h` from `orb_create`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_song_info(h: *mut Ctx) -> i32 {
    unsafe { cx(h) }.song_info();
    0
}

/// # Safety
/// `h` from `orb_create`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_patterns(h: *mut Ctx) -> *const u8 {
    let c = unsafe { cx(h) };
    c.read_patterns();
    debug_assert_eq!(c.patterns.len(), PATTERNS_LEN);
    c.patterns.as_ptr()
}

/// # Safety
/// `h` from `orb_create`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_defaults(h: *mut Ctx) -> *const u8 {
    unsafe { cx(h) }.defaults.as_ptr()
}

/// # Safety
/// `h` from `orb_create`; `ptr..ptr+len` readable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_push(h: *mut Ctx, ptr: *const u8, len: u32, seq: u32) -> i32 {
    if unsafe { cx(h) }.push(unsafe { bytes(ptr, len) }, seq) {
        0
    } else {
        1
    }
}

/// # Safety
/// `h` from `orb_create`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_apply(h: *mut Ctx) {
    unsafe { cx(h) }.drain();
}

/// # Safety
/// `h` from `orb_create`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_render(h: *mut Ctx) -> *const f32 {
    let c = unsafe { cx(h) };
    c.render_call();
    c.planar.as_ptr()
}

/// # Safety
/// `h` from `orb_create`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_snapshot(h: *mut Ctx) -> *const u8 {
    let c = unsafe { cx(h) };
    snapshot::write(c);
    c.snap.as_ptr()
}

/// # Safety
/// `h` from `orb_create`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_startup_sound(h: *mut Ctx) {
    unsafe { cx(h) }.startup_sound();
}

/// # Safety
/// `h` from `orb_create`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_io_ptr(h: *mut Ctx) -> *const u8 {
    unsafe { cx(h) }.io.as_ptr()
}

/// # Safety
/// `h` from `orb_create`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_io_len(h: *mut Ctx) -> u32 {
    unsafe { cx(h) }.io.len() as u32
}

/// # Safety
/// `h` from `orb_create`; `song_ptr..song_ptr+song_len` readable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn orb_export_wav(
    h: *mut Ctx,
    song_ptr: *const u8,
    song_len: u32,
    mode: u32,
    bars: u32,
    idle_blocks: u32,
) -> i32 {
    let data = unsafe { bytes(song_ptr, song_len) };
    export::export_wav(unsafe { cx(h) }, data, mode, bars, idle_blocks)
}
