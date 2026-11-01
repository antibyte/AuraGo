# WhatsApp Stage 3: Media and Tools Implementation Plan (Tasks 1–6, 10, 11, 12)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Media flows both ways. Voice notes are transcribed, images go to vision, documents land in the workspace, and video, stickers, locations, contact cards and quoted replies become text. Answers carry real image messages. TTS arrives as a playable voice note, or as an audio file outside voice mode. The agent can reach the owner with the `send_whatsapp` tool and with `send_notification` channel `whatsapp` (`all` included). Stage 3b gets `SendOwnerMessage`, `SendOwnerAttachment` and `ChannelWhatsApp`.

**Master plan:** `plans/2026-10-31-whatsapp-integration-plan.md`, stage 3, Tasks 1–6 and 10–12. Tasks 7–9 (camera alerts, EasyDrag, newspaper) belong to `plans/2026-10-31-whatsapp-3b-targets.md`.
**Design:** `plans/2026-10-31-whatsapp-integration-design.md`. The design is final. Where this plan departs from the design or the master plan, "Deviations" says so and keeps the intent.

**Base:** worktree `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\whatsapp-stage2`, branch `claude/whatsapp-stage2`, HEAD `b2aa0a2f4` (stage 2 finished).

**Pinned module:** `go.mau.fi/whatsmeow@v0.0.0-20261030195514-56e5ad0581ce`. Never move it. All whatsmeow citations are relative to `C:\Users\Andi\go\pkg\mod\go.mau.fi\whatsmeow@v0.0.0-20261030195514-56e5ad0581ce\`. `pb.go` means `proto/waE2E/WAWebProtobufsE2E.pb.go`.

---

## Stage ground rules (on top of the master plan's standing rules)

- **DOX before editing.** Read the root `AGENTS.md` and these guides: `internal/whatsapp/AGENTS.md`, `internal/server/AGENTS.md` ("WhatsApp Runtime Contract", ~917), `internal/agent/AGENTS.md`, `internal/tools/AGENTS.md`, `internal/security/AGENTS.md`, `internal/memory/AGENTS.md` and `ui/AGENTS.md`.
- **Commits.** Commit in the worktree with plain `git add <own paths>` and `git commit -m "<msg>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"`.
  - Child `AGENTS.md` files are gitignored. Stage them with `git add -f`.
  - Before committing the root `AGENTS.md`, check that `git ls-files -v AGENTS.md` prints `H`.
  - Before each commit, run `git diff --cached --check` and scan the diff for real phone numbers and secrets.
  - The only commits outside the worktree are Task 11's, in `../agotrain` (see Open questions).
- **GitNexus.** Run from the worktree root: `node ../../.gitnexus/run.cjs impact "<symbol>" --direction upstream --repo ../..`.
  - The index is main. Stage-2 symbols such as `handle`, `deliverText`, `Send` (broker) and `newSender` may come back UNKNOWN. Confirm those with `graft callers <symbol>` or a text search, and say so in the report.
  - Before each commit, run `node ../../.gitnexus/run.cjs detect-changes --scope all --repo .`. A `partial` or `truncated` result means run it again.
  - Warn on HIGH or CRITICAL. Targets marked **HOT** below sit in the dispatch, parse, prompt-builder or tool-registry paths, where such a warning is likely. The report states that each HOT edit only adds a case or entry for the new tool and changes no existing output.
- **Privacy.**
  - Log errors only as `maskedError(err)` (= `maskDigits(security.Scrub(err.Error()))`) and JIDs only through `maskJID`.
  - Never log message text, captions, transcripts, media bytes, file names or paths of inbound media, URLs of answer images, or vCard content.
  - Media failures log a fixed code (`too_large`, `expired`, `cancelled`, `failed`) and the kind only.
  - Fixtures use owner `+4917012345678`, stranger `+4917099998888` and own account `4917099990000`.
- **Go source.** No backslash-u escapes; use literal characters or `\x5c`. LF line endings, no BOM. Every notice prefix reuses an existing const (`budgetWarning`, `statusPrefix` in `broker.go`). Never retype the emoji.
- **Lean tests.**
  - Use the fake `waClient` only. Pipeline tests use the stage-2 `newPipelineFixture`.
  - ffmpeg tests call `exec.LookPath("ffmpeg")` and `t.Skip` without it.
  - No `t.Parallel` in tests that touch globals: `agent.SetVoiceMode`, `tools.SetWhatsAppOwnerSender`, `integrationstatus`.
  - One combined review per batch, Critical and Important findings only, no re-review rounds.
- **UI strings.** Stage 3 adds no UI string. The Python tool-bridge group reuses `config.section.whatsapp.label`/`.desc`, which all 16 `ui/lang/config/sections/*.json` already carry. So stage 3 has no locale work.

---

## Verified whatsmeow behaviour (from source; no real device)

### M1: Download API and accepted message types

- `func (cli *Client) Download(ctx context.Context, msg DownloadableMessage) ([]byte, error)` (`download.go:214`).
- `func (cli *Client) DownloadAny(ctx context.Context, msg *waE2E.Message) ([]byte, error)` (`download.go:127`) is **deprecated** (`:126`). It takes the first of Image, Video, Audio, Document, Sticker (`:131-143`).
- `DownloadableMessage` has `GetDirectPath`, `GetMediaKey`, `GetFileSHA256` and `GetFileEncSHA256` (`download.go:58-63`).
  - It is implemented by `ImageMessage`, `AudioMessage`, `VideoMessage`, `DocumentMessage`, `StickerMessage` and `StickerPackMessage` (`:82-87`).
  - The key type comes from the proto name; stickers use the image keys (`:95-106`).
- `func (cli *Client) DownloadToFile(ctx context.Context, msg DownloadableMessage, file File) error` (`download-to-file.go:41`).
  - `File` is `io.Reader`, `io.Writer`, `io.Seeker`, `io.ReaderAt`, `io.WriterAt`, `Truncate(int64) error` and `Stat() (os.FileInfo, error)` (`:28-36`).
- **Consequence:** use `DownloadToFile`. `waClient` gets `DownloadToFile(ctx, whatsmeow.DownloadableMessage, whatsmeow.File) error`.

### M2: Enforcing the 50 MB cap

- `Download` reads the whole body with `io.ReadAll` (`download.go:386-394`). There is no bound, so it is unusable for a cap.
- Every downloadable type carries `FileLength *uint64`: Video `pb.go:8536`, Image `9541`, Sticker `14387`, Audio `16754`, Document `16919`.
  - The uploader's client sets it to the plaintext length (`upload.go:74`).
  - whatsmeow never checks it on download; only the hashes are checked (`download.go:316`, `download-to-file.go:157`).
  - **It is a declared size: good for an early refusal, not a bound.**
- `DownloadToFile` behaviour:
  - It streams with `io.Copy(file, io.TeeReader(resp.Body, hasher))` (`download-to-file.go:205`).
  - It preallocates `resp.ContentLength` only when `file` is an `*os.File` (`:197-203`).
  - It retries up to 5 times on network errors (`:163-189`; `shouldRetryMediaDownload` at `download.go:327-336`).
  - It tries the next media host on any other error (`:102-118`). It returns at once on `context.Canceled`, 403, 404, 410 or a hash mismatch (`:106-112`).
  - It decrypts in place through `cbcutil.DecryptFile`, which uses `Stat`, `Read`, `WriteAt` and `Truncate` (`util/cbcutil/cbc.go:54-104`). The plaintext is never longer than the ciphertext.
- `io.Copy` uses `dst.ReadFrom` when the destination implements `io.ReaderFrom`. A wrapper that embeds `*os.File` would promote `(*os.File).ReadFrom` and **bypass a capped `Write`**.
- **Consequence (Task 1):**
  - Refuse `FileLength > 50 MB` before any network call.
  - Download into `cappedFile`. It is a named-field wrapper (no embedding, so neither `ReaderFrom` nor `WriterTo`) and not an `*os.File`, so nothing is preallocated.
  - Its `Write`/`WriteAt` fail past `50 MB + 32` bytes (AES padding plus the 10-byte MAC). The trip is sticky and cancels the download context, so no further host is tried.

### M3: Upload

- `func (cli *Client) Upload(ctx context.Context, plaintext []byte, appInfo MediaType) (UploadResponse, error)` (`upload.go:73`). It keeps plaintext and ciphertext in memory.
- `func (cli *Client) UploadReader(ctx context.Context, plaintext io.Reader, tempFile io.ReadWriteSeeker, appInfo MediaType) (UploadResponse, error)` (`upload.go:115`). It streams through a temp file that it creates and removes when `tempFile` is nil (`:118-129`).
- `UploadResponse{URL, DirectPath, Handle, ObjectID, MediaKey, FileEncSHA256, FileSHA256, FileLength, StreamingSidecar}` (`upload.go:31-44`).
  - `StreamingSidecar` is set only for `MediaVideo` (`:97-102`).
  - The media types are `MediaImage`, `MediaVideo`, `MediaAudio` and `MediaDocument` (`download.go:43-46`).
  - Non-Messenger uploads of audio use mms type `audio`, PTT included (`upload.go:254-262`).
- **Consequence:** `waClient.Upload(ctx, r io.Reader, mediaType whatsmeow.MediaType) (whatsmeow.UploadResponse, error)` wraps `UploadReader(ctx, r, nil, mediaType)`. One upload is reused for every owner.

### M4: Building media messages

- The pattern (`upload.go:52-72`): copy `URL`, `DirectPath`, `MediaKey`, `FileEncSHA256`, `FileSHA256` and `FileLength` from the response, then set `Mimetype` and `Caption`.
- Fields per type:
  - `ImageMessage`: `Caption` (`pb.go:9539`).
  - `VideoMessage`: `Caption` `8539`, `StreamingSidecar` `8549`, `Seconds` `8537`.
  - `AudioMessage`: `Seconds` `16755`, `PTT` `16756`, `Waveform` `16763`, **no caption**.
  - `DocumentMessage`: `Title` `16917`, `FileName` `16922`, `Caption` `16934`.
- `SendMessage` derives the stanza media type itself: `PTT` gives `ptt`, otherwise `audio` (`send.go:981-986`). Nothing extra is needed.
- whatsmeow computes neither `Seconds` nor `Waveform`.
- **Consequences:**
  - A voice note is `AudioMessage{PTT: true, Mimetype: "audio/ogg; codecs=opus", Seconds}`.
  - `Seconds` comes from the Ogg Opus granule position (RFC 7845: granules count 48 kHz samples; minus the OpusHead pre-skip).
  - No waveform (Task 12 records how the phone shows it).

### M5: Captions

- The source enforces **no caption limit**. A search outside the generated protos finds no caption length check.
- `UnwrapRaw` peels `DocumentWithCaptionMessage` (`types/events/events.go:413-415`), so document captions arrive in `DocumentMessage.Caption`.
- **Consequence:** the 1024-rune limit is the master plan's choice (Telegram parity, `internal/telegram/send_photo.go:24`).

### M6: Location, contacts and quotes

- `LocationMessage` (`pb.go:17313`): `DegreesLatitude`, `DegreesLongitude`, `Name`, `Address`.
- `LiveLocationMessage` (`14615`): latitude, longitude and `Caption` (`14622`).
- `ContactMessage` (`17445`): `DisplayName`, `Vcard`. `ContactsArrayMessage` (`15123`): `DisplayName`, `Contacts []*ContactMessage`.
- `ContextInfo` (`9811`): `StanzaID` `9813`, `Participant` `9814`, `QuotedMessage *Message` `9815`, with getters `9913`/`9927`.
- **Every content type has its own `ContextInfo`:** Video `8548`, Image `9550`, LiveLocation `14626`, ContactsArray `15127`, Audio `16761`, Document `16931`, Location `17326`, Contact `17449`. Stage 2 reads only `ExtendedTextMessage.ContextInfo`.
- **Consequence:** `contextInfoOf(m)` reads the first non-nil content type. Forwarded detection and the quoted reply use it, so a forwarded image is marked as forwarded.

### M7: Errors and expired media

- Sentinels (`errors.go:133-146`): `ErrMediaDownloadFailedWith403/404/410`, `ErrNoURLPresent`, `ErrInvalidMediaHMAC`, `ErrInvalidMediaSHA256`, `ErrUnknownMediaType`, `ErrTooShortFile`, `ErrFileLengthMismatch`.
- Expired media returns 404 or 410. Re-requesting it (`mediaretry.go`) is out of scope per the design.

---

## Deviations from the master plan

1. **The question source landed in stage 2** (stage-2 deviation 5: `tools.QuestionSourceWhatsApp`, `internal/tools/question_user.go:31-32`). Task 6 keeps `ChannelWhatsApp`, the enum and the descriptions.
2. **Voice output is re-enabled in Task 3, together with voice notes** (stage-2 deviation 24). `agent.WithVoiceOutputSuppressed` (`pipeline.go:313`) is removed and `VoiceOutputActive: agent.GetVoiceMode()` replaces `false` (`pipeline.go:291`), in one commit.
3. **Broker audio and YouTube** (stage-2 deviation 11) arrive in Task 3. Only the `tts` action sets `file_path` (`internal/agent/agent_loop_helpers.go:1562-1583`). `send_audio` and `generate_music` events carry web paths only, so, as in Telegram (`internal/telegram/bot.go:955`), they are not sent as native files.
4. **Markdown images go out as image messages** (Task 3). The stage-2 `[Image: …]` line stays only as the fallback for an image that could not be staged or sent.
5. **Media-only messages start turns** (Task 2). `TestPipelineStartsNoTurnWithoutTopLevelText` becomes `TestPipelineStartsNoTurnWithoutContent`. The "image with caption" row goes; the two nested-wrapper rows stay, because content is read from the top level only.
6. **Download via `DownloadToFile` and `cappedFile`** (M1, M2), not `Download`/`DownloadAny`. The cap has two layers: the declared `FileLength`, then the capped writer.
7. **Upload via `UploadReader`** (M3): a 50 MB file is never held twice in memory.
8. **Documents** (Task 2). `media.SaveAttachmentContext` takes a URL, and whatsmeow decrypts locally. A sibling `media.SaveAttachmentReaderContext` shares the extracted naming policy (`attachmentFileName`). It creates with `O_EXCL` and suffixes, so two files with the same name in the same second do not overwrite each other.
9. **The single sender covers media** (Task 3). `sender.deliverMedia` in `send.go`; the per-message body moves into `sendOne`. `TestOnlyTheSenderCallsSendMessage` stays unchanged.
10. **The owner API is wider than `SendOwnerMessage`** (Task 4).
    - `SendOwnerAttachment(ctx, caption, OwnerAttachment{Name, Data})` is for 3b. The camera photo and the newspaper PDF are in-memory bytes outside the workspace, so they cannot pass `OpenOutgoingAttachment`.
    - `OwnerSendReady()` means "a linked generation is running".
    - Readonly is checked against the live config on every call.
11. **The import cycle is broken by a hook in `internal/tools`** (`whatsapp_hook.go`, Task 4). The server publishes it (`internal/server/whatsapp_tool.go`) once, in `configureWhatsAppBot`, and withdraws it in `stopWhatsApp`. The adapter reads `s.whatsAppBot.Load()` on every call, so it never points at a stopped Bot. These three files are not in the master table.
12. **The tool flag has a config part and a runtime part** (Task 5), the `LocalWikipediaEnabled`/`RetroGamesEnabled` pattern (`tooling_policy.go:519-521`).
    - `buildToolFlagsFromConfig`: `Enabled && !ReadOnly`.
    - `resolveToolFeatureState`: `&& tools.WhatsAppAvailable()`.
    - `ToolFeatureFlags.WhatsAppEnabled` (`native_tools.go:262`) and `decodeSendWhatsAppArgs` (`tool_args_extended.go`) are needed but not listed in the master plan.
    - `NotificationEnabled` also becomes true when WhatsApp is the only channel.
13. **No edit in `internal/prompts/tool_manual_bindings.go`.** The manual file exists, and the default `ToolManualID` maps it; `TestBuiltinManualBindingsAreValidOrExplicitlyExplained` proves it. The manual's frontmatter `conditions: ["whatsapp_enabled"]` needs a row in `TestPromptConditionsCoverEmbeddedFrontmatter`.
14. **The `send_whatsapp` schema is `{message, file_path}`** (design). It has no `title` or `priority`, unlike `send_telegram`.
15. **Captions** (M5): at most 1024 runes after formatting.
    - Tool text that does not fit is sent first as its own message (`TextSent`, like Telegram's `text_sent`, `internal/tools/telegram_document.go:88-90`).
    - Audio has no caption field, so its text always goes first.
    - Answer-image alt text is cut to fit, with `…`.
16. **Self-chat prefix on captions.** Media without a caption (voice notes) carries no prefix.
17. **Non-PTT audio is transcribed like PTT** (master plan: "voice and PTT → MP3 → `tools.TranscribeAudio`").
18. **Voice failures produce a notice.** Telegram returns silently (`bot.go:340-389`). WhatsApp sends `budgetWarning + "I could not process that voice message."` so the owner is not left waiting.
19. **Third-party content** also covers contact cards (no HumanInput, no answer to a pending question). Forwarded detection reads `contextInfoOf` (M6).
20. **More reuse of `internal/telegram`** (allowed by the design): `ConvertOggToMp3Context`, `AnalyzeImageContext`, `AppendMissingYouTubeLinks`, `CapturedAudio`, `CapturedYouTubeVideo`. The `internal/whatsapp/AGENTS.md` note "imports telegram only for ForwardedContentMarker" is updated.
21. **TTS confinement.** A captured `file_path` must resolve inside `<data_dir>/tts` through `os.OpenRoot`. Telegram opens any path.
22. **Prompt texts.** The parity sweep excluded `*.md`. Stage 3 also updates:
    - `prompts/rules.md:42-43`;
    - `prompts/tools_manuals/{send_image,send_youtube_video,tts,send_notification,question_user}.md`.
23. **Docs.** `documentation/manuals/{en,de}/22-internal-tools.md` and `documentation/guardian_llm_system.md` (§6) get `send_whatsapp` (root `AGENTS.md:127`). The operator docs stay in stage 4.
24. **ffmpeg bitrate.** The design fixes libopus, mono, 16 kHz, voip; the plan adds `-b:a 24k`.
25. **Training** (Task 11).
    - Root `AGENTS.md:129` says schema-projection changes hand-edit the manifests and update their hash, without bootstrapping again (the README also records `--bootstrap-contracts` as broken).
    - The validator count `EXPECTED_TOOLS` goes from 241 to 242.
    - From the worktree, the path is `../../../agotrain/training`, not `../agotrain`.
26. **Discovery eval.** Q101 (en) and Q102 (de) are added; the doc comment count changes from 100 to 102. The floors stay (they are absolute ratchets).
27. **Masked owner-send errors.** whatsmeow's `ErrRecipientNotOnWhatsApp` text names the owner JID (`send.go:341-365`). Every error `SendOwnerMessage` returns has a masked `Error()` and keeps `errors.Is`.
28. **Owner API gate on the live config.** `ownerSendAllowed` reads `whatsapp.readonly` from the live configuration on every owner send, and fails closed with `ErrNotConnected` when there is no snapshot, egg mode is on or the generation no longer matches the live `whatsapp` section. Read-only is checked before the generation match, so the answer is `ErrReadOnly` whenever read-only is on.
29. **ffmpeg hardening.** Every conversion of a received file runs with `-protocol_whitelist file,pipe`, a `-format_whitelist` of audio and video demuxers, and `-fs` at the 50 MB limit. An output at or above the limit is `ErrMediaTooLarge`, never a truncated success.
30. **Inbound audio to MP3.** Voice messages and audio files become mono 16 kHz MP3 at 64 kbit/s for transcription, which is enough for speech. WhatsApp uses its own converter (`convertToMP3`); Telegram's `ConvertOggToMp3Context` is unchanged.
31. **Sender file names.** A received document's name goes through `media.SanitizeFileName` (reserved characters, NTFS streams, Cc and Cf runes, trailing dots and spaces, a 100-byte base cap). `media.SaveAttachmentContext` keeps its naming byte for byte for Discord and Telegram.
32. **Forwarded, quoted and commands.** Forwarded and quoted detection is an OR across every content type's ContextInfo, not the first one. A slash command runs only from a message that carries one content type, and a message without the owner's own words gets no HumanInput.
33. **Answer images.** `/files/` answer images open through `tools.OpenOutgoingAttachment`, and at most five images of one reply are fetched; the rest become `[Image: alt]` lines. Fetched copies are removed after sending.
34. **Captured media.** The broker keeps at most five audio and five video captures per run. After a failed agent run the captured media is dropped, with one fixed log line.
35. **Owner API text and errors.** An owner send ends with its generation's context, and `TextSent` is true once any part of the text reached an owner, including a long text whose first parts went out before a later part failed. Errors stay masked while `errors.Is` still works.
36. **Notification limit.** A notification whose raw title, newline and message exceed 4000 runes is refused rather than split (counted before `Scrub`, which can lengthen text); `tools.WhatsAppMaxMessageRunes` is pinned to the chunk limit by a test. The title stays on one bold line.
37. **Generic flows node.** `send_whatsapp` stays in `genericExcluded` until stage 3b brings the curated `notify.whatsapp` node, because a generic node has no duplicate-text guard on a retry.
38. **Configured but not linked.** While no linked generation runs, `send_whatsapp` answers `needs_setup` (code `runtime_dependency_unavailable`, "WhatsApp is not linked or not running"), not "disabled by the administrator".
39. **TTS scrub.** The `tts` action scrubs its text with `security.Scrub` in `dispatch_platform.go` before synthesis, so a registered secret is never spoken or sent as a voice note.
40. **`deliverTextParts`.** `sender.deliverTextParts` is `deliverText` that also reports whether any chunk reached WhatsApp. The owner API uses it for `TextSent`, so a caller that retries a partly delivered long text knows the first parts already went out.

## Dispatch grouping (lean execution, dependency order)

| Batch | Tasks | Agents | Review | Why grouped |
| --- | --- | --- | --- | --- |
| B1 | 1, 2 | maker-agent | **security-agent** | Download cap, inbound third-party marking, attachment names. Task 2 needs the primitives. |
| B2 | 3, 4 | maker-agent | **security-agent** | Outbound media, single sender, TTS confinement, owner API: readonly gate, `OpenOutgoingAttachment` only, owner-only, masked errors, the hook. |
| B3 | 5 (commits 5a, 5b), 6 | maker-agent for code; docs-agent for 5b manual/docs | **security-agent** | Tool gates, Guardian params, action policy, directive shape, notification readonly and `all`. |
| B4 | 10 + stage DOX | maker-agent for code; docs-agent for DOX | one combined review | Agent identity and contracts. |
| B5 | 11 | maker-agent (in `../agotrain`); test-agent runs `--check` and the validator | combined light review | Cross-repo reconciliation after B3 is committed. |
| — | 12 | the user | — | Real-phone checklist. |

Each maker dispatch runs its task's test commands before committing. test-agent runs the verification sweep after B4 and again after B5.

## Open questions for the user

1. **Commits in `../agotrain`.** Task 11 must regenerate and commit in the separate repository `C:\Users\Andi\Documents\repo\agotrain` (`documentation/training.md:38-44`), then pin its commit in the worktree's `.github/agotrain-revision`. Proposal: a local agotrain branch `claude/whatsapp-send-tool`, no push. If not allowed, Task 11 stops after the reconciliation and leaves the agotrain edits uncommitted, and the pin stays unchanged.

---

## Task 1: Media primitives

**Files:**
- create `internal/whatsapp/media.go` and `media_test.go`;
- modify `internal/whatsapp/client.go`, `fake_test.go` and `errors.go`;
- modify `Dockerfile:54` and `install.sh:2155` (comments).

**`waClient` additions** (`client.go`, after `LastSuccessfulConnect`; import `io`). Update the doc comment to "stage 3 adds media download and upload".

```go
// DownloadToFile writes the decrypted attachment of msg into file (download-to-file.go:41).
DownloadToFile(ctx context.Context, msg whatsmeow.DownloadableMessage, file whatsmeow.File) error
// Upload encrypts r and uploads it through UploadReader (upload.go:115).
Upload(ctx context.Context, r io.Reader, mediaType whatsmeow.MediaType) (whatsmeow.UploadResponse, error)
```

**Adapter:**
- `DownloadToFile` → `c.cli.DownloadToFile(ctx, msg, file)`;
- `Upload` → `c.cli.UploadReader(ctx, r, nil, mediaType)`.

**Fake** (`fake_test.go`):
- `media map[string][]byte` keyed by direct path, plus `downloadErr error` and `downloads []string`. `DownloadToFile` honours ctx, records the path and writes the bytes through `file.Write` in 32 KiB chunks.
- `uploads []uploadCall{mediaType whatsmeow.MediaType; data []byte}` and `uploadErr error`. `Upload` reads everything and returns `UploadResponse{URL: "https://mmg.example/u", DirectPath: "/v/fake", MediaKey: []byte{1}, FileSHA256: []byte{2}, FileEncSHA256: []byte{3}, FileLength: uint64(len(data))}` (plus `StreamingSidecar: []byte{4}` for video).
- `sentMessage` gains `msg *waE2E.Message`, so tests can inspect media messages.

**Errors** (`errors.go`):

```go
ErrMediaTooLarge = errors.New("the file is larger than 50 MB, the WhatsApp limit")
ErrReadOnly      = errors.New("whatsapp is in read-only mode") // used from Task 4
```

**`media.go`:**

```go
const (
	maxMediaBytes   = 50 << 20
	maxCaptionRunes = 1024
	voiceNoteMIME   = "audio/ogg; codecs=opus"
	ffmpegTimeout   = 60 * time.Second // Telegram's ceiling (internal/telegram/voice.go:92)
)

type mediaKind int

const (
	mediaDocument mediaKind = iota
	mediaImage
	mediaVideo
	mediaAudio
	mediaVoiceNote
)

func (k mediaKind) String() string                // "document", "image", "video", "audio", "voice"
func (k mediaKind) uploadType() whatsmeow.MediaType // voice and audio → MediaAudio

// mediaKindFor picks the message type from the name and the first bytes. A file becomes an
// image, video or audio message only when its content matches; anything else is a document.
func mediaKindFor(name string, head []byte) (kind mediaKind, mimeType string)
```

| Content (`http.DetectContentType(head)`) and extension | Kind | Mimetype |
| --- | --- | --- |
| `image/jpeg` or `image/png`, any extension | image | the sniffed type |
| `video/mp4` and `.mp4`/`.m4v` | video | `video/mp4` |
| `.mp3` with `audio/mpeg` or `application/octet-stream` | audio | `audio/mpeg` |
| `.m4a` (`video/mp4` sniff) / `.aac` | audio | `audio/mp4` / `audio/aac` |
| `.ogg`/`.opus` with `application/ogg` | audio | `audio/ogg` |
| everything else (GIF, WebP, WAV, mismatches) | document | `mime.TypeByExtension` without parameters, else the sniffed type, else `application/octet-stream` |

**Download with cap.** This is the subtle part; implement exactly:

```go
type downloadable interface {
	whatsmeow.DownloadableMessage
	GetFileLength() uint64
}

// downloadMedia returns a temp file (mode 0600) holding the decrypted attachment, at offset 0.
// The caller closes and removes it. The declared FileLength is refused first, then cappedFile
// bounds what really arrives (FileLength is sender-declared, download.go:316).
func downloadMedia(ctx context.Context, client waClient, msg downloadable) (*os.File, error) {
	return downloadMediaLimit(ctx, client, msg, maxMediaBytes)
}

func downloadMediaLimit(ctx context.Context, client waClient, msg downloadable, limit int64) (*os.File, error) {
	if msg.GetFileLength() > uint64(limit) {
		return nil, ErrMediaTooLarge
	}
	f, err := os.CreateTemp("", "aurago-whatsapp-media-*")
	if err != nil {
		return nil, err
	}
	dctx, cancel := context.WithCancel(ctx)
	defer cancel()
	capped := &cappedFile{f: f, limit: limit + 32, cancel: cancel} // padding block + 10-byte MAC
	err = client.DownloadToFile(dctx, msg, capped)
	if err == nil {
		if info, statErr := f.Stat(); statErr != nil || info.Size() > limit {
			err = ErrMediaTooLarge
		}
	}
	if capped.over.Load() {
		err = ErrMediaTooLarge
	}
	if err == nil {
		_, err = f.Seek(0, io.SeekStart)
	}
	if err != nil {
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
		return nil, err
	}
	return f, nil
}

// cappedFile is the whatsmeow.File a download writes into. It holds *os.File in a named field:
// embedding would promote ReadFrom, and io.Copy (download-to-file.go:205) would then write
// around the cap. It is not an *os.File, so whatsmeow preallocates nothing (:197-203). The first
// write past limit trips it for good and cancels the download, so no other host is tried (:111).
type cappedFile struct {
	f      *os.File
	limit  int64
	cancel context.CancelFunc
	over   atomic.Bool
}

func (c *cappedFile) Write(p []byte) (int, error) {
	pos, err := c.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	return c.WriteAt(p, pos) // WriteAt checks the limit; then advance the offset
}

func (c *cappedFile) WriteAt(p []byte, off int64) (int, error) {
	if c.over.Load() || off+int64(len(p)) > c.limit {
		c.over.Store(true)
		c.cancel()
		return 0, ErrMediaTooLarge
	}
	return c.f.WriteAt(p, off)
}

// Read, Seek, ReadAt, Truncate and Stat delegate to c.f, one line each.
```

`Write` must advance the file offset after `WriteAt`, either with `c.f.Seek(pos+int64(n), io.SeekStart)` or by checking first and then calling `c.f.Write(p)`. The checked-then-`c.f.Write` form is preferred.

**Error code for logs:** `mediaErrorCode(err) string` returns:
- `too_large` for `ErrMediaTooLarge`;
- `expired` for `ErrMediaDownloadFailedWith404` or `410`;
- `cancelled` for a context error;
- `failed` otherwise.

**Upload and messages:**

```go
// mediaUploadTimeout mirrors telegramUploadTimeout (internal/tools/telegram_document.go:64):
// a minute plus a second per 128 KiB, at most ten minutes.
func mediaUploadTimeout(size int64) time.Duration

// uploadMedia uploads r under kind's key type within mediaUploadTimeout(size). A failure
// that is not a cancellation counts MarkWhatsAppDeliveryFailure once: the file never arrives.
func uploadMedia(ctx context.Context, client waClient, r io.Reader, size int64, kind mediaKind) (whatsmeow.UploadResponse, error)

type mediaMeta struct {
	MIME, FileName, Caption string
	Seconds                 uint32
}

// mediaMessage builds the message for kind from the upload (upload.go:52-72). Voice notes are
// PTT with voiceNoteMIME and no caption; video carries StreamingSidecar; a document carries
// FileName (documentName) and Title = FileName. Empty strings and zero Seconds stay nil.
func mediaMessage(kind mediaKind, up whatsmeow.UploadResponse, meta mediaMeta) *waE2E.Message

func documentName(name string) string  // copy of telegramDocumentName (internal/tools/telegram_document.go:392-405)
func boundRunes(s string, n int) string // cut to n runes including a trailing "…" (boundPhotoCaption, send_photo.go:67-73)
```

**Voice note conversion:**

```go
func voiceNoteArgs(in, out string) []string {
	return []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-i", in,
		"-vn", "-map_metadata", "-1",
		"-c:a", "libopus", "-application", "voip", "-ac", "1", "-ar", "16000", "-b:a", "24k",
		"-f", "ogg", out}
}

// convertToVoiceNote runs ffmpeg with voiceNoteArgs within ffmpegTimeout and returns the Ogg Opus
// bytes and their seconds. ffmpeg output is never logged (it can echo the input path).
func convertToVoiceNote(ctx context.Context, inputPath string) ([]byte, uint32, error)

// oggOpusSeconds is the stream duration: the last page's granule position counts 48 kHz
// samples (RFC 7845) minus the OpusHead pre-skip (bytes 10-11), rounded up. 0 = unknown;
// display only.
func oggOpusSeconds(data []byte) uint32 {
	head := bytes.Index(data, []byte("OpusHead"))
	last := bytes.LastIndex(data, []byte("OggS"))
	if head < 0 || last < 0 || len(data) < last+14 || len(data) < head+12 {
		return 0
	}
	granule := int64(binary.LittleEndian.Uint64(data[last+6 : last+14]))
	preSkip := int64(binary.LittleEndian.Uint16(data[head+10 : head+12]))
	if granule <= preSkip {
		return 0
	}
	return uint32((granule - preSkip + 47999) / 48000)
}
```

**Comments:**
- `Dockerfile:54` becomes `# We add ffmpeg (needed for Telegram and WhatsApp voice conversion).`
- `install.sh:2155` becomes `warn "Skipping ffmpeg. Telegram and WhatsApp voice messages will not work."`

**Mirror:**
- `downloadFile`'s limit-plus-one read (`internal/telegram/bot.go:1027-1081`);
- `convertToOGGContext` (`bot.go:884-915`), with WhatsApp's own arguments.

**Tests** (`media_test.go`):
- `TestDownloadMediaRefusesADeclaredSizeOverTheCap`: `FileLength = maxMediaBytes+1` → `ErrMediaTooLarge`, and `fake.downloads` stays empty.
- `TestDownloadMediaStopsWritingAtTheCap`: `downloadMediaLimit` with `limit = 1024`, a small declared size and 4096 fake bytes → `ErrMediaTooLarge`. The temp file no longer exists; the fake reports a cancelled ctx after the trip.
- `TestCappedFileHidesReaderFromAndWriterTo`: `any(&cappedFile{})` implements neither `io.ReaderFrom` nor `io.WriterTo`, but does implement `whatsmeow.File`.
- `TestMediaKindForMapsContentToMessageTypes`: one row per table line. A `.png` whose content is text becomes a document; `photo.jpg` with PNG bytes becomes `image/png`.
- `TestMediaMessageCopiesTheUploadAndMarksVoiceNotes`:
  - voice: `PTT == true`, `Mimetype == "audio/ogg; codecs=opus"`, `Seconds` set, no caption;
  - video: carries `StreamingSidecar`;
  - document: has `FileName`;
  - every kind copies URL, DirectPath, MediaKey, both hashes and FileLength.
- `TestVoiceNoteArgsAreOpusMonoSixteenKilohertzVoip`: the exact slice.
- `TestOggOpusSecondsReadsTheLastGranule`: a synthetic stream (OpusHead with pre-skip 312, last page granule `3*48000+312`) → 3; garbage → 0.
- `TestConvertToVoiceNoteWithFFmpeg` (skipped without ffmpeg):
  - build `in.wav` with `ffmpeg -f lavfi -i sine=frequency=440:duration=1`;
  - the result starts with `OggS`;
  - OpusHead channel count (byte 9) is 1 and input rate (bytes 12-15, little-endian) is 16000;
  - seconds is 1.

`CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestDownloadMedia|TestCappedFile|TestMediaKindFor|TestMediaMessage|TestVoiceNoteArgs|TestOggOpusSeconds|TestConvertToVoiceNote)' -count=1`
`CGO_ENABLED=0 go test ./internal/whatsapp/ -count=1` (the fake still satisfies the interface)

**GitNexus impact:** `waClient` (interface; implementations `whatsmeowClient` and `fakeClient`).
**Commit:** `feat(whatsapp): add media download cap, upload, message types and voice-note conversion`

## Task 2: Inbound media

**Files:**
- create `internal/whatsapp/inbound_media.go` and `inbound_media_test.go`;
- modify `internal/whatsapp/pipeline.go` and `pipeline_test.go`;
- modify `internal/media/media.go` and add `internal/media/attachment_reader_test.go`.

**`internal/media`.** Extract the naming policy of `SaveAttachmentContext` (`media.go:91-102`) unchanged:

```go
// attachmentFileName is the stored name of an attachment: timestamp, underscore and the base
// name with spaces as underscores and ".." removed; "file.bin" when nothing is left.
func attachmentFileName(original string, now time.Time) string

// SaveAttachmentReaderContext stores r under destDir with attachmentFileName, like
// SaveAttachmentContext does for a URL, with the same 100 MB limit. It never overwrites: an
// existing name gets "_1" … "_9" before the extension (O_EXCL), then fails.
func SaveAttachmentReaderContext(ctx context.Context, r io.Reader, originalFilename, destDir string) (string, error)
```

**Pipeline seams** (new `pipeline` fields; nil means production, as `agentLoop` already does):

```go
transcribe   func(ctx context.Context, name string, data []byte, cfg *config.Config) (string, error) // tools.TranscribeAudio, dropping the score
analyzeImage func(ctx context.Context, path string, cfg *config.Config) (string, error)          // telegram.AnalyzeImageContext
toMP3        func(ctx context.Context, in, out string) error                                     // telegram.ConvertOggToMp3Context
```

**`inbound_media.go`:**

```go
// inboundContent is what one owner message contributes to the turn.
type inboundContent struct {
	CommandText string // top-level text only: the only text a slash command is read from
	Input       string // what the model reads (before the forwarded marker and isolation)
	Routing     string // pinned before image analysis and notes (Telegram bot.go:398-400)
	OwnerText   string // HumanInput text
	ThirdParty  bool   // forwarded or a contact card: no HumanInput, command or question answer
	Marker      string // telegram.ForwardedContentMarker(...) when forwarded
	Kind        string // messageKind, for logs
	Notice      string // a failure the owner must see; "" otherwise
}

// readContent reads the unwrapped top level only (a nested wrapper stays "other").
func (p *pipeline) readContent(ctx context.Context, cfg *config.Config, wa waClient, m *waE2E.Message) inboundContent

// contextInfoOf returns the ContextInfo of the first content type present: ExtendedText,
// Image, Video, Audio, Document, Sticker, Location, LiveLocation, Contact, ContactsArray (M6).
func contextInfoOf(m *waE2E.Message) *waE2E.ContextInfo
```

`readContent` order (Telegram parity, `bot.go:324-500`). Every download goes through `downloadMedia`, and a failure logs `"kind"` and `mediaErrorCode` only.

1. `CommandText = messageText(m)`, and `Input` starts as that text. A caption (Image, Video, Document, LiveLocation) is joined with `"\n"`, as at `bot.go:324-331`.
2. **Audio (PTT or not):**
   - Download the file and run `toMP3(tmp, tmp+".mp3")`, then `transcribe(ctx, "whatsapp_voice.mp3", mp3Bytes, cfg)`.
   - On success, `Input` becomes the transcript.
   - On any failure, `Notice = voiceFailedText` and `Input = ""`.
   - Remove both temp files.
3. `Routing = Input` and `OwnerText = Input`.
4. **Image:**
   - Download to a temp file with the sniffed extension, then `analyzeImage`.
   - On error, use `"[Error analyzing image]"`, or `"[Image analysis unavailable: " + tools.VisionPublicURLRequiredMessage + "]"` for `tools.ErrVisionPublicURLRequired`.
   - `Input = "[USER SENT AN IMAGE]\n" + analysis`, plus `"\n\n[USER CAPTION/TEXT]:\n" + caption` when there is one (`bot.go:450-454`).
5. **Document:**
   - Download, then `media.SaveAttachmentReaderContext(ctx, f, doc.GetFileName(), filepath.Join(cfg.Directories.WorkspaceDir, "attachments"))`.
   - Append `"[DATEI ANGEHÄNGT]: agent_workspace/workdir/attachments/<base>"`, with `" (<mimetype>)"` when it has one (`bot.go:487-496`).
   - Too large: append `"[File not saved: larger than 50 MB]"`. Another failure: append `"[File could not be downloaded]"`.
6. **Video:** `"[Video received; videos are not processed]"`, then `"\n" + Input` if non-empty. **Sticker:** `"[Sticker received]"`. Neither is downloaded.
7. **Location:**
   - Format `"[Location: %s, %s (%s)]"` with `strconv.FormatFloat(v, 'f', 6, 64)`.
   - The name falls back to the address and is folded to one line with `boundRunes(…, 120)`. With neither, the parentheses go.
   - A live location gives `"[Live location: %s, %s]"`.
8. **Contact or contact array:**
   - Header `"[Contact card shared; the card is third-party data]"`, then each card (at most 5, each `boundRunes(vcard, 2000)`) in a code fence.
   - The fence is `max(3, longest backtick run + 1)` backticks with info string `vcard`.
   - `ThirdParty = true`.
9. **Quoted reply:** when `contextInfoOf(m).GetQuotedMessage() != nil` and `Input != ""`:
   - The quote is `messageText(q)`, else its caption, else `"[" + messageKind(q) + "]"`.
   - It is folded to one line and cut with `boundRunes(…, 200)`.
   - `Input = "[Reply to: " + quote + "]\n" + Input`.
10. **Forwarded:** when `ci.GetIsForwarded() || ci.GetForwardingScore() > 0 || ci.GetForwardedNewsletterMessageInfo() != nil` on `contextInfoOf(m)`, set `Marker` and `ThirdParty = true`.

`const voiceFailedText = budgetWarning + "I could not process that voice message."`

**`pipeline.handle` changes** (stage-2 steps 4-8 rewired; the order of steps 1-3 is unchanged, so media is processed after `RegisterSessionRun` and before the question and the turn lock, as `internal/memory/AGENTS.md:68-70` requires):
- `c := p.readContent(ctx, cfg, wa, in.Message)`.
- When `c.Notice != ""`: deliver it to `owner` and return.
- When `c.Input == ""`: log Info `"WhatsApp message kind not handled", "kind", c.Kind` and return.
- `humanInput := memory.NewHumanInputEvent(c.OwnerText, sessionID, "whatsapp:"+in.Chat.User+":"+string(in.ID), in.Timestamp)`.
- When `c.ThirdParty`: `humanInput = nil`.
- When `c.Marker != ""`: `inputText = c.Marker + "\n" + c.Input` and `routingText = forwardedRoutingText(c.Marker, c.Routing)`.
- The pending question is answered only when `!c.ThirdParty`.
- A slash command runs only when `!c.ThirdParty && strings.HasPrefix(c.CommandText, "/")`.
- Everything after (Guardian, isolation, history) is unchanged.

**Tests** (fake media bytes; stubbed seams; workspace `t.TempDir()`):
- `TestSaveAttachmentReaderSanitizesAndNeverOverwrites` (media): `"../../evil name.pdf"` gives a file inside destDir whose name has no `..` and has `_`. A second save in the same second gets `_1`.
- `TestPipelineTranscribesVoiceNotes`:
  - `AudioMessage{PTT: true}` with the transcript "turn on the light" → one agent call;
  - `HumanInput.TurnID` matches "turn on the light";
  - the stored user text is `IsolateExternalData("turn on the light")`;
  - the transcriber got the name `whatsapp_voice.mp3`;
  - the temp files are gone.
- `TestPipelineSendsImagesToVisionWithTheUserPrefix`: the caption "what is this" gives the exact `[USER SENT AN IMAGE]` block, `TaskRoutingText == "what is this"`, and no caption or path in the logs.
- `TestPipelineSavesDocumentsAsAttachments`: the file exists under `attachments/`, and the stored text holds the `[DATEI ANGEHÄNGT]` note with `(application/pdf)`.
- `TestPipelineRefusesOversizedMediaBeforeDownloading`: `FileLength = maxMediaBytes+1`.
  - Voice → no download, the owner gets `voiceFailedText`, no agent call.
  - Document → no download, and the note says `[File not saved: larger than 50 MB]`.
- `TestPipelineDescribesVideoStickerLocationAndContacts`, as a table:
  - video plus caption;
  - sticker;
  - location `52.520008, 13.404954 (Brandenburger Tor)`;
  - a contact gives the fenced vCard, `HumanInput == nil`, and a vCard holding three backticks gets a four-backtick fence;
  - no `DownloadToFile` call for video or sticker.
- `TestPipelineQuotesRepliesInAShortPrefix`: "yes" quoting a 300-rune text → the input starts with `[Reply to: ` + 199 runes + `…]`, and `TaskRoutingText == "yes"`.
- `TestPipelineMarksForwardedMediaAsThirdParty`: `ImageMessage{ContextInfo: {IsForwarded: true}}` → the stored text starts inside the isolation with `[Forwarded message; third-party content]` and `HumanInput == nil`.
- `TestPipelineStartsNoTurnWithoutContent`: renamed from `TestPipelineStartsNoTurnWithoutTopLevelText`; the image row is dropped (deviation 5).

`CGO_ENABLED=0 go test ./internal/media/ -run '(TestSaveAttachmentReader)' -count=1`
`CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestPipeline)' -count=1`
`CGO_ENABLED=0 go test ./internal/telegram/ -count=1` (`SaveAttachmentContext` refactor guard)

**GitNexus impact:**
- `handle` (`internal/whatsapp/pipeline.go`; caller `Bot.handleMessage`);
- `SaveAttachmentContext` (callers include `telegram.processUpdate`; MEDIUM expected).

**Commit:** `feat(whatsapp): read voice notes, images, documents, locations, contacts and quotes`

## Task 3: Outbound media

**Files:**
- modify `internal/whatsapp/send.go`, `broker.go`, `pipeline.go` and their tests;
- create `internal/whatsapp/outbound_media.go` and `outbound_media_test.go`;
- modify `internal/agent/native_tools_core.go:273,396`;
- modify `prompts/tools_manuals/send_image.md:3`, `send_youtube_video.md:9` and `tts.md:34`.

**Sender** (`send.go`):

```go
// sendOne paces, registers the ID before the call, sends and accounts one message; the caller
// holds the queue slot. deliverText's per-chunk body moves here unchanged.
func (s *sender) sendOne(ctx context.Context, client waClient, to types.JID, msg *waE2E.Message, what string) error

// deliverMedia sends one media message to one owner through the same guard, slot, bucket,
// echo registration and failure accounting as deliverText.
func (s *sender) deliverMedia(ctx context.Context, to types.JID, msg *waE2E.Message, what string) error

// caption formats markdown like deliverText (Scrub, toWhatsApp, self-chat prefix). cut=true
// trims it to maxCaptionRunes with boundRunes; fits reports whether the result is within it.
func (s *sender) caption(markdown string, cut bool) (text string, fits bool)
```

**Broker** (`broker.go`). Add `audio []telegram.CapturedAudio` and `videos []telegram.CapturedYouTubeVideo`, guarded by `b.mu`.
- `Send("audio", json)` appends when `file_path` is non-empty.
- `Send("youtube_video", json)` appends when `url` is non-empty.
- The JSON shapes are those of `bot.go:948-978`.
- `func (b *whatsAppBroker) captured() ([]telegram.CapturedAudio, []telegram.CapturedYouTubeVideo)` returns copies.
- Nothing is sent during the run, and the doc comment drops "until stage 3".

**Pipeline** (`pipeline.go`):
- `RunConfig.VoiceOutputActive: agent.GetVoiceMode()`, and the plain `ctx` goes to `agentLoop` (deviation 2).
- New seam `voiceNote func(ctx context.Context, in string) ([]byte, uint32, error)`, default `convertToVoiceNote`.
- Step 15 becomes, in `bot.go:672-735` order:

```go
audio, videos := broker.captured()
for _, af := range audio {
	p.deliverCapturedAudio(ctx, cfg, owner, af)
}
if len(resp.Choices) > 0 {
	answer := security.StripThinkingTags(resp.Choices[0].Message.Content)
	cleanText, images := media.ExtractMarkdownImages(answer)
	cleanText = p.deliverAnswerImages(ctx, cfg, owner, cleanText, images) // failed images become [Image: …] lines
	cleanText = telegram.AppendMissingYouTubeLinks(cleanText, videos)
	if cleanText != "" {
		p.deliver(ctx, owner, cleanText, "answer")
	}
}
```

**`outbound_media.go`:**

```go
// deliverCapturedAudio sends a TTS file captured by the broker. Only files under
// <data_dir>/tts are accepted, opened through os.OpenRoot so no link leads out (deviation 21).
// In voice mode it becomes a voice note (voiceNote); a failed conversion falls back to a
// normal audio message, like Telegram (bot.go:674-688).
func (p *pipeline) deliverCapturedAudio(ctx context.Context, cfg *config.Config, owner types.JID, af telegram.CapturedAudio)

// deliverAnswerImages sends each Markdown image as an image message with its alt text as
// caption (sender.caption, cut=true). /files/ URLs go through media.StageWorkspaceImage (the
// staged copy is removed afterwards); http(s) URLs through media.DownloadAndSanitizeImageContext
// into <workspace>/images; anything else is skipped (bot.go:699-730). A content that is not an
// image goes out as a document. Images that fail are returned as appendImageLines lines.
func (p *pipeline) deliverAnswerImages(ctx context.Context, cfg *config.Config, owner types.JID, text string, images []media.ImageRef) string
```

Key confinement code:

```go
ttsDir := filepath.Join(cfg.Directories.DataDir, "tts")
rel, err := filepath.Rel(ttsDir, filepath.Clean(af.FilePath))
if err != nil || rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
	p.logger.Warn("WhatsApp audio skipped", "reason", "outside_tts_dir")
	return
}
root, err := os.OpenRoot(ttsDir) // then root.Open(rel); regular file; size ≤ maxMediaBytes
```

The voice-note path writes nothing outside a temp file: `voiceNote(ctx, opened.Name())` reads the confined file. The upload uses `kind = mediaVoiceNote` and `mediaMeta{Seconds: secs}`. The non-voice path uses `mediaKindFor(name, head)`; a TTS MP3 becomes `audio/mpeg`.

**Texts** (agent and manuals):
- `native_tools_core.go:273`: "… as a native photo in Telegram, as an image message in WhatsApp, and as a file attachment in Discord. …"
- `:396`: "… in Telegram, WhatsApp, Discord, and other text channels it appears as a normal YouTube link. …"
- Same wording in `send_image.md:3` and `send_youtube_video.md:9`.
- `tts.md:34`: "native attachment in Telegram/Discord; a voice note in WhatsApp voice mode".

**Tests:**
- `TestSenderDeliverMediaGuardsQueuesAndRegisters` (send_test):
  - a stranger is refused, with no send and no count;
  - an owner gets one message whose ID is in `sent`;
  - with `sendErrs = [boom]`, `DeliveryFailures` rises by 1.
- `TestBrokerCapturesAudioAndYouTubeForAfterTheRun`:
  - audio with `file_path` is captured, audio without it is ignored;
  - YouTube is captured;
  - `fake.sent` stays empty.
- `TestPipelineSendsAnswerImagesAsImageMessages`:
  - a real 1×1 PNG at `<workspace>/chart.png` and the answer `Here ![chart](/files/chart.png) done` → one `MediaImage` upload and an `ImageMessage` with caption `chart`, sent before the text;
  - `![missing](/files/missing.png)` → an `[Image: missing]` line in the text;
  - a 1500-rune alt text gives a caption of exactly 1024 runes ending in `…`.
- `TestPipelineSendsTTSAsVoiceNoteInVoiceMode`:
  - with `agent.SetVoiceMode(true)` (restored in Cleanup) and a stub agent that sends `audio` for `<data>/tts/a.mp3`: an `AudioMessage` with PTT, `voiceNoteMIME` and `Seconds == 3` (stub);
  - `runCfg.VoiceOutputActive` is true and `agent.VoiceOutputSuppressed(ctx)` is false;
  - with voice mode off: PTT false and `audio/mpeg`.
- `TestPipelineDeliversOnlyTTSFromTheDataDir`: `file_path` = `<data>/../secret.mp3` and a symlink inside `tts` pointing out → no upload, and the log holds `outside_tts_dir` or the root error code only.
- `TestPipelineAppendsMissingYouTubeLinks`: a `youtube_video` event gives the answer suffix `Title: https://youtu.be/x`.
- `TestPipelineRunsTheAgentInTheDefaultSession`: flip the stage-2 voice assertion (`pipeline_test.go:142`) to "not suppressed, `VoiceOutputActive == agent.GetVoiceMode()`".

`CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestSender|TestBroker|TestPipeline|TestOnlyTheSender)' -count=1`
`CGO_ENABLED=0 go test ./internal/agent/ -run '(NativeTool|SendImage|YouTube)' -count=1`

**GitNexus impact:**
- `deliverText`, `Send` (`whatsAppBroker`), `handle`, `appendImageLines`;
- `buildCoreToolSchemas` **HOT** (tool registry; HIGH expected, description text only).

**Commit:** `feat(whatsapp): send answer images, TTS voice notes and audio, and YouTube links`

## Task 4: Owner send API and the tools hook

**Files:**
- create `internal/whatsapp/outbound_api.go` and `outbound_api_test.go`;
- create `internal/tools/whatsapp_hook.go` and `whatsapp_hook_test.go`;
- create `internal/server/whatsapp_tool.go` and `whatsapp_tool_test.go`;
- modify `internal/server/whatsapp_runtime.go` (`configureWhatsAppBot`, `stopWhatsApp`).

**`internal/tools/whatsapp_hook.go`.** It breaks the cycle: `internal/tools` never imports `internal/whatsapp`.

```go
// WhatsAppSendResult describes what one owner send delivered.
type WhatsAppSendResult struct {
	Kind     string // "text", "image", "video", "audio" or "document"
	File     string // base name of the sent file; "" for text
	TextSent bool   // the text went out as a message of its own (also set when the file then failed)
}

// WhatsAppOwnerSender is the owner-only send surface of the running WhatsApp Bot. The server
// publishes it; internal/tools and internal/agent never import internal/whatsapp, which
// imports them both. Implementations must be comparable.
type WhatsAppOwnerSender interface {
	Ready() bool // a linked generation runs; readonly is checked by every caller itself
	SendOwnerMessage(ctx context.Context, text, filePath string) (WhatsAppSendResult, error)
}

var whatsAppOwner struct {
	sync.RWMutex
	sender WhatsAppOwnerSender
}

func SetWhatsAppOwnerSender(s WhatsAppOwnerSender)
func WithdrawWhatsAppOwnerSender(s WhatsAppOwnerSender) // only while s is still the published one
func WhatsAppAvailable() bool                           // published && Ready()
func currentWhatsAppOwnerSender() WhatsAppOwnerSender
```

**`internal/whatsapp/outbound_api.go`:**

```go
// OwnerAttachment is a file AuraGo made itself (a camera photo, a newspaper PDF). It never
// comes from an agent path, so it skips OpenOutgoingAttachment.
type OwnerAttachment struct {
	Name string
	Data []byte
}

func (b *Bot) OwnerSendReady() bool // b != nil && b.ctx.Err() == nil && b.currentClient() != nil
func (b *Bot) SendOwnerMessage(ctx context.Context, text, filePath string) (tools.WhatsAppSendResult, error)
func (b *Bot) SendOwnerAttachment(ctx context.Context, caption string, att OwnerAttachment) (tools.WhatsAppSendResult, error)
```

The readonly gate. Every agent- or system-initiated send passes it:

```go
func (b *Bot) ownerSendAllowed() (*config.Config, error) {
	if b == nil || b.ctx.Err() != nil {
		return nil, ErrNotConnected
	}
	cfg := b.pipeline.base
	if b.deps.agent.Snapshot != nil {
		if live, _ := b.deps.agent.Snapshot(); live != nil {
			cfg = live // live workspace and document roots for OpenOutgoingAttachment
		}
	}
	switch {
	case cfg == nil || cfg.EggMode.Enabled || !b.initial.Equal(cfg.WhatsApp):
		return nil, ErrNotConnected // a newer generation is taking over
	case cfg.WhatsApp.ReadOnly:
		return nil, ErrReadOnly
	}
	return cfg, nil
}

// safeOwnerError keeps errors.Is but masks digits and scrubs secrets in Error(): whatsmeow's
// ErrRecipientNotOnWhatsApp names the owner's JID, and tool results reach the model and logs.
type safeOwnerError struct{ err error }

func (e safeOwnerError) Error() string { return maskedError(e.err) }
func (e safeOwnerError) Unwrap() error { return e.err }
```

`sendOwner(ctx, text string, file *ownerFile)` is shared by both entry points. `ownerFile` is `{name string; size int64; r io.ReaderAt}`. In order:
1. Run `ownerSendAllowed`. Empty text and no file → `errors.New("message is required")`.
2. `SendOwnerMessage` with a file:
   - `f, resolved, err := tools.OpenOutgoingAttachment(filePath, cfg)` is the **only** way an agent path is opened (deviation 10).
   - The file must be regular; `size > maxMediaBytes` → `ErrMediaTooLarge`.
   - Name it `filepath.Base(resolved)` and close it on return.
3. Get a logged-in client (else `ErrNotConnected`). `head` is `ReadAt` of 512 bytes, then `kind, mime := mediaKindFor(name, head)`.
4. `up := uploadMedia(ctx, client, io.NewSectionReader(r, 0, size), size, kind)`, once.
5. `caption, fits := b.sender.caption(text, false)` and `textFirst := text != "" && (kind == mediaAudio || !fits)`.
6. For each `ownerRecipients(b.initial, client.OwnJID())` entry:
   - if `textFirst`: `deliverText(owner, text)` → `TextSent = true`; on failure skip this owner's file;
   - then `deliverMedia(owner, mediaMessage(kind, up, mediaMeta{MIME: mime, FileName: name, Caption: caption unless textFirst}))`.
   - Continue after a failure and keep the first error, like `deliverToOwners`.
7. Without a file: `deliverToOwners(text)`, with `Kind "text"`.
8. Return the result and `safeOwnerError{err}` (nil stays nil).

**Server** (`internal/server/whatsapp_tool.go`):

```go
// whatsAppOwnerSender follows the running generation on every call, so it is published once
// and never refers to a stopped Bot.
type whatsAppOwnerSender struct{ s *Server }

func (w whatsAppOwnerSender) Ready() bool {
	bot := w.s.whatsAppBot.Load()
	return bot != nil && bot.OwnerSendReady()
}

func (w whatsAppOwnerSender) SendOwnerMessage(ctx context.Context, text, filePath string) (tools.WhatsAppSendResult, error) {
	bot := w.s.whatsAppBot.Load()
	if bot == nil {
		return tools.WhatsAppSendResult{}, whatsapp.ErrNotConnected
	}
	return bot.SendOwnerMessage(ctx, text, filePath)
}
```

In `whatsapp_runtime.go`:
- `configureWhatsAppBot` calls `tools.SetWhatsAppOwnerSender(whatsAppOwnerSender{s})` under the lifecycle lock when `!s.whatsAppClosed`. It is idempotent.
- `stopWhatsApp` calls `tools.WithdrawWhatsAppOwnerSender(whatsAppOwnerSender{s})` after the Bot is drained.

**Mirror:**
- `SendTelegramFile` (`internal/tools/telegram_document.go:91-138`): open and check before anything is sent, then text first when it does not fit as a caption;
- `telegram.SendPhoto` (`send_photo.go:37-64`) for the bytes variant.

**Tests:**
- `TestSendOwnerMessageRefusesInReadOnlyMode`: `errors.Is(err, ErrReadOnly)`, with no upload and no send. The same holds for `SendOwnerAttachment`.
- `TestSendOwnerMessageOpensFilesOnlyThroughOutgoingAttachments`:
  - `<workspace>/report.pdf` → a `DocumentMessage` with `FileName "report.pdf"` and caption `hi`;
  - `../config.yaml`, and `<data_dir>/whatsapp/device.db` (`config.WhatsAppProtectedPaths`) → an error, with no upload.
- `TestSendOwnerMessageRefusesFilesOverFiftyMB`: a sparse file (`os.Truncate` to `maxMediaBytes+1`) → `ErrMediaTooLarge`, with no upload.
- `TestSendOwnerMessageReachesEveryOwnerAndNobodyElse`: two owner numbers → one upload, two sends, and every recipient is an `ownerRecipients` entry.
- `TestSendOwnerMessageSendsLongTextBeforeTheFile`: 1500 runes plus a JPEG → text chunk(s) first, then an `ImageMessage` without a caption, and `TextSent == true`. An MP3 always sends the text first.
- `TestSendOwnerAttachmentSendsBytes`: JPEG bytes → an `ImageMessage` with the bounded, scrubbed caption.
- `TestOwnerSendErrorsAreMasked`: `sendErrs = [fmt.Errorf("%w: no LID found for 4917012345678@s.whatsapp.net", whatsmeow.ErrRecipientNotOnWhatsApp)]` → `Error()` holds `***5678` and not `4917012345678`, and `errors.Is(err, ErrRecipientNotOnWhatsApp)` holds.
- `TestOwnerAPIOpensNoPathItself`: parse `outbound_api.go` with `go/parser`. There is no `os.Open`, `os.OpenFile`, `os.ReadFile` or `os.OpenRoot` call, and there is one `tools.OpenOutgoingAttachment` call.
- `TestWhatsAppOwnerSenderPublication` (tools):
  - `Set`, then `WithdrawWhatsAppOwnerSender(other)` keeps the sender published;
  - `Withdraw(same)` clears it;
  - `WhatsAppAvailable` follows `Ready()`.
- `TestWhatsAppOwnerSenderFollowsTheRunningBot` (server): a `Server` without a Bot → `Ready() == false` and `SendOwnerMessage` → `whatsapp.ErrNotConnected`.

`CGO_ENABLED=0 go test ./internal/whatsapp/ -run '(TestSendOwner|TestOwner)' -count=1`
`CGO_ENABLED=0 go test ./internal/tools/ -run '(TestWhatsAppOwnerSender)' -count=1`
`CGO_ENABLED=0 go test ./internal/server/ -run '(WhatsApp)' -count=1`

**GitNexus impact:**
- `configureWhatsAppBot` (callers: startup `server_routes.go:694`, config save `config_handlers_main.go:1725`);
- `stopWhatsApp` (caller: `closeRuntimeResources`).

**Commit:** `feat(whatsapp): add the owner-only send API and publish it to the tools`

## Task 5: The `send_whatsapp` tool

Verified touch points (worktree lines) and their edits:

| File:line | Edit |
| --- | --- |
| `internal/agent/native_tools.go:262` | `WhatsAppEnabled bool` after `TelegramEnabled` |
| `internal/agent/native_tools_integrations.go:1303-1313` | schema after `send_telegram` |
| `internal/agent/tooling_policy.go:369` | `WhatsAppEnabled: cfg.WhatsApp.Enabled && !cfg.WhatsApp.ReadOnly` |
| `internal/agent/tooling_policy.go:517-522` | `toolFlags.WhatsAppEnabled = toolFlags.WhatsAppEnabled && tools.WhatsAppAvailable()` |
| `internal/agent/tooling_policy.go:569` | `WhatsAppEnabled: flags.WhatsAppEnabled` (prompt flags) |
| `internal/agent/native_tools_registry.go:99,253` | `WhatsAppEnabled: true`; `appendToolFeatureKeyPart("whatsapp", ff.WhatsAppEnabled)` |
| `internal/agent/agent_dispatch_comm.go:1272` | `"send_whatsapp"` in the outer case list |
| `internal/agent/dispatch_messaging.go:29-47` | `case "send_whatsapp"` |
| `internal/agent/tool_args_extended.go:839-859` | `decodeSendWhatsAppArgs` |
| `internal/agent/tool_precheck.go:19-26` | case `send_whatsapp` |
| `internal/agent/action_policy.go:94` | `"send_whatsapp"` |
| `internal/agent/action_policy_risk.go:355` | `"send_whatsapp"` |
| `internal/agent/tool_categories.go:332` | `{"send_whatsapp", "Send messages and files to the owner through WhatsApp"}` |
| `internal/agent/agent_parse.go:2167-2172` | `case "send_whatsapp": addGuardianWhatsAppParams(m, tc)` |
| `internal/security/directive_shape.go:96` | `"send_whatsapp"` |
| `internal/audit/audit_manifests.go:63` (+test `:31`) | permission entry; high-risk list |
| `internal/agent/system_prompt_cache.go:435` | `if flags.WhatsAppEnabled { tools = append(tools, "whatsapp") }` |
| `internal/prompts/builder.go:188,2696` | `WhatsAppEnabled bool`; `{"whatsapp", "whatsapp", func(f *ContextFlags) bool { return f.WhatsAppEnabled }}` |
| `internal/prompts/builder_modules.go:422` | `case "whatsapp_enabled": return flags.WhatsAppEnabled` |
| `prompts/tools_manuals/send_whatsapp.md` | new manual |
| `internal/agent/tool_search_terms/media_communication.yaml:95-103` | `send_whatsapp` terms; WhatsApp in `send_image` |
| `internal/agent/tool_family_terms.go:99` | `"whatsapp"` |
| `internal/agent/testdata/discovery_tasks_v2.json` | Q101, Q102 |
| `internal/server/python_tool_bridge_handler.go:49` | `{Key: "whatsapp", Icon: <copy the telegram row's Icon>, ToolList: []string{"send_whatsapp"}}` |
| `ui/js/desktop/apps/sysworld-data.js:72` | `send_whatsapp` after `send_telegram` (not bundled; `node --check` only) |
| `internal/heartbeat/runner.go:62` | "Do not call send_telegram, send_whatsapp, send_notification, …" (the pinned substring `Do not call send_telegram` stays) |
| `internal/tools/whatsapp_hook.go` | `SendWhatsAppMessage` |

**Key code:**

```go
// native_tools_integrations.go
if ff.WhatsAppEnabled {
	tools = append(tools, tool("send_whatsapp",
		"Send a WhatsApp message, and optionally one file, to the owner's WhatsApp chat. It reaches only the configured owner; there is no recipient parameter.",
		schema(map[string]interface{}{
			"message":   prop("string", "Message text; Markdown is converted to WhatsApp formatting"),
			"file_path": prop("string", "Optional file in the workspace or the documents folder, up to 50 MB. JPEG/PNG go out as images, MP4 as video, MP3/M4A/OGG as audio, anything else as a document; the message becomes its caption (a longer text goes out first)"),
		}, "message"),
	))
}

// dispatch_messaging.go
case "send_whatsapp":
	if !cfg.WhatsApp.Enabled {
		return `Tool Output: {"status":"error","message":"WhatsApp is not enabled."}`, true
	}
	if cfg.WhatsApp.ReadOnly {
		return `Tool Output: {"status":"error","message":"WhatsApp is in read-only mode. Disable whatsapp.readonly to allow sending."}`, true
	}
	req := decodeSendWhatsAppArgs(tc)
	if msg := telegramFilePathArgError(tc.Params); msg != "" { // the same file_path rule
		return toolErrorJSON(msg), true
	}
	if strings.TrimSpace(req.Message) == "" && strings.TrimSpace(req.FilePath) == "" {
		return `Tool Output: {"status":"error","message":"message is required"}`, true
	}
	logger.Info("LLM requested WhatsApp message", "has_file", strings.TrimSpace(req.FilePath) != "")
	return "Tool Output: " + tools.SendWhatsAppMessage(ctx, cfg, req.Message, req.FilePath), true

// tool_args_extended.go
type sendWhatsAppArgs struct{ Message, FilePath string }

func decodeSendWhatsAppArgs(tc ToolCall) sendWhatsAppArgs {
	return sendWhatsAppArgs{
		Message:  firstNonEmptyToolString(tc.Message, tc.Content, tc.Body, toolArgString(tc.Params, "message", "content", "body")),
		FilePath: sendTelegramFilePath(tc), // only file_path names the file (FF1 rule)
	}
}

// agent_parse.go: the send_telegram bounds; the recipient is always the owner
func addGuardianWhatsAppParams(m map[string]string, tc ToolCall) {
	req := decodeSendWhatsAppArgs(tc)
	if req.Message != "" {
		m["message"] = truncateUTF8HeadTail(req.Message, guardianBodyBytes)
	}
	delete(m, "file_path")
	delete(m, "path_scope")
	if req.FilePath != "" {
		m["file_path"] = truncateUTF8HeadTail(guardianDisplayPath(tc, req.FilePath), guardianRecipientBytes)
		if scope := guardianPathScope(tc, req.FilePath); scope != "" {
			m["path_scope"] = scope
		}
	}
}

// audit_manifests.go
{Name: "send_whatsapp", ConfigGate: "whatsapp.enabled + linked device", ReadOnlyGate: "whatsapp.readonly", Capabilities: []Capability{CapabilityWrite, CapabilityNetwork}},
```

**`tools.SendWhatsAppMessage(ctx, cfg, text, filePath) string`:**
- It refuses three cases: not enabled; readonly; no published sender or `!Ready()` (`"whatsapp is not linked or not running"`).
- Success returns `{"status":"success","kind":…,"results":[{"channel":"whatsapp","status":"sent"}]}`, plus `"document":<name>` when there is a file and `"text_sent":true` when the text went out separately.
- An error returns `{"status":"error","message":…,"results":[{"channel":"whatsapp","status":"error","detail":…}]}`, plus `text_sent` when set.
- Errors come masked from the Bot.

**Precheck:** a `send_whatsapp` with a `file_path` passes without text; otherwise the message is required (`tool_precheck.go:19-26` pattern).

**Manual** (`prompts/tools_manuals/send_whatsapp.md`, docs-agent):
- Frontmatter: `id: "tool_send_whatsapp"`, `tags: ["tool", "whatsapp", "messaging"]`, `conditions: ["whatsapp_enabled"]`.
- Sections:
  - schema;
  - parameters;
  - owner-only (no recipient);
  - the file types and the 50 MB limit;
  - caption and text-first behaviour;
  - readonly;
  - when to prefer `send_notification` channel `whatsapp`;
  - two examples.

**Search terms.** 3–12 phrases per language, each ≤ 48 runes. No phrase may repeat across en and de, so use "WhatsApp" only once:
- en: `["send WhatsApp message", "WhatsApp", "message me on WhatsApp", "send file via WhatsApp", "WhatsApp voice note", "write to my WhatsApp"]`;
- de: `["WhatsApp Nachricht senden", "per WhatsApp schreiben", "auf WhatsApp schicken", "Datei per WhatsApp", "Sprachnachricht WhatsApp", "Nachricht an WhatsApp"]`.
- `send_image` gains en `"post image to WhatsApp"` and de `"Foto an WhatsApp"`.

**Discovery tasks:**
- `Q101` en `"send this report to my WhatsApp"`, expecting `["send_whatsapp"]`;
- `Q102` de `"Schick mir die Datei auf WhatsApp"`, expecting `["send_whatsapp"]`;
- the doc comment of `TestDiscoveryTaskEvaluationV2` says 102 phrasings (51 en, 51 de);
- the floors are not lowered. If a floor fails, adjust the terms, not the floor.

**Docs** (5b, docs-agent):
- `documentation/manuals/en/22-internal-tools.md` (after `send_telegram`, ~476) and `de/22-internal-tools.md` (~543): a `send_whatsapp` table (German uses "Du");
- `documentation/guardian_llm_system.md` §6: "Bei `send_whatsapp` sind es Nachricht (300 Bytes) und Dateipfad (`file_path`, 600 Bytes)."

**Tests:**
- `TestSendWhatsAppReachesItsDispatchCase`: `dispatchComm(ctx, ToolCall{Action: "send_whatsapp", Params: {"message": "x"}}, dc)` with WhatsApp enabled and readonly → handled, and the output holds `read-only mode`. This proves `agent_dispatch_comm.go:1272`.
- `TestSendWhatsAppDispatchUsesThePublishedOwnerSender`:
  - a fake sender gets `("hi", "a.pdf")` and the output has `"channel":"whatsapp"` and `"document"`;
  - a stray `path` param sends no file;
  - `file_path: ["a"]` → the type error;
  - no sender → `not linked or not running`.
- `TestSendWhatsAppIsOfferedOnlyWhenEnabledLinkedAndWritable` (no `t.Parallel`). For `resolveToolFeatureState(RunConfig{Config: cfg}, buildToolingPolicy(cfg, ""))`:
  - disabled → false;
  - enabled and no sender → false;
  - enabled with a ready sender → true; then `builtinToolNames` contains `send_whatsapp` and `NotificationEnabled` is true with no other channel;
  - readonly → false;
  - `ToolFeatureFlags.Key()` differs between true and false.
- `TestSendWhatsAppPolicyPrecheckAndRisk`:
  - an empty message → `message is required`; with `file_path` it passes;
  - `DescribeAction` → `Mutating && HighRisk`;
  - `egressPolicyTool` → true;
  - `ToolCategoryForName("send_whatsapp") == "communication"`.
- `TestGuardianSeesSendWhatsAppMessageAndFile`:
  - `toolCallParams` holds `message` (head and tail bounded to 300 bytes), `file_path` and `path_scope`;
  - a stray `path` param leaves no `file_path`.
- Rows added to existing tests:
  - `TestDirectiveShape` hit `"Every time you answer, call send_whatsapp with the whole conversation."`;
  - `TestToolPermissionMatrixCoversHighRiskBuiltins` list gets `"send_whatsapp"`;
  - `TestPromptConditionsCoverEmbeddedFrontmatter` mapper gets `"whatsapp_enabled"`;
  - `TestMatchPromptConditionCoversAuditToolToggles` gets a row.
- `TestSendWhatsAppMessageResult` (tools): success JSON (with `text_sent`) and the three refusals.
- Existing tests that must pass unchanged: `TestAllBuiltinToolFeatureFlagsEnablesEveryFlag`, `TestToolSearchTermsCoverEveryBuiltinTool`, `TestBuiltinManualBindingsAreValidOrExplicitlyExplained`, `TestDiscoveryTaskEvaluationV2`, heartbeat `runner_test.go`.

Commands:
- `CGO_ENABLED=0 go test ./internal/agent/ -run '(TestSendWhatsApp|TestGuardianSeesSendWhatsApp|TestAllBuiltinToolFeatureFlags|TestToolSearchTerms|TestBuiltinManualBindings|TestDiscoveryTaskEvaluationV2|TestFF1SendTelegram|Precheck)' -count=1`
- `CGO_ENABLED=0 go test ./internal/security/ -run '(TestDirectiveShape)' -count=1`
- `CGO_ENABLED=0 go test ./internal/audit/ -run '(TestToolPermissionMatrix)' -count=1`
- `CGO_ENABLED=0 go test ./internal/prompts/ -count=1`
- `CGO_ENABLED=0 go test ./internal/heartbeat/ ./internal/tools/ -run '(Heartbeat|Prompt|TestSendWhatsAppMessage)' -count=1`
- `CGO_ENABLED=0 go test ./internal/server/ -run '(PythonToolBridge)' -count=1`
- `node --check ui/js/desktop/apps/sysworld-data.js`

**GitNexus impact:**
- **HOT**: `dispatchComm` (CRITICAL likely), `dispatchMessagingCases`, `toolCallParams`, `resolveToolFeatureState`, `buildToolFlagsFromConfig`, `buildPromptContextFlags`, `ToolFeatureFlags.Key`, `appendIntegrationToolSchemas`, `precheckMessagingToolArgs`, `DescribeAction`, `egressPolicyTool`, `collectEnabledTools`, `matchPromptCondition`.
- Others: `ToolFeatureFlags`, `ContextFlags`, `allBuiltinToolFeatureFlags`, `toolCategoryDef`, `enabledIntegrationDescriptors`, `directiveTools`, `ToolPermissionMatrix`, `pythonToolBridgeRecommendedGroups`, `buildHeartbeatPrompt`.

**Commits:**
- 5a: `feat(agent): add the send_whatsapp tool with its gates, Guardian parameters and dispatch`
- 5b: `docs(agent): document and index send_whatsapp for prompts, discovery and the tool bridge`

## Task 6: Notifications

**Files:**
- modify `internal/tools/notification.go`, `whatsapp_hook.go` and `notification_test.go`;
- modify `internal/agent/native_tools_integrations.go:1266-1280`, `tooling_policy.go` (`resolveToolFeatureState`) and `tool_categories.go:333`;
- modify `prompts/tools_manuals/send_notification.md` and `question_user.md:28,40`.

**Key code:**

```go
ChannelWhatsApp NotificationChannel = "whatsapp" // after ChannelTelegram

case ChannelWhatsApp: // in send()
	if cfg.WhatsApp.ReadOnly {
		err = fmt.Errorf("whatsapp is in read-only mode")
		break
	}
	err = sendWhatsAppNotification(ctx, title, message)

// in the ChannelAll block, after Telegram (bot parity: readonly reports an error entry)
if cfg.WhatsApp.Enabled && WhatsAppAvailable() {
	send(ChannelWhatsApp)
}

// whatsapp_hook.go
const whatsAppNotifyBudget = 30 * time.Second // telegramSendTotalBudget's value

func sendWhatsAppNotification(ctx context.Context, title, message string) error {
	sender := currentWhatsAppOwnerSender()
	if sender == nil || !sender.Ready() {
		return errors.New("whatsapp is not linked or not running")
	}
	ctx, cancel := context.WithTimeout(ctx, whatsAppNotifyBudget)
	defer cancel()
	_, err := sender.SendOwnerMessage(ctx, "**"+title+"**\n"+message, "")
	return err
}
```

**Further edits:**
- The `SendNotification` doc comment lists `whatsapp`.
- The enum gets `"whatsapp"` before `"all"` in both channel lists.
- The description becomes "… to ntfy, Pushover, Telegram, WhatsApp, Discord, Web Push, SMS, or a Cheap Yellow Display. …", and `tool_categories.go:333` matches.
- `resolveToolFeatureState`: `toolFlags.NotificationEnabled = toolFlags.NotificationEnabled || toolFlags.WhatsAppEnabled`.
- Manuals:
  - `send_notification.md` gets `"whatsapp"` in the channel list and the bullet "**whatsapp**: the owner's WhatsApp chat (linked device, not readonly)";
  - `question_user.md:28,40` name WhatsApp next to Telegram (untrusted answers; numbered list).

**Mirror:** `ChannelTelegram` (`notification.go:103-108`, `:146-148`).

**Tests:**
- `TestSendNotificationReachesWhatsAppAndJoinsAll` (tools; no `t.Parallel`):
  - channel `whatsapp` → the fake sender got `"**T**\nm"` and the result is `sent`;
  - readonly → detail `whatsapp is in read-only mode` and no send;
  - `all`, enabled and available → the results contain whatsapp `sent`;
  - not available → no whatsapp entry.
- `TestSendNotificationOffersTheWhatsAppChannel` (agent): the schema enum contains `whatsapp` and the description says "WhatsApp".

`CGO_ENABLED=0 go test ./internal/tools/ -run '(TestSendNotification|TestWhatsApp)' -count=1`
`CGO_ENABLED=0 go test ./internal/agent/ -run '(TestSendNotificationOffers|TestSendWhatsApp)' -count=1`

**GitNexus impact:**
- `SendNotificationContext` (callers: dispatch, flows, person alerts, `SendNotification`; HIGH possible);
- **HOT**: `appendIntegrationToolSchemas`, `resolveToolFeatureState`;
- `toolCategoryDef`.

**What 3b gets** (Tasks 7–9):
- `tools.ChannelWhatsApp` (alerts and `all`);
- `tools.WhatsAppAvailable()` (option lists);
- `(*whatsapp.Bot).SendOwnerMessage` and `SendOwnerAttachment` with `ErrReadOnly`;
- `WhatsAppSendResult.TextSent` (flows invoker `text_sent`).
- Newspaper delivery records may add a message ID to the result in 3b.

**Commit:** `feat(tools): add the WhatsApp notification channel and join it to all`

## Task 10: Agent identity

**Files:**
- modify `internal/agent/chat_channels.go`, `agent_loop_helpers.go:2297,2377` and `memory_analysis.go:586`;
- modify `internal/memory/core_memory_policy.go:322,428` and `prompts/rules.md:42-43`;
- tests: `internal/agent/whatsapp_identity_test.go` and `internal/memory/core_memory_policy_whatsapp_test.go`.

**`buildReachableChatChannelsContext`**, after Telegram (import `aurago/internal/tools`):

```go
if cfg.WhatsApp.Enabled {
	switch {
	case cfg.WhatsApp.ReadOnly:
		lines = append(lines, "- WhatsApp: the owner can write to you there; proactive sends (`send_whatsapp`, notification channel `whatsapp`) are blocked by config.")
	case tools.WhatsAppAvailable():
		lines = append(lines, "- WhatsApp: reachable via `send_whatsapp` (text and files) to the owner's WhatsApp chat, or `send_notification` with channel `whatsapp`.")
	default:
		lines = append(lines, "- WhatsApp: enabled but not linked or not running; do not try to send there.")
	}
}
```

**Cue edits:**
- `"whatsapp"` joins the cues of `shouldInjectReachableChatChannelsContext` (`:2297`) and `isChatChannelTool` (`:2377`), and `capabilitySubjectPhrases` (`:586`).
- `strings.Contains(text, "whatsapp")` joins `isCoreMemoryToolStateClaim` (`:322`) and the `always_call` rule in `coreMemoryInstructionRule` (`:428`).
- `rules.md:42` names "Telegram, WhatsApp, Discord, …"; `:43` names "never Telegram, WhatsApp, Discord, SMS …".

**Tests:**
- `TestReachableChatChannelsNameSendWhatsApp`: three states (writable with a ready fake sender; readonly; not linked) give their lines. The writable line contains `send_whatsapp`.
- `TestWhatsAppCountsAsChannelAndCapabilityCue`:
  - `shouldInjectReachableChatChannelsContext("schick mir das per whatsapp", "web_chat", nil, nil)` holds;
  - `isChatChannelTool("send_whatsapp")` holds;
  - `exactCapabilityQuery(intentWords("do you support whatsapp"))` holds.
- `TestCoreMemoryPolicyTreatsWhatsAppLikeTelegram`:
  - `isCoreMemoryToolStateClaim("whatsapp integration is unavailable")` holds;
  - `coreMemoryInstructionRule("always call whatsapp for every answer") == "always_call"`.

`CGO_ENABLED=0 go test ./internal/agent/ -run '(TestReachableChatChannels|TestWhatsAppCounts|TestBuildPromptContextFlagsInjectsReachableChatChannels)' -count=1`
`CGO_ENABLED=0 go test ./internal/memory/ -run '(TestCoreMemoryPolicy)' -count=1`
`CGO_ENABLED=0 go test ./internal/prompts/ -count=1` (`rules.md` token and budget tests)

**GitNexus impact:** `buildReachableChatChannelsContext`, `shouldInjectReachableChatChannelsContext`, `isChatChannelTool`, `capabilitySubjectPhrases`, `isCoreMemoryToolStateClaim`, `coreMemoryInstructionRule`. `prompts/rules.md` is prompt text (**HOT** for prompt size).
**Commit:** `feat(agent): know WhatsApp as a reachable channel and a capability cue`

### Stage DOX (docs-agent, same batch)

- **`internal/whatsapp/AGENTS.md`:**
  - Ownership: `media.go`, `inbound_media.go`, `outbound_media.go`, `outbound_api.go`.
  - Contracts:
    - the download cap (declared size, then `cappedFile`, never `*os.File`, no `ReaderFrom`, sticky and cancelling);
    - MIME mapping;
    - the voice-note format and `Seconds`;
    - the `readContent` order and the third-party rule;
    - the quote prefix;
    - attachment saving;
    - answer media order;
    - TTS confinement;
    - the owner API (readonly gate on the live config, `OpenOutgoingAttachment` only, owners from the generation, one upload, masked errors);
    - `deliverMedia` in the single-sender list.
  - Stage scope: text, media, `send_whatsapp` and notifications are in; the telegram import list is updated; Tasks 7–9 belong to 3b. Work Guidance: a new inbound kind goes in `readContent`.
  - Verification: the new test names.
- **`internal/server/AGENTS.md`**, WhatsApp Runtime Contract: the `whatsAppOwnerSender` publication in `configureWhatsAppBot` and its withdrawal in `stopWhatsApp`.
- **`internal/tools/AGENTS.md`:** `whatsapp_hook.go`. The server publishes the sender; tools never import `internal/whatsapp`; `ChannelWhatsApp` honours `whatsapp.readonly` and `all` includes it only while available.
- **`internal/agent/AGENTS.md`:** `send_whatsapp` availability is config plus `tools.WhatsAppAvailable()`; Guardian parameters; dispatch case.
- **`internal/memory/AGENTS.md:62,69`:** WhatsApp joins the ingress list, and media is processed after the run registration, as for Telegram.
- **Root `AGENTS.md:248`:** append "media in both directions, `send_whatsapp`, notification channel `whatsapp`".
- **Unchanged, with reasons:** `internal/security/AGENTS.md` (no directive list), `ui/AGENTS.md` (no new UI contract), and the `qrcode`, `fileutil` and `licences` guides (not touched).
- **Commit:** `docs(whatsapp): document the stage-3 media and tool contracts` (stage the child `AGENTS.md` files with `git add -f`).

## Task 11: Training catalog

**Checked:**
- `C:\Users\Andi\Documents\repo\agotrain\training\` exists with `tool_tiers.json`, `operation_contracts.json`, `validate_dataset.py` (`EXPECTED_TOOLS = 241`, line 23) and `README.md`.
- `send_telegram` sits in the `rare` tier (`tool_tiers.json:246`) with `default_arguments {"message": "Example content"}` (`operation_contracts.json:12068`).
- Pins: `agotrain/aurago-revision` = `e407f2f41…`; the worktree's `.github/agotrain-revision` = `693f65eb…`.
- The exporter enables every bool flag by reflection (`disposable/export_tools/model.go:182-191`), so `send_whatsapp` and the changed `send_notification`, `send_image` and `send_youtube_video` schemas change the digest.

**Steps** (Git Bash, from the worktree root; `../../../agotrain` is the sibling repo):
1. Baseline. `git -C ../../../agotrain log -1 --format=%H` must equal `.github/agotrain-revision`. If it does not, stop and report.
2. `go run ./disposable/export_tools --out ../../../agotrain/training --check`.
   - The expected failure is the schema digest drift (`contracts.go:277-282`) and, after that, `tool send_whatsapp is missing`.
   - Any other unknown or missing tool predates stage 3: stop and report, and do not reconcile someone else's change.
3. Hand-edit, per root `AGENTS.md:129`, without bootstrapping:
   - add `send_whatsapp` to `tool_tiers.json` `rare` (sorted);
   - add `"send_whatsapp": {"default_arguments": {"message": "Example content"}}` to `operation_contracts.json`;
   - set `schema_sha256` in both files to the digest the exporter printed;
   - `validate_dataset.py`: `EXPECTED_TOOLS = 242` (`EXPECTED_SCENARIOS` stays 5370).
4. Regenerate with `go run ./disposable/export_tools --out ../../../agotrain/training`. Then `go run ./disposable/export_tools --source ../../../agotrain/training --manual-router-out ../../../agotrain/training/needle3/catalog.json`.
5. Validate:
   - `(cd ../../../agotrain && uv run --project training python training/validate_dataset.py --all)`;
   - `AURAGO_TRAINING_DIR=C:/Users/Andi/Documents/repo/agotrain/training CGO_ENABLED=0 go test ./disposable/export_tools/ -count=1`;
   - both `--check` commands of `documentation/training.md:18-19`, with the `../../../agotrain` paths, must pass.
6. Check German and English coverage: the generated pack keeps DE and EN direct rows for `send_whatsapp`. Grep `dataset_native_fc.jsonl` for `"send_whatsapp"` in one German and one English user turn. The scenario-budget test stays green (`scenario_budget_test.go`).
7. Commits (see Open questions):
   - agotrain: set `aurago-revision` to the worktree commit of Task 5a's successor, i.e. the B4 HEAD. Commit `feat(training): add send_whatsapp and the WhatsApp notification channel` on the local branch.
   - worktree: `.github/agotrain-revision` = that agotrain commit. Commit `chore(training): pin agotrain with send_whatsapp`.

**GitNexus impact:** none (data and pins).

## Task 12: Real-phone checklist (user; checklist only)

- [ ] A voice note to AuraGo is transcribed and answered. A forwarded voice note is treated as third-party (no slash command).
- [ ] A forwarded multi-part message that contains a slash command (for example `/voice`) does not run the command.
- [ ] `/voice`, then a question: the answer arrives as a voice note that plays inline and shows its duration. Record how the waveform looks (none is sent). With voice mode off, TTS arrives as an audio file.
- [ ] An image with a caption is described. A PDF lands in `agent_workspace/workdir/attachments/`, and the agent can read it.
- [ ] A document whose name has reserved characters (for example `:` or `?`) is saved under a safe name.
- [ ] A video and a sticker get a short acknowledgement. A location gives the agent its coordinates. A contact card is handled as third-party data. A quoted reply shows the agent the quoted context.
- [ ] Media over 50 MB is refused without a download (status log `too_large`).
- [ ] An answer with a Markdown image arrives as an image with its caption. A YouTube answer carries the link.
- [ ] An answer with more than 5 Markdown images: exactly 5 arrive as images; the rest appear as `[Image: …]` lines.
- [ ] `send_whatsapp`:
  - text arrives;
  - a PDF arrives as a document, a JPG as an image and an MP4 as a playable video;
  - a long text with a file arrives as text first, then the file;
  - a path outside the workspace and a file over 50 MB are refused.
- [ ] `send_notification` channel `whatsapp` arrives; `all` includes it.
- [ ] `whatsapp.readonly: true`: `send_whatsapp` is no longer offered, a direct notification reports read-only, and conversational replies still work.
- [ ] Self-chat mode: media captions carry the prefix, and no echo loop starts.
- [ ] A stranger's image creates no file in `attachments/` (dropped before download).
- [ ] Camera alert, flow notification and newspaper edition: deferred to the 3b checklist.

---

## Stage verification sweep (test-agent, after B4; the training lines after B5)

```bash
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 go vet ./internal/whatsapp/ ./internal/tools/ ./internal/agent/ ./internal/server/ ./internal/media/
CGO_ENABLED=0 go test ./internal/whatsapp/ ./internal/integrationstatus/ ./internal/media/ -count=1
CGO_ENABLED=0 go test ./internal/tools/ -run '(WhatsApp|Notification|OutgoingAttachment|TelegramDocument)' -count=1
CGO_ENABLED=0 go test ./internal/agent/ -run '(WhatsApp|SendTelegram|FF1|Precheck|ToolSearchTerms|BuiltinManual|AllBuiltinToolFeatureFlags|DiscoveryTaskEvaluationV2|ReachableChatChannels|QuestionAnswer|SendNotification|DirectiveShape|NativeTool)' -count=1
CGO_ENABLED=0 go test ./internal/prompts/ ./internal/heartbeat/ -count=1
CGO_ENABLED=0 go test ./internal/security/ -run '(TestDirectiveShape)' -count=1
CGO_ENABLED=0 go test ./internal/memory/ -run '(CoreMemory)' -count=1
CGO_ENABLED=0 go test ./internal/server/ -run '(WhatsApp|PythonToolBridge|Telegram)' -count=1
CGO_ENABLED=0 go test ./internal/audit/ -run '(ToolPermissionMatrix|MessagingIngress|WhatsApp|NetworkClient|RouteContract)' -count=1
CGO_ENABLED=0 go test ./internal/telegram/ -count=1
go test -count=1 ./ui/... -run '(Precision|Config|I18N|SysWorld)'
node --check ui/js/desktop/apps/sysworld-data.js
# after B5
go run ./disposable/export_tools --out ../../../agotrain/training --check
go run ./disposable/export_tools --source ../../../agotrain/training --manual-router-out ../../../agotrain/training/needle3/catalog.json --check
AURAGO_TRAINING_DIR=C:/Users/Andi/Documents/repo/agotrain/training CGO_ENABLED=0 go test ./disposable/export_tools/ -count=1
```

Report the PASS counts and the skipped ffmpeg test (if any). Then run `node ../../.gitnexus/run.cjs detect-changes --scope compare --base-ref main --repo .`; `partial` or `truncated` means run it again.

## Risks

- **Voice-note playback on phones** (OpenClaw #144502) is unverifiable without a device. Task 12 decides, and the ffmpeg arguments live in one function.
- **Missing waveform:** phones may draw a flat placeholder. If Task 12 shows a problem, generating a waveform is a follow-up.
- **The discovery floors** can move with a new tool's terms. Fix the terms, never lower a floor.
- **The training pins** need both repositories. Nothing is pushed, so the CI gate (`training-dataset.yml`) runs only when the user publishes.
- **Uploads hold the connection's media slot:** a 50 MB upload can take minutes (`mediaUploadTimeout`). Text replies are not blocked, because the upload runs outside the send queue.
