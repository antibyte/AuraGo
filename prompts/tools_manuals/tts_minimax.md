---
id: "tool_tts_minimax"
tags: ["tool", "conditional"]
priority: 51
conditions: ["minimax_tts_enabled"]
---
## MiniMax TTS Speech Control Tags (speech-2.8-hd and speech-2.8-turbo only)
Use these inline in the `text` parameter to control speech dynamics:

| Tag | Effect |
|------|--------|
| `<#1.5#>` | Pause for 1.5 seconds (replace number with desired duration) |
| `(laughs)` | Laughter |
| `(chuckle)` | Chuckle |
| `(coughs)` | Coughing |
| `(clear-throat)` | Clearing throat |
| `(groans)` | Groaning |
| `(breath)` | Breathing sound |
| `(pant)` | Panting |
| `(inhale)` | Inhale |
| `(exhale)` | Exhale |
| `(gasps)` | Gasp |
| `(sniffs)` | Sniff |
| `(sighs)` | Sigh |
| `(snorts)` | Snort |
| `(burps)` | Burp |
| `(lip-smacking)` | Lip smacking |
| `(humming)` | Humming |
| `(hissing)` | Hissing |
| `(emm)` | Thinking sound ("emm") |
| `(sneezes)` | Sneeze |

Example: `"Hello! <#0.5#> (laughs) That's great news."`
