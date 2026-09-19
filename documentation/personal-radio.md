# Personal Radio

Personal Radio is a built-in Virtual Desktop app for a personal station. It
plays imported and/or generated music, prepares themed moderation with the
configured LLM, and optionally researches and speaks news through the active
speech output. The existing Internet Radio app remains available separately.

## Set up a station

Open **Personal Radio** from the desktop launcher and choose **New station**.
The four setup pages configure:

1. Station name, themes, spoken language, moderation style and frequency.
2. Local, generated or mixed music; weighted genres, mood, vocals, tempo and
   the desired generated share of music minutes. Enter one genre per line,
   for example `Jazz:2` and `Ambient:1`. Tempo `0` is automatic; otherwise use
   30–300 BPM. Vocal lyrics use the station language.
3. News off, every 30 minutes or every 60 minutes. Combine station themes,
   international, national and regional coverage. National news needs a
   two-letter country code; regional news also needs a region or city.
4. Start reserve, rotation, library target, storage and daily production limits.

Stop a station before changing or deleting its profile. Revisions prevent a
second window from silently overwriting an edit. The saved station settings
include a voice preview using the active TTS, with the same character allowance
as broadcasts. No microphone permission is required.

## Music and startup

The library imports browser files, desktop files or a selected desktop folder.
Folder imports are paginated and cancellable by closing their app window.
Select already generated music from the shared media registry as well. Imported
originals remain untouched; the radio stores a decoded copy. Choose an import
genre or leave it unspecified. Generated music retains its generated origin.

Supported inputs are MP3 and PCM/float WAV, one or two channels, 8–96 kHz, at
most 128 MiB and 20 minutes per file. A complete decode validates each import.
The app detects identical input files and reuses one stored copy.

The default start reserve is **30 minutes and eight different music tracks**.
A measured slow generator increases the time requirement. Running jobs and
speech never count toward the reserve. Local mode waits for sufficient imports
and does not generate music. The browser must also preload the first transition
before sounding on air. The configured start reserve may be 5–180 minutes.

The library grows toward 120 music minutes by default. Existing tracks are
reused; the station does not continually buy new music once the target is met.
Default daily limits are 20 music generations, 48 editorial requests and 30,000
TTS characters. Counters reset at midnight UTC. Failed/uncertain requests retain
their reservation because providers may still charge them. AuraGo's global
budget checks apply as well. Counts and cost estimates are not billing totals.

The radio saves originals through the existing music integration. If a produced
file cannot yet be registered or imported, its durable receipt is retried before
requesting another generation. An interrupted provider request with no known
result is not automatically replayed as that old request.

## Playback and rotation

Only one station and one browser owner run at a time. **Start** on another
desktop offers an explicit takeover. Closing or minimizing the app window keeps
music running in that desktop. The small desktop control opens the window,
pauses or stops the station. Media controls belong to Personal Radio while it
is active and return to the existing desktop media handler after stopping.

Closing the browser tab, leaving the desktop, losing the listener heartbeat or
restarting AuraGo ends that session. The server stops new production when its
three-minute listener lease expires. Reopening never silently starts playback.
OS sleep, browser suspension, network loss beyond the loaded audio and an
unusable library cannot be covered by a finite audio reserve.

The default rotation aims for 60 minutes and 20 other tracks between repeats.
It also considers play count, genre weights, favourites, origin mix and the LLM
programme. Soft mode relaxes cooldowns when necessary and shows that condition.
It never repeats a track immediately or selects a blocked track. Strict mode
waits when its rules cannot be met. Paused news is checked again before resume.

Music uses a crossfade of up to two seconds. Speech starts at a track boundary.
Two music segments plus intervening speech are reserved. Thirty-second PCM
windows and 90 seconds of lookahead per segment bound browser
audio memory (roughly 140 MiB at the maximum input rate/channels, plus transport
overhead; typical 44.1 kHz stereo uses substantially less). A limiter controls
peaks; this is not an integrated loudness-normalization service.

Library controls let you favour a track, play it less often, block it or remove
its station association. Removing a station or association preserves originals.
**Free unused storage** explicitly deletes only radio copies no station uses;
stop playback first. It never deletes shared generated originals or imports.
The storage limit covers the radio's decoded library, not the shared media
registry's originals. Each station accepts up to 1,000 associated music files.

## Moderation and news

The active LLM plans a rolling programme and writes moderation in the station
language. It receives bounded station/library data and recent scripts, without
private chat history, memories or general tools. The scheduler validates track
IDs, repetition constraints, speech length and news source references. Turning
moderation off still allows silent programme planning.

News requires configured Brave Search, the enabled web scraper and network
access. Research starts up to eight minutes before a due time while music is
playing. Deadlines are `:00` and optionally `:30` in the selected time zone.
The current and preloaded next segment remain stable, so a bulletin begins at
the next safe programme boundary after its deadline, not mid-song.

Search results locate pages; fetched page content supplies evidence. Undated,
older-than-24-hour and future-dated publications are rejected. The editor must
use known source IDs. Already aired, unchanged sources are omitted. Bulletins
expire after one news interval; missed editions are not replayed after a pause
or restart. News settings off produce no research or news synthesis.

The News tab retains recent bulletin text and clickable sources. Speech uses
the effective chat TTS; Speech Lab snapshots backend and voice together and
rejects a provider response that changes either. Complete synthesis precedes
airtime. LLM, research or TTS failure leaves available music playing and records
a sanitized operational issue through AuraGo's existing issue lifecycle.

## Implementation and verification

Data lives in `<data_dir>/personal-radio/radio.db` with validated PCM assets in
the same managed directory. Profiles, history, news and quota/job receipts
survive restart. Playback queues are transient and restart stopped.
The API is `/api/desktop/personal-radio/`, with existing desktop scopes, session
authentication, CSRF origin checks and read-only controls. It exposes public
asset IDs, never arbitrary filesystem paths for audio playback.

Focused tests cover empty/generated startup, 90 minutes of simulated rotation,
registration recovery without regeneration, quota persistence, ownership and
idempotence, corrupt audio, source validation, two news deadlines, DST, expired
speech, active Speech Lab voice and authentication. Opt-in real-shell browser
tests cover pause, close/reopen, settings and narrow Standard/Fruity windows.
OfflineAudioContext measures the production player across 120 music transitions
and 120 transitions with changing sample rates and 100 ms speech clips.

These deterministic tests do not replace a live provider/hardware acceptance
session. Deployment must package the new assets and rebuild the binary with
the generated asset flags; see [web assets](web-assets.md).
