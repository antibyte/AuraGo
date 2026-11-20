// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors

//! The session: one engine, its document, the command queue and the output buffers.
//!
//! It stands for `orb_player::{Player, Audio}` and `orb_ui::{Link, Document}` on one thread:
//!
//! - **create / new song:** `Player::with_call_frames` and `Link::load_new`
//!   (`orb-player/src/lib.rs:498-545`, `link.rs:807-824`);
//! - **load:** `session::open_bytes` (`orb-ui/src/session.rs:31-56`) and `Player::build`;
//! - **activate:** `Audio::activate` for a stopped song (`lib.rs:915-925`): the old song is
//!   stopped, slot 6 (`silence_all`) runs on its devices (which refills the shared delay line),
//!   then the new song is current. `Engine::activate` is the rbhost path and is not used;
//! - **render call:** `Audio::drain` and `Audio::render_call` (`lib.rs:690-698`, `751-773`).

use std::sync::Arc;

use orb_engine::frame::export::REALTIME_FRAMES;
use orb_engine::frame::{Devices, Engine, LoadRefused};
use orb_engine::process::ProcessState;
use orb_engine::samples::{STARTUP_ID, Sample, SampleSet};
use orb_engine::song::Song;
use orb_engine::tables::Tables;
use rbs::rb1x::{Block, Kind, Song1x};

use crate::{compose, kit, snapshot, wire};

/// The frames of one engine call: one MME buffer of the original.
pub const CALL_FRAMES: usize = REALTIME_FRAMES;
/// The queue's byte limit; a push over it is refused.
pub const QUEUE_LIMIT: usize = 4 << 20;

/// What File > Info shows of a song file (`orb-cli/src/info.rs`), reduced to the panel's needs.
#[derive(Clone, Debug, Default)]
pub struct SongInfo {
    /// "4.2" for an RBS file, the "File version" of a 1.x file.
    pub version: String,
    /// A ReBirth 1.x MIDI file.
    pub is_1x: bool,
    pub mod_name: String,
    pub title: String,
    /// The more text, the file's newlines as `\n`.
    pub text: String,
}

/// The open song as a document: the file as read (the base of a save) and its information.
pub struct Doc {
    /// The song file as read, an empty song for a new one and a stand-in for a 1.x import.
    pub base: rbs::Song,
    pub info: SongInfo,
}

impl Doc {
    /// A new, empty song on the standard kit (`Document::new`, `document.rs:50-59`).
    pub fn new_song() -> Doc {
        let mut base = rbs::Song::empty();
        base.glob.mod_name = rbm::standard::NAME.into();
        Doc {
            info: SongInfo {
                version: base.head.version(),
                is_1x: false,
                mod_name: base.glob.mod_name.clone(),
                title: base.usri.window_title.clone(),
                text: newlines(&base.usri.text),
            },
            base,
        }
    }
}

/// A song file read: the engine's song and its document.
pub struct Opened {
    pub song: Song,
    pub doc: Doc,
}

/// The text of a file with its newlines (0x0d, or CR LF) as `\n` (`info.rs:57-59`).
fn newlines(text: &str) -> String {
    text.replace("\r\n", "\n").replace('\r', "\n")
}

/// Where the song information starts in the decoded 1.x sysex (`info.rs:21`).
const RB1X_INFO: usize = 0x3160;

/// `info.rs:65-83`: the string of a 1.x song's block the way the original copies it.
fn usri_copy(raw: &[u8], off: usize, max: usize) -> String {
    let mut out = String::new();
    let mut n = 0;
    let mut i = off;
    while n < max {
        let c = raw.get(i).copied().unwrap_or(0);
        if c == 0 {
            break;
        }
        out.push(c as char);
        n += 1;
        if c == 0x0d && n < max {
            out.push('\n');
            n += 1;
        }
        i += 1;
    }
    out
}

/// `Usri1x::of` (`info.rs:99-118`), without the panic: `None` for a song with no sysex block.
fn usri_1x(song: &Song1x) -> Option<(String, String, String, u8)> {
    let block = song.tracks.iter().flatten().find_map(|e| match &e.kind {
        Kind::Sysex(d) => Block::decode(d).ok(),
        _ => None,
    })?;
    let b = block.bytes();
    Some((
        usri_copy(b, RB1X_INFO, 0x28),
        usri_copy(b, RB1X_INFO + 0x29, 0xC8),
        usri_copy(b, RB1X_INFO + 0xF2, 0x64),
        (b.get(RB1X_INFO + 0x157).copied().unwrap_or(0) != 0) as u8,
    ))
}

/// `source_of_1x` (`document.rs:82-93`): the empty song stands in for a 1.x song, with the
/// engine song's mod name and the song information.
fn source_of_1x(engine: &Song, song: &Song1x) -> rbs::Song {
    let mut s = rbs::Song::empty();
    s.glob.mod_name = engine.mod_name.clone();
    if let Some((title, text, web, show)) = usri_1x(song) {
        s.usri.window_title = title;
        s.usri.text = text;
        s.usri.web_page = web;
        s.usri.show_on_open = show;
    }
    s
}

/// Reads and converts a song file, an RBS 4.x file or a ReBirth 1.x MIDI file
/// (`orb-ui/src/session.rs:31-56`). The error is the ABI code: 1 not a song, 2 cannot convert.
pub fn open_bytes(data: &[u8]) -> Result<Opened, i32> {
    if rbs::rb1x::is_1x(data) {
        let parsed = Song1x::parse(data).map_err(|_| 1)?;
        let song = orb_engine::load::convert_1x(&parsed).map_err(|_| 2)?;
        let base = source_of_1x(&song, &parsed);
        let info = SongInfo {
            version: format!("{}.{}", parsed.version.0, parsed.version.1),
            is_1x: true,
            mod_name: song.mod_name.clone(),
            title: base.usri.window_title.clone(),
            text: newlines(&base.usri.text),
        };
        return Ok(Opened {
            song,
            doc: Doc { base, info },
        });
    }
    let base = rbs::Song::parse(data).map_err(|_| 1)?;
    let song = orb_engine::load::convert(&base).map_err(|_| 2)?;
    let info = SongInfo {
        version: base.head.version(),
        is_1x: false,
        mod_name: base.glob.mod_name.clone(),
        title: base.usri.window_title.clone(),
        text: newlines(&base.usri.text),
    };
    Ok(Opened {
        song,
        doc: Doc { base, info },
    })
}

/// The preview player (`Audio::preview*`): one mono sample mixed after the song.
#[derive(Default)]
pub struct Preview {
    sample: Option<Arc<Sample>>,
    pos: usize,
    pub on: bool,
}

/// Mixes the preview `audio` from frame `pos` into the interleaved stereo int16 `buf`
/// (`orb_player::mix_preview`, `lib.rs:294-304`): each sample halved toward zero and added to L
/// and R, each saturated to int16. Returns the new position.
pub fn mix_preview(buf: &mut [i16], audio: &[i16], pos: usize) -> usize {
    let n = audio.len().saturating_sub(pos).min(buf.len() / 2);
    for (k, &s) in audio[pos..pos + n].iter().enumerate() {
        let h = s as i32 / 2;
        for c in &mut buf[2 * k..2 * k + 2] {
            *c = (*c as i32 + h).clamp(i16::MIN as i32, i16::MAX as i32) as i16;
        }
    }
    pos + n
}

/// The wasm context behind the handle the host holds.
pub struct Ctx {
    pub tables: Arc<Tables>,
    pub process: ProcessState,
    pub kit: Arc<SampleSet>,
    pub engine: Box<Engine>,
    pub doc: Doc,
    /// The wire batches pushed and not yet applied.
    queue: Vec<u8>,
    queued_seq: u32,
    pub applied_seq: u32,
    /// The int16 output of the last engine call, `2 * CALL_FRAMES` samples.
    buf: Vec<i16>,
    /// Planar f32 of the last call: L[1024] then R[1024].
    pub planar: Vec<f32>,
    preview: Preview,
    pub snap: [u8; snapshot::SIZE],
    /// Result bytes of save, info and export.
    pub io: Vec<u8>,
    /// The frames rendered so far.
    pub frames: u64,
    /// The songs made current so far (0: the initial empty song).
    pub generation: u32,
    /// The last call faulted (the PCF's division by zero).
    pub faulted: bool,
    /// The pattern arrays as `orb_patterns` last copied them.
    pub patterns: Vec<u8>,
    /// The controller values of a new empty song (`orb_defaults`).
    pub defaults: Vec<u8>,
}

impl Ctx {
    /// A context with the kit in `kit_bytes` and a new empty song (`Player::with_call_frames`).
    pub fn create(kit_bytes: &[u8]) -> Result<Box<Ctx>, u32> {
        let kit = kit::load(kit_bytes)?;
        let tables = Arc::new(Tables::new());
        let process = ProcessState::new();
        let engine = Box::new(Engine::new(
            tables.clone(),
            Devices::implemented_with(Some(kit.clone()), process.clone()),
        ));
        let defaults = snapshot::defaults(&engine);
        let mut c = Box::new(Ctx {
            tables,
            process,
            kit,
            engine,
            doc: Doc::new_song(),
            queue: Vec::new(),
            queued_seq: 0,
            applied_seq: 0,
            buf: vec![0; 2 * CALL_FRAMES],
            planar: vec![0.0; 2 * CALL_FRAMES],
            preview: Preview::default(),
            snap: [0; snapshot::SIZE],
            io: Vec::new(),
            frames: 0,
            generation: 0,
            faulted: false,
            patterns: Vec::new(),
            defaults,
        });
        snapshot::write(&mut c);
        Ok(c)
    }

    fn build(&self, song: Option<&Song>) -> Result<Box<Engine>, LoadRefused> {
        let mut e = Box::new(Engine::new(
            self.tables.clone(),
            Devices::implemented_with(Some(self.kit.clone()), self.process.clone()),
        ));
        if let Some(s) = song {
            e.load(s)?;
        }
        Ok(e)
    }

    /// Appends a wire batch; false when the queue is full.
    pub fn push(&mut self, batch: &[u8], seq: u32) -> bool {
        if self.queue.len() + batch.len() > QUEUE_LIMIT {
            return false;
        }
        self.queue.extend_from_slice(batch);
        self.queued_seq = seq;
        true
    }

    /// Applies the queued messages to the newest song, in order (`Audio::drain`).
    pub fn drain(&mut self) {
        if !self.queue.is_empty() {
            let q = std::mem::take(&mut self.queue);
            wire::apply_batch(&mut self.engine, &q);
            self.queue = q;
            self.queue.clear();
        }
        self.applied_seq = self.queued_seq;
    }

    /// One engine call (`Audio::call`): the queued commands, then 1024 frames, the preview, the
    /// planar f32 output and the snapshot.
    pub fn render_call(&mut self) {
        self.drain();
        // a faulting call writes nothing (`Engine::fault`): silence rather than the last call
        self.buf.fill(0);
        let faults = self.fault_calls();
        self.engine.render(&mut self.buf[..], CALL_FRAMES);
        self.faulted = self.fault_calls() != faults;
        if self.faulted {
            self.buf.fill(0);
        }
        if self.preview.on
            && let Some(s) = &self.preview.sample
        {
            let audio = s.audio();
            self.preview.pos = mix_preview(&mut self.buf[..], audio, self.preview.pos);
            if self.preview.pos == audio.len() {
                self.preview.on = false;
            }
        }
        self.frames += CALL_FRAMES as u64;
        let (l, r) = self.planar.split_at_mut(CALL_FRAMES);
        for i in 0..CALL_FRAMES {
            l[i] = self.buf[2 * i] as f32 / 32768.0;
            r[i] = self.buf[2 * i + 1] as f32 / 32768.0;
        }
        snapshot::write(self);
    }

    fn fault_calls(&self) -> u64 {
        self.engine.fault().map_or(0, |f| f.calls)
    }

    pub fn preview_on(&self) -> bool {
        self.preview.on
    }

    /// Makes `engine` current, after the open song's queued messages went to it and it stopped
    /// (`Link::queue_song` pushes Stop then Load; a stopped current song is replaced at once,
    /// `Audio::activate`, `lib.rs:915-925`).
    fn make_current(&mut self, engine: Box<Engine>, doc: Doc) {
        self.drain();
        self.engine.stop();
        self.engine.frame.devices.silence_all();
        self.engine = engine;
        self.doc = doc;
        self.generation += 1;
        snapshot::write(self);
    }

    /// File > New (`Link::load_new`).
    pub fn new_song(&mut self) {
        // a new empty song cannot be refused: nothing is loaded into it
        if let Ok(e) = self.build(None) {
            self.make_current(e, Doc::new_song());
        }
    }

    /// Loads a song file: 0 ok, 1 not a song, 2 cannot convert, 3 the engine's `LoadRefused`.
    /// A song that fails leaves the open one as it was.
    pub fn load(&mut self, data: &[u8]) -> i32 {
        let o = match open_bytes(data) {
            Ok(o) => o,
            Err(code) => return code,
        };
        match self.build(Some(&o.song)) {
            Ok(e) => {
                self.make_current(e, o.doc);
                0
            }
            Err(_) => 3,
        }
    }

    /// The preview player plays the startup sound (`Player::startup_sound`).
    pub fn startup_sound(&mut self) {
        self.preview.sample = Some(self.kit.shared(STARTUP_ID));
        self.preview.pos = 0;
        self.preview.on = true;
    }

    /// Saves the open song as RBS 4.2 into `io`: 0 ok, 1 the writer refused.
    pub fn save(&mut self) -> i32 {
        self.drain();
        let song = compose::compose(&mut self.engine, &self.doc);
        match song.write() {
            Ok(bytes) => {
                self.io = bytes;
                0
            }
            Err(_) => 1,
        }
    }

    /// The song information as a small JSON object into `io`.
    pub fn song_info(&mut self) {
        let i = &self.doc.info;
        let mut s = String::from("{\"version\":");
        json_str(&mut s, &i.version);
        s.push_str(",\"form\":");
        json_str(&mut s, if i.is_1x { "1x" } else { "rbs" });
        s.push_str(",\"mod\":");
        json_str(&mut s, &i.mod_name);
        s.push_str(",\"title\":");
        json_str(&mut s, &i.title);
        s.push_str(",\"text\":");
        json_str(&mut s, &i.text);
        s.push('}');
        self.io = s.into_bytes();
    }

    /// Copies the four pattern arrays out of the engine: 303#1, 303#2, 808, 909 (32 patterns of
    /// 98, 98, 194 and 194 bytes, `link.rs:186-192`).
    pub fn read_patterns(&mut self) {
        const LEN: [usize; 4] = [98, 98, 194, 194];
        self.patterns.clear();
        for (c, len) in LEN.into_iter().enumerate() {
            let mut part = vec![0u8; 32 * len];
            if let Some(p) = self.engine.frame.devices.channels[c].patterns_mut() {
                let n = part.len().min(p.len());
                part[..n].copy_from_slice(&p[..n]);
            }
            self.patterns.extend_from_slice(&part);
        }
    }
}

/// Appends `v` as a JSON string.
pub fn json_str(out: &mut String, v: &str) {
    out.push('"');
    for ch in v.chars() {
        match ch {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            c if (c as u32) < 0x20 => {
                out.push('\\');
                out.push('u');
                out.push_str(&format!("{:04x}", c as u32));
            }
            c => out.push(c),
        }
    }
    out.push('"');
}
