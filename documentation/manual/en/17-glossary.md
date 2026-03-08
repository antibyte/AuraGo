# Chapter 17: Glossary

A comprehensive reference of technical terms, abbreviations, and concepts used in AuraGo.

---

## Table of Contents

1. [Abbreviations and Acronyms](#abbreviations-and-acronyms)
2. [Agent Terminology](#agent-terminology)
3. [Memory System Terms](#memory-system-terms)
4. [Tool Names and Functions](#tool-names-and-functions)
5. [Configuration Terms](#configuration-terms)
6. [Security Vocabulary](#security-vocabulary)
7. [Integration Terms](#integration-terms)
8. [Personality and Behavior](#personality-and-behavior)
9. [Technical Architecture](#technical-architecture)
10. [Cross-Reference Index](#cross-reference-index)

---

## Abbreviations and Acronyms

| Abbreviation | Full Term | Description |
|--------------|-----------|-------------|
| **AES** | Advanced Encryption Standard | Symmetric encryption algorithm used in vault |
| **AI** | Artificial Intelligence | Computer systems performing tasks requiring human intelligence |
| **API** | Application Programming Interface | Interface for software communication |
| **CLI** | Command Line Interface | Text-based user interface |
| **CGO** | C Go | Go's FFI for calling C code (AuraGo doesn't use it) |
| **CSS** | Cascading Style Sheets | Styling language for Web UI |
| **DB** | Database | Organized data storage |
| **FAQ** | Frequently Asked Questions | Common questions and answers |
| **FFI** | Foreign Function Interface | Interface for calling code in other languages |
| **GC** | Garbage Collection | Automatic memory management |
| **GCM** | Galois/Counter Mode | Encryption mode for AES (AES-256-GCM) |
| **HA** | Home Assistant | Open-source home automation platform |
| **HTML** | HyperText Markup Language | Markup language for Web UI |
| **HTTP** | HyperText Transfer Protocol | Protocol for web communication |
| **IMAP** | Internet Message Access Protocol | Protocol for email retrieval |
| **IoT** | Internet of Things | Network of connected devices |
| **JSON** | JavaScript Object Notation | Data interchange format |
| **LLM** | Large Language Model | AI model for natural language processing |
| **LAN** | Local Area Network | Local computer network |
| **LTM** | Long-Term Memory | Persistent memory storage with semantic search |
| **MCP** | Model Context Protocol | Protocol for context sharing |
| **MQTT** | Message Queuing Telemetry Transport | Lightweight messaging protocol |
| **NAT** | Network Address Translation | Method for remapping IP addresses |
| **Ollama** | Ollama | Tool for running local LLMs |
| **OS** | Operating System | System software managing computer hardware |
| **PID** | Process Identifier | Unique number identifying a process |
| **Proxmox** | Proxmox VE | Open-source server virtualization platform |
| **RAG** | Retrieval-Augmented Generation | Technique combining retrieval with generation |
| **REST** | Representational State Transfer | Architectural style for APIs |
| **ROM** | Read-Only Memory | Memory that can only be read |
| **SDK** | Software Development Kit | Tools for software development |
| **SFTP** | SSH File Transfer Protocol | Secure file transfer protocol |
| **SMTP** | Simple Mail Transfer Protocol | Protocol for email sending |
| **SPA** | Single Page Application | Web app that loads single HTML page |
| **SQL** | Structured Query Language | Language for database queries |
| **SQLite** | SQLite | Embedded SQL database engine |
| **SSE** | Server-Sent Events | Technology for server-to-client streaming |
| **SSH** | Secure Shell | Cryptographic network protocol |
| **SSL** | Secure Sockets Layer | Deprecated security protocol (use TLS) |
| **STM** | Short-Term Memory | Recent conversation storage |
| **STT** | Speech-to-Text | Converting speech to written text |
| **TLS** | Transport Layer Security | Cryptographic protocol for secure communication |
| **TOTP** | Time-based One-Time Password | 2FA method using time-synchronized codes |
| **TS** | TypeScript | Programming language (Web UI) |
| **TTS** | Text-to-Speech | Converting text to spoken audio |
| **UI** | User Interface | Visual interface for interaction |
| **URI** | Uniform Resource Identifier | String identifying a resource |
| **URL** | Uniform Resource Locator | Web address |
| **UUID** | Universally Unique Identifier | 128-bit unique identifier |
| **VM** | Virtual Machine | Emulated computer system |
| **VPN** | Virtual Private Network | Secure network connection |
| **WAL** | Write-Ahead Logging | Database durability technique |
| **WebDAV** | Web Distributed Authoring and Versioning | HTTP extension for file management |
| **YAML** | YAML Ain't Markup Language | Human-readable data serialization format |
| **2FA** | Two-Factor Authentication | Security requiring two verification methods |

---

## Agent Terminology

### Core Agent Concepts

| Term | Definition | Related Terms |
|------|------------|---------------|
| **Agent Loop** | The main execution cycle where the agent processes user input, decides on actions, and generates responses | Tool Dispatch, Reasoning |
| **Chain of Thought** | The agent's step-by-step reasoning process | Reasoning Loop |
| **Co-Agent** | Parallel sub-agent spawned for complex tasks | Sub-Agent, Parallel Processing |
| **Context Window** | The amount of text (in tokens) the LLM can process at once | Token Limit, Memory |
| **Function Calling** | LLM capability to call functions/tools with structured parameters | Native Functions, Tools |
| **Inference** | The process of generating output from an LLM | Prediction, Generation |
| **Orchestrator** | Main agent that coordinates co-agents | Main Agent, Controller |
| **Prompt** | Instructions sent to the LLM | System Prompt, User Prompt |
| **Reasoning** | The agent's logical thinking process | Chain of Thought |
| **Session** | A continuous conversation context | Chat, Conversation |
| **System Prompt** | Base instructions defining agent behavior | Personality, Context |
| **Temperature** | Parameter controlling randomness in LLM output | Creativity, Randomness |
| **Token** | Unit of text processed by LLMs (~4 chars in English) | Tokenization |
| **Tool Call** | Invocation of a specific capability by the agent | Action, Function |
| **Tool Dispatch** | Process of routing tool calls to implementations | Tool Registry |

### Agent Actions

| Action | Description | Example Tools |
|--------|-------------|---------------|
| **Analyze** | Examine content for insights | analyze_image, analyze_document |
| **Create** | Generate new content or files | write_file, create_note |
| **Execute** | Run code or commands | execute_shell, execute_python |
| **Modify** | Change existing content | edit_file, update_memory |
| **Query** | Request information | query_memory, web_search |
| **Schedule** | Set up future actions | cron_scheduler, follow_up |
| **Transform** | Convert content format | transcribe, translate |

---

## Memory System Terms

### Memory Types

| Term | Definition | Persistence | Access |
|------|------------|-------------|--------|
| **Core Memory** | Permanent facts the agent always remembers | Persistent | Always included |
| **Ephemeral Memory** | Temporary storage for single session | Session-only | Current chat |
| **Knowledge Graph** | Structured entity-relationship storage | Persistent | Query/Search |
| **Long-Term Memory (LTM)** | Vector-based semantic memory | Persistent | Similarity search |
| **Persistent Summary** | Compressed background information | Persistent | Background context |
| **Short-Term Memory (STM)** | Recent conversation history | Persistent | Sliding window |
| **Working Memory** | Active processing space | Temporary | Current operation |

### Memory Operations

| Operation | Description | Tool |
|-----------|-------------|------|
| **Compression** | Reducing context size while preserving meaning | Automatic |
| **Embedding** | Converting text to numerical vectors | query_memory |
| **Indexing** | Adding content to searchable storage | store_memory |
| **Query** | Requesting specific information | query_memory |
| **Recall** | Retrieving stored information | Various |
| **Storage** | Saving information for later | manage_memory |
| **Summarization** | Creating condensed versions | Automatic |
| **Vector Search** | Semantic similarity search | query_memory |

### Memory Data Structures

| Term | Description | Used In |
|------|-------------|---------|
| **Chunk** | Segment of text for embedding | LTM storage |
| **Document** | Unit of stored information | VectorDB |
| **Entity** | Named object in knowledge graph | Knowledge Graph |
| **Node** | Element in knowledge graph | Knowledge Graph |
| **Edge** | Relationship between entities | Knowledge Graph |
| **Vector** | Numerical representation of text | Embeddings |

---

## Tool Names and Functions

### System Tools

| Tool | Function | Category |
|------|----------|----------|
| **execute_shell** | Execute shell commands | System |
| **execute_python** | Run Python code | System |
| **read_file** | Read file contents | Filesystem |
| **write_file** | Write/create files | Filesystem |
| **list_directory** | List directory contents | Filesystem |
| **get_system_metrics** | Retrieve CPU, memory, disk info | System |
| **get_current_time** | Get current date/time | System |

### Communication Tools

| Tool | Function | Category |
|------|----------|----------|
| **send_email** | Send email via SMTP | Communication |
| **telegram_send** | Send Telegram message | Communication |
| **discord_send** | Send Discord message | Communication |
| **tts_speak** | Text-to-Speech output | Communication |
| **notify_push** | Send push notification | Communication |

### Web and Search Tools

| Tool | Function | Category |
|------|----------|----------|
| **web_search** | Search the web | Web |
| **web_scraper** | Extract content from websites | Web |
| **fetch_url** | Retrieve web page content | Web |
| **wikipedia_search** | Search Wikipedia | Web |
| **ddg_search** | DuckDuckGo search | Web |

### Smart Home Tools

| Tool | Function | Category |
|------|----------|----------|
| **home_assistant_get_state** | Read device states | Smart Home |
| **home_assistant_call_service** | Execute HA services | Smart Home |
| **home_assistant_toggle** | Toggle devices on/off | Smart Home |
| **chromecast_control** | Control Chromecast devices | Smart Home |
| **chromecast_tts** | Send TTS to Chromecast | Smart Home |

### Infrastructure Tools

| Tool | Function | Category |
|------|----------|----------|
| **docker_list** | List Docker containers | Infrastructure |
| **docker_start** | Start containers | Infrastructure |
| **docker_stop** | Stop containers | Infrastructure |
| **docker_logs** | View container logs | Infrastructure |
| **proxmox_list_vms** | List Proxmox VMs | Infrastructure |
| **proxmox_start_vm** | Start VM | Infrastructure |
| **proxmox_stop_vm** | Stop VM | Infrastructure |
| **ollama_list** | List Ollama models | Infrastructure |
| **ollama_pull** | Download Ollama model | Infrastructure |

### Memory and Knowledge Tools

| Tool | Function | Category |
|------|----------|----------|
| **manage_memory** | Add/update/delete memories | Memory |
| **query_memory** | Search long-term memory | Memory |
| **knowledge_graph_query** | Query knowledge graph | Knowledge |
| **manage_notes** | CRUD operations on notes | Memory |
| **manage_todos** | Manage todo items | Memory |

### Development Tools

| Tool | Function | Category |
|------|----------|----------|
| **git_clone** | Clone git repositories | Development |
| **git_commit** | Commit changes | Development |
| **github_create_issue** | Create GitHub issues | Development |
| **github_list_repos** | List repositories | Development |
| **code_surgery** | Modify source code | Development |

### Integration Tools

| Tool | Function | Category |
|------|----------|----------|
| **webdav_list** | List WebDAV files | Storage |
| **webdav_download** | Download from WebDAV | Storage |
| **webdav_upload** | Upload to WebDAV | Storage |
| **koofr_list** | List Koofr files | Storage |
| **google_workspace_query** | Query Google services | Productivity |
| **mqtt_publish** | Publish MQTT message | IoT |
| **mqtt_subscribe** | Subscribe to MQTT topic | IoT |

### Utility Tools

| Tool | Function | Category |
|------|----------|----------|
| **cron_scheduler** | Schedule recurring tasks | Utility |
| **follow_up** | Schedule future actions | Utility |
| **analyze_image** | Analyze image content | Vision |
| **transcribe_audio** | Convert speech to text | Audio |
| **stop_process** | Kill background processes | System |

---

## Configuration Terms

### Server Configuration

| Term | Description | Default |
|------|-------------|---------|
| **host** | Network interface to bind | `127.0.0.1` |
| **port** | HTTP server port | `8088` |
| **max_body_bytes** | Maximum request body size | `10485760` (10MB) |

### Agent Configuration

| Term | Description | Default |
|------|-------------|---------|
| **context_window** | LLM context size in tokens | Auto-detect |
| **max_tool_calls** | Maximum tool calls per request | `12` |
| **step_delay_seconds** | Delay between tool calls | `0` |
| **system_language** | Agent response language | `German` |
| **personality_engine** | Enable mood adaptation | `true` |
| **core_personality** | Base personality template | `friend` |
| **debug_mode** | Enable debug instructions | `false` |

### LLM Configuration

| Term | Description | Example |
|------|-------------|---------|
| **provider** | LLM service provider | `openrouter` |
| **base_url** | API endpoint URL | `https://openrouter.ai/api/v1` |
| **api_key** | Authentication token | `sk-or-v1-...` |
| **model** | Specific model identifier | `gpt-4` |
| **temperature** | Output randomness (0-2) | `0.7` |
| **structured_outputs** | Use constrained decoding | `false` |

### Circuit Breaker

| Term | Description | Default |
|------|-------------|---------|
| **max_tool_calls** | Hard limit on tool calls | `20` |
| **llm_timeout_seconds** | LLM call timeout | `180` |
| **retry_intervals** | Backoff between retries | `[10s, 2m, 10m]` |

---

## Security Vocabulary

### Authentication

| Term | Description | Implementation |
|------|-------------|----------------|
| **API Key** | Token for LLM service access | Config file |
| **bcrypt** | Password hashing algorithm | Web UI auth |
| **Master Key** | AES-256 encryption key | Environment variable |
| **Session Secret** | Cookie encryption key | Config file |
| **TOTP** | Time-based One-Time Password | 2FA implementation |

### Authorization

| Term | Description | Usage |
|------|-------------|-------|
| **Danger Zone** | Capability toggles | Enable/disable tools |
| **Guardian** | Security checking system | Path validation |
| **Read-Only Mode** | Restrict write operations | Per-integration |
| **User ID** | Telegram/Discord user restriction | Access control |

### Encryption

| Term | Description | Algorithm |
|------|-------------|-----------|
| **AES-256-GCM** | Symmetric encryption | AES with GCM mode |
| **Vault** | Encrypted secrets storage | AES-256-GCM |
| **TLS** | Transport security | HTTPS connections |

### Threats and Mitigations

| Term | Description | Mitigation |
|------|-------------|------------|
| **Path Traversal** | Unauthorized file access | Guardian validation |
| **Prompt Injection** | Malicious prompt manipulation | Input validation |
| **Rate Limiting** | Prevent abuse | Request throttling |
| **Sandbox** | Isolated execution environment | Python venv |

---

## Integration Terms

### Messaging Platforms

| Term | Description | Protocol |
|------|-------------|----------|
| **Bot Token** | Authentication for bot | Telegram Bot API |
| **Guild ID** | Discord server identifier | Discord API |
| **Channel ID** | Specific channel reference | Discord API |
| **User ID** | Platform-specific user identifier | Various |
| **Webhook** | HTTP callback mechanism | HTTP POST |

### Smart Home

| Term | Description | Protocol |
|------|-------------|----------|
| **Access Token** | Home Assistant authentication | REST API |
| **Entity ID** | Device identifier in HA | HA conventions |
| **Service Call** | Execute HA action | HA WebSocket/REST |
| **State** | Current device condition | HA state machine |

### Container Orchestration

| Term | Description | Command |
|------|-------------|---------|
| **Container** | Running instance of image | `docker ps` |
| **Image** | Template for containers | `docker images` |
| **Volume** | Persistent data storage | `docker volume` |
| **Network** | Container communication | `docker network` |

### Cloud Storage

| Term | Description | Service |
|------|-------------|---------|
| **App Password** | Koofr authentication | Koofr |
| **Base URL** | Service endpoint | Various |
| **Mount Point** | Access location | WebDAV |

---

## Personality and Behavior

### Personality Engine

| Term | Description | Range |
|------|-------------|-------|
| **Trait** | Persistent behavioral characteristic | 0.0 - 1.0 |
| **Mood** | Temporary emotional state | Various |
| **Confidence** | Self-assurance level | 0.0 - 1.0 |
| **Curiosity** | Desire to explore/learn | 0.0 - 1.0 |
| **Empathy** | Understanding of user emotions | 0.0 - 1.0 |
| **Thoroughness** | Attention to detail | 0.0 - 1.0 |
| **Creativity** | Novelty in responses | 0.0 - 1.0 |

### Mood States

| Mood | Description | Trigger |
|------|-------------|---------|
| **Curious** | Inquisitive, exploring | Questions, new topics |
| **Focused** | Concentrated, task-oriented | Complex tasks |
| **Creative** | Imaginative, unconventional | Open-ended requests |
| **Analytical** | Logical, detailed | Data analysis |
| **Cautious** | Careful, risk-averse | Errors, sensitive topics |
| **Playful** | Humorous, light | Casual conversation |

### Personality Profiles

| Profile | Characteristics | Use Case |
|---------|-----------------|----------|
| **Friend** | Casual, warm, chatty | Personal assistance |
| **Professional** | Formal, efficient, direct | Business tasks |
| **Neutral** | Balanced, adaptable | General purpose |
| **Punk** | Rebellious, unconventional | Creative tasks |
| **Terminator** | Direct, minimal, robotic | System administration |
| **MCP** | Protocol-focused | Technical debugging |

---

## Technical Architecture

### Core Components

| Component | Responsibility | File Location |
|-----------|----------------|---------------|
| **Agent** | Main reasoning loop | `internal/agent/` |
| **Server** | HTTP/WebSocket handling | `internal/server/` |
| **Memory** | All memory subsystems | `internal/memory/` |
| **Tools** | Tool implementations | `internal/tools/` |
| **LLM** | Language model client | `internal/llm/` |
| **Config** | Configuration management | `internal/config/` |
| **Security** | Vault and encryption | `internal/security/` |
| **Prompts** | System prompt building | `internal/prompts/` |

### Data Storage

| Storage | Technology | Purpose |
|---------|------------|---------|
| **Short-Term Memory** | SQLite | Conversation history |
| **Long-Term Memory** | chromem-go | Vector embeddings |
| **Knowledge Graph** | SQLite | Entity relationships |
| **Vault** | AES-256-GCM file | Encrypted secrets |
| **Chat History** | JSON file | UI state |
| **Config** | YAML file | Settings |

### Runtime Concepts

| Term | Description | Example |
|------|-------------|---------|
| **Goroutine** | Lightweight thread (Go) | Co-agent execution |
| **Mutex** | Mutual exclusion lock | Database access |
| **Context** | Request-scoped values | Cancellation signals |
| **Broker** | Event distribution | SSE updates |
| **Registry** | Tool/process tracking | Background processes |

---

## Cross-Reference Index

### By Category

#### Memory System
- Short-Term Memory (STM) → [Memory System Terms](#memory-system-terms)
- Long-Term Memory (LTM) → [Memory System Terms](#memory-system-terms)
- Knowledge Graph → [Memory System Terms](#memory-system-terms)
- Core Memory → [Memory System Terms](#memory-system-terms)
- Vector Search → [Memory Operations](#memory-operations)

#### Security
- Vault → [Encryption](#encryption)
- Master Key → [Authentication](#authentication)
- TOTP → [Authentication](#authentication)
- Danger Zone → [Authorization](#authorization)
- AES-256-GCM → [Encryption](#encryption)

#### Integrations
- Telegram → [Messaging Platforms](#messaging-platforms)
- Discord → [Messaging Platforms](#messaging-platforms)
- Home Assistant → [Smart Home](#smart-home)
- Docker → [Container Orchestration](#container-orchestration)
- WebDAV → [Cloud Storage](#cloud-storage)

#### Agent Behavior
- Personality Engine → [Personality Engine](#personality-engine)
- Co-Agent → [Core Agent Concepts](#core-agent-concepts)
- Context Window → [Core Agent Concepts](#core-agent-concepts)
- Tool Call → [Core Agent Concepts](#core-agent-concepts)
- Temperature → [Core Agent Concepts](#core-agent-concepts)

### Common Synonyms

| Term | Also Known As |
|------|---------------|
| Short-Term Memory | STM, Working Memory, Conversation History |
| Long-Term Memory | LTM, Vector Memory, RAG Memory |
| System Prompt | Base Prompt, Instructions, Context |
| Tool | Function, Action, Capability |
| Agent Loop | Reasoning Loop, Main Loop |
| Vault | Secrets Store, Encrypted Storage |
| Master Key | Encryption Key, AURAGO_MASTER_KEY |
| Circuit Breaker | Safety Limits, Timeouts |
| Co-Agent | Sub-Agent, Worker, Parallel Agent |

### Acronym Expansions Quick Reference

```
STM  → Short-Term Memory
LTM  → Long-Term Memory
RAG  → Retrieval-Augmented Generation
LLM  → Large Language Model
API  → Application Programming Interface
TOTP → Time-based One-Time Password
MCP  → Model Context Protocol
SSE  → Server-Sent Events
UI   → User Interface
URL  → Uniform Resource Locator
YAML → YAML Ain't Markup Language
JSON → JavaScript Object Notation
SQL  → Structured Query Language
```

---

> 💡 **Tip:** Bookmark this glossary for quick reference when reading other documentation or configuration files. Many terms are used consistently across AuraGo's codebase and documentation.
