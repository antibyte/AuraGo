# Chapter 5: Chat Basics

Effective communication with AuraGo – from simple messages to complex workflows.

## How to Communicate Effectively with AuraGo

AuraGo understands natural language, but a few principles help you get the best results:

### Be Specific

| ❌ Vague | ✅ Specific |
|----------|-------------|
| "Do something with files" | "List all PDF files in /documents and sort by size" |
| "Check the system" | "Show CPU usage and available RAM" |
| "Fix it" | "Restart the Docker container named 'nginx'" |

### Provide Context

```
You: Create a backup script
Agent: [Creates generic script]

You: Create a backup script for my PostgreSQL database 
      that runs daily at 2 AM and keeps 7 days of backups
Agent: [Creates specific, targeted script]
```

### Use Step-by-Step for Complex Tasks

For multi-part tasks, break them down:

```
You: First, find all log files larger than 100MB
Agent: Found 3 files: /var/log/syslog.1, /var/log/nginx/access.log

You: Now compress them and move to /backup/logs
Agent: Compressed and moved files

You: Finally, create a summary of what was done
Agent: Summary created
```

## Message Types and Formats

### Plain Text Messages

Standard text input works for most interactions:

```
You: What is the weather like today?
You: Explain Docker containers
You: Write a Python function to calculate fibonacci numbers
```

### Code Blocks

Share code for analysis or modification:

<pre>
You: ```python
def calculate_total(items):
    return sum(item['price'] for item in items)
```
Can you add error handling to this function?
</pre>

### Structured Data

Provide data in JSON, YAML, or Markdown:

<pre>
You: ```json
{
  "server": "192.168.1.100",
  "port": 8080,
  "ssl": true
}
```
Generate a curl command from this config
</pre>

### Commands with Parameters

Some interactions work best with clear parameter style:

```
You: Search web query="Go 1.21 release date" max_results=5
You: Create file path="config.yml" content="[YAML content]"
```

> 💡 **Tip:** AuraGo is flexible. Experiment to find the style that works best for your use case.

## File Uploads and Handling

### Uploading Files in Web UI

1. **Click** the paperclip icon 📎 below the chat input
2. **Select** one or more files
3. **Add context** in your message
4. **Send**

### Supported File Types

| Category | Formats | Use Cases |
|----------|---------|-----------|
| **Documents** | PDF, DOCX, TXT, MD | Analysis, summarization, extraction |
| **Code Files** | GO, PY, JS, TS, JSON, YAML | Review, debugging, refactoring |
| **Images** | JPG, PNG, GIF, WebP | Vision analysis, OCR, description |
| **Data** | CSV, XLSX, XML | Processing, conversion, reporting |
| **Archives** | ZIP, TAR (extracted) | Content inspection |

### Upload Examples

**Document Analysis:**
```
You: [Upload: report.pdf]
You: Summarize the key findings in this report
Agent: 📄 Summary: The report identifies three main trends...
```

**Code Review:**
```
You: [Upload: main.go]
You: Review this code for security issues
Agent: 🔍 Code Review: Found 2 potential issues...
```

**Data Processing:**
```
You: [Upload: sales.csv]
You: Calculate total revenue by month
Agent: 📊 Monthly Revenue:
   January: $12,450
   February: $15,230
   ...
```

> 💡 **Tip:** Large files are automatically summarized. For specific sections, mention page numbers or line ranges.

## Image Analysis

AuraGo can analyze images using vision capabilities. This works in both Web UI and Telegram.

### How to Use Image Analysis

**Web UI:**
1. Upload an image (JPG, PNG)
2. Ask your question about the image

```
You: [Upload: screenshot.png]
You: What error message do you see in this screenshot?
Agent: 🔍 I can see an error dialog showing "Connection timeout"...
```

**Telegram:**
1. Send or forward an image
2. Add caption with your question

### Use Cases for Image Analysis

| Scenario | Example Query |
|----------|---------------|
| **Screenshots** | "What error is shown?" / "Explain this UI element" |
| **Documents** | "Extract text from this scanned page" |
| **Diagrams** | "Explain this architecture diagram" |
| **Photos** | "What's in this image?" / "Identify this object" |
| **Charts** | "What trend does this graph show?" |
| **Code Screenshots** | "Convert this code snippet to text" |

> 🔍 **Deep Dive:** Vision processing uses multimodal LLM capabilities. The image is encoded and analyzed alongside your text query. Results depend on the LLM model's vision capabilities.

## Voice Messages (Telegram)

When using AuraGo via Telegram, you can send voice messages for hands-free interaction.

### How Voice Messages Work

1. **Record** a voice message in Telegram
2. **Send** it to the AuraGo bot
3. **Receive** text transcription + agent response
4. **Optional:** Get voice response (if TTS is enabled)

### Voice Message Flow

```
[You: 🎤 Voice message]
      ↓
[Transcription] → "What's the system CPU usage?"
      ↓
[Agent processes request]
      ↓
[Text response] → "💻 CPU Usage: 23% across 4 cores"
[Optional TTS] → 🔊 Voice response
```

### Best Practices for Voice

| Do | Don't |
|----|-------|
| Speak clearly and at moderate pace | Mumble or speak too fast |
| Use in quiet environments | Send from noisy locations |
| Keep messages under 1 minute for best results | Record very long monologues |
| Mention context explicitly | Assume previous context is known |

> 💡 **Tip:** Voice messages are transcribed using OpenAI's Whisper or similar services. The transcription accuracy affects the quality of responses.

## Chat History Management

### Understanding Conversation Flow

AuraGo maintains a continuous conversation context:

```
You: My name is Alex
Agent: Nice to meet you, Alex!

You: What's my name?
Agent: Your name is Alex. ← Remembers from earlier
```

### Chat Commands for History

| Command | Effect |
|---------|--------|
| `/reset` | Clears all history, fresh start |
| `/stop` | Interrupts current agent action |
| `/debug on` | Shows detailed tool outputs |
| `/debug off` | Returns to compact output |

### When to Reset

**Reset when:**
- Starting a completely new topic
- Previous context is confusing the agent
- Experiencing unexpected behavior
- Want to clear sensitive information

**Don't reset when:**
- In the middle of a multi-step task
- The agent needs previous context
- You want to build on earlier work

> ⚠️ **Warning:** `/reset` clears short-term memory. Long-term memories, notes, and core facts are preserved.

### Message Threading (Web UI)

The Web UI displays messages in a threaded format:

```
┌─────────────────────────────────────────┐
│ 👤 User: Task A                         │
│ 🤖 Agent: Working on Task A...          │
│    ├─ Tool: filesystem.list_files       │
│    ├─ Tool: filesystem.read_file        │
│    └─ Result: Done with Task A          │
├─────────────────────────────────────────┤
│ 👤 User: Now do Task B                  │
│ 🤖 Agent: Working on Task B...          │
└─────────────────────────────────────────┘
```

## Context and Memory in Conversations

### Types of Memory

AuraGo uses multiple memory systems:

| Memory Type | What It Stores | Duration |
|-------------|----------------|----------|
| **Short-term** | Recent conversation | Last N messages |
| **Core Memory** | Important facts about you | Permanent |
| **Long-term (RAG)** | Semantic search index | Permanent |
| **Knowledge Graph** | Entities and relationships | Permanent |
| **Notes/To-dos** | Explicitly saved items | Until deleted |

### How Context Works

**Within a Conversation:**
```
You: Create a Python script to backup files
Agent: [Creates script]

You: Add error handling to it
Agent: [Modifies same script - knows what "it" refers to]
```

**Across Conversations (via Memory):**
```
[Yesterday] You: I work as a software engineer
[Today]   You: What do I do for work?
Agent: You're a software engineer.
```

### Managing What the Agent Remembers

**Explicitly save important facts:**
```
You: Remember that my database password is in /secrets/db.env
Agent: ✅ Saved to core memory

You: Note: Server reboot scheduled for Sunday 2 AM
Agent: ✅ Created note with reminder
```

**Review stored information:**
```
You: What do you know about me?
Agent: Here's what I remember:
   - Name: Alex
   - Work: Software engineer
   - Interests: AI, home automation
```

> 🔍 **Deep Dive:** The agent uses RAG (Retrieval-Augmented Generation) to fetch relevant memories. When you ask a question, it searches its memory store for semantically similar information and includes relevant facts in its context.

## Best Practices for Prompts

### Prompt Structure

Effective prompts typically include:

1. **Task** - What you want done
2. **Context** - Relevant background
3. **Format** - How you want the output
4. **Constraints** - Limitations or requirements

### Example Templates

**Analysis Task:**
```
Analyze [thing] and provide:
- Key findings
- Risks or issues
- Recommendations

Context: [relevant background]
Format: [bullet points/paragraphs/table]
```

**Creation Task:**
```
Create a [thing] that does [purpose].
Requirements:
- Must support [feature]
- Should handle [edge case]
- Use [technology/approach]
```

**Comparison Task:**
```
Compare [option A] vs [option B] for [use case].
Consider:
- Performance
- Ease of use
- Maintenance
- Cost
```

### Prompt Tips

| Technique | Example |
|-----------|---------|
| **Role assignment** | "Act as a senior DevOps engineer..." |
| **Step-by-step** | "First do X, then Y, finally Z" |
| **Output format** | "Return results as JSON..." |
| **Examples** | "For example, if input is A, output should be B" |
| **Constraints** | "Limit to 100 words" / "Exclude personal data" |

### Common Prompt Mistakes

| Mistake | Better Approach |
|---------|-----------------|
| Too vague | Add specific details and constraints |
| Multiple unrelated tasks | Split into separate messages |
| Assuming knowledge | Provide necessary context |
| No output specification | State desired format clearly |

> 💡 **Tip:** If you're not getting the desired result, try rephrasing or breaking the request into smaller steps.

## Example Conversations

### Beginner Level

**Simple Information Request:**
```
You: What time is it?
Agent: 🕐 Current time: 14:32 UTC

You: Show me the files in my workspace
Agent: 📁 Workspace contents:
   - document.txt
   - backup.zip
   - notes/
```

**Basic File Operation:**
```
You: Create a text file called hello.txt
Agent: ✅ Created hello.txt

You: Write "Hello, AuraGo!" in it
Agent: ✅ Updated hello.txt with content

You: Show me what's in the file
Agent: 📄 Content of hello.txt:
   Hello, AuraGo!
```

### Intermediate Level

**Web Research:**
```
You: Search for the latest Go programming language release
Agent: 🔍 Searching...
   
   Go 1.21 was released in August 2023 with features:
   - Built-in min, max functions
   - New slog package for structured logging
   - Improved generics

You: Show me how to use the min/max functions
Agent: 📖 Example:
   ```go
   minVal := min(10, 5, 8)  // returns 5
   maxVal := max(10, 5, 8)  // returns 10
   ```
```

**System Information:**
```
You: Check system resources and alert me if disk usage is over 80%
Agent: 🔍 System check:
   💻 CPU: 12% usage
   🧠 RAM: 4.2 GB / 16 GB used
   💾 Disk: 67% used (334 GB / 500 GB)
   
   ✅ All metrics within normal range
```

### Advanced Level

**Multi-Step Automation:**
```
You: I need to analyze all log files from last week
Agent: 🔍 Finding log files from last 7 days...
   Found: 12 files (45 MB total)

You: Extract all ERROR entries and save to a summary
Agent: 📊 Processing...
   - Parsed 12 log files
   - Found: 47 ERROR entries
   - Saved summary to: error_summary_2024.txt

You: Now create a script to do this automatically every Monday
Agent: ✅ Created weekly_log_analyzer.py
   - Scheduled via cron for Mondays at 9 AM
   - Outputs to /reports/weekly/
```

**Docker Management:**
```
You: List all running containers and their resource usage
Agent: 🐳 Docker Containers:
   ┌─────────────┬────────┬─────────┬──────────┐
   │ Name        │ Status │ CPU     │ Memory   │
   ├─────────────┼────────┼─────────┼──────────┤
   │ nginx       │ Up 3d  │ 0.5%    │ 15 MB    │
   │ postgres    │ Up 3d  │ 2.1%    │ 128 MB   │
   │ redis       │ Up 3d  │ 0.3%    │ 8 MB     │
   └─────────────┴────────┴─────────┴──────────┘

You: Restart nginx and check it's healthy
Agent: 🔄 Restarting nginx...
   ✅ Container restarted successfully
   🔍 Health check: PASS
   📊 Response time: 23ms
```

**Custom Tool Creation:**
```
You: Create a tool that fetches weather data for a given city
Agent: 🛠️ Creating weather tool...
   ✅ Created: get_weather.py
   
   Usage: get_weather(city="Berlin")
   
   Features:
   - Current temperature
   - Humidity
   - Weather condition
   - 3-day forecast

You: Test it with London
Agent: 🌤️ London Weather:
   Temperature: 14°C
   Condition: Light rain
   Humidity: 82%
   
   Forecast:
   - Tomorrow: 16°C, Cloudy
   - Day after: 18°C, Sunny
```

> 💡 **Tip:** Start simple and gradually increase complexity. AuraGo learns your preferences and working style over time.

---

## Quick Reference Card

### Essential Commands
```
/help          - Show all commands
/reset         - Clear chat history
/stop          - Cancel current action
/debug on/off  - Toggle debug mode
/budget        - Show API costs
/personality   - Switch personality
```

### Message Patterns That Work Well
```
"Show me..."          - For information retrieval
"Create a..."         - For file/script generation
"Explain..."          - For learning and documentation
"Compare..."          - For analysis and decisions
"Fix/improve..."      - For code editing
"Set up..."           - For configuration tasks
"Remember that..."    - For storing facts
"What do you know..." - For memory retrieval
```

---

> 🎓 **Next Chapter:** [Chapter 6: Tools](06-tools.md) – Explore AuraGo's 30+ built-in tools in detail.
