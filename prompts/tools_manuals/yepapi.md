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
- `extract` — CSS/XPath structured extraction. Provide either `selector` or `xpath`.
- `ai_extract` — AI-powered data extraction via natural language prompt. Provide the extraction instruction via the `prompt` parameter.
- `search_google` — Google search through YepAPI search endpoint. Provide the search text via `query`.

**Parameters:**
- `url` — URL to scrape (required for scrape/js/stealth/screenshot/extract/ai_extract)
- `query` — Search query (required for search_google)
- `selector` — CSS selector for extract
- `xpath` — XPath selector for extract
- `format` — Output format for scrape operation
- `prompt` — Natural language extraction instruction for ai_extract
- `limit` — Max search results for search_google

**Pricing:** $0.01–$0.03 per call.

## yepapi_youtube

YouTube data without quota limits.

**Operations:**
- `search` — Search videos, channels, playlists
- `video` — Full video metadata and formats by video ID
- `video_info` — Lightweight video information by video ID
- `metadata` — Video metadata by video ID
- `transcript` — Video transcript / captions by video ID
- `subtitles` — Video subtitles by video ID
- `comments` — Video comments by video ID
- `channel` — Channel overview by channel ID
- `channel_videos` — Channel videos list by channel ID
- `channel_shorts` — Channel shorts by channel ID
- `channel_livestreams` — Channel live streams by channel ID
- `channel_live` — Backward-compatible alias for `channel_livestreams`
- `channel_playlists` — Channel playlists by channel ID
- `channel_community` — Channel community posts by channel ID
- `channel_about` — Channel about information by channel ID
- `channel_channels` — Related/subscribed channels by channel ID
- `channel_store` — Channel store/merch info by channel ID
- `channel_search` — Search within a channel by channel ID and query
- `playlist` — Playlist details and videos by playlist ID
- `playlist_info` — Playlist metadata by playlist ID
- `trending` — Trending videos
- `related` — Related videos by video ID
- `screenshot` — Video screenshot by video ID
- `shorts` — Shorts feed
- `shorts_info` — Shorts details by video ID
- `suggest` — Search suggestions / autocomplete
- `hashtag` — Hashtag feed by tag
- `post` — Single community post by post ID
- `post_comments` — Community post comments by post ID
- `home` — YouTube home feed
- `hype` — YouTube hype feed
- `resolve` — Resolve a YouTube URL

**Parameters:**
- `query` — Search query (for search, suggest, shorts, channel_search)
- `video_id` — YouTube video ID (for video, video_info, metadata, transcript, subtitles, comments, related, screenshot, shorts_info)
- `channel_id` — YouTube channel ID (for channel and channel_* operations)
- `playlist_id` — YouTube playlist ID (for playlist and playlist_info)
- `url` — YouTube URL (for resolve)
- `tag` — Hashtag/tag without # (for hashtag)
- `post_id` — YouTube community post ID (for post and post_comments)
- `country` / `language` — Optional feed localization
- `limit` — Max results (default: 10)

**Pricing:** $0.01–$0.02 per call.

## yepapi_tiktok

TikTok data access.

**Operations:**
- `search` — Search TikTok videos by keyword
- `search_user` — Search TikTok users by keyword
- `search_challenge` — Search TikTok challenges by keyword
- `search_photo` — Search TikTok photo posts by keyword
- `video` — Full video details by URL
- `user` — User profile by username/unique_id
- `user_posts` — User's posted videos by username
- `user_followers` — User followers by username
- `user_following` — User following by username
- `user_favorites` — User favorites by username
- `user_reposts` — User reposts by username
- `user_story` — User story by username
- `comments` — Video comments by video URL
- `comment_replies` — Replies for a comment ID
- `music` — Music/sound info by URL
- `music_videos` — Videos using a music/sound URL
- `challenge` — Challenge info by name
- `challenge_videos` — Videos for a challenge name

**Parameters:**
- `query` — Search query (for search, search_user, search_challenge, search_photo)
- `url` — TikTok video or music URL (for video, comments, music, music_videos)
- `username` — TikTok username/unique_id (for user and user_* operations)
- `name` — Challenge name (for challenge and challenge_videos)
- `comment_id` — Comment ID (for comment_replies)
- `limit` — Max results (default: 10)

**Pricing:** $0.01 per call.

## yepapi_instagram

Instagram data access.

**Operations:**
- `search` — Search users, hashtags, and places
- `user` — User profile by username
- `userinfo`, `user_info`, `profile`, `user_profile` — Backward-compatible aliases for `user`; prefer `user` in new calls
- `user_about` — User about/profile details by username
- `user_posts` — User's posts by username
- `user_reels` — User's reels by username
- `user_stories` — User stories by username
- `user_highlights` — User highlights by username
- `user_tagged` — Posts where user is tagged
- `user_followers` — User followers by username
- `user_similar` — Similar users by username
- `post` — Post details by shortcode
- `post_comments` — Post comments by shortcode
- `post_likers` — Post likers by shortcode
- `hashtag` — Hashtag top and recent posts
- `media_id` — Resolve media ID by shortcode

**Parameters:**
- `search_query` — Search query (for search operation).
- `username_or_url` — Instagram username or profile URL (e.g. `https://www.instagram.com/natgeo/`) for user and user_* operations. Supported directly.
- `shortcode` — Instagram post shortcode (for post, post_comments, post_likers, media_id)
- `tag` — Hashtag without # (for hashtag)
- `limit` — Max results (default: 10)

**Pricing:** $0.01 per call.

## yepapi_amazon

Amazon product data.

**Operations:**
- `search` — Search Amazon products by keyword
- `product` — Full product details for an ASIN (50+ fields)
- `reviews` — Product reviews with star/verified filters
- `product_offers` — Product offers for an ASIN
- `products_by_category` — Products for a category slug or browse node
- `categories` — Amazon category list for a marketplace
- `deals` — Amazon's live deals feed
- `best_sellers` — Best sellers per category
- `influencer` — Amazon influencer page data
- `seller` — Amazon seller profile data
- `seller_reviews` — Amazon seller reviews

**Parameters:**
- `query` — Search query (for search)
- `asin` — Amazon ASIN (for product, reviews, product_offers)
- `country` — Marketplace country code: "US", "UK", "DE", etc. (default: "US")
- `category` — Category slug or browse node ID (for products_by_category, deals, best_sellers)
- `handle` — Amazon influencer handle (for influencer)
- `seller_id` — Amazon seller ID (for seller and seller_reviews)
- `limit` — Max results (default: 10)
- `page` — Page number for paginated operations
- `sort_by` — Review sort: "TOP_REVIEWS" or "MOST_RECENT" (for reviews)

**Pricing:** $0.01–$0.02 per call.

## General Notes

- All YepAPI tools are **read-only** (data retrieval only).
- **Pay-per-call pricing:** Costs are deducted from your YepAPI prepaid balance. Failed requests are never charged.
- **Rate limit:** 60 requests/minute per API key.
- **Response format:** All tools return JSON with a `{ "status": "success", "data": { ... } }` envelope on success, or `{ "status": "error", "message": "..." }` on failure.
