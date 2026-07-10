# Precision Slate Palette

## Goal

Replace the green/teal Precision Workspace color family with a restrained
slate-blue palette. The result should feel like a focused companion to the
Virtual Desktop's standard dark surfaces without copying its orange accent.

## Scope

- Apply only to Precision consumers under `.pw-page`: Config, the ten
  operational workspaces, Login, Setup, and 404.
- Keep Web Chat, Virtual Desktop, Gallery, shared assets, fonts, routes,
  DOM hooks, REST behavior, density behavior, and information architecture
  unchanged.
- No new user-facing text or translations are required.

## Token design

Dark Precision uses a cool slate progression:

- canvas `#10161e`; surface `#18212b`; elevated surface `#202b37`; soft
  surface `#2a3745`.
- text `#edf2f7`; muted `#aab7c4`; subtle `#7d8b99`.
- steel-blue accent `#6f98bd`, with hover/strong `#91b5d6`.

Light Precision keeps the same hue family:

- canvas `#eef2f6`; surface `#fbfcfe`; elevated surface `#f1f5f9`; soft
  surface `#e3eaf1`.
- text `#182431`; muted `#5f6f7f`; subtle `#7b8997`.
- steel-blue accent `#426d93`, with hover/strong `#5d87aa`.

Existing semantic danger, warning, and success meanings remain recognizable.
Their presentation continues to use Precision semantic tokens rather than
introducing a new accent color.

## Implementation

Update the shared Precision token definitions and their compatibility aliases
(`--bg-*`, `--text-*`, `--accent`, and border mixes) in
`ui/css/precision-workspace.css`. Page styles continue to consume those tokens;
no page-specific palette overrides and no changes to protected Desktop/Chat
assets are allowed.

## Verification

- Add a static UI contract for the dark and light slate-blue tokens and for
  the absence of the former green/teal accent values in the Precision token
  source.
- Run the focused UI contract, all changed JavaScript through `node --check`,
  `go test -count=1 ./ui/...`, and the Precision browser smoke matrix in dark
  and light themes.
- Confirm the protected Desktop/Chat/Gallery/shared-asset diff remains empty.
