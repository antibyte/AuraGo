# Storymaker phase 2: measured

## Motion lint rates (2A, real catalog, no audio)

| Story | Scenes | line_unread | dead_stretch | first_frame_empty | text_too_small | fade_between_worlds | entry_monotony | transition_variety | cue_late |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| firewall | 3 | 0 | 0 | 0 | 0 | 1 | 0 | 0 | 0 |
| minimal | 1 | 0 | 0 | 1 | 0 | 0 | 0 | 0 | 0 |
| castsheet | 27 | 17 | 0 | 0 | 0 | 0 | 1 | 0 | 0 |
| all | 31 | 17 | 0 | 1 | 0 | 1 | 1 | 0 | 0 |

Rate per scene: line_unread 0.548, dead_stretch 0.000, first_frame_empty 0.032, text_too_small 0.000, fade_between_worlds 0.032, entry_monotony 0.032, transition_variety 0.000, cue_late 0.000. Cued beats: 1, with cue_late: 0.

## Cue share, batch 2B (spec line 853)

One scene per language, with the marker at the second sentence. The values are `cue_precision`.

language | measured | speech_ok false | chars off by one | no audio
--- | --- | --- | --- | ---
de | 1 | 0 | 0 | 0
el | 1 | 0 | 0 | 0
hi | 1 | 0 | 0 | 0
ja | 1 | 0 | 0 | 0

These blocks are hand-built, so the table proves the rule and its fallback on de, el, hi and ja. It does not measure a provider. A real voice's share can be read from `cue_precision` in the compile report.
