# Local Wikipedia

Local Wikipedia puts one complete Wikipedia edition on your AuraGo host. AuraGo
downloads a [Kiwix](https://kiwix.org) ZIM file once, reads it and the search
index inside it in pure Go, and then works without any network access: the agent
gets the `local_wikipedia` tool, and the Virtual Desktop gets a **Wikipedia** app
for searching and reading. There is no container, no sidecar and no kiwix-serve.

The integration is off by default. Nothing is downloaded until an administrator
clicks **Install**.

## What you get

- **One edition at a time.** You pick one language and one variant. Installing
  another edition replaces the current one.
- **Two variants.** *Without media* (Kiwix `nopic`: text, tables and infoboxes,
  no images) and *with media* (Kiwix `maxi`: articles with images). Kiwix `mini`
  editions and topic collections are not offered.
- **The 16 AuraGo UI languages.** The default is the AuraGo system language.
- **Offline search.** Title suggestions while you type, plus full-text search over
  the index Kiwix ships inside the ZIM file. Results combine both: an exact title
  match comes first, then the best full-text hits.
- **Updates on your terms.** AuraGo checks the Kiwix catalog once a day and shows
  a hint when a newer edition exists. It never installs an update on its own.

## Sizes

Sizes from the Kiwix catalog on 9 October 2026, in decimal gigabytes as Kiwix
rounds them. Kiwix publishes new editions every few months, so the config page
shows the exact size of the current edition before you install.

| Language | Without media (`nopic`) | With media (`maxi`) | Articles |
|---|---:|---:|---:|
| Czech (`cs`) | 3.4 GB (2026-09) | 10.0 GB (2026-09) | 975,533 |
| Danish (`da`) | 1.4 GB (2026-09) | 4.1 GB (2026-09) | 505,788 |
| German (`de`) | 18.6 GB (2026-10) | 52.4 GB (2026-01) | 5,153,780 |
| Greek (`el`) | 2.0 GB (2026-09) | 5.2 GB (2026-09) | 385,920 |
| English (`en`) | 52.7 GB (2026-06) | 127.4 GB (2026-08) | 19,191,219 |
| Spanish (`es`) | 12.3 GB (2026-08) | 40.8 GB (2026-05) | 4,195,672 |
| French (`fr`) | 12.9 GB (2026-05) | 55.4 GB (2026-05) | 4,587,621 |
| Hindi (`hi`) | 0.9 GB (2026-10) | 1.9 GB (2026-10) | 246,697 |
| Italian (`it`) | 9.0 GB (2026-08) | 31.8 GB (2026-08) | 3,110,338 |
| Japanese (`ja`) | 15.9 GB (2026-06) | 30.8 GB (2026-06) | 2,447,471 |
| Dutch (`nl`) | 5.8 GB (2026-04) | 23.5 GB (2026-01) | 3,078,764 |
| Norwegian Bokmål (`no`, Kiwix `nb`) | 3.0 GB (2026-09) | 8.5 GB (2026-09) | 1,061,557 |
| Polish (`pl`) | 8.1 GB (2026-08) | 22.7 GB (2026-08) | 2,303,267 |
| Portuguese (`pt`) | 7.1 GB (2026-05) | 20.6 GB (2026-05) | 1,989,025 |
| Swedish (`sv`) | 6.3 GB (2026-07) | 18.8 GB (2026-07) | 4,606,037 |
| Chinese (`zh`) | 14.8 GB (2026-07) | 26.6 GB (2026-08) | 3,541,817 |

Article counts are those of the without-media edition. Full-text search depends
on the language:

- Danish, German, English, Spanish, French, Italian, Dutch, Norwegian,
  Portuguese and Swedish use stemming, so related word forms match.
- Czech, Greek, Hindi and Polish match the word forms you type (Kiwix indexes
  them without stemming).
- Japanese and Chinese use Kiwix's CJK word splitting (single characters and
  character pairs), which AuraGo reproduces, so full-text search works for
  them too.

No UI language is title-search-only by itself. Title search alone happens for an
edition whose search index is missing or cannot be read; the config page then
shows **Title search only** and the status carries `fulltext_unsupported`.

## Install, update and disk space

1. Open **Config** and select **Local Wikipedia** in the **Agent Tools** group of
   the sidebar (or search for it). Turn on **Enable Local Wikipedia**, pick the
   **Language** and the **Variant**, and click **Save**.
2. The info box shows the selected edition (date, exact size, article count)
   and the free space at the target directory.
3. Click **Install** and confirm. The download runs in the background with a
   progress bar (percent, rate, remaining time); you can close the page.
4. AuraGo verifies the SHA-256 checksum from Kiwix's metalink (`.meta4`) file,
   opens the archive, checks its search index and switches over. The status is
   now **Ready**.

**Disk space.** AuraGo starts a download only when the free space is at least the
remaining size plus a safety margin of max(1 GiB, 1 % of the edition size). It
re-checks about every gigabyte it writes and pauses with `insufficient_disk_space`
before the disk fills up. If the free space cannot be measured (some network or
FUSE file systems), the dialog asks for an explicit confirmation.

**How the download works.**

- HTTPS only. A redirect to plain HTTP, to a URL with credentials or to a local
  host is refused, and AuraGo does not connect to mirror addresses that resolve to
  loopback, private, link-local or other local networks (checked on the address
  of every connection attempt).
- The edition size comes from the metalink and must not exceed 1 TiB. The file
  is hashed with SHA-256 while it downloads.
- Mirrors are tried in the order of the metalink; Kiwix's load balancer is the
  last resort. Every connection has a two-minute stall watchdog. A round that
  makes progress simply repeats, a round without progress ends the download as
  `download_failed` (state **Download interrupted**, partial file kept).
- If every mirror refuses to continue the partial file (they answer *range not
  satisfiable* or ignore the range request) in two rounds in a row, AuraGo starts
  over into `<name>.zim.part.restart`. That file replaces the partial download only
  once it holds more bytes, so the progress never shrinks. The restart needs the
  space for a second copy, which AuraGo counts exactly.
- A mismatch of the SHA-256 deletes the partial file (`checksum_mismatch`).

**Cancel and resume.** **Cancel download** keeps the partial download
(`<name>.zim.part`). **Resume** first re-checks the existing part (**Checking
data**) and then continues where it stopped. After an AuraGo restart an unfinished
download shows **Download interrupted**; nothing resumes on its own.

**Updates.** With **Check daily for a newer edition** on and an edition installed,
AuraGo asks the Kiwix catalog about two minutes after it starts and then once a
day; **Check for updates** asks right away. A newer edition of the same language
and variant shows up on the config page, as a **New edition** hint on the
dashboard badge and as a banner in the desktop app. **Update** uses the same flow
as Install. The old edition stays usable until the new one is verified, so you
need room for both. If there is room for only one, AuraGo asks whether it may
delete the old edition first; Wikipedia is then offline until the new download is
finished.

**A failed install, resume or update never takes a working edition away.** The
status stays **Ready** (`readable` is still true) and carries the `error_code` of
the failed operation until you start the next one. The same holds for
`interrupted` while an installed edition is open.

**Changing language or variant** never starts a download. The status shows that
your selection differs from the installed edition, and **Install** replaces it
under the same rules.

**Delete edition** (with confirmation) removes the ZIM file, any partial download
and the state files. It is refused while a download is running.

## Where the files live

| Installation | Directory | Notes |
|---|---|---|
| Native Linux, Windows, macOS | `<data_dir>/wikipedia` by default (for example `/home/aurago/aurago/data/wikipedia`, `/root/aurago/data/wikipedia` for a root install or `C:\ProgramData\AuraGo\data\wikipedia`), or an absolute directory you choose under **Storage directory** | Must be absolute and writable, not a system or program directory, and not AuraGo's own data directory itself (use a subfolder). Any subfolder of AuraGo's data directory is fine, even when the data directory lives in a protected place. |
| Native Linux with the AuraGo systemd service | same | The unit from `install_service_linux.sh` uses `ProtectSystem=strict` and only lets AuraGo write below its install directory (`ReadWritePaths`). To use another disk, mount it below the install directory, or add it with `sudo systemctl edit aurago` (`[Service]` and `ReadWritePaths=/mnt/wiki`) and restart. Otherwise the install fails with `data_dir_invalid`. |
| Docker | `/app/data/wikipedia` inside the `aurago_data` volume | Fixed; the config field is read-only. Make sure the disk that holds Docker's volumes has room (`docker system df -v`). |

**Refused directories.** The check looks at the path as written and at where it
really points, so a symbolic link, a Windows junction or an 8.3 short name cannot
smuggle the edition into a protected place. Refused are, among others:

- Linux and macOS: `/`, `/bin`, `/sbin`, `/usr`, `/lib*`, `/etc`, `/root`, `/boot`,
  `/dev`, `/proc`, `/sys`, `/run`, `/var/lib/docker`, `/mnt` itself, and on macOS
  `/System`, `/Library`, `/Applications`, `/private`, `/Volumes` itself and
  `~/Library`. Data disks below `/mnt/<disk>` or `/Volumes/<disk>` are fine.
- Windows: drive roots, the `Windows`, `Program Files`, `Program Files (x86)` and
  `ProgramData` folders, administrative shares (`\\host\C$`) and device paths
  such as `\\?\Volume{…}`.
- WSL: Windows drives under `/mnt/<letter>` are judged like the Windows path
  (`/mnt/c/Windows` is refused).

**Exception: AuraGo's own data directory.** Subfolders of AuraGo's data directory
(`directories.data_dir`) are always allowed, even when the data directory itself
lies in one of the places above — for example `/root/aurago/data/wikipedia` (an
`install.sh` service running as root), `/usr/local/aurago/data/wikipedia`,
`C:\ProgramData\AuraGo\data\wikipedia` or
`~/Library/Application Support/aurago/data/wikipedia`. The data directory itself
stays refused. A subfolder that is a link pointing somewhere else is judged by
its target. The exception does not apply when the data directory is a drive or
file system root or a folder directly below one, or to names ending in a dot or
a space or containing a colon.

If an invalid storage directory ends up in `config.yaml` (for example after
editing it by hand), saving other settings still works: a save checks only the
Local Wikipedia values it changes, and the app reports the directory as
`data_dir_invalid` until you choose a valid one.

AuraGo creates the directory when it is missing and writes a small probe file to
prove it can write there.

Files in that directory:

- `<name>.zim` — the installed edition.
- `<name>.zim.part` — an unfinished download; `<name>.zim.part.restart` — the
  temporary file of a restart (see above), never resumed after a restart of AuraGo.
- `state.json` — the installed edition: file name, Kiwix name, variant and date,
  size, SHA-256, UUID, article count and install time, plus the update hint and
  files still waiting for deletion.
- `download.json` — resume data: URL, mirrors, size, SHA-256 and target name.

AuraGo only ever deletes files it knows from `state.json` or `download.json`, and
only ZIM files that follow Kiwix's naming scheme. Other files in the directory
are left alone. After replacing an edition the old file is deleted once the last
reader has released it. Windows refuses to delete a file that is still open, so
AuraGo keeps the name in `state.json` and retries after loads and once an hour.

There is nothing to back up: an edition can always be downloaded again.

## The agent tool

The `local_wikipedia` tool appears when the integration is on, **Agent may use
Local Wikipedia** is on and an edition is installed and readable. It is
read-only.

- `search` — `query` (required) and `limit` (1–10, default 5). Returns the
  edition (language, variant, date), whether full-text search is available, and
  the results (title, path, snippet). The top three results also carry the lead
  section as Markdown (at most 2,000 characters).
- `read` — `title` or `path`, optionally `section` (heading text or index) and
  `offset`. Returns the article as Markdown in chunks of at most 8,000 characters,
  the list of sections and `next_offset` for the next chunk. Infoboxes become
  key-value lists, tables become Markdown tables (at most 50 rows), and reference
  lists, navigation boxes and edit links are removed.

Tool output is marked as external data, like web content. The agent is told to
prefer `local_wikipedia` over the online `wikipedia_search` tool and web search for
encyclopedic questions, to cite the article and the edition date, and to use web
search for recent events. The online `wikipedia_search` tool is unchanged.

Try: "What does the local Wikipedia say about the Rhine?" or "Lies den Abschnitt
Geschichte im Artikel Berlin aus der lokalen Wikipedia."

## The desktop app

**Wikipedia** (Office category) appears in the Virtual Desktop while the
integration is on. Type in the search box for title suggestions, press Enter for
the result list, and open an article. Articles keep the original light Wikipedia
style (with images in *with media* editions); the toolbar has back, forward, main
page and a random article. The footer shows language, variant and edition date.

Without an installed edition the app explains what to do (administrators get a
link to the config page). During a download it shows the progress, a pending
update shows a banner, and *title search only* editions show a hint. Scripts
inside ZIM articles never run. The app's UI contract is in
`ui/js/desktop/apps/AGENTS.md`.

## Dashboard

The **Local Wikipedia** badge in **System → Operations & Services** is lit while
the integration is on. When a newer edition is available it turns yellow and
shows **New edition** with the date; a click opens the config section.

## Permissions and API

| Action | Who |
|---|---|
| Turn on, configure, choose directory, install, update, cancel, delete | Administrators |
| Search, suggestions, read articles, browse the app | Users with `desktop:read` |
| Agent tool | Read-only, only with **Agent may use Local Wikipedia** |

```http
GET  /api/local-wikipedia/catalog?lang=de          # Kiwix catalog for one language (cached 6 h)
GET  /api/local-wikipedia/status                   # state, progress, edition, update hint, disk numbers
POST /api/local-wikipedia/install                  # 202; {"replace_mode":"keep_old","confirm_unknown_space":false}
POST /api/local-wikipedia/cancel                   # keeps the partial download
POST /api/local-wikipedia/delete                   # removes ZIM, partial download and state
POST /api/local-wikipedia/check-update             # asks the catalog now, answers the status
GET  /api/desktop/local-wikipedia/status           # state, progress (0..1), edition, fulltext, update_available, error_code, can_manage
GET  /api/desktop/local-wikipedia/suggest?q=       # up to 10 titles
GET  /api/desktop/local-wikipedia/search?q=&limit= # up to 30 results (default 20)
GET  /api/desktop/local-wikipedia/random
GET  /api/desktop/local-wikipedia/main
GET  /api/desktop/local-wikipedia/content/{path}   # original ZIM content with Range and ETag
```

The administrator routes need an administrator session or Bearer token, answer
`Cache-Control: no-store`, and POSTs from a browser session must be same-origin.
`catalog?lang=` defaults to the system language and answers `{"language","fulltext",
"variants":{"nopic":{…},"maxi":{…}}}` with name, date, approximate size, article
count and metalink URL per variant. The install body is one JSON object of at most
16 KiB: `replace_mode` is `keep_old` (default) or `delete_old_first`, and
`confirm_unknown_space` answers the unknown-free-space question. Success is
`202 {"status":"accepted"}`; cancel answers `{"status":"cancelled"}` and delete
`{"status":"deleted"}`. Content paths are names inside the ZIM file, never
file-system paths. HTML content is sent with a sandboxing Content-Security-Policy
that forbids scripts.

**Status.** `GET /api/local-wikipedia/status` answers `state`
(`not_installed`, `downloading`, `verifying`, `ready`, `interrupted`, `error`),
`progress` (0..1), `bytes_done`, `bytes_total`, `rate`, `eta_seconds`, `edition`,
`selection`, `selection_matches_installed`, `update_available`, `fulltext`,
`readable`, `loading`, `free_bytes` (-1 when unknown), `required_bytes`,
`data_dir`, `data_dir_locked`, `operation_in_progress`, `error_code`,
`recommendation`, `system_language` and `languages`. Two fields matter for
clients:

- `readable` is true whenever an installed edition is open and being served, in
  every state (also while an update downloads or after a failed update). Decide
  whether Wikipedia content is available from `readable`, not from `state` or the
  presence of `edition`. It does not include the on/off switch.
- `loading` is true for the moment after AuraGo starts in which the storage
  directory is read for the first time (longer on a slow or hung network share).
  The state is then `not_installed`, `readable` is false, `error_code` is `busy`,
  and install and delete answer `busy`. Poll until `loading` is false.

**Errors.** Refusals have the form `{"error","error_code","recommendation"}`
and never contain paths or host names:

| HTTP | `error_code` |
|---|---|
| 409 | `busy`, `disabled`, `free_space_unknown`, `already_installed`, `no_operation` |
| 422 | `insufficient_disk_space` (also `required_bytes`, `free_bytes`, `can_delete_old`), `data_dir_invalid` |
| 502 | `catalog_unreachable` |
| 400 | `unknown_language`, `invalid_request` |
| 503 | `localwiki_unavailable` (no manager) |
| 500 | `localwiki_error` (see the AuraGo log) |

## Troubleshooting

| `error_code` | Meaning | What to do |
|---|---|---|
| `insufficient_disk_space` | Not enough free space; the status shows the required and available bytes. | Free space, choose *without media*, or pick another directory. For updates, let AuraGo delete the old edition first. |
| `free_space_unknown` | AuraGo cannot measure free space at the target (some network or FUSE file systems). | Confirm in the dialog if you are sure there is room, or use a local disk. |
| `checksum_mismatch` | The download does not match Kiwix's SHA-256; the partial file was deleted. | Install again. If it repeats, check proxies, antivirus and the disk. |
| `download_failed` | All mirrors failed; the state is `interrupted`. | Check internet access, DNS and the firewall for the hosts below, then **Resume**. |
| `catalog_unreachable` | The Kiwix catalog cannot be reached. An installed edition keeps working. | Retry later; check access to `opds.library.kiwix.org`. |
| `zim_unreadable` | With an installed edition and `readable` false: the file cannot be opened (damaged, truncated or unsupported), the tool is hidden. After an install: the downloaded file could not be read and was removed. | In the first case **Delete edition** and install again; in the second just install again. |
| `state_unreadable` | The state file `state.json` in the storage directory cannot be read (damaged or invalid), so AuraGo cannot tell which files belong to the installed edition. It touches nothing and offers no edition. | Check the AuraGo log for the reason. AuraGo does not repair the file: stop AuraGo, move the damaged `state.json` out of the storage directory (and delete old `.zim` files you no longer need), start AuraGo and install again. |
| `fulltext_unsupported` | The edition has no usable full-text index; title search still works. This is a warning. | Nothing to do. Newer index formats may need a newer AuraGo. |
| `already_installed` | The newest catalog edition of your selection is already installed. | Nothing to do; choose another language or variant to replace it. |
| `busy` | Another download, verification or delete is running, or AuraGo is still loading the storage directory after its start. | Wait, or cancel the running operation. |
| `disabled` | The integration is off or the change is not saved. | Turn it on and **Save**. |
| `data_dir_invalid` | The directory is not absolute, not writable or a system or program directory. | Choose another directory; see the systemd note above. In Docker the directory is fixed. |

**Install** stays disabled while the config page has unsaved changes: save first.
Download speed depends on the mirror Kiwix's load balancer picks; a resumed
download continues from the same mirror if it can, and AuraGo does not limit
bandwidth.

## Privacy and network

Searches and article reads never leave your host. Local Wikipedia talks only
HTTPS (redirects to plain HTTP are refused), and only to:

- `opds.library.kiwix.org` — the catalog: when the config page asks for sizes,
  on Install, and once a day while an edition is installed and **Check daily for a
  newer edition** is on. The request names the edition family, for example
  `wikipedia_de_all`.
- `download.kiwix.org` and `lb.download.kiwix.org` — the `.meta4` metalink file
  with size, checksum and mirror list.
- The mirrors listed in that file, currently `ftp.fau.de`, `ftp.nluug.nl`,
  `mirror.download.kiwix.org`, `mirror.accum.se`, `www.mirrorservice.org`,
  `wi.mirror.driftle.ss` and `dumps.wikimedia.org` (the list changes).

No account, no API key, no telemetry. Turn the update check off to stop the daily
catalog request. For a firewall allowlist, use the hosts above.

## Licenses

- Wikipedia text is licensed under
  [CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/) (older revisions
  also under the GFDL). Images and other media in *with media* editions carry
  their own licenses, listed on their Wikimedia Commons pages. The ZIM files are
  built by Kiwix/openZIM from Wikipedia.
- AuraGo does not ship Wikipedia content. Your AuraGo downloads it from Kiwix
  when you click Install. If you republish text from it (for example an agent
  answer in a public post), credit Wikipedia and the article, and share it under
  the same license.
- Wikipedia is a trademark of the Wikimedia Foundation. AuraGo is not affiliated
  with or endorsed by the Wikimedia Foundation or Kiwix.
- AuraGo's ZIM and search-index readers are its own MIT-licensed code; the GPL
  libzim and Xapian sources were only used to understand the file formats. The Go
  libraries involved (`klauspost/compress`, `ulikunitz/xz`,
  `blevesearch/snowballstem`, `golang.org/x/text`) are listed in
  [`THIRD_PARTY_NOTICES.md`](../THIRD_PARTY_NOTICES.md).

## Limits

- One edition at a time; no `mini` or topic ZIM files; no non-Wikipedia ZIM files.
- No automatic updates and no automatic resume after a restart.
- Keyword and title search only, no semantic search.
- Articles keep Wikipedia's light style; there is no dark mode for article pages.
- No bandwidth limit, and no Kiwix server for the rest of your LAN.
- Reading needs little memory: at most 64 MiB of decompressed data are cached,
  and at most four searches run at once with a five-second limit each.
- A future Kiwix index format may fall back to title search until AuraGo
  supports it.
