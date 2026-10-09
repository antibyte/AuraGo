# Retro-Net terminal

Retro-Net turns the Virtual Desktop **Terminal** into a dialing directory for
text services that are still online: Telnet bulletin board systems (BBSes),
multi-user dungeons (MUDs), Telehack, an ASCII world map, the Star Wars ASCII
animation, the Free Internet Chess Server and a few anonymous SSH games. You
pick an entry, the terminal "dials" with a short modem sequence, and the
session runs inside the existing retro terminal styles and CRT effects.

Retro-Net is **disabled by default**. Enable it with
`virtual_desktop.retronet_enabled` (Config → Virtual Desktop → "Retro-Net in
the Terminal"). Read-only mode disables it. Every Desktop user with write
access can dial; only administrators manage own entries.

## Privacy and safety

- Every connection runs through the AuraGo server. Service operators see the
  server's public IP address; some BBSes print it on the login screen.
- Telnet is unencrypted. Never type a password you use anywhere else. The
  terminal repeats this reminder right after `CONNECT` of every Telnet session.
- The server dials only entries stored in the directory, by ID. It never
  accepts a host or port from the browser. It resolves the name once and
  refuses the call when any resolved address is not a public internet address
  (LAN, loopback and other private or reserved ranges), then connects only to
  the checked addresses, IPv4 first. Mail ports 25, 465 and 587 are refused.
- Session bytes are never written to logs. The Desktop audit log records each
  connection attempt and, per session, the entry, resolved target address,
  result code and reason, duration and byte counts.

## Using the directory

With Retro-Net enabled the Terminal opens on the directory. Entry `00` is the
local shell (the Code Studio terminal); "Open Terminal Here" from Files still
opens the shell directly, and `Ctrl+]` in the shell returns to the directory.

| Key | Action |
| --- | --- |
| Arrow keys, Page Up/Down, Home/End | Move the selection |
| Digits | Jump to that entry number; a second digit within one second makes it two-digit |
| Enter | Dial the selected entry |
| `R` | Check again which services are reachable (or retry a failed directory load) |
| `N` / `E` / `Del` | New / edit / delete an own entry (administrators) |
| `Ctrl+]` | During a call: hang up (`NO CARRIER`, "You hung up"). In the local shell: return to the directory at once |

Click or tap selects an entry; a second click or tap on it, or a double-click,
dials it. Any key skips the modem sequence. When a call ends, any key pressed
after a short pause returns to the directory. The toolbar offers **Directory** / **Hang up** and a **Modem
speed** selector (Unlimited, 300, 1200, 2400, 9600 or 14400 baud) that slows
output to the chosen speed; Unlimited is the default and the choice is kept per
browser. Modem sounds follow the terminal's sound switch and are silent in the
Modern style; with reduced motion or disabled animations the dial text appears
at once without sound.

Status markers: `[*]` reachable, `[ ]` not reachable (with the last time it was
reachable, when known), `[?]` not checked yet. A check is a plain TCP
connection under the same address rules, 3 seconds per service. Opening the
directory starts one when the last check is older than 10 minutes or an entry
was never checked; `R` asks for one at once. Whatever the trigger, a new check
starts at most once per minute.

## Display modes

- **Mailboxes** (type Mailbox, including the door-game boards under Games) use
  a fixed 80×25 grid with the IBM VGA font, scaled to the window, like a DOS
  or Amiga terminal. Catalog mailboxes use CP437 or Latin-1; own Mailbox
  entries can choose UTF-8, CP437 or Latin-1.
- **Text worlds** (MUDs, Telehack, MapSCII and the other classics) and SSH
  entries follow the window size and report every resize to the service.
  While a Telnet text world does not echo, you type a whole line locally
  (Backspace edits, Up/Down recall the last 50 lines) and Enter sends it. At a
  password prompt the MUD takes over echo: you still type a whole line, but
  nothing is shown and the line is not kept in the history. When a service
  switches to character mode, and always over SSH, every key is sent at once.

## Results

When a call ends the terminal prints a modem result code and a one-line
explanation:

| Code | Meaning |
| --- | --- |
| `BUSY` | The service refused the connection, or 4 Retro-Net sessions are already running (the limit is shared by everyone using this AuraGo installation) |
| `NO ANSWER` | The service did not answer within 10 seconds |
| `NO DIALTONE` | The name did not resolve, or the target is blocked (private address or mail port) |
| `NO CARRIER` | The service hung up; 30 minutes passed without your input; the session reached 4 hours; Retro-Net was switched off or your access ended; AuraGo shut down; an SSH host key did not match or was not accepted; you hung up; or the browser lost its connection to AuraGo |

## Own entries

Administrators can add up to 64 own entries (`N` in the directory): name (1 to
40 characters), description (up to 80), protocol (Telnet or SSH), host name or
public IP address and port; for Telnet also the type (Mailbox or MUD/text
world) and character set (UTF-8, CP437, ISO 8859-1); for SSH a user name (up to
32 characters: a–z, 0–9, dot, hyphen, underscore). The form refuses private,
local and reserved addresses, `localhost` and the mail ports. Saving re-reads
the stored entries first, so changes made in another window are kept.

SSH entries log in anonymously only (no password or key); use Quick Connect for
real SSH logins. The first call to an own SSH entry shows the server's key
fingerprint and asks for confirmation; answer with the letters shown (`Y` and
`N` always work). No answer within 60 seconds rejects the key. When an
administrator confirms, the key is stored with the entry and a different key
later blocks the call; other users' confirmations allow only that one session
and the question comes back next time. Changing the protocol, host, port or
user name of an SSH entry drops its stored key, and a key confirmed while the
entry was changed that way is not stored. To clear a wrong stored key, delete
and re-create the entry or change its host, port or user.

Own entries are stored in the Desktop setting `retronet.entries`.

There is no free dialing of an arbitrary `host:port`, no LAN target, no real
SSH login and no C64 (PETSCII) board.

## Catalog maintenance

The curated catalog lives in `internal/retronet/catalog.go`. Before a release,
run the live check, which dials every entry and expects readable output:

```powershell
$env:AURAGO_RETRONET_LIVE='1'; go test ./internal/retronet -run TestLiveCatalog -count=1 -v
```

Entries that stay unreachable are removed (catalog row, description strings in
all 16 desktop locales, and this guide's examples if they mention them).
