# YepAPI Tools

YepAPI provides unified access to SEO data, search results, web scraping, and social media APIs through a single API key.

## Authentication

All YepAPI tools share the same API key. Configure a provider with `type: yepapi` in the Providers section, or set the `yepapi_api_key` vault secret directly.

## yepapi_seo

SEO analysis tools including keyword research, domain overview, competitor analysis, backlinks, and on-page audits.

**Operations:**
- `keywords` — Bulk keyword metrics: search volume, CPC, competition, difficulty, intent, trends, SERP features. Provide a JSON array of keywords via the `keywords` parameter.
- `keyword_ideas` — Keyword suggestions from a seed keyword. Provide the seed via the `seed` parameter.
- `domain_overview` — Domain metrics: organic traffic, keywords, backlinks, referring domains, domain rank. Provide the domain via the `domain` parameter.
- `domain_keywords` — Keywords a domain ranks for. Provide the domain via the `domain` parameter.
- `competitors` — Domains competing for a keyword set. Provide either `domain` or `keywords`.
- `backlinks` — Backlink profile summary for a target domain or URL. Provide the target via the `target` parameter.
- `onpage` — Technical page audit (meta, headings, links, images). Provide the URL via the `url` parameter.
- `trends` — Google Trends interest over time for up to 5 keywords. Provide a JSON array via the `keywords` parameter.

**Pricing:** $0.02–$0.15 per call depending on the endpoint.

## yepapi_serp

Search engine results from Google, Bing, Yahoo, Baidu, YouTube, and more.

**Operations:**
- `google` — Google organic results
- `google_images` — Google Images results
- `google_news` — Google News results
- `google_maps` — Google Maps Places with full place data (ratings, reviews, hours, photos)
- `google_datasets` — Google Dataset Search
- `google_autocomplete` — Google Autocomplete suggestions
- `google_ads` — Google Ads Transparency Center
- `google_ai_mode` — Google AI Mode / AI Overviews
- `google_finance` — Google Finance results
- `yahoo` — Yahoo organic results
- `bing` — Bing organic results
- `baidu` — Baidu organic results
- `youtube` — YouTube search via SERP

**Parameters:**
- `query` (required) — Search query
- `depth` — Number of results (default: 10)
- `location` — Country code, e.g. "us", "de", "uk" (default: "us")
- `language` — Language code, e.g. "en", "de" (default: "en")
- `limit` — Max results for Google Maps (default: 10)
- `open_now` — Filter Google Maps for currently open places

**Pricing:** $0.01 per call.

## yepapi_scrape

Web scraping with multiple modes.

**Operations:**
- `scrape` — Standard page scrape to markdown or HTML. Set `format` to "markdown" or "html" (default: markdown).
- `js` — JavaScript-rendered page scrape
- `stealth` — Stealth scrape with anti-bot bypass
- `screenshot` — Full-page screenshot as base64 PNG
- `ai_extract` — AI-powered data extraction via natural language prompt. Provide the extraction instruction via the `prompt` parameter.

**Parameters:**
- `url` (required) — URL to scrape
- `format` — Output format for scrape operation
- `prompt` — Natural language extraction instruction for ai_extract

**Pricing:** $0.01–$0.03 per call.

## yepapi_youtube

YouTube data without quota limits.

**Operations:**
- `search` — Search videos, channels, playlists
- `video` — Full video metadata and formats by video ID
- `transcript` — Video transcript / captions by video ID
- `comments` — Video comments by video ID
- `channel` — Channel overview by channel ID
- `channel_videos` — Channel videos list by channel ID
- `playlist` — Playlist details and videos by playlist ID
- `trending` — Trending videos
- `shorts` — Shorts feed
- `suggest` — Search suggestions / autocomplete

**Parameters:**
- `query` — Search query (for search, suggest)
- `video_id` — YouTube video ID (for video, transcript, comments)
- `channel_id` — YouTube channel ID (for channel, channel_videos)
- `playlist_id` — YouTube playlist ID (for playlist)
- `limit` — Max results (default: 10)

**Pricing:** $0.01–$0.02 per call.

## yepapi_tiktok

TikTok data access.

**Operations:**
- `search` — Search TikTok videos by keyword
- `search_user` — Search TikTok users by keyword
- `video` — Full video details by URL
- `user` — User profile by username/unique_id
- `user_posts` — User's posted videos by username
- `comments` — Video comments by video URL
- `music` — Music/sound info by URL
- `challenge` — Challenge info by name

**Parameters:**
- `query` — Search query (for search, search_user)
- `url` — TikTok video or music URL (for video, comments, music)
- `username` — TikTok username/unique_id (for user, user_posts)
- `name` — Challenge name (for challenge)
- `limit` — Max results (default: 10)

**Pricing:** $0.01 per call.

## yepapi_instagram

Instagram data access.

**Operations:**
- `search` — Search users, hashtags, and places
- `user` — User profile by username
- `user_posts` — User's posts by username
- `user_reels` — User's reels by username
- `post` — Post details by shortcode
- `post_comments` — Post comments by shortcode
- `hashtag` — Hashtag top and recent posts

**Parameters:**
- `query` — Search query (for search)
- `username` — Instagram username (for user, user_posts, user_reels)
- `shortcode` — Instagram post shortcode (for post, post_comments)
- `tag` — Hashtag without # (for hashtag)
- `limit` — Max results (default: 10)

**Pricing:** $0.01 per call.

## yepapi_amazon

Amazon product data.

**Operations:**
- `search` — Search Amazon products by keyword
- `product` — Full product details for an ASIN (50+ fields)
- `reviews` — Product reviews with star/verified filters
- `deals` — Amazon's live deals feed
- `best_sellers` — Best sellers per category

**Parameters:**
- `query` — Search query (for search)
- `asin` — Amazon ASIN (for product, reviews)
- `country` — Marketplace country code: "US", "UK", "DE", etc. (default: "US")
- `category` — Category slug or browse node ID (for deals, best_sellers)
- `limit` — Max results (default: 10)
- `sort_by` — Review sort: "TOP_REVIEWS" or "MOST_RECENT" (for reviews)

**Pricing:** $0.01–$0.02 per call.

## General Notes

- All YepAPI tools are **read-only** (data retrieval only).
- **Pay-per-call pricing:** Costs are deducted from your YepAPI prepaid balance. Failed requests are never charged.
- **Rate limit:** 60 requests/minute per API key.
- **Response format:** All tools return JSON with a `{ "status": "success", "data": { ... } }` envelope on success, or `{ "status": "error", "message": "..." }` on failure.
