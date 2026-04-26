# YepAPI Integration Plan for AuraGo

> **Date:** 2026-04-26
> **Status:** Planning Complete
> **Priority:** High

## Executive Summary

YepAPI (https://yepapi.com) ist eine unified pay-per-call API mit 118+ Endpunkten, 69 LLM-Modellen und 9 Media-Generation-Modellen. Die Integration soll YepAPI **zweifach** nutzbar machen:

1. **Als LLM-Provider** – OpenAI-kompatibler Endpoint (`/v1/ai/chat/completions`) für Chat-Completion-Requests
2. **Als Tool-Suite** – Einzeln aktivierbare Tools für SEO, SERP, Web Scraping, YouTube, TikTok, Instagram, Amazon

---

## 1. YepAPI Overview & API Architecture

### Base URL
```
https://api.yepapi.com
```

### Authentication
- Header: `x-api-key: yep_sk_...`
- Response Format: Consistent JSON envelope `{ "ok": true, "data": { ... } }` / `{ "ok": false, "error": { ... } }`
- Rate Limit: 60 requests/minute per API key
- Pricing: Pay-per-call, keine Subscription

### Key Endpoint Categories (118 endpoints)

| Category | Endpoints | Kosten | Relevanz |
|----------|-----------|--------|----------|
| **SEO – Keywords** | 3 | $0.02–$0.15 | Sehr hoch |
| **SERP** | 13 Engines | $0.01 | Sehr hoch |
| **SEO – Domain** | 6 | $0.02–$0.04 | Hoch |
| **SEO – Competitors** | 4 | $0.02+ | Hoch |
| **SEO – Backlinks** | 6 | $0.03+ | Mittel |
| **SEO – On-Page** | 3 | $0.03–$0.05 | Mittel |
| **SEO – Content** | 3 | $0.02+ | Mittel |
| **Web Scraping** | 7 | $0.01–$0.03 | Sehr hoch |
| **YouTube** | 30 | $0.01–$0.02 | Sehr hoch |
| **TikTok** | 18 | $0.01 | Sehr hoch |
| **Instagram** | 15 | $0.01 | Sehr hoch |
| **Amazon** | 11 | $0.01–$0.02 | Hoch |
| **AI Chat** | 3 | variable | Als Provider |
| **AI Media** | 2 (async) | variable | Optional |

### LLM-Provider Fähigkeit
- Endpoint: `POST /v1/ai/chat/completions` (OpenAI-compatible)
- Alternative: `POST /v1/ai/chat` (YepAPI-native, camelCase)
- Models: 69 Modelle von OpenAI, Anthropic, Google, Meta, DeepSeek, Mistral, etc.
- Streaming: SSE wird unterstützt
- Drop-in replacement für OpenAI SDK

---

## 2. Integration Architecture

### 2.1 Design Principles

1. **Provider als First-Class Citizen** – YepAPI wird als neuer `ProviderType` (`yepapi`) hinzugefügt, analog zu `openrouter`, `anthropic`, etc.
2. **Granulare Tool-Aktivierung** – Jedes Tool-Cluster (SEO, SERP, Scraping, YouTube, TikTok, Instagram, Amazon) ist einzeln aktivierbar
3. **Single API Key** – Ein Vault-Eintrag für den Provider, alle Tools nutzen denselben Key
4. **Read-Only by Default** – Alle YepAPI-Tools sind inhärent read-only (nur Datenabfrage), daher kein `ReadOnly`-Flag nötig
5. **Konsistente Fehlerbehandlung** – Alle Tools verarbeiten das `{ "ok": false, "error": { ... } }`-Pattern

### 2.2 Files to Create/Modify

```
internal/config/config_types.go          # Add YepAPIConfig
internal/config/config.go                # Add defaults
internal/config/config_migrate.go        # Add vault key resolution

internal/llm/client.go                   # Add yepapi provider support

internal/agent/native_tools.go           # Add feature flags
internal/agent/tooling_policy.go         # Add flag resolution
internal/agent/native_tools_edge.go      # Add tool schemas (SEO, SERP, Scraping)
internal/agent/native_tools_content.go   # Add tool schemas (YouTube, TikTok, IG, Amazon)
internal/agent/agent_dispatch_exec.go    # Add dispatch cases
internal/agent/agent_dispatch_services.go # Add dispatch routing

internal/tools/yepapi.go                 # Main YepAPI client + tool implementations
internal/tools/yepapi_seo.go             # SEO tool implementations
internal/tools/yepapi_serp.go            # SERP tool implementations
internal/tools/yepapi_scrape.go          # Scraping tool implementations
internal/tools/yepapi_youtube.go         # YouTube tool implementations
internal/tools/yepapi_tiktok.go          # TikTok tool implementations
internal/tools/yepapi_instagram.go       # Instagram tool implementations
internal/tools/yepapi_amazon.go          # Amazon tool implementations
internal/tools/yepapi_test.go            # Unit tests

prompts/tools_manuals/yepapi.md          # Agent documentation

ui/lang/*/config.json                    # Translations for all 15 languages
```

---

## 3. Detailed Implementation Plan

### Phase 1: Provider Integration (LLM)

#### 3.1.1 Config Types (`internal/config/config_types.go`)

YepAPI kann über den **bestehenden Provider-Mechanismus** integriert werden, da es OpenAI-kompatibel ist. Keine neuen Config-Typen nötig – es wird als `type: yepapi` im ProviderEntry unterstützt.

**Änderung in `ProviderEntry.Type`-Validierung:**
```go
// In config validation / provider resolution:
// Add "yepapi" to valid provider types
```

**Zusätzliche Config-Struktur für Tool-Features:**
```go
// YepAPIConfig holds settings for YepAPI tool suite
type YepAPIConfig struct {
    Enabled  bool `yaml:"enabled" json:"enabled"`
    
    // Service toggles (granular)
    SEO       YepAPIServiceConfig `yaml:"seo" json:"seo"`
    SERP      YepAPIServiceConfig `yaml:"serp" json:"serp"`
    Scraping  YepAPIServiceConfig `yaml:"scraping" json:"scraping"`
    YouTube   YepAPIServiceConfig `yaml:"youtube" json:"youtube"`
    TikTok    YepAPIServiceConfig `yaml:"tiktok" json:"tiktok"`
    Instagram YepAPIServiceConfig `yaml:"instagram" json:"instagram"`
    Amazon    YepAPIServiceConfig `yaml:"amazon" json:"amazon"`
}

type YepAPIServiceConfig struct {
    Enabled bool `yaml:"enabled" json:"enabled"`
}
```

#### 3.1.2 LLM Client (`internal/llm/client.go`)

YepAPI verwendet den **OpenAI-Client mit angepasster BaseURL** – identisch zu OpenRouter.

```go
// In NewClient() or provider resolution:
case "yepapi":
    // YepAPI is OpenAI-compatible
    // BaseURL: https://api.yepapi.com/v1/ai
    // Auth: x-api-key header
    // No special transport needed – standard openai client works
```

**Wichtig:** Der API-Key für den LLM-Provider wird aus dem Vault geladen (`provider_<id>_api_key`). Die Tools nutzen **denselben Key** – daher muss der YepAPI-Client in `internal/tools/yepapi.go` den Key aus der ProviderEntry auflösen.

#### 3.1.3 Vault Integration

Der API Key wird als `provider_<id>_api_key` im Vault gespeichert (Standard-Mechanismus). Für direkten Tool-Zugriff ohne Provider:
- Alternative: `yepapi_api_key` als dedizierter Vault-Eintrag
- Empfohlene Lösung: Tools lesen den Key aus der konfigurierten ProviderEntry

### Phase 2: Tool Feature Flags

#### 3.2.1 `internal/agent/native_tools.go`

```go
type ToolFeatureFlags struct {
    // ... existing flags ...
    
    // YepAPI services
    YepAPIEnabled           bool
    YepAPISEOEnabled        bool
    YepAPISERPEnabled       bool
    YepAPIScrapingEnabled   bool
    YepAPIYouTubeEnabled    bool
    YepAPITikTokEnabled     bool
    YepAPIInstagramEnabled  bool
    YepAPIAmazonEnabled     bool
}
```

#### 3.2.2 `internal/agent/tooling_policy.go`

```go
func buildToolFlagsFromConfig(cfg *config.Config) ToolFeatureFlags {
    ff := ToolFeatureFlags{}
    // ... existing ...
    
    if cfg.YepAPI.Enabled {
        ff.YepAPIEnabled = true
        ff.YepAPISEOEnabled = cfg.YepAPI.SEO.Enabled
        ff.YepAPISERPEnabled = cfg.YepAPI.SERP.Enabled
        ff.YepAPIScrapingEnabled = cfg.YepAPI.Scraping.Enabled
        ff.YepAPIYouTubeEnabled = cfg.YepAPI.YouTube.Enabled
        ff.YepAPITikTokEnabled = cfg.YepAPI.TikTok.Enabled
        ff.YepAPIInstagramEnabled = cfg.YepAPI.Instagram.Enabled
        ff.YepAPIAmazonEnabled = cfg.YepAPI.Amazon.Enabled
    }
    return ff
}
```

### Phase 3: Tool Schemas

#### 3.3.1 Schema Design Strategy

Statt 118 einzelner Tools zu erstellen (was den Kontext überladen würde), werden **kategoriale Composite-Tools** definiert:

| Tool Name | Funktion | Mapped Endpoints |
|-----------|----------|------------------|
| `yepapi_seo` | SEO-Daten (Keywords, Domain, Competitors) | /v1/seo/* |
| `yepapi_serp` | SERP-Abfragen | /v1/serp/* |
| `yepapi_scrape` | Web Scraping | /v1/scrape* |
| `yepapi_youtube` | YouTube-Daten | /v1/youtube/* |
| `yepapi_tiktok` | TikTok-Daten | /v1/tiktok/* |
| `yepapi_instagram` | Instagram-Daten | /v1/instagram/* |
| `yepapi_amazon` | Amazon-Daten | /v1/amazon/* |

Jedes Tool hat einen `operation`-Parameter, der den spezifischen Endpunkt auswählt.

#### 3.3.2 Example Schema: `yepapi_youtube`

```go
if ff.YepAPIYouTubeEnabled {
    tools = append(tools, tool("yepapi_youtube",
        "Access YouTube data via YepAPI: search videos, get video details, transcripts, comments, channel info, playlists, trending, shorts.",
        schema(map[string]interface{}{
            "operation": prop("string", `Operation to perform: "search", "video", "transcript", "comments", "channel", "channel_videos", "playlist", "trending", "shorts", "suggest"`),
            "query":     prop("string", "Search query (for search operation)"),
            "video_id":  prop("string", "YouTube video ID (for video, transcript, comments)"),
            "channel_id": prop("string", "YouTube channel ID (for channel operations)"),
            "playlist_id": prop("string", "YouTube playlist ID"),
            "limit":     prop("number", "Max results to return (default 10)"),
        }, "operation")))
}
```

#### 3.3.3 Schema Files

- `native_tools_edge.go` – `yepapi_seo`, `yepapi_serp`, `yepapi_scrape`
- `native_tools_content.go` – `yepapi_youtube`, `yepapi_tiktok`, `yepapi_instagram`, `yepapi_amazon`

### Phase 4: Tool Implementations

#### 3.4.1 Core Client (`internal/tools/yepapi.go`)

```go
package tools

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

const YepAPIBaseURL = "https://api.yepapi.com"

type YepAPIClient struct {
    apiKey     string
    httpClient *http.Client
    baseURL    string
}

func NewYepAPIClient(apiKey string) *YepAPIClient {
    return &YepAPIClient{
        apiKey:  apiKey,
        baseURL: YepAPIBaseURL,
        httpClient: &http.Client{Timeout: 60 * time.Second},
    }
}

// YepAPIResponse is the standard envelope
type YepAPIResponse struct {
    OK    bool            `json:"ok"`
    Data  json.RawMessage `json:"data"`
    Error *struct {
        Code    string `json:"code"`
        Message string `json:"message"`
    } `json:"error"`
}

func (c *YepAPIClient) Post(ctx context.Context, endpoint string, payload interface{}) ([]byte, error) {
    url := c.baseURL + endpoint
    body, _ := json.Marshal(payload)
    
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("x-api-key", c.apiKey)
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("yepapi request failed: %w", err)
    }
    defer resp.Body.Close()
    
    respBody, _ := io.ReadAll(resp.Body)
    
    var env YepAPIResponse
    if err := json.Unmarshal(respBody, &env); err != nil {
        return nil, fmt.Errorf("yepapi invalid JSON: %w", err)
    }
    
    if !env.OK {
        if env.Error != nil {
            return nil, fmt.Errorf("yepapi error [%s]: %s", env.Error.Code, env.Error.Message)
        }
        return nil, fmt.Errorf("yepapi unknown error")
    }
    
    return env.Data, nil
}
```

#### 3.4.2 Service-Specific Implementations

**Pattern für jedes Service-File:**

```go
// internal/tools/yepapi_youtube.go
func (c *YepAPIClient) YouTubeSearch(ctx context.Context, query string, limit int) (string, error) {
    payload := map[string]interface{}{"query": query}
    if limit > 0 {
        payload["depth"] = limit
    }
    data, err := c.Post(ctx, "/v1/youtube/search", payload)
    if err != nil {
        return "", err
    }
    return string(data), nil
}

// ... weitere Operationen ...
```

**Dispatch-Funktion pro Service:**

```go
// internal/tools/yepapi_youtube.go
func DispatchYepAPIYouTube(ctx context.Context, client *YepAPIClient, operation string, args map[string]interface{}) (string, error) {
    switch operation {
    case "search":
        query, _ := args["query"].(string)
        limit, _ := args["limit"].(float64)
        return client.YouTubeSearch(ctx, query, int(limit))
    case "video":
        videoID, _ := args["video_id"].(string)
        return client.YouTubeVideo(ctx, videoID)
    case "transcript":
        videoID, _ := args["video_id"].(string)
        return client.YouTubeTranscript(ctx, videoID)
    // ... etc ...
    default:
        return "", fmt.Errorf("unknown yepapi_youtube operation: %s", operation)
    }
}
```

### Phase 5: Agent Dispatch

#### 3.5.1 `internal/agent/agent_dispatch_exec.go`

```go
case "yepapi_youtube":
    if !cfg.YepAPI.Enabled || !cfg.YepAPI.YouTube.Enabled {
        return "YepAPI YouTube is disabled", true
    }
    client := tools.NewYepAPIClient(resolveYepAPIKey(cfg, vault))
    result, err := tools.DispatchYepAPIYouTube(ctx, client, args.Operation, args.Params)
    if err != nil {
        return fmt.Sprintf("YouTube API error: %v", err), true
    }
    return result, true

case "yepapi_seo":
    // ... analogous ...

case "yepapi_serp":
    // ... analogous ...

case "yepapi_scrape":
    // ... analogous ...
```

### Phase 6: Key Resolution Helper

```go
// internal/tools/yepapi.go
func ResolveYepAPIKey(cfg *config.Config, vault security.SecretReader) (string, error) {
    // Strategy 1: Find a provider with type "yepapi" and use its key
    for _, p := range cfg.Providers {
        if p.Type == "yepapi" {
            key, err := vault.ReadSecret(fmt.Sprintf("provider_%s_api_key", p.ID))
            if err == nil && key != "" {
                return key, nil
            }
            if p.APIKey != "" {
                return p.APIKey, nil
            }
        }
    }
    
    // Strategy 2: Dedicated vault key
    key, err := vault.ReadSecret("yepapi_api_key")
    if err == nil && key != "" {
        return key, nil
    }
    
    return "", fmt.Errorf("no YepAPI API key found")
}
```

---

## 4. Configuration Specification

### 4.1 Minimal Config (Provider Only)

```yaml
providers:
  - id: yepapi
    type: yepapi
    name: "YepAPI"
    base_url: https://api.yepapi.com/v1/ai
    model: openai/gpt-4o

llm:
  provider: yepapi
```

### 4.2 Full Config (Provider + Tools)

```yaml
providers:
  - id: yepapi
    type: yepapi
    name: "YepAPI"
    base_url: https://api.yepapi.com/v1/ai
    model: openai/gpt-4o

llm:
  provider: main  # oder yepapi

yepapi:
  enabled: true
  seo:
    enabled: true
  serp:
    enabled: true
  scraping:
    enabled: true
  youtube:
    enabled: true
  tiktok:
    enabled: true
  instagram:
    enabled: true
  amazon:
    enabled: true
```

### 4.3 Config Template Entry

In `config_template.yaml` unter dem Integrations-Abschnitt:

```yaml
# YepAPI Integration
# Unified API for SEO, SERP, Scraping, Social Media, Amazon, and LLM access.
# Get your API key at: https://yepapi.com/dashboard/api-keys
yepapi:
  enabled: false
  # Each service can be enabled independently
  seo:
    enabled: false
  serp:
    enabled: false
  scraping:
    enabled: false
  youtube:
    enabled: false
  tiktok:
    enabled: false
  instagram:
    enabled: false
  amazon:
    enabled: false
```

---

## 5. Tool Manual (`prompts/tools_manuals/yepapi.md`)

```markdown
# YepAPI Tools

YepAPI provides unified access to SEO data, search results, web scraping, and social media APIs.

## yepapi_seo
SEO analysis tools including keyword research, domain overview, competitor analysis, backlinks, and on-page audits.

Operations:
- `keywords` — Bulk keyword metrics (volume, CPC, difficulty, intent)
- `keyword_ideas` — Keyword suggestions from seed
- `domain_overview` — Domain traffic, backlinks, rank
- `domain_keywords` — Keywords a domain ranks for
- `competitors` — Competing domains for keywords
- `backlinks` — Backlink profile
- `onpage` — Technical page audit
- `trends` — Google Trends data

## yepapi_serp
Search engine results from Google, Bing, Yahoo, Baidu, YouTube, and more.

Operations:
- `google` — Google organic results
- `google_maps` — Google Maps places
- `google_images` — Google Images
- `google_news` — Google News
- `bing` — Bing results
- `youtube` — YouTube search via SERP

Parameters:
- `query` (required) — Search query
- `depth` — Number of results (default 10)
- `location` — Country code (default "us")
- `language` — Language code (default "en")

## yepapi_scrape
Web scraping with multiple modes.

Operations:
- `scrape` — Standard page to markdown/HTML
- `js` — JavaScript-rendered page
- `stealth` — Anti-bot bypass
- `screenshot` — Full-page screenshot (base64 PNG)
- `ai_extract` — AI-powered data extraction

## yepapi_youtube
YouTube data without quota limits.

Operations:
- `search` — Search videos
- `video` — Full video metadata
- `transcript` — Video transcripts
- `comments` — Video comments
- `channel` — Channel info
- `playlist` — Playlist details
- `trending` — Trending videos
- `shorts` — Shorts feed

## yepapi_tiktok
TikTok data access.

Operations:
- `search` — Search videos
- `search_user` — Search users
- `video` — Video details by URL
- `user` — User profile
- `user_posts` — User's videos
- `comments` — Video comments

## yepapi_instagram
Instagram data access.

Operations:
- `search` — Search users/hashtags/places
- `user` — User profile
- `user_posts` — User's posts
- `user_reels` — User's reels
- `post` — Post details by shortcode
- `hashtag` — Hashtag posts

## yepapi_amazon
Amazon product data.

Operations:
- `search` — Product search
- `product` — Product details by ASIN
- `reviews` — Product reviews
- `deals` — Live deals
- `best_sellers` — Best sellers

## Pricing
All YepAPI tools are pay-per-call. Costs are deducted from your YepAPI balance.
Failed requests are never charged.
```

---

## 6. UI Translations

Neue Keys für alle 15 Sprachen (`ui/lang/*/config.json`):

```json
{
  "yepapi": {
    "title": "YepAPI",
    "description": "Unified API for SEO, SERP, scraping, and social media",
    "enabled": "Enable YepAPI",
    "provider_title": "YepAPI Provider",
    "provider_description": "Use YepAPI as LLM provider (OpenAI-compatible)",
    "services": {
      "seo": "SEO Tools",
      "serp": "SERP Search",
      "scraping": "Web Scraping",
      "youtube": "YouTube Data",
      "tiktok": "TikTok Data",
      "instagram": "Instagram Data",
      "amazon": "Amazon Data"
    }
  }
}
```

---

## 7. Testing Plan

### 7.1 Unit Tests (`internal/tools/yepapi_test.go`)

```go
func TestYepAPIClient_Post(t *testing.T) {
    // Mock server testing standard envelope
}

func TestYepAPIClient_YouTubeSearch(t *testing.T) {
    // Test YouTube search dispatch
}

func TestYepAPIClient_SEOKeywords(t *testing.T) {
    // Test SEO keywords dispatch
}

func TestYepAPIClient_ErrorHandling(t *testing.T) {
    // Test INVALID_API_KEY, NO_CREDITS, etc.
}
```

### 7.2 Integration Tests

- Config-Validierung mit `yepapi`-Provider
- Vault-Key-Auflösung
- Tool-Schema-Generierung mit korrekten Feature Flags
- Dispatch-Routing für alle 7 Tools

---

## 8. Security & Safety

### 8.1 Vault Integration

- API Key wird als `provider_<id>_api_key` im Vault gespeichert
- Falls kein Provider konfiguriert: dedizierter `yepapi_api_key` Vault-Eintrag
- Key wird **niemals** in Logs oder LLM-Outputs angezeigt
- `security.RegisterSensitive(apiKey)` aufrufen

### 8.2 Python-Sandbox-Exclusion

- YepAPI API Key muss in die Liste der **verbotenen Exporte** für Python-Tools aufgenommen werden
- Location: `internal/security/vault.go` oder äquivalente Stelle

### 8.3 Read-Only Nature

- Alle YepAPI-Tools sind **rein lesend** (Datenabfrage)
- Keine Write/Delete-Operationen möglich
- Daher kein separates `ReadOnly`-Flag nötig

---

## 9. Implementation Order (Prioritized)

| Phase | Task | Estimated Effort | Priority |
|-------|------|-----------------|----------|
| 1 | Provider support (`type: yepapi` in LLM client) | 30 min | P0 |
| 2 | Config types + defaults + migration | 45 min | P0 |
| 3 | Core YepAPI client (`yepapi.go`) | 60 min | P0 |
| 4 | Feature flags + policy resolution | 30 min | P0 |
| 5 | Tool schemas (7 tools) | 90 min | P0 |
| 6 | Tool implementations (7 service files) | 180 min | P0 |
| 7 | Agent dispatch wiring | 60 min | P0 |
| 8 | Tool manual | 30 min | P1 |
| 9 | Unit tests | 90 min | P1 |
| 10 | UI translations (15 languages) | 60 min | P1 |
| 11 | Config template update | 15 min | P1 |
| 12 | Integration test + build verification | 30 min | P1 |

**Total Estimated Effort: ~12 hours**

---

## 10. Future Enhancements (Backlog)

1. **AI Media Generation** — Bild/Video/Musik-Generierung über `/v1/media/queue` (async polling)
2. **AI Chat als Direct Tool** — Direkter Zugriff auf 69 Modelle über `yepapi_chat`-Tool
3. **Streaming Support** — Für LLM-Provider-Integration
4. **Caching Layer** — SERP- und SEO-Ergebnisse cachen (kostenintensive Requests)
5. **Batch Operations** — Mehrere Keywords/Domains in einem Request

---

## 11. Open Questions / Decisions

1. **Soll YepAPI auch als Fallback-Provider nutzbar sein?** → Ja, über Standard-Provider-Mechanismus
2. **Sollen die 69 LLM-Modelle in der UI als Dropdown angezeigt werden?** → Nein, Modelle sind dynamisch über `/v1/ai/models` verfügbar
3. **Soll es ein Test-Connection-Button in der UI geben?** → Ja, für Provider + jeden Service
4. **Wie wird das Rate-Limiting (60/min) gehandhabt?** → Client-seitiger Rate-Limiter optional

---

*Plan erstellt von Kimi Code CLI für AuraGo*
*Basierend auf: YepAPI llms.txt (2026-04-26) + AuraGo AGENTS.md + Codebase-Exploration*
