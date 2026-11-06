# Chapter 33: Aura Drift — racing on the Virtual Desktop

Aura Drift is a top-down car racer that lives on the Virtual Desktop. It runs entirely in
your browser: no account, no provider, no network call. Race three AI opponents, chase a
personal best, or work through an eight-track cup.

## Opening and starting

Launch **Aura Drift** from the start menu under **Games**. Choose a mode, a track and the
number of laps; switching tracks applies its recommended two laps (about three minutes per
race). **Quick race** is the fast entry point, **Time trial** drives solo against a ghost
replay, **Cup** chains all eight tracks, and **Daily route** builds a new validated layout
from the UTC date.

## Controls

| Input | Action |
| --- | --- |
| Arrow keys / WASD | Steer, throttle, brake |
| Space | Fire the equipped weapon |
| Shift | Handbrake — controlled oversteer |
| Ctrl / Alt | Nitro |
| P / Escape | Pause |
| R | Restart the race |

Touch devices show steering, throttle, brake, fire and handbrake buttons. Losing window
focus pauses the race and releases every held key.

## Drive with grip, not against it

The car uses a real tyre model: front and rear axles have separate slip angles, load shifts
forward under braking, and the handbrake removes rear grip. Wet asphalt, gravel, mud, sand,
ice and oil change traction noticeably. Lift before the corner, turn in, and feed the
throttle back at the apex. Yanking the wheel too early produces understeer — especially on
ice.

## Weapons

Pickups grant an oil slick, mine, EMP, homing rocket, MG salvo or shield. Each has a
telegraph, a cooldown and a counter, and AI follows the same rules. Weapons can be switched
off in the menu and are always disabled in Time trial.

## Tracks

| Track | Character |
| --- | --- |
| Alpenpass | Switchbacks, fog, gravel runoff |
| Wüsten-Canyon | Long straights, heat, sand |
| Neon-Hafen | Night, rain, reflective asphalt |
| Salzsee | Wide open flats, very high grip |
| Wald-Rallye | Narrow, muddy, dusk |
| Industrie-Ring | Oval core with factory shortcuts |
| Gletscherpass | Ice and snow, aurora light, low grip |
| Metropolis-Ring | City at night, neon, wet asphalt |

The world is generated in chunks around the camera instead of loading one huge image. Four
graphics tiers (plus an automatic choice) scale particles, weather, lighting and cache size.
**High** and **Ultra** add a WebGL2 post-processing pass: bloom around headlights, vertical
reflections on wet asphalt, heat shimmer, rain on the lens, per-track colour grading and a
brief chromatic shock on impact. Without WebGL2, or after a context loss, the game continues
with Canvas 2D alone.

## Weather and style points

The weather changes during the race on a fixed schedule: rain in the harbour and in the
city, snow on the glacier, plus fog, mist, heat and glare. It hits every car equally and
takes grip away from all of them; the HUD shows the current condition.

Sliding sideways pays: hold more than 10 degrees of slip angle and you start a drift
chain. Keep the chain alive within 1.5 seconds and the multiplier grows to x8; the chain
only counts once it ends. Loose surfaces and the marked drift zones pay extra, and so does
a rival you pass within arm's length. The HUD shows the pending points, the multiplier and
the time left to link the next slide; the result screen shows the total and the best chain.

Every lap is split into three sectors, and the split times in the HUD show you where you
are losing it.

Opponents use the same tyre model you do. A car far behind receives at most 6% more target
speed, never extra grip — the menu discloses this. After a spin the AI briefly reverses;
if it stays pinned, a marshal puts it back on the racing line.

## Settings and best times

Volume, mute, graphics tier, reduced motion, ghost display and the weapon switch live in
the in-game menu. Best times and settings stay local to this browser. A read-only Desktop
stores nothing.

## If the window stays empty

The first release used class names such as `ad-overlay` and `ad-panel`. Ad blockers
(AdGuard, uBlock, EasyList) treat `ad-` classes as advertising and hide them with
`display: none` — the game was running but invisible, and the console stayed empty.
Current versions use the `agdrift-` prefix and are no longer affected. If it still
fails, whitelist the Desktop address in your ad blocker and reload the page.

## Boundaries

Aura Drift offers believable, deterministic driving physics — not an engineering-grade
vehicle simulation. The world is drawn with Canvas 2D; WebGL2 handles only the optional
high-tier post pass. There is no multiplayer or online leaderboard. See the
[technical guide](../../aura-drift.md) for details.
