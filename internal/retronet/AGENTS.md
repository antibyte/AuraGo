# Retro-Net

## Purpose

Dialing directory engine for the Desktop Terminal: curated catalog, own-entry
validation, guarded outbound dialing, Telnet negotiation, charset conversion,
the PETSCII engine for Commodore 64 boards, anonymous SSH, the session pump
with limits, and reachability probing. No HTTP.

## Ownership

`internal/retronet` owns protocol and policy logic. `internal/server/desktop_retronet*.go`
owns auth, the toggle gate, JSON endpoints, the WebSocket adapter, audit and
admin-only host-key persistence. `internal/desktop` stores own entries in the
Desktop setting `retronet.entries` and validates them through
`ValidateEntriesDocument`. The browser modules `ui/js/desktop/apps/terminal.js`,
`terminal-text.js`, `terminal-geometry.js`, `terminal-modem.js` and
`terminal-retronet-*.js` own rendering, the screen geometry, the modem
sequence, the baud throttle and the entry editor.
Operator docs: `documentation/retro-net.md`.

## Local Contracts

- Dial only stored entries by ID (`Lookup`); never accept host or port from a
  client. `Dialer.DialEntry` refuses ports 0/25/465/587 and values outside
  1-65535, resolves once (IPv4-mapped addresses are unmapped), refuses the
  entry when any resolved address is `security.IsRestrictedNetworkIP`, then
  tries the validated addresses sequentially (IPv4 first, duplicates dropped),
  each dial pinned to that exact IP, within one 10 s budget (at most 4 s per
  attempt while others remain). `AllowRestricted` exists for tests and is never
  set in production code (`TestAllowRestrictedOnlyInTests`).
- This package imports only the standard library, `aurago/internal/security`
  and `golang.org/x/crypto/ssh`; it must not import `internal/desktop` or
  `internal/server` (`TestProductionImportsStayInsideContract`).
- Own entries (`entries.go`): document `{"version":1,"entries":[...]}`, at most
  `MaxOwnEntries` (50) entries and 64 KiB, IDs `own-[a-z0-9]{8,32}` unique.
  Telnet entries, catalog ones included (`validTarget`), have kind `c64` if
  and only if their charset is `petscii` (`KindC64`/`CharsetPETSCII`); SSH
  entries have neither. Catalog entries sit in category `commodore` if and
  only if they are `c64` (`catalog_test.go`); own C64 entries stay in `own`.
  Object keys must be exactly the stored field names, unique per object
  (`hasExactKeys`; JSON's
  case folding must not let the browser see another value). Name 1-40 and
  description up to 80 runes reject Unicode Cc/Cf/Co/Zl/Zp and U+FFFD (ZWJ
  allowed). Hosts are public IP literals or RFC 1123 names whose last label is
  neither all digits nor `0x` hex; `localhost` and `*.localhost` are refused
  (any case, trailing dot included) like the editor does, so a direct settings
  write cannot store them. Every validation error is the same non-echoing
  message.
- Telnet accepts BINARY, ECHO and SGA from the server and BINARY, SGA, TTYPE
  (`ANSI` for `bbs`, `XTERM-256COLOR` for `world`, `PETSCII` for `c64`) and
  NAWS on our side; it refuses every other option once, including COMPRESS2, GMCP, MSDP, MSSP,
  CHARSET, LINEMODE and NEW-ENVIRON, and answers only on state changes.
  `RemoteEcho` (character mode) is true only when the server WILL ECHO and WILL
  SGA; `ServerEcho` is the ECHO state alone. Echo controls carry
  `remote = RemoteEcho` and `hidden = ServerEcho && !RemoteEcho` (MUD password
  prompts: line mode without local echo, never in history), initially and on
  every change. Enter arrives as `\r`: `world` sends CR LF, `bbs` sends CR NUL
  unless we transmit BINARY.
- Charsets: cp437 maps 0x7F-0xFF and keeps C0 controls; latin1 maps 0xA0-0xFF,
  turns 0x9B into `ESC [` and drops other C1 bytes; utf8 passes through.
  Unmappable input becomes `?`. petscii is the PETSCII engine below.
- C64 boards (`KindC64`, `session_c64.go`) run over raw TCP: no Telnet
  negotiation, 0xFF is the PETSCII character pi, input is not IAC-escaped.
  Only when the board's very first chunk begins with IAC WILL/WONT/DO/DONT
  (`FF FB`..`FF FE`, `opensTelnet`) does the call switch to Telnet for the
  rest of its lifetime: `Initial()` goes out then, TTYPE answers `PETSCII`,
  NAWS 40x25, input is IAC-escaped; a later IAC stays data. `connected`
  carries protocol `telnet`, kind `c64`, charset `petscii`; the size stays
  40x25 (`sessionSize`), browser resizes are ignored and no `echo` control is
  sent. The auto-detect only looks at the first received chunk and needs IAC
  plus its verb (`FF FB`..`FF FE`) in that chunk: a board whose first read is
  a lone `FF` stays raw, by design.
- PETSCII engine (`petscii*.go`): `NewCodec(CharsetPETSCII)` returns a new
  stateful engine per call; one per session, never shared, no locking. Its
  40x25 screen model (blank, cursor at row 25 column 1, light blue text,
  lower/upper set) is authoritative: every glyph is the C64 Pro Mono code
  point `U+EF00 + screen code` (lower/upper set) or `U+EE00 + screen code`
  (upper/graphics set, `C64Glyph`; the font's coverage is pinned by
  `TestDesktopTerminalC64FontCoversEngineGlyphs` in `ui/`), colours are 24-bit
  SGR from the Colodore palette (`c64Palette`) on black, reverse swaps them,
  and CUP is emitted whenever xterm's cursor is not known to be at the model
  cell (the engine never relies on autowrap); each `Decode` ends with xterm's
  cursor on the model cursor. Glyphs, scrolling (SU on black), CLR and BEL go
  out as they come; rows changed by DEL or INST, and all 25 rows after a
  charset switch (0x0E/0x8E), are redrawn once at the end of each `Decode`
  inside one `ESC[?2026h`...`ESC[?2026l` block. Output per call is at most
  48 KiB + 64 bytes per input byte (`TestPetsciiOutputIsBounded`, fuzz
  targets). ANSI probes (`ESC [`, at most 16 parameter bytes, final byte) are
  swallowed. `Encode` is an allow-list: printable ASCII the C64 keyboard has
  plus £, ↑ and ←, CR (RETURN), DEL/BS (DEL 0x14), Ctrl+C or ESC (RUN/STOP),
  the xterm sequences in `petsciiKeySequences` (cursor keys, HOME, Shift+Home
  CLR, INST, Delete, F1-F8) and the browser's colour-key sequence `ESC[64;n~`
  only for the 16 colour codes and RVS ON/OFF (`petsciiPrivateAllowed`);
  everything else is dropped. Known simplification: the C64 screen editor's linked
  80-column logical lines are not modelled, so INST on a full row drops the
  row's last cell instead of opening a new row, and DEL never pulls up a
  continuation row.
- SSH logs in anonymously (`none`, then keyboard-interactive/password with
  empty answers; user `guest` unless set) with `HostKeyAlgorithms` pinned to
  OpenSSH's order (ed25519 first), so fingerprints match `ssh`/known_hosts.
  Catalog entries require their pinned `SHA256:` key; own entries with a key
  must match. Own entries without a key ask the browser (`hostkey_prompt`, no
  answer within 60 s rejects); on acceptance `OnHostKeyAccepted` is called and
  the server persists the key only for an administrator. Handshake, PTY and
  shell share a 10 s deadline, lifted while the user decides; cancellation
  closes the connection at any point and never hands out a late client.
- `Manager.Run` always sends exactly one final `result` control. Limits (one
  Manager per server, shared by all users): 4 concurrent sessions (`limit`
  without dialing), 30 min without user input (only browser keystrokes reset
  it; terminal status replies don't count: a message that consists only of
  cursor position, status, device attribute, mode or focus reports still
  goes to the service but leaves the timer, see `terminalReportsOnly`; mouse
  reports count), 4 h maximum. Context causes `ErrDisabled`/`ErrShutdown` map to
  `disabled`/`server_shutdown`; `ErrClientGone` (the server's adapter noticed
  the browser left, also during the dial or SSH handshake) maps to
  `remote_closed`, and cancellation closes the connection at once, so no PTY
  or shell is requested for a browser that is gone. BBS sessions stay 80x25
  and C64 sessions 40x25 (`sessionSize`); other sizes clamp to
  cols 20-400, rows 5-200. Every write to the service is bounded by 10 s:
  Telnet through the TCP write deadline, SSH writes and window changes through
  an abort timer that closes the connection.
- Reason → Hayes code mapping lives only in `CodeFor`; the browser translates
  `desktop.terminal_retronet_result_<reason>`.
- Never log, audit or forward session payload bytes.
- Status probes are TCP connects through the same guard: 3 s timeout, 16 in
  parallel, cache 10 min, one probe in flight, and no probe run starts sooner
  than 60 s after the previous start for any trigger (stale snapshot, never
  probed entry, forced refresh). Callers pass the full directory (catalog plus
  own entries) every time; a run prunes entries it was not given. `c64`
  entries are never probed (`probeable`: many C64 boards have a single line
  and a probe would occupy it) and never appear in the status map; the
  browser marks them `[C]`.

## Work Guidance

- Catalog changes: edit `catalog.go`, keep IDs stable, update the spec table
  and count in `catalog_test.go`, add/remove the description key
  `desktop.terminal_retronet_entry_<id with _>` in all 16 desktop locales,
  keep catalog plus `MaxOwnEntries` at most 99 (two-digit directory numbers
  01-99, `00` is the local shell;
  `TestDesktopTerminalRetroNetDirectoryNumbersFitTwoDigits`), update
  `documentation/retro-net.md` when it mentions the entry, and run the live
  check. SSH catalog pins are the fingerprint of the ed25519 key where offered.
- C64 catalog boards (category `commodore`, kind `c64`, charset `petscii`)
  pass the live check with at least 10 printable PETSCII characters (non-space,
  counted on the wire before the codec) within 45 s of `connected`, or with a
  `BUSY` banner (a single-line board in use is reachable). They get no
  `liveKick`; never send a C64 board more than one detection key and never
  answer its login or new-user prompts.
- New Telnet options or charsets need an explicit row in the negotiation or
  charset tests before they are accepted.

## Verification

- `go test ./internal/retronet -count=1` (includes the PETSCII reference
  terminal, split-read and output-bound tests; fuzz targets
  `FuzzPetsciiDecode` and `FuzzPetsciiEncode`). A change to `C64Glyph` also
  needs `go test ./ui -run TestDesktopTerminalC64FontCoversEngineGlyphs -count=1`.
- Live: `$env:AURAGO_RETRONET_LIVE='1'; go test ./internal/retronet -run TestLiveCatalog -count=1 -v`
- Race: `CGO_ENABLED=1 go test -race ./internal/retronet -count=1` on a Linux
  host with gcc.
- Server, Desktop and browser sides: see `internal/server/AGENTS.md` (Retro-Net
  Terminal), `internal/desktop/AGENTS.md` and the Terminal entry in
  `ui/js/desktop/apps/AGENTS.md`.

## Child DOX Index

None.
