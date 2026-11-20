// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors

//! The offline WAV export. It never touches the live engine of the context: it builds an engine
//! of its own on the context's tables and kit, as `orb render` does.
//!
//! - **Song mode (0):** `parse -> convert -> Engine::new(tables, Devices::implemented(Some(kit)))
//!   -> export::export_song(engine, song, idle_blocks) -> wav_bytes` (`orb-cli/src/render.rs:172-187`,
//!   `frame/export.rs:129-143, 165-172`). The corpus tests always pass the kit, so it is always
//!   passed: the WAV equals `orb render`'s byte for byte.
//! - **Pattern mode (1)**, our extension (the original disables the export in pattern mode):
//!   `load` + `activate(ReplacesCurrent { current_playing: false })` + `export` over
//!   `ExportRange { tempo, loop_start: 0, loop_end: bars * 768 }` (`frame/export.rs:103-123`). The
//!   song is rendered in pattern mode whatever its file says.

use orb_engine::frame::export::{self, ExportRange};
use orb_engine::frame::{Activation, Devices, Engine};
use orb_engine::song::TICKS_PER_BAR;

use crate::session::{Ctx, open_bytes};

/// The most bars a pattern-mode export renders.
pub const MAX_BARS: u32 = 256;

/// Renders the song file `data` into `c.io` as a WAV. 0 ok, 1 not a song, 2 cannot convert,
/// 3 the engine refused the load.
pub fn export_wav(c: &mut Ctx, data: &[u8], mode: u32, bars: u32, idle_blocks: u32) -> i32 {
    let mut o = match open_bytes(data) {
        Ok(o) => o,
        Err(code) => return code,
    };
    let mut engine = Engine::new(
        c.tables.clone(),
        Devices::implemented(Some(c.kit.clone())),
    );
    let samples = if mode == 0 {
        match export::export_song(&mut engine, &o.song, idle_blocks) {
            Ok(s) => s,
            Err(_) => return 3,
        }
    } else {
        o.song.globals.song_mode = 0;
        if engine.load(&o.song).is_err() {
            return 3;
        }
        engine.activate(Activation::ReplacesCurrent {
            current_playing: false,
        });
        let range = ExportRange {
            tempo: o.song.globals.tempo,
            loop_start: 0,
            loop_end: bars.clamp(1, MAX_BARS) * TICKS_PER_BAR,
        };
        export::export(&mut engine, range, idle_blocks)
    };
    c.io = export::wav_bytes(&samples);
    0
}
