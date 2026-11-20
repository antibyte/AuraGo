// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors

//! Save: the song file as RBS 4.2 from the open document and the engine.
//!
//! A port of `orb_ui::document::compose` and `set_engine_tracks` (`orb-ui/src/document.rs:101-279`)
//! that reads the **engine** where the original reads the panel and the link:
//!
//! - device values: `SongDevice::saved_block(song_mode)`, which is the live block in pattern
//!   mode and the pattern-mode copy in song mode (what `compose` takes from the panel and from
//!   `Link::song_mode_blocks`);
//! - the mixer: `MixerDevice::saved(song_mode)` (the live master and channel enables in both
//!   modes, the rest from the pattern copy in song mode);
//! - the patterns: the channel devices' pattern arrays;
//! - the tracks: the sequencer's lists; the transport: the sequencer's globals.
//!
//! The rules of `compose` stay: a byte the load made and the user did not touch keeps the file's
//! byte (for a current 4.2.0 file, with `convert(base)` computed on the fly), the 303's flag bits
//! 5..7 stay as read, the 808 stores a step as `raw != 0`, the 909's controllers are mapped
//! through `internal_index`, the version is always written as 4.2.0 and the vintage byte as
//! `(vintage != 0) as u8`.

use orb_engine::frame::Engine;
use orb_engine::song::Device;
use orb_engine::tr909;

use crate::session::Doc;

/// The device pairs of `link::controls()` (`link.rs:236-343`): controller 0 (on/off) and the
/// knobs of each device; the pattern selection (controller 1) is handled apart.
fn controls() -> Vec<(Device, u8)> {
    let mut v = Vec::new();
    let mut add = |d: Device, cs: &[u8]| v.extend(cs.iter().map(|&c| (d, c)));
    let range = |a: u8, b: u8| (a..=b).collect::<Vec<u8>>();
    for d in [Device::Tb303_1, Device::Tb303_2] {
        add(d, &[0]);
        add(d, &range(2, 8));
    }
    add(Device::Tr808, &[0]);
    add(Device::Tr808, &range(2, 28));
    add(Device::Tr909, &[0]);
    add(Device::Tr909, &range(2, 29));
    add(Device::Delay, &range(0, 4));
    add(Device::Distortion, &range(0, 2));
    add(Device::Compressor, &range(0, 2));
    add(Device::Pcf, &range(0, 6));
    v
}

/// The parameter block `saved_block` replies with for device `d` (not the mixer).
fn saved_block(e: &Engine, d: Device, song_mode: bool) -> &[u8] {
    let dv = &e.frame.devices;
    match d {
        Device::Tb303_1 | Device::Tb303_2 | Device::Tr808 | Device::Tr909 => {
            dv.channels[d as usize - 1].saved_block(song_mode)
        }
        Device::Delay => dv.delay.saved_block(song_mode),
        Device::Distortion => dv.dist.saved_block(song_mode),
        Device::Pcf => dv.pcf.saved_block(song_mode),
        Device::Compressor => dv.comp.saved_block(song_mode),
        Device::Mixer => &[],
    }
}

/// Controller `c` of device `d` in a saved block, which is indexed as the live block.
fn param(block: &[u8], d: Device, c: u8) -> Option<u8> {
    let i = if d == Device::Tr909 {
        tr909::internal_index(c as usize)
    } else {
        c as usize
    };
    block.get(i).copied()
}

/// The song to write: the document's file as read with the engine's state over it.
pub fn compose(e: &mut Engine, doc: &Doc) -> rbs::Song {
    let base = &doc.base;
    let mut s = base.clone();
    // What the load made of the file's bytes, for a file of the current version: a controller
    // still as loaded keeps the byte of the file, so the quirks of the load (the 909's mid tom
    // decay is copied from the low tom's) do not rewrite a byte the user did not touch. An older
    // file is written with the values the load made of it: it is a 4.2 file now.
    let current = (base.head.major, base.head.minor, base.head.revision) == (4, 2, 0);
    let converted = orb_engine::load::convert(base).ok();
    let loaded = converted.as_ref().filter(|_| current);

    // the pattern arrays first: reading them needs the engine mutably
    let mut pats: [Vec<u8>; 4] = Default::default();
    for (c, p) in pats.iter_mut().enumerate() {
        *p = e.frame.devices.channels[c]
            .patterns_mut()
            .map(|p| p.to_vec())
            .unwrap_or_default();
    }
    let e = &*e;

    // the tracks: `set_engine_tracks` (`document.rs:101-116`), each tick as `tick / 24`
    for (t, track) in s.tracks.iter_mut().enumerate() {
        track.events = e
            .frame
            .seq
            .events(t)
            .iter()
            .map(|ev| rbs::Event {
                pos: ev.tick as u32 / 24,
                ctrl: ev.ctrl,
                value: ev.value,
            })
            .collect();
    }
    s.head.major = 4;
    s.head.minor = 2;
    s.head.revision = 0;
    // sets a byte to the engine's value (when it has one) unless that is what the load made
    let put = |slot: &mut u8, new: Option<u8>, was: Option<u8>| {
        if let Some(n) = new
            && was != Some(n)
        {
            *slot = n;
        }
    };

    let globals = &e.frame.seq.globals;
    let song_mode = globals.song_mode != 0;
    let g = &mut s.glob;
    g.song_mode = song_mode as u8;
    g.loop_on = (globals.loop_on != 0) as u8;
    g.tempo = globals.tempo;
    g.loop_start = globals.loop_start;
    g.loop_end = globals.loop_end;
    g.shuffle = globals.shuffle;
    g.mod_name = doc.info.mod_name.clone();
    g.vintage = e.frame.vintage as u8;

    // The mixer: the saved set (the live one in pattern mode; in song mode the pattern-mode
    // copy with the live master and channel enables).
    let sm = e.frame.devices.mixer.saved(song_mode);
    let lm = loaded.map(|l| &l.mixer);
    put(&mut s.mixer.master, Some(sm.master), lm.map(|m| m.master));
    put(&mut s.mixer.comp_id, Some(sm.comp_id), lm.map(|m| m.comp_id));
    put(&mut s.mixer.pcf_id, Some(sm.pcf_id), lm.map(|m| m.pcf_id));
    for (c, ch) in s.mixer.channels.iter_mut().enumerate() {
        let was = lm.map(|m| m.channels[c]);
        let now = sm.channels[c];
        put(&mut ch.enabled, Some(now.enabled), was.map(|w| w.enabled));
        put(&mut ch.level, Some(now.level), was.map(|w| w.level));
        put(&mut ch.pan, Some(now.pan), was.map(|w| w.pan));
        put(&mut ch.delay_send, Some(now.send), was.map(|w| w.delay_send));
        put(&mut ch.dist, Some(now.dist), was.map(|w| w.dist));
    }

    // The controllers: controller c of a device is parameter c of its block.
    let was = |d: Device, c: usize| -> Option<u8> {
        let l = loaded?;
        match d {
            Device::Tb303_1 => l.tb303[0].params.get(c),
            Device::Tb303_2 => l.tb303[1].params.get(c),
            Device::Tr808 => l.tr808.params.get(c),
            Device::Tr909 => l.tr909.params.get(c),
            Device::Delay => l.delay.params.get(c),
            Device::Pcf => l.pcf.params.get(c),
            Device::Distortion => l.dist.params.get(c),
            Device::Compressor => l.comp.params.get(c),
            Device::Mixer => None,
        }
        .copied()
    };
    for (d, c) in controls() {
        let v = param(saved_block(e, d, song_mode), d, c);
        let c = c as usize;
        let slot = match d {
            Device::Tb303_1 => s.tb303[0].params.get_mut(c),
            Device::Tb303_2 => s.tb303[1].params.get_mut(c),
            Device::Tr808 => s.tr808.params.get_mut(c),
            Device::Tr909 => s.tr909.params.get_mut(c),
            Device::Delay => s.delay.values.get_mut(c),
            Device::Pcf => s.pcf.values.get_mut(c),
            Device::Distortion => s.dist.values.get_mut(c),
            Device::Compressor => s.comp.values.get_mut(c),
            Device::Mixer => None,
        };
        if let Some(slot) = slot {
            put(slot, v, was(d, c));
        }
    }
    // The selected pattern of each pattern device (controller 1): the pattern-mode copy's in
    // song mode, the live one (clamped to the 32 patterns, as the panel shows it) otherwise.
    for d in [
        Device::Tb303_1,
        Device::Tb303_2,
        Device::Tr808,
        Device::Tr909,
    ] {
        let b = param(saved_block(e, d, song_mode), d, 1);
        let p = if song_mode { b } else { b.map(|p| p.min(31)) };
        let slot = match d {
            Device::Tb303_1 => &mut s.tb303[0].params[1],
            Device::Tb303_2 => &mut s.tb303[1].params[1],
            Device::Tr808 => &mut s.tr808.params[1],
            _ => &mut s.tr909.params[1],
        };
        put(slot, p, was(d, 1));
    }

    // The patterns: 98 bytes (a 303) or 194 (a drum machine) each, as the engine holds them.
    for (synth, bytes) in s.tb303.iter_mut().zip(&pats) {
        if bytes.len() < 32 * 98 {
            continue;
        }
        for (p, pat) in synth.patterns.iter_mut().enumerate() {
            let b = &bytes[p * 98..(p + 1) * 98];
            pat.shuffle = b[0];
            pat.length = b[1];
            for (i, st) in pat.steps.iter_mut().enumerate() {
                let b = &b[2 + 6 * i..8 + 6 * i];
                // the flag bits 5..7 are not the engine's: they stay as read
                (st.note, st.slide, st.accent) = (b[0], b[1], b[2]);
                (st.up, st.down, st.rest) = (b[3], b[4], b[5]);
            }
        }
    }
    for (row, drums, is808) in [(2, &mut s.tr808, true), (3, &mut s.tr909, false)] {
        if pats[row].len() < 32 * 194 {
            continue;
        }
        for (p, pat) in drums.patterns.iter_mut().enumerate() {
            let b = &pats[row][p * 194..(p + 1) * 194];
            pat.shuffle = b[0];
            pat.length = b[1];
            for (i, st) in pat.steps.iter_mut().enumerate() {
                for (j, v) in st.iter_mut().enumerate() {
                    let new = b[2 + 12 * i + j];
                    // the 808 stores a step as `raw != 0`: a nonzero byte read stays as read
                    if !is808 || (*v != 0) != (new != 0) {
                        *v = new;
                    }
                }
            }
        }
    }
    s
}
