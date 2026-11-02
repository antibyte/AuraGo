# Chapter 10: Personality

<p align="center">
  <a href="../../../assets/readme/persona-party.webp"><img src="../../../assets/readme/persona-party.webp" width="640" alt="Ten AuraGo personalities as an illustrated group"></a>
</p>

Same toolbox. Different attitude. Permissions do not come from the persona.

---

## Table of Contents

1. [Overview](#overview)
2. [Personality Engine](#personality-engine)
3. [Mood States (V1/V2)](#mood-states-v1v2)
4. [Available Personalities](#available-personalities)
5. [User Profiling (V2)](#user-profiling-v2)
6. [Temperature Modulation (V2)](#temperature-modulation-v2)
7. [Emotion Synthesizer (V2)](#emotion-synthesizer-v2)
8. [Inner Voice (V2)](#inner-voice-v2)
9. [Persona Booster](#persona-booster)
10. [Adaptive Dynamics](#adaptive-dynamics)
11. [Example Comparison](#example-comparison)
12. [Best Practices](#best-practices)
13. [Troubleshooting](#troubleshooting)
14. [Summary](#summary)

---

## Overview

Personality influences:

| Aspect | Description |
|--------|-------------|
| **Tone** | Formal, casual, sarcastic, friendly |
| **Response Length** | Short and concise vs. detailed |
| **Emojis** | Usage and type of emojis |
| **Language Style** | Technical, colloquial, poetic |

---

## Personality Engine

AuraGo has a local personality engine with optional asynchronous V2 enrichment:

### Personality Engine V1 (Heuristic)

The V1 engine uses predefined prompt templates without additional LLM calls.

**Configuration:**

### Web UI Setup
1. Open **Config → Personality**.
2. Enable **Personality Engine V1** and select a **Core Personality** profile.
3. Save and restart if prompted.

### YAML Reference
```yaml
# config.yaml
personality:
  engine: true
  core_personality: "friend"
```

### Personality Engine V2 (LLM-Based)

> [!WARNING]
> **Requires additional API calls:** The V2 engine analyzes mood and context with a separate LLM call.

The V2 engine provides:
- Dynamic mood analysis
- Automatic temperature modulation
- User profiling (optional)

**Configuration:**

### Web UI Setup
1. Open **Config → Personality** and enable **Personality Engine V2**.
2. Open **Config → LLM Settings** and enable the **Helper LLM** (recommended; a bounded main-model fallback is available).
3. Optionally enable **User Profiling**, **Emotion Synthesizer**, and **Inner Voice** on the Personality page.
4. Save changes.

### YAML Reference
```yaml
# config.yaml
personality:
  engine: true
  engine_v2: true
  v2_provider: ""                    # deprecated – V2 now uses llm.helper_*
  user_profiling: false
  user_profiling_threshold: 3
  emotion_synthesizer:
    enabled: false
    min_interval_seconds: 60
    max_history_entries: 100
    trigger_on_mood_change: true
    trigger_always: false
  inner_voice:
    enabled: false                   # requires emotion_synthesizer + engine_v2
    min_interval_secs: 60
    max_per_session: 20
    decay_turns: 3
    decay_max_age_secs: 300
```

> [!NOTE]
> `v2_provider` is deprecated. The V2 engine now uses the Helper LLM configuration (`llm.helper_enabled`, `llm.helper_provider`, `llm.helper_model`). See [Chapter 9: Helper LLM](./09-memory.md#helper-llm--automated-maintenance).

### Disabling Both Engines

### Web UI Setup
1. Open **Config → Personality**.
2. Disable **Personality Engine V1** and **Personality Engine V2**.
3. Save changes.

### YAML Reference
```yaml
personality:
  engine: false
  engine_v2: false
```

---

## Mood States (V1/V2)

The personality engine tracks the agent's current mood. V1 uses heuristic keyword/emoji detection; V2 can refine mood via the Helper LLM.

| Mood | Typical Trigger | Behavioral Effect |
|------|-------------------|-------------------|
| `curious` | Questions, exploration requests | Neutral temperature; encourages follow-up |
| `focused` | Positive feedback, working state | Slightly lower temperature; decisive |
| `creative` | Brainstorming, design requests | Higher temperature; unconventional ideas |
| `analytical` | "Why?", comparisons, deep dives | Lower temperature; thorough analysis |
| `cautious` | Tool errors, negative feedback | Lower temperature; double-checks actions |
| `playful` | Humor, jokes, casual banter | Higher temperature; light tone |
| `frustrated` | Repeated failures, user frustration | Lower temperature; asks for clarification |
| `concerned` | Risk, worry, uncertainty | Careful, explicit about concerns |
| `relaxed` | Low-pressure, satisfied interactions | Slightly higher temperature; conversational |

Default mood when no history exists: `curious`.

---

## Available Personalities

| Profile | Description | Ideal For |
|---------|-------------|-----------|
| `neutral` | Objective, balanced | General tasks, technical documentation |
| `friend` | Warm, supportive, informal "you" | Personal conversations, everyday tasks |
| `professional` | Polite, efficient, formal | Business contexts, formal communication |
| `punk` | Rebellious, direct, unconventional | Creative projects, brainstorming |
| `terminator` | Extremely short, direct, no fluff | Quick information, command-line mode |
| `psycho` | Chaotic, unpredictable, neurotic | Experiments, entertainment |
| `mcp` | Master Control Program (TRON-style), cold, imperious | System monitoring, authoritative mode |
| `secretary` | Efficient, proactive, organized | Task management, scheduling |
| `servant` | Extremely submissive, obedient | Roleplay, entertainment |
| `thinker` | Analytical, philosophical, questioning | Deep analysis, complex problems |
| `evil` | Megalomaniac, theatrical, domineering | Humorous interactions, roleplay |
| `mistress` | Dominant, strict, uncompromising | Roleplay, disciplined interactions |

### Switching Personality

#### Via Web UI (Recommended)

1. Open **Config → Personality**.
2. Select **Core Personality** from the dropdown (e.g. `professional`).
3. Save and restart AuraGo.

#### YAML Reference

```yaml
personality:
  core_personality: "professional"
```

> [!TIP]
> Changes require a restart of AuraGo.

> 🎭 **Animated personas:** In Webchat and Desktop **Live Speech**, every personality has an animated persona (Rive) that listens, thinks and moves its mouth while speaking — from butler to terminator. `custom` keeps its static image. Details: [chapter 4](04-webui.md#animated-personas-in-live-speech) and the [animated personas deep dive](../../animated-personas.md).

---

## User Profiling (V2)

When `personality.user_profiling: true` is set (**Config → Personality**), AuraGo automatically learns:

- Preferred level of detail (technical vs. general)
- Programming languages and tools
- Communication style
- Experience level

**Example - Learned Preferences:**
```text
User: Can you help me with the Python script?

[AuraGo learned: User uses Python]

Later:
User: How do I best solve this?
Agent: In Python, you could use a dictionary comprehension for this...
```

**Privacy:**
- Profile data is stored locally
- No transmission to external servers
- Can be disabled at any time

---

## Temperature Modulation (V2)

The V2 engine can dynamically adjust the LLM temperature:

| Situation | Temperature | Reasoning |
|-----------|-------------|-----------|
| Fact queries | Lower | Precision is important |
| Code generation | Lower | Deterministic |
| Brainstorming | Higher | Creativity desired |
| Conversation | Medium | Balance |

**Configuration:**

> 🖥️ **Web UI:** Set the base temperature under **Config → LLM Settings**.

### YAML Reference
```yaml
llm:
  temperature: 0.7  # Base temperature
```

The V2 engine modulates around this base value based on context.

---

## Emotion Synthesizer (V2)

When `personality.emotion_synthesizer.enabled: true`, the Helper LLM generates a structured emotion state after mood changes (or every turn with `trigger_always: true`). AuraGo stores a short natural-language emotion note and exposes it in the chat widget and dashboard.

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | `false` | Enable emotion synthesis |
| `min_interval_seconds` | `60` | Minimum interval between synthesis runs |
| `max_history_entries` | `100` | Maximum emotion history entries to keep |
| `trigger_on_mood_change` | `true` | Synthesize when V2 detects a mood change |
| `trigger_always` | `false` | Synthesize on every message |

**Requirements:** `personality.engine_v2: true` and Helper LLM enabled (`llm.helper_enabled: true`).

---

## Inner Voice (V2)

The inner voice is a brief, private agent thought. After a chat turn, the helper writes at most one short first-person thought, or none when nothing is worth noting. The thought is injected into the next system prompt as untrusted data that may influence tone only, never intent, safety or tool use. It is left out during tool-error recovery. While the thought is injected, its category (for example `caution` or `focus`) selects one of twelve fixed stance hints written by AuraGo itself, not by the model, which goes into the trusted PERSONA STATE section of the prompt. The hints only adjust working style, such as naming the main risk or keeping a calm step-by-step tone, and never change tools, confirmations, intent or safety; unknown categories add no hint.

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | `false` | Enable inner voice generation |
| `min_interval_secs` | `60` | Minimum seconds between inner voice thoughts |
| `max_per_session` | `20` | Maximum inner voice thoughts per session |
| `decay_turns` | `3` | Thought expires after N user turns |
| `decay_max_age_secs` | `300` | Absolute age ceiling in seconds, applies even between turns |

`error_streak_min` is retired and ignored. Older config files still load, with a deprecation notice.

**Requirements:** `personality.engine_v2: true`, `personality.emotion_synthesizer.enabled: true`. A configured Helper LLM is preferred; without one, a bounded main-model fallback is used and charged to the personality budget. Inner voice is explicit opt-in and is not auto-enabled with the emotion synthesizer.

---

## Persona Booster

The persona booster makes the selected persona noticeably more distinct. It works on two levels:

- **Base amplifier:** every persona has a fixed extra paragraph that sharpens its voice for good. Punk swears more freely and talks back, Evil gets more pompous, Psycho jumpier. With `intensity: mild` the paragraph is left out.
- **Phases:** from time to time the persona goes through a phase, for example Punk `rage`, Thinker `existential` or Friend `worried`. A phase lasts a number of minutes or turns. It ends when its time runs out, when a counter-trigger arrives (such as praise) or when a stronger trigger replaces it.

Phases are triggered by the time of day (0–4 am), a long absence (≥ 24 h), long sessions (≥ 2 h or ≥ 40 messages), simple text cues (capital letters, "!!!", swearing, "stop it") and chance. With the personality engine on, mood signals are added. With engine V2 the helper analysis also supplies a context cue (provocation, criticism, praise, distress and so on). No extra LLM call is made.

In some phases the edge may also be aimed at you, in the persona's own style: Punk crude but humorous, Evil condescending, Mistress cutting, Friend at most sulky. Fixed limits always apply: no slurs or attacks on identity, no real threats, no self-harm references, no sexual content, no conspiracy claims about real people. Facts, code, tool rules and confirmations stay exact. If you seem distressed or write "stop it" or "calm down", a hostile phase ends at once and stays blocked for 6 hours.

The phase is invisible: the chat shows no indicator. The log records `[PersonaBooster] phase started|ended|replaced` with persona, phase, variant and trigger. Missions, co-agents, autonomous runs and the neutral persona get no phases.

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | `true` | Enable the base amplifier and phases |
| `intensity` | `strong` | `mild` (no base amplifier, phases ×0.6), `strong` (phases ×1) or `wild` (phases ×1.5, more extreme) |
| `spontaneous` | `true` | Allow random phases |
| `cooldown_minutes` | `20` | For this long after a phase, only strong triggers (criticism, provocation, distress) start a new one (0–1440) |

### YAML Reference

```yaml
personality:
  persona_booster:
    enabled: true
    intensity: strong
    spontaneous: true
    cooldown_minutes: 20
```

### Custom phases

The catalogs are embedded as `prompts/persona_phases/<persona>.yaml`. A file with the same name in `<prompts_dir>/persona_phases/` overrides the built-in catalog. An invalid file is skipped with a warning in the log. Custom personas without a catalog use `_generic.yaml`. Which generic phases they get depends on `volatility`, `empathy_bias` and `conflict_response` in the profile.

---

## Adaptive Dynamics

The personality engine maintains transactional dynamics values that describe how the character changes while working with you:

| Value | Meaning |
|-------|---------|
| **Load** | Rises through criticism, conflict, and demanding interactions |
| **Familiarity** | Grows through repeated, positive interaction |
| **Friction** | Short-lived tension that decays over time |

The values are written transactionally with revision guards; load and friction decay over time while familiarity grows. They influence voice and response style live. **Trend:** steady, recovering, or strained.

### Web UI Setup

The Dashboard shows the current dynamics on the **Personality** card (**Personality** section, `#personality`) with a trend badge; **Config → Personality** renders the same block. The **Reset current dynamics** button zeroes the dynamics and returns affect to rest values without changing traits, lived notes, history, or the active persona (epoch and revision increase, in-flight results are invalidated). The reset is a same-origin POST with normal session authentication and requires the personality engine to be active.

### API Reference

```bash
GET  /api/personality/state            # includes dynamics: load, familiarity, friction, trend, revision
POST /api/personality/dynamics/reset   # zero dynamics, return affect to rest
```

---

## Example Comparison

**Same request, different personalities:**

| Personality | Response |
|-------------|----------|
| **terminator** | `Error in line 42. Variable 'x' not defined.` |
| **professional** | `Upon reviewing your code, I found an error. In line 42, the variable 'x' is used without being defined first.` |
| **friend** | `Oh, there's a small error! 😅 In line 42, you're trying to access 'x', but you haven't defined it first. No problem, happens to everyone!` |
| **punk** | `Dude, someone fell asleep! 😂 Line 42: 'x' doesn't exist in nirvana! You gotta breathe life into that variable first! 🤘` |

---

## Best Practices

### Choosing the Right Personality

```text
Use Case → Recommended Personality
─────────────────────────────────────────────
Customer Support   → professional
Code Review        → neutral
Brainstorming      → punk
Learning/Coaching  → friend
Quick Info         → terminator
```

### What to Avoid

| ❌ Anti-Pattern | Reasoning |
|-----------------|-----------|
| `punk` for formal documents | Unprofessional |
| `terminator` for first contact | Too cold |
| V2 without API budget | Additional costs |

---

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Personality is ignored | `personality.engine: false` | Set to `true` |
| No mood adaptation | V2 disabled | Set `personality.engine_v2: true` |
| High API costs | V2 with expensive model | Choose cheaper model for V2 |

---

## Summary

| Feature | Configuration | Recommended Usage |
|---------|--------------|-------------------|
| **V1 Engine** | `personality.engine: true` | Standard, low cost |
| **V2 Engine** | `personality.engine_v2: true` | Dynamic adaptation |
| **Base Personality** | `personality.core_personality` | Style selection |
| **User Profiling** | `personality.user_profiling: true` | Personalization |
| **Emotion Synthesizer** | `personality.emotion_synthesizer.enabled: true` | Natural-language emotion notes |
| **Inner Voice** | `personality.inner_voice.enabled: true` | Private thought that colors tone and working style |

> [!TIP]
> Start with V1 and `personality.core_personality: friend` or `professional`. Enable V2 only when you need dynamic adaptation and have the additional API budget. Configure a cost-efficient Helper LLM before enabling V2, emotion synthesis, or inner voice.

*End of Part 2 — The Toys. On to Part 3: Autopilot.*

---

**Previous Chapter:** [Chapter 9: Memory & Knowledge](./09-memory.md)
**Next Chapter:** [Chapter 11: Mission Control](./11-missions.md)

Defaults: `engine: false`, `engine_v2: false`, `core_personality: neutral`, profiling threshold `3`. Enabling Inner Voice also enables synthesis, V2 and the engine. `decay_max_age_secs` bounds how long a thought stays in the prompt, even between turns. See [provenance, deletion and legacy review](../../personality-dynamics.md).
