# Chapter 10: Personality

AuraGo offers a personality system that influences the AI's behavior and communication style.

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

AuraGo has two personality engines that can be activated independently:

### Personality Engine V1 (Heuristic)

The V1 engine uses predefined prompt templates without additional LLM calls.

**Configuration:**
```yaml
# config.yaml
personality:
  engine: true
  core_personality: "friend"
```

### Personality Engine V2 (LLM-based)

> ⚠️ **Requires additional API calls:** The V2 engine analyzes mood and context with a separate LLM call.

The V2 engine provides:
- Dynamic mood analysis
- Automatic temperature modulation
- User profiling (optional)

**Configuration:**
```yaml
# config.yaml
personality:
  engine: true
  engine_v2: true
  v2_provider: ""                    # deprecated – V2 now uses llm.helper_*
  user_profiling: false
  user_profiling_threshold: 2
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
    error_streak_min: 2
```

> ⚠️ **Note:** `v2_provider` is deprecated. The V2 engine now uses the Helper LLM configuration (`llm.helper_enabled`, `llm.helper_provider`, `llm.helper_model`). See [Chapter 9: Helper LLM](./09-memory.md#helper-llm--automated-maintenance).

### Disabling Both Engines

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

#### Via Config

```yaml
personality:
  core_personality: "professional"
```

> 💡 Changes require a restart of AuraGo.

#### Via Web UI

1. Open the web interface
2. Go to "Config"
3. Search for `personality.core_personality`
4. Select a personality from the dropdown
5. Save and restart

---

## User Profiling (V2)

When `personality.user_profiling: true` is set, AuraGo automatically learns:

- Preferred level of detail (technical vs. general)
- Programming languages and tools
- Communication style
- Experience level

**Example - Learned Preferences:**
```
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

The inner voice is a subconscious nudge engine that injects brief, private agent thoughts into the system prompt. It adds subtle behavioral hints without extra user-visible messages.

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | `false` | Enable inner voice generation |
| `min_interval_secs` | `60` | Minimum seconds between inner voice thoughts |
| `max_per_session` | `20` | Maximum inner voice thoughts per session |
| `decay_turns` | `3` | Thought expires after N conversation turns |
| `error_streak_min` | `2` | Minimum consecutive errors before error-streak trigger |

**Requirements:** `personality.engine_v2: true`, `personality.emotion_synthesizer.enabled: true`, and Helper LLM enabled. Inner voice is explicit opt-in and is not auto-enabled with the emotion synthesizer.

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

```
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
| **Inner Voice** | `personality.inner_voice.enabled: true` | Subconscious behavioral nudges |

> 💡 **Pro Tip:** Start with V1 and `personality.core_personality: friend` or `professional`. Enable V2 only when you need dynamic adaptation and have the additional API budget. Configure a cost-efficient Helper LLM before enabling V2, emotion synthesis, or inner voice.

---

**Previous Chapter:** [Chapter 9: Memory & Knowledge](./09-memory.md)  
**Next Chapter:** [Chapter 11: Mission Control](./11-missions.md)
