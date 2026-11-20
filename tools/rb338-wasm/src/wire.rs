// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors

//! The original panel's messages (`orb-ui/src/link.rs`, `Msg::encode` and `Link::command`),
//! applied to the engine as `orb_player::Audio::apply` does.
//!
//! A batch is a sequence of `u16 LE length` + message bytes. A message is `[35, id, value]`
//! (song msg 35; the value is a big-endian u32 for the ids 4, 8, 12 and 16, one byte for the
//! others) or `[41, dev, sub, data...]` (a device message).
//!
//! | message | engine call |
//! |---|---|
//! | `[35,0,m]` `[35,1,v]` `[35,2,v]` `[35,3,v]` | song mode, play/stop, record, loop |
//! | `[35,4,u32]` `[35,8,u32]` `[35,12,u32]` `[35,16,u32]` | tempo, locate, loop start, loop end |
//! | `[35,20,v]` `[35,21,v]` | shuffle, vintage |
//! | `[41,dev,sub,ctrl,v]` (sub 40/57/74/84/92/102/114) | `device_param` |
//! | `[41,0,145,id,v]` / `[41,0,147,c,k,v]` | `mixer_global` / `mixer_channel` |
//! | `[41,dev,42|59|76,p,field,v]` | `pattern_field` |
//! | `[41,dev,44|61|78,p,bytes]` | `pattern` |
//! | `[41,dev,45|62|79,p,s,field,v]` | `step_field` |

use orb_engine::frame::Engine;
use orb_engine::song::Device;

/// The `{ctrl, v}` sub of a device (`link.rs:161-172`).
pub fn param_sub(d: Device) -> Option<u8> {
    Some(match d {
        Device::Tb303_1 | Device::Tb303_2 => 40,
        Device::Tr808 => 57,
        Device::Tr909 => 74,
        Device::Delay => 84,
        Device::Distortion => 92,
        Device::Compressor => 102,
        Device::Pcf => 114,
        Device::Mixer => return None,
    })
}

/// The pattern header sub of a pattern device (303 42, 808 59, 909 76); the whole pattern (44,
/// 61, 78) and step byte (45, 62, 79) subs follow at +2 and +3 (`link.rs:176-182`).
pub fn header_sub(d: Device) -> u8 {
    match d {
        Device::Tb303_1 | Device::Tb303_2 => 42,
        Device::Tr808 => 59,
        _ => 76,
    }
}

/// The highest controller a device message may address: the live blocks are at most 33 bytes
/// and the panel sends 0..=29; anything above 31 is not a panel message.
const MAX_CTRL: u8 = 31;

/// Applies every message of a batch, in order; a malformed message is skipped.
pub fn apply_batch(e: &mut Engine, mut batch: &[u8]) {
    while batch.len() >= 2 {
        let n = u16::from_le_bytes([batch[0], batch[1]]) as usize;
        let rest = &batch[2..];
        if n > rest.len() {
            return;
        }
        apply(e, &rest[..n]);
        batch = &rest[n..];
    }
}

/// Applies one message.
pub fn apply(e: &mut Engine, m: &[u8]) {
    match m {
        [35, id, value @ ..] => global(e, *id, value),
        [41, dev, sub, data @ ..] => {
            if let Some(d) = Device::from_id(*dev) {
                device(e, d, *sub, data);
            }
        }
        _ => {}
    }
}

fn global(e: &mut Engine, id: u8, value: &[u8]) {
    let v = if matches!(id, 4 | 8 | 12 | 16) {
        match value {
            [a, b, c, d, ..] => u32::from_be_bytes([*a, *b, *c, *d]),
            _ => return,
        }
    } else {
        match value.first() {
            Some(&b) => b as u32,
            None => return,
        }
    };
    let on = (v != 0) as u8;
    match id {
        0 => e.set_song_mode(on),
        1 if v != 0 => e.play(),
        1 => e.stop(),
        2 => e.set_record(on),
        3 => e.set_loop_on(on),
        4 => e.set_tempo(v),
        8 => e.locate(v),
        12 => e.set_loop_start(v),
        16 => e.set_loop_end(v),
        20 => e.set_shuffle(v as u8),
        21 => e.set_vintage(on),
        _ => {}
    }
}

fn device(e: &mut Engine, d: Device, sub: u8, data: &[u8]) {
    let b = |i: usize| data.get(i).copied();
    if Some(sub) == param_sub(d) {
        if let (Some(ctrl), Some(value)) = (b(0), b(1))
            && ctrl <= MAX_CTRL
        {
            e.device_param(d, ctrl, value);
        }
        return;
    }
    if d == Device::Mixer {
        match sub {
            145 => {
                if let (Some(id), Some(v)) = (b(0), b(1)) {
                    e.mixer_global(id, v);
                }
            }
            147 => {
                if let (Some(c), Some(k), Some(v)) = (b(0), b(1), b(2)) {
                    e.mixer_channel(c, k, v);
                }
            }
            _ => {}
        }
        return;
    }
    if !(1..=4).contains(&d.id()) {
        return;
    }
    let h = header_sub(d);
    if sub == h {
        if let (Some(p), Some(f), Some(v)) = (b(0), b(1), b(2)) {
            e.pattern_field(d, p, f, v);
        }
    } else if sub == h + 2 {
        if let Some(p) = b(0) {
            e.pattern(d, p, &data[1..]);
        }
    } else if sub == h + 3
        && let (Some(p), Some(s), Some(f), Some(v)) = (b(0), b(1), b(2), b(3))
    {
        e.step_field(d, p, s, f, v);
    }
}
