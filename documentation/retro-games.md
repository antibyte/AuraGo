# Retro Games

Retro Games is a Virtual Desktop app for the browser-playable games of the Internet Archive
(archive.org): MS-DOS, the Internet Arcade, the Console Living Room, Commodore 64, Amiga, Apple II,
ZX Spectrum and Windows 3.x — about 72,500 titles (the exact number depends on archive.org). You search
them, keep favorites, see what you played last, and play a game in a desktop window. A curated Hall of
Fame of 46 classics is the start page.

## How it works

- **Index:** on first use AuraGo downloads the list of playable games from archive.org's search API
  (about a dozen requests, one second apart) into `<data_dir>/retrogames/retrogames.db`
  (`directories.data_dir`) and searches it locally. Each platform refreshes itself when its list is older
  than seven days and the app or the agent tool is used; after a failed automatic refresh AuraGo waits an
  hour before it tries again. "Update index" in the app refreshes everything at once. A platform is replaced
  in one step, and an empty answer from archive.org (or one with less than half of the known games) keeps the
  old list. Nothing is downloaded while Retro Games is off.
- **Playing:** games run in archive.org's own emulator (DOSBox, MAME, VICE, SAE and others) inside the
  window. Click the power button on the game screen to start; the first start of an arcade or console
  game can take up to 40 seconds while the emulator downloads. Click into the screen to play with the
  keyboard; click outside to release it.
- **Doesn't start:** not every archived title runs. Mark such a game with ⚠ (the button on the game screen,
  or "Mark as not starting" in the context menu of a game card). It then no longer shows up in search results
  — in the app and for the agent — and appears dimmed in the Hall of Fame, Recently played and Favorites. The
  "Doesn't start" view lists all marked games; "It starts after all" (context menu, or ⚠ again on the game
  screen) undoes the mark.
- **Lists:** Recently played shows your last 50 games; Favorites and "Doesn't start" hold up to 500 games
  each (a full list answers "This list is full"). A favorite that archive.org no longer lists stays in your
  list and is labeled "No longer on archive.org". While the Virtual Desktop is read-only, favorites, marks and
  "Update index" are refused and starts are not recorded.
- **Agent:** with "Let the agent search and start games" on, the agent's `retro_games` tool searches the
  index. It opens a game in the app only when the Virtual Desktop also allows agent control; otherwise it
  names the game and you start it yourself.

## Settings

Config → Virtual Desktop (group Agent Tools) → Retro Games:

```yaml
retro_games:
  enabled: true        # show the app and allow the index download
  agent_access: true   # allow the retro_games agent tool
```

The toggles are called **Enable Retro Games** and **Let the agent search and start games**. **Check index**
reports how many games the index holds. Both settings are on by default. The Virtual Desktop must be
enabled for the app.

## Privacy and rights

- AuraGo talks only to `archive.org` (search API over HTTPS, no redirects, no private addresses).
  Thumbnails and the emulator load directly from archive.org in your browser, so archive.org sees your
  IP address while you browse and play.
- AuraGo does not download, host or redistribute game files; it only embeds archive.org's player.
  Availability on archive.org does not mean you may redistribute a game.
- If SQLite reports the index file as corrupt, AuraGo moves it aside (`retrogames.db.corrupt-<UTC time as
  YYYYMMDD-HHMMSS>`, with its `-wal` and `-shm` files) and builds a new one; favorites and history start empty
  in that case. Other open errors never move the file.
- **Backups:** favorites, history and "Doesn't start" marks live in `retrogames.db` together with the game
  index, so AuraGo backups include that file (a consistent snapshot, about 30 MB with a full index).

## Related

- Tool parameters: [Internal tools — `retro_games`](manual/en/22-internal-tools.md).
- HTTP contract of the app: [Desktop HTTP and SDK contract](desktop-api.md#retro-games-apidesktopretro-games).
