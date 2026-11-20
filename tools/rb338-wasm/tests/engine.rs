// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors

//! The ABI end to end, as the host drives it. The kit is `rbm::standard::EMBEDDED`, used by
//! the tests only (the library never names it, so the wasm stays small).

use aurago_rb338::session::Ctx;
use aurago_rb338::snapshot::{OFF_PLAY_STEP, SIZE};
use aurago_rb338::*;
use sha2::{Digest, Sha256};

const S909_BASS: &[u8] = include_bytes!("data/909_bass.rbs");
const S303_SLIDE: &[u8] = include_bytes!("data/303_slide.rbs");
const WAV_909_BASS: &str = "62b37761b09a6cc439a2dd8432a78a0c3444dc33e196138c0b8792086878c80c";
const WAV_303_SLIDE: &str = "558cc7b9d788e07e2784ade269401051271ff8026a966fc89c9bc071a203d59a";

/// A context on the embedded kit, through the ABI.
fn create() -> *mut Ctx {
    let kit = rbm::standard::EMBEDDED;
    let h = unsafe { orb_create(kit.as_ptr(), kit.len() as u32) };
    assert!(!h.is_null(), "orb_create failed: {}", orb_last_error());
    h
}

/// A wire batch: each message with its `u16 LE` length.
fn batch(msgs: &[&[u8]]) -> Vec<u8> {
    let mut b = Vec::new();
    for m in msgs {
        b.extend((m.len() as u16).to_le_bytes());
        b.extend_from_slice(m);
    }
    b
}

fn push(h: *mut Ctx, msgs: &[&[u8]], seq: u32) {
    let b = batch(msgs);
    assert_eq!(unsafe { orb_push(h, b.as_ptr(), b.len() as u32, seq) }, 0);
}

fn load(h: *mut Ctx, song: &[u8]) {
    assert_eq!(unsafe { orb_load_song(h, song.as_ptr(), song.len() as u32) }, 0);
}

fn io(h: *mut Ctx) -> Vec<u8> {
    unsafe { std::slice::from_raw_parts(orb_io_ptr(h), orb_io_len(h) as usize).to_vec() }
}

fn patterns(h: *mut Ctx) -> Vec<u8> {
    unsafe { std::slice::from_raw_parts(orb_patterns(h), abi::PATTERNS_LEN).to_vec() }
}

fn snapshot(h: *mut Ctx) -> Vec<u8> {
    unsafe { std::slice::from_raw_parts(orb_snapshot(h), SIZE).to_vec() }
}

fn render(h: *mut Ctx) -> Vec<f32> {
    unsafe { std::slice::from_raw_parts(orb_render(h), 2048).to_vec() }
}

fn sha256(b: &[u8]) -> String {
    Sha256::digest(b).iter().map(|b| format!("{b:02x}")).collect()
}

#[test]
fn a_909_step_message_changes_the_pattern_byte() {
    let h = create();
    assert_eq!(orb_abi_version(), 1);
    // 909 pattern 0, step 0, instrument 1: [41, dev 4, sub 79, p, s, field, v]
    let at = 2 * 32 * 98 + 32 * 194 + 2 + 1;
    assert_eq!(patterns(h)[at], 0);
    push(h, &[&[41, 4, 79, 0, 0, 1, 1]], 7);
    unsafe { orb_apply(h) };
    assert_eq!(patterns(h)[at], 1);
    assert_eq!(u32::from_le_bytes(snapshot(h)[4..8].try_into().unwrap()), 7);
    unsafe { orb_destroy(h) };
}

#[test]
fn play_renders_909_bass_and_lights_a_step() {
    let h = create();
    load(h, S909_BASS);
    push(h, &[&[35, 1, 1]], 1);
    let mut peak = 0f32;
    let mut lit = false;
    for _ in 0..24 {
        peak = render(h).iter().fold(peak, |p, s| p.max(s.abs()));
        lit |= snapshot(h)[OFF_PLAY_STEP + 3] < 16;
    }
    assert!(peak > 0.0, "the render is silent");
    assert!(lit, "no 909 step light while playing");
    unsafe { orb_destroy(h) };
}

#[test]
fn song_mode_export_matches_the_corpus_wavs() {
    let h = create();
    for (song, want) in [(S909_BASS, WAV_909_BASS), (S303_SLIDE, WAV_303_SLIDE)] {
        let r = unsafe { orb_export_wav(h, song.as_ptr(), song.len() as u32, 0, 0, 9088) };
        assert_eq!(r, 0);
        let wav = io(h);
        assert_eq!(wav.len(), 44 + 4 * 176_400);
        assert_eq!(sha256(&wav), want);
    }
    unsafe { orb_destroy(h) };
}

#[test]
fn save_after_load_converts_to_the_same_song() {
    // The save writes the tracks as the engine holds them (OpenRB's own rule, `document.rs`
    // module doc): the constructor's events lead each tick, so the order within a tick may
    // differ from the file's. Everything else must be equal, the events as a set, and a second
    // save of the saved file must be byte-identical (the order is stable).
    let h = create();
    for song in [S909_BASS, S303_SLIDE] {
        load(h, song);
        assert_eq!(unsafe { orb_save_song(h) }, 0);
        let saved = io(h);
        let mut a = orb_engine::load::convert(&rbs::Song::parse(song).unwrap()).unwrap();
        let mut b = orb_engine::load::convert(&rbs::Song::parse(&saved).unwrap()).unwrap();
        for t in 0..9 {
            assert_eq!(a.tracks[t].len(), b.tracks[t].len(), "track {t}");
            let key = |e: &orb_engine::song::TrackEvent| (e.tick, e.ctrl, e.value);
            a.tracks[t].sort_by_key(key);
            b.tracks[t].sort_by_key(key);
        }
        assert_eq!(a.tracks, b.tracks);
        // the rest of the song (the batches follow the track order) must be equal
        a.tracks = Default::default();
        b.tracks = Default::default();
        assert_eq!(a, b);
        load(h, &saved);
        assert_eq!(unsafe { orb_save_song(h) }, 0);
        assert_eq!(io(h), saved, "a second save differs");
    }
    unsafe { orb_destroy(h) };
}

#[test]
fn a_new_song_saves_to_a_file_that_parses() {
    let h = create();
    load(h, S909_BASS);
    assert_eq!(unsafe { orb_new_song(h) }, 0);
    assert_eq!(unsafe { orb_save_song(h) }, 0);
    let saved = io(h);
    let song = rbs::Song::parse(&saved).expect("the saved new song parses");
    orb_engine::load::convert(&song).expect("and converts");
    unsafe { orb_song_info(h) };
    let info = String::from_utf8(io(h)).unwrap();
    assert!(info.contains("\"form\":\"rbs\"") && info.contains("Standard ReBirth"), "{info}");
    unsafe { orb_destroy(h) };
}
