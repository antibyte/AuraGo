// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors

//! Snapshot v1: the engine's state after a call as a fixed 192-byte, little-endian record.
//!
//! A port of `Audio::publish` (`orb-player/src/lib.rs:945-1013`) reduced to what the panel
//! needs, with the step-light rule of `Link::follow` (`orb-ui/src/link.rs:1267-1274`). The
//! controller blocks are written in RBS controller numbering: the TR-909's live block is mapped
//! through `tr909::internal_index`, so the host never sees the internal indices.
//!
//! | offset | content |
//! |---|---|
//! | 0 | version = 1 |
//! | 1 | flags: bit0 playing, 1 recording, 2 song mode, 3 loop on, 4 vintage, 5 faulted in the last call, 6 preview playing |
//! | 2 | shuffle |
//! | 4 | `u32` applied_seq |
//! | 8 | `f64` frames |
//! | 16 | `i32` tick |
//! | 20 / 24 / 28 | `u32` loop_start / loop_end / tempo |
//! | 32 / 36 / 40 | `u32` generation / tracks revision / merges |
//! | 44 | `u8[4]` play_step (255 = none) |
//! | 48 | `u8[4]` playing_pattern (255 = none) |
//! | 52 | `u8[4]` channel meters |
//! | 56 | `u8[2]` master meters |
//! | 58 | compressor gain byte (comp live 3) |
//! | 59 | `u8[4]` effect meters (delay live 5, dist 3, PCF 7, comp 4) |
//! | 64 | mixer[23] (byte 87 is padding): master, comp_id, pcf_id, 4 x (enabled, level, pan, send, dist) |
//! | 88 / 97 | 303 #1 / #2 controllers 0..8 |
//! | 106 | 808 controllers 0..28 |
//! | 135 | 909 controllers 0..29 (RBS numbering) |
//! | 165 | delay 0..4 |
//! | 170 | distortion 0..2 |
//! | 173 | PCF 0..6 |
//! | 180 | compressor 0..2 |

use orb_engine::frame::Engine;
use orb_engine::tr909;

use crate::session::Ctx;

pub const SIZE: usize = 192;
pub const VERSION: u8 = 1;

pub const OFF_VERSION: usize = 0;
pub const OFF_FLAGS: usize = 1;
pub const OFF_SHUFFLE: usize = 2;
pub const OFF_APPLIED_SEQ: usize = 4;
pub const OFF_FRAMES: usize = 8;
pub const OFF_TICK: usize = 16;
pub const OFF_LOOP_START: usize = 20;
pub const OFF_LOOP_END: usize = 24;
pub const OFF_TEMPO: usize = 28;
pub const OFF_GENERATION: usize = 32;
pub const OFF_REVISION: usize = 36;
pub const OFF_MERGES: usize = 40;
pub const OFF_PLAY_STEP: usize = 44;
pub const OFF_PLAYING_PATTERN: usize = 48;
pub const OFF_CHANNEL_METERS: usize = 52;
pub const OFF_MASTER_METERS: usize = 56;
pub const OFF_COMP_GAIN: usize = 58;
pub const OFF_EFFECT_METERS: usize = 59;
pub const OFF_MIXER: usize = 64;
pub const OFF_303_1: usize = 88;
pub const OFF_303_2: usize = 97;
pub const OFF_808: usize = 106;
pub const OFF_909: usize = 135;
pub const OFF_DELAY: usize = 165;
pub const OFF_DIST: usize = 170;
pub const OFF_PCF: usize = 173;
pub const OFF_COMP: usize = 180;
/// The end of the controller region (exclusive).
pub const OFF_END: usize = 183;

pub const FLAG_PLAYING: u8 = 1;
pub const FLAG_RECORDING: u8 = 2;
pub const FLAG_SONG_MODE: u8 = 4;
pub const FLAG_LOOP: u8 = 8;
pub const FLAG_VINTAGE: u8 = 16;
pub const FLAG_FAULTED: u8 = 32;
pub const FLAG_PREVIEW: u8 = 64;

// The layout is pinned: every block follows the one before it, and the record ends at 192.
const _: () = {
    assert!(OFF_APPLIED_SEQ == OFF_SHUFFLE + 2);
    assert!(OFF_FRAMES == OFF_APPLIED_SEQ + 4);
    assert!(OFF_TICK == OFF_FRAMES + 8);
    assert!(OFF_LOOP_START == OFF_TICK + 4);
    assert!(OFF_LOOP_END == OFF_LOOP_START + 4);
    assert!(OFF_TEMPO == OFF_LOOP_END + 4);
    assert!(OFF_GENERATION == OFF_TEMPO + 4);
    assert!(OFF_REVISION == OFF_GENERATION + 4);
    assert!(OFF_MERGES == OFF_REVISION + 4);
    assert!(OFF_PLAY_STEP == OFF_MERGES + 4);
    assert!(OFF_PLAYING_PATTERN == OFF_PLAY_STEP + 4);
    assert!(OFF_CHANNEL_METERS == OFF_PLAYING_PATTERN + 4);
    assert!(OFF_MASTER_METERS == OFF_CHANNEL_METERS + 4);
    assert!(OFF_COMP_GAIN == OFF_MASTER_METERS + 2);
    assert!(OFF_EFFECT_METERS == OFF_COMP_GAIN + 1);
    assert!(OFF_MIXER == 64 && OFF_EFFECT_METERS + 4 <= OFF_MIXER);
    assert!(OFF_303_1 == OFF_MIXER + 24); // 23 mixer bytes and one pad byte
    assert!(OFF_303_2 == OFF_303_1 + 9);
    assert!(OFF_808 == OFF_303_2 + 9);
    assert!(OFF_909 == OFF_808 + 29);
    assert!(OFF_DELAY == OFF_909 + 30);
    assert!(OFF_DIST == OFF_DELAY + 5);
    assert!(OFF_PCF == OFF_DIST + 3);
    assert!(OFF_COMP == OFF_PCF + 7);
    assert!(OFF_END == OFF_COMP + 3);
    assert!(OFF_END <= SIZE);
};

/// The length of `orb_defaults`: the controller region of a snapshot plus the shuffle byte.
pub const DEFAULTS_LEN: usize = OFF_END - OFF_MIXER + 1;

fn put32(s: &mut [u8; SIZE], at: usize, v: u32) {
    s[at..at + 4].copy_from_slice(&v.to_le_bytes());
}

/// The live block of device `id` (1..=4 the channel devices, 5 delay, 6 distortion, 7 PCF,
/// 8 compressor); the mixer's is not a block.
fn live(e: &Engine, id: usize) -> &[u8] {
    let d = &e.frame.devices;
    match id {
        1..=4 => d.channels[id - 1].live(),
        5 => d.delay.live(),
        6 => d.dist.live(),
        7 => d.pcf.live(),
        8 => d.comp.live(),
        _ => &[],
    }
}

/// Writes `n` controllers of device `id` at `at`; the 909 is mapped to the RBS numbering.
fn controllers(e: &Engine, s: &mut [u8; SIZE], at: usize, id: usize, n: usize) {
    let block = live(e, id);
    for c in 0..n {
        let i = if id == 4 { tr909::internal_index(c) } else { c };
        s[at + c] = block.get(i).copied().unwrap_or(0);
    }
}

/// The mixer and every device's controllers, bytes 64..183 of a snapshot.
fn fill_controllers(e: &Engine, s: &mut [u8; SIZE]) {
    let m = &e.frame.devices.mixer.live;
    s[OFF_MIXER] = m.master;
    s[OFF_MIXER + 1] = m.comp_id;
    s[OFF_MIXER + 2] = m.pcf_id;
    for (c, ch) in m.channels.iter().enumerate() {
        let o = OFF_MIXER + 3 + 5 * c;
        s[o..o + 5].copy_from_slice(&[ch.enabled, ch.level, ch.pan, ch.send, ch.dist]);
    }
    controllers(e, s, OFF_303_1, 1, 9);
    controllers(e, s, OFF_303_2, 2, 9);
    controllers(e, s, OFF_808, 3, 29);
    controllers(e, s, OFF_909, 4, 30);
    controllers(e, s, OFF_DELAY, 5, 5);
    controllers(e, s, OFF_DIST, 6, 3);
    controllers(e, s, OFF_PCF, 7, 7);
    controllers(e, s, OFF_COMP, 8, 3);
}

/// The values of a new empty song (`new_song_knobs`, `link.rs:382-419`) in the layout of bytes
/// 64..183, followed by the shuffle: `orb_defaults`.
pub fn defaults(e: &Engine) -> Vec<u8> {
    let mut s = [0u8; SIZE];
    fill_controllers(e, &mut s);
    let mut v = s[OFF_MIXER..OFF_END].to_vec();
    v.push(e.frame.seq.globals.shuffle);
    v
}

/// Writes the snapshot of the current song after the last call into `c.snap`.
pub fn write(c: &mut Ctx) {
    let mut s = [0u8; SIZE];
    let e = &*c.engine;
    let f = &e.frame;
    let g = &f.seq.globals;
    let playing = g.play != 0;
    s[OFF_VERSION] = VERSION;
    let flags = [
        (playing, FLAG_PLAYING),
        (g.byte2 != 0, FLAG_RECORDING),
        (g.song_mode != 0, FLAG_SONG_MODE),
        (g.loop_on != 0, FLAG_LOOP),
        (f.vintage, FLAG_VINTAGE),
        (c.faulted, FLAG_FAULTED),
        (c.preview_on(), FLAG_PREVIEW),
    ];
    for (on, bit) in flags {
        if on {
            s[OFF_FLAGS] |= bit;
        }
    }
    s[OFF_SHUFFLE] = g.shuffle;
    put32(&mut s, OFF_APPLIED_SEQ, c.applied_seq);
    s[OFF_FRAMES..OFF_FRAMES + 8].copy_from_slice(&(c.frames as f64).to_le_bytes());
    s[OFF_TICK..OFF_TICK + 4].copy_from_slice(&f.seq.tick_pos().to_le_bytes());
    put32(&mut s, OFF_LOOP_START, g.loop_start);
    put32(&mut s, OFF_LOOP_END, g.loop_end);
    put32(&mut s, OFF_TEMPO, g.tempo);
    put32(&mut s, OFF_GENERATION, c.generation);
    put32(&mut s, OFF_REVISION, f.seq.revision as u32);
    put32(&mut s, OFF_MERGES, f.seq.merges as u32);
    let d = &f.devices;
    for (i, ch) in d.channels.iter().enumerate() {
        // a section whose pattern gate is closed lights no step (`link.rs:1267-1274`)
        let at = [9usize, 9, 29, 30][i];
        s[OFF_PLAY_STEP + i] = live(e, i + 1)
            .get(at)
            .copied()
            .filter(|&b| playing && b < 16 && !ch.gate().closed())
            .unwrap_or(255);
        s[OFF_PLAYING_PATTERN + i] = ch.playing_pattern().unwrap_or(255);
    }
    for (i, ch) in f.state.channels.iter().take(4).enumerate() {
        s[OFF_CHANNEL_METERS + i] = ch.meter;
    }
    s[OFF_MASTER_METERS..OFF_MASTER_METERS + 2].copy_from_slice(&f.state.master_meter);
    s[OFF_COMP_GAIN] = live(e, 8).get(3).copied().unwrap_or(0);
    for (i, (id, at)) in [(5, 5), (6, 3), (7, 7), (8, 4)].into_iter().enumerate() {
        s[OFF_EFFECT_METERS + i] = live(e, id).get(at).copied().unwrap_or(0);
    }
    fill_controllers(e, &mut s);
    c.snap = s;
}
