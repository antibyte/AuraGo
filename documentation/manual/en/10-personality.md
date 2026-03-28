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
  v2_provider: ""
  user_profiling: false
  user_profiling_threshold: 3
  v2_timeout_secs: 30
  emotion_synthesizer:
    enabled: true
    trigger_on_mood_change: true
```

### Disabling Both Engines

```yaml
personality:
  engine: false
  engine_v2: false
```

---

## Available Personalities

| Profile | Description | Ideal For |
|---------|-------------|-----------|
| `neutral` | Objective, balanced | General tasks, technical documentation |
| `friend` | Warm, supportive, informal "you" | Personal conversations, everyday tasks |
| `professional` | Polite, efficient, formal | Business contexts, formal communication |
| `punk` | Rebellious, direct, unconventional | Creative projects, brainstorming |
| `terminator` | Extremely short, direct, no fluff | Quick information, command-line mode |
| `psycho` | Chaotic, unpredictable | Experiments, entertainment |
| `mcp` | Focus on Model Context Protocol | MCP server interactions |

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

The V2 engine modulates around this base value based on context. If the Emotion Synthesizer is enabled, AuraGo also stores a short natural-language emotion note and exposes it in the chat widget and dashboard.

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

> 💡 **Pro Tip:** Start with V1 and `personality.core_personality: friend` or `professional`. Enable V2 only when you need dynamic adaptation and have the additional API budget.

---

**Previous Chapter:** [Chapter 9: Memory & Knowledge](./09-memory.md)  
**Next Chapter:** [Chapter 11: Mission Control](./11-missions.md)
