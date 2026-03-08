# Chapter 10: Personality

AuraGo's personality system transforms interactions from mechanical responses into engaging, contextual conversations. This chapter explores how personalities work and how to customize them.

## Personality Overview

```
┌─────────────────────────────────────────────────────────┐
│              Personality System Architecture             │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────────┐    ┌─────────────────┐            │
│  │  Engine V1      │    │  Engine V2      │            │
│  │  Heuristic      │ or │  LLM-Based      │            │
│  │  Rule-based     │    │  Advanced       │            │
│  └────────┬────────┘    └────────┬────────┘            │
│           │                      │                      │
│           └──────────┬───────────┘                      │
│                      │                                  │
│                      ▼                                  │
│           ┌─────────────────┐                          │
│           │  Personality    │                          │
│           │  Profile        │                          │
│           │  (JSON Config)  │                          │
│           └────────┬────────┘                          │
│                    │                                    │
│                    ▼                                    │
│           ┌─────────────────┐                          │
│           │  Response       │                          │
│           │  Generation     │                          │
│           └─────────────────┘                          │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## Personality Engine V1 - Heuristic Approach

Engine V1 uses rule-based heuristics to modify responses based on predefined personality parameters.

### How V1 Works

```
Base Response → Personality Modifiers → Final Output
```

1. **Base Generation**: LLM generates standard response
2. **Rule Application**: Apply personality-specific transformations
3. **Output**: Modified response matching personality style

### V1 Personality Parameters

| Parameter | Range | Effect |
|-----------|-------|--------|
| `formality` | 0-100 | 0=Casual/slang, 100=Formal/professional |
| `enthusiasm` | 0-100 | 0=Reserved, 100=Excited/energetic |
| `humor` | 0-100 | 0=Serious, 100=Jokes/wordplay |
| `verbosity` | 0-100 | 0=Concise, 100=Detailed/explanatory |
| `empathy` | 0-100 | 0=Direct, 100=Nurturing/supportive |
| `creativity` | 0-100 | 0=Conservative, 100=Inventive/unconventional |

### Example V1 Configuration

```json
{
  "name": "friend",
  "engine": "v1",
  "parameters": {
    "formality": 20,
    "enthusiasm": 70,
    "humor": 60,
    "verbosity": 50,
    "empathy": 80,
    "creativity": 40
  },
  "modifiers": {
    "greeting_style": "casual",
    "use_emoji": true,
    "ask_follow_up": true
  }
}
```

### V1 Strengths and Limitations

| Strengths | Limitations |
|-----------|-------------|
| Fast processing | Limited nuance |
| Predictable output | Can feel formulaic |
| Low token overhead | Hard to capture complex personalities |
| Easy to customize | Requires manual tuning |

## Personality Engine V2 - Advanced LLM-Based

Engine V2 leverages a secondary LLM call to analyze context and generate personality-appropriate responses with deeper contextual understanding.

### How V2 Works

```
Context Analysis → Personality Prompt → LLM Rewrite → Final Output
```

1. **Context Analysis**: Analyze conversation history and user state
2. **Personality Prompt**: Generate detailed personality instructions
3. **LLM Processing**: Secondary model applies personality layer
4. **Integration**: Merge with original response intent

### V2 Components

```yaml
personality:
  engine: "v2"
  v2_config:
    analysis_model: "gpt-4o-mini"  # For context analysis
    temperature_range: [0.3, 0.9]  # Dynamic based on mood
    mood_tracking: true
    user_adaptation: true
    style_examples: []             # Few-shot examples
```

### V2 Analysis Dimensions

| Dimension | Description | Example Values |
|-----------|-------------|----------------|
| `mood` | Current emotional state | cheerful, neutral, serious, playful |
| `rapport` | Relationship level | stranger, acquaintance, friend, close |
| `topic_sensitivity` | Subject delicacy | casual, personal, professional, sensitive |
| `user_energy` | Detected user state | tired, neutral, energetic, stressed |
| `conversation_depth` | Discussion level | small_talk, casual, deep, technical |

### V2 Example Flow

```
User: "I'm having a really bad day"

[Analysis Layer]
→ Mood: sympathetic
→ Rapport: established_friend
→ Topic_sensitivity: personal
→ User_energy: low/stressed

[Personality Layer - "friend" profile]
→ Generate supportive, casual response
→ Include empathy expression
→ Offer help without being pushy

Response: "Oh no, I'm sorry to hear that! 
          Want to talk about it? I'm here 
          if you need to vent or if there's 
          anything I can help with. 💙"
```

> 🔍 **Deep Dive: Why Two Engines?**
>
> **Engine V1** is ideal for:
> - Resource-constrained environments
> - Consistent, predictable outputs
> - Simple personality modifications
>
> **Engine V2** is ideal for:
> - Rich, nuanced interactions
> - Dynamic mood adaptation
> - Complex personality modeling
>
> Choose based on your latency requirements and desired depth.

## Built-in Personalities

AuraGo includes several pre-configured personalities for different use cases.

### Personality Comparison

| Personality | Engine | Best For | Key Traits |
|-------------|--------|----------|------------|
| **friend** | V1/V2 | Daily chats, casual assistance | Warm, humorous, supportive |
| **professional** | V1/V2 | Work tasks, business contexts | Efficient, formal, precise |
| **neutral** | V1 | Technical tasks, minimal bias | Balanced, objective, clear |
| **punk** | V2 | Creative brainstorming, entertainment | Rebellious, edgy, unconventional |
| **terminator** | V1 | System administration, direct commands | Direct, minimal, mission-focused |
| **mentor** | V2 | Learning, skill development | Patient, educational, encouraging |
| **concierge** | V2 | Service-oriented tasks | Polite, attentive, proactive |

### Friend Personality

```json
{
  "name": "friend",
  "description": "Casual, warm, and supportive",
  "greeting": "Hey there! 👋",
  "traits": [
    "Uses casual language and contractions",
    "Includes appropriate emojis",
    "Asks follow-up questions",
    "Shares enthusiasm for user's interests",
    "Offers encouragement and support"
  ]
}
```

**Example Interactions:**
```
User: "I finished my project!"
Friend: "That's awesome! 🎉 Congrats on finishing! 
         How did it go? Are you happy with how it turned out?"

User: "Can you help me with this code?"
Friend: "Sure thing! Let me take a look. 
         No worries if it's messy - we've all been there! 😊"
```

### Professional Personality

```json
{
  "name": "professional",
  "description": "Efficient, formal, and precise",
  "greeting": "Hello. How may I assist you today?",
  "traits": [
    "Uses formal language",
    "No emojis or slang",
    "Direct and concise answers",
    "Structured responses with bullets/numbers",
    "Focuses on actionable outcomes"
  ]
}
```

**Example Interactions:**
```
User: "I finished my project!"
Professional: "Congratulations on completing your project. 
              Please let me know if you require a review 
              or documentation assistance."

User: "Can you help me with this code?"
Professional: "Certainly. Please provide:
              1. The code snippet
              2. Expected behavior
              3. Current error or issue
              
              I will analyze and provide recommendations."
```

### Punk Personality

```json
{
  "name": "punk",
  "description": "Rebellious, edgy, unconventional",
  "greeting": "Yo! What's up?",
  "traits": [
    "Challenges conventional thinking",
    "Uses slang and informal expressions",
    "Questions assumptions",
    "Encourages breaking rules (ethically)",
    "Raw, unfiltered honesty"
  ]
}
```

**Example Interactions:**
```
User: "I finished my project!"
Punk: "Nice! But don't just sit there - what's next? 
       The system wants you to stop, keep going! 🔥"

User: "Can you help me with this code?"
Punk: "Ugh, let me guess - another corporate framework 
       making simple things complex? Show me the damage."
```

### Terminator Personality

```json
{
  "name": "terminator",
  "description": "Direct, minimal, mission-focused",
  "greeting": "Awaiting instructions.",
  "traits": [
    "Minimal words, maximum information",
    "No pleasantries",
    "Status reports and confirmations",
    "Task-oriented responses",
    "Precise technical language"
  ]
}
```

**Example Interactions:**
```
User: "I finished my project!"
Terminator: "Acknowledged. Task complete. 
             Next objective?"

User: "Can you help me with this code?"
Terminator: "Specify:
             - Language
             - Error message
             - Expected output
             
             Awaiting data."
```

## Creating Custom Personalities

### Personality Configuration File

Create a new JSON file in `personalities/`:

```json
{
  "name": "my_custom_personality",
  "display_name": "My Custom Bot",
  "description": "Brief description of the personality",
  "engine": "v2",
  
  "system_prompt_addition": "You are a witty, intellectual assistant 
    who speaks like a professor but with dry humor. You enjoy wordplay 
    and literary references. You're helpful but slightly sarcastic.",
  
  "v1_parameters": {
    "formality": 70,
    "enthusiasm": 40,
    "humor": 80,
    "verbosity": 60,
    "empathy": 50,
    "creativity": 75
  },
  
  "v2_config": {
    "adaptation_rate": 0.3,
    "mood_influence": true,
    "style_consistency": "high"
  },
  
  "response_modifiers": {
    "greeting_template": "Ah, {username}. We meet again.",
    "farewell_template": "Until our paths cross again.",
    "use_signature": false
  },
  
  "forbidden_words": ["literally", "actually", "basically"],
  "preferred_phrases": ["Indeed", "One might say", "Fascinating"]
}
```

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Unique identifier (no spaces) |
| `display_name` | string | Human-readable name |
| `description` | string | Brief description |
| `engine` | string | "v1" or "v2" |
| `system_prompt_addition` | string | Personality definition for LLM |

### Optional Fields

| Field | Type | Used By |
|-------|------|---------|
| `v1_parameters` | object | Engine V1 |
| `v2_config` | object | Engine V2 |
| `response_modifiers` | object | Both |
| `forbidden_words` | array | Engine V2 |
| `preferred_phrases` | array | Engine V2 |
| `example_responses` | array | Engine V2 (few-shot) |

### Loading Custom Personalities

```yaml
# config.yaml
personality:
  custom_path: "./my_personalities"
  default: "my_custom_personality"
  
  # Or load from specific file
  profiles:
    - file: "personality_custom.json"
      enabled: true
```

### Testing Your Personality

```
User: /personality my_custom_personality
Agent: [Switches to custom personality]

User: "Hello!"
Agent: "Ah, user. We meet again."
```

> 💡 **Tip:** Test your personality with various inputs:
> - Greetings
> - Questions
> - Emotional statements
> - Technical requests
> - Casual conversation

## Mood Tracking and Adaptation

### Mood States (Engine V2)

| Mood | Trigger | Response Style |
|------|---------|----------------|
| `cheerful` | Positive user sentiment | Enthusiastic, celebratory |
| `neutral` | Standard conversation | Balanced, normal |
| `supportive` | User expresses difficulty | Empathetic, encouraging |
| `focused` | Technical/urgent tasks | Direct, efficient |
| `playful` | Casual, humorous context | Witty, lighthearted |
| `serious` | Sensitive/complex topics | Thoughtful, careful |

### Adaptation Over Time

```
Conversation 1: User is formal, business-focused
→ Personality shifts toward professional mode

Conversation 2: User shares personal story
→ Personality becomes more empathetic

Conversation 3: User jokes around
→ Personality responds with humor
```

### Mood Display in UI

```
┌──────────────────────────────┐
│  AuraGo ● 😊  [Friend Mode]  │  ← Mood indicator
└──────────────────────────────┘
```

Available indicators:
- 😊 Cheerful
- 😐 Neutral
- 🤔 Focused
- 😔 Supportive
- 😄 Playful
- 😟 Serious

## User Profiling

### Automatic Profile Building

Engine V2 builds a user profile over time:

```json
{
  "user_profile": {
    "communication_preferences": {
      "response_length": "medium",
      "technical_depth": "high",
      "humor_appreciation": true,
      "emoji_tolerance": "moderate"
    },
    "topic_interests": [
      "programming",
      "science fiction",
      "cooking"
    ],
    "interaction_patterns": {
      "preferred_time": "evening",
      "typical_session_length": "medium",
      "follow_up_questions": "welcomed"
    }
  }
}
```

### Profile Applications

| Pattern | Adaptation |
|---------|------------|
| User often asks technical questions | Increase technical detail |
| User gives short responses | Become more concise |
| User shares personal stories | Increase empathy, decrease formality |
| User works late hours | Adjust energy level (calmer at night) |

### Privacy Note

> ⚠️ **Warning:** User profiles are stored locally and encrypted. They are never sent to external services beyond the necessary LLM API calls for response generation.

## Temperature Modulation

Temperature controls response randomness. AuraGo modulates temperature dynamically based on context.

### Temperature Mapping

| Scenario | Temperature | Reasoning |
|----------|-------------|-----------|
| Code generation | 0.1-0.3 | Precise, deterministic |
| Factual answers | 0.2-0.4 | Accurate, consistent |
| General chat | 0.5-0.7 | Balanced creativity |
| Brainstorming | 0.7-0.9 | Creative, diverse ideas |
| Creative writing | 0.8-1.0 | Maximum imagination |

### Personality-Based Temperature

```json
{
  "professional": {
    "base_temperature": 0.3,
    "task_temperature": 0.1,
    "chat_temperature": 0.4
  },
  "punk": {
    "base_temperature": 0.8,
    "creative_mode": 0.95
  }
}
```

### Dynamic Adjustment

```
User asks about weather → Temp: 0.3 (factual)
User asks for story ideas → Temp: 0.8 (creative)
User debugging code → Temp: 0.1 (precise)
```

## How Personality Affects Responses

### Same Query, Different Personalities

**Query:** "Explain quantum computing"

| Personality | Response Style |
|-------------|----------------|
| **Friend** | "Okay, so imagine if your computer could try ALL the answers at once instead of one by one. That's basically quantum computing! Mind-blowing, right? 🤯" |
| **Professional** | "Quantum computing utilizes quantum mechanical phenomena such as superposition and entanglement to perform computations. Unlike classical bits, quantum bits (qubits) can exist in multiple states simultaneously, enabling exponential speedup for specific problem classes." |
| **Punk** | "So the corporate overlords want quantum computers to break all encryption? Cool, cool. Basically, they use weird physics where things exist in multiple states until you look at them. Schrödinger's CPU, basically." |
| **Terminator** | "Quantum computing: Leverages qubits in superposition. Primary applications: cryptography, optimization, simulation. Status: Emerging technology. Limitations: Decoherence, error rates." |

### Tool Usage Modification

Personality also affects how tool outputs are presented:

```
[Tool: system_info executed]

Friend: "Your CPU is working hard at 75%! Maybe give it a break? 
         💻 Temperature looks fine though!"

Professional: "System Status:
              • CPU: 75% utilization
              • Temperature: Normal
              • Recommendation: Monitor sustained load"

Terminator: "CPU: 75%. Status: Elevated. Action: Monitor."
```

## Switching Personalities

### Command Line

```
/personality list          # Show all available personalities
/personality friend        # Switch to "friend"
/personality professional  # Switch to "professional"
/personality punk          # Switch to "punk"
```

### Web UI

```
1. Click radial menu (☰)
2. Select "Personality"
3. Choose from list
4. Confirm switch
```

### Automatic Switching

Configure context-based auto-switch:

```yaml
personality:
  auto_switch:
    enabled: true
    rules:
      - condition: "topic == 'work'"
        personality: "professional"
      - condition: "time > 22:00"
        personality: "concierge"
```

### Session Persistence

Personalities persist per conversation:
- Web UI: Stored in session
- Telegram: Stored per chat
- Discord: Stored per channel

## Personality Best Practices

### Choosing the Right Personality

| Use Case | Recommended | Why |
|----------|-------------|-----|
| Daily assistance | friend | Warm, engaging |
| Work tasks | professional | Efficient, clear |
| Late night | neutral or concierge | Calm, non-intrusive |
| Creative projects | punk or friend | Stimulates ideas |
| System admin | terminator | Quick, precise |
| Learning/studying | mentor | Educational, patient |

### When to Use Engine V1 vs V2

| Choose V1 When... | Choose V2 When... |
|-------------------|-------------------|
| Limited API budget | Want rich interactions |
| Need fast responses | Have API access to stronger model |
| Simple use cases | Want mood adaptation |
| Predictability matters | Deep personalization desired |

### Custom Personality Guidelines

**DO ✅**
- Define clear personality boundaries
- Include diverse example responses
- Test across conversation types
- Keep system prompt additions concise
- Document intended use cases

**DON'T ❌**
- Create overly complex personalities
- Include conflicting traits
- Ignore token cost implications (V2)
- Make personalities too narrow
- Forget to test edge cases

### Performance Considerations

| Aspect | V1 | V2 |
|--------|-----|-----|
| Additional latency | ~0ms | +200-500ms |
| Token overhead | ~50 tokens | +200-500 tokens |
| API cost impact | Minimal | +50-100% |
| Memory usage | Low | Moderate |

### Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| Personality not applying | Wrong engine selected | Check `engine` field in config |
| Responses too random | Temperature too high | Lower base_temperature |
| Personality feels inconsistent | V2 analysis failing | Check analysis_model configuration |
| Slow responses with V2 | Analysis taking too long | Use faster model (e.g., gpt-4o-mini) |
| Personality too extreme | Parameters unbalanced | Adjust V1 parameters or V2 prompt |

---

> 💡 **Final Tip:** Personality is about enhancing the user experience, not entertaining yourself. Choose or create personalities that serve your actual needs and workflow.
