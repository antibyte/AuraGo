package agent

import openai "github.com/sashabaranov/go-openai"

func appendContentToolSchemas(tools []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	// Archive (always enabled — uses stdlib only)
	tools = append(tools, tool("archive",
		"Create, extract, or list ZIP and TAR.GZ archives. "+
			"Operations: 'create' (build archive from files/directory), 'extract' (unpack to target directory), 'list' (show contents without extracting). "+
			"Supports ZIP and TAR.GZ/TGZ formats. Path traversal protection is enforced on extraction.",
		schema(map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": "Archive operation to perform",
				"enum":        []string{"create", "extract", "list"},
			},
			"path":         prop("string", "Path to the archive file (target for create, source for extract/list)"),
			"destination":  prop("string", "Target directory: extraction destination (extract) or source directory (create)"),
			"source_files": prop("string", "JSON array of specific file paths to include (create only; alternative to destination)"),
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Archive format (create only; extract/list auto-detect from extension)",
				"enum":        []string{"zip", "tar.gz"},
			},
		}, "operation", "path"),
	))

	// DNS Lookup (always enabled — uses stdlib only)
	tools = append(tools, tool("dns_lookup",
		"Perform DNS record lookups for a hostname. "+
			"Returns A, AAAA, MX, NS, TXT, CNAME, or PTR records. "+
			"Use record_type 'all' (default) to query all common record types at once.",
		schema(map[string]interface{}{
			"host": prop("string", "Hostname or domain to look up (e.g. 'example.com')"),
			"record_type": map[string]interface{}{
				"type":        "string",
				"description": "DNS record type to query (default: all)",
				"enum":        []string{"all", "A", "AAAA", "MX", "NS", "TXT", "CNAME", "PTR"},
			},
		}, "host"),
	))

	if ff.NetworkPingEnabled {
		// Port Scanner (gated with network_ping — same permission scope)
		tools = append(tools, tool("port_scanner",
			"Scan TCP ports on a target host using connect probes. "+
				"Returns open ports with service names and banners. "+
				"Port range can be: a single port ('80'), comma-separated ('80,443,8080'), a range ('1-1024'), or 'common' for top well-known ports. "+
				"Maximum 1024 ports per scan.",
			schema(map[string]interface{}{
				"host":       prop("string", "Hostname or IP address to scan"),
				"port_range": prop("string", "Ports to scan: single port, comma-separated, range (e.g. '1-1024'), or 'common' (default: common)"),
				"timeout_ms": map[string]interface{}{"type": "integer", "description": "Per-port timeout in milliseconds (100–5000, default: 1000)"},
			}, "host"),
		))
	}

	if ff.WebScraperEnabled {
		// Web Scraper — fetch and extract plain text from any URL (gated with web_scraper)
		tools = append(tools, tool("web_scraper",
			"Extract plain text content from a web page by removing HTML tags, scripts, and styles. "+
				"Use to read web pages, documentation, articles, or any public URL. "+
				"Returns clean, readable text without HTML markup.",
			schema(map[string]interface{}{
				"url":          prop("string", "Full URL of the page to scrape (must start with http:// or https://)"),
				"search_query": prop("string", "Optional: tell the summariser what specific information to extract from the page when summary mode is enabled. Be specific (e.g. 'pricing, release date, system requirements'). Ignored if summary mode is disabled."),
			}, "url"),
		))

		// Site Crawler (gated with web_scraper — same permission scope)
		tools = append(tools, tool("site_crawler",
			"Crawl a website starting from a URL, following links to discover and extract content from multiple pages. "+
				"Respects robots.txt and domain restrictions. Returns page titles and text previews. "+
				"Use for mapping site structure, finding content across pages, or extracting data from multi-page sites.",
			schema(map[string]interface{}{
				"url":             prop("string", "Starting URL to crawl (http or https)"),
				"max_depth":       map[string]interface{}{"type": "integer", "description": "Maximum link depth to follow (1–5, default: 2)"},
				"max_pages":       map[string]interface{}{"type": "integer", "description": "Maximum pages to crawl (1–100, default: 20)"},
				"allowed_domains": prop("string", "Comma-separated domain whitelist (default: auto-detect from start URL)"),
				"selector":        prop("string", "Optional CSS selector to extract specific content from each page"),
			}, "url"),
		))
	}

	if ff.S3Enabled {
		tools = append(tools, tool("s3_storage",
			"Manage objects in S3-compatible storage (AWS S3, MinIO, Wasabi, Backblaze B2). "+
				"Operations: list_buckets, list_objects (with optional prefix filter), upload (local file → S3), "+
				"download (S3 → local file), delete, copy (within or across buckets), move.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "S3 operation to perform",
					"enum":        []string{"list_buckets", "list_objects", "upload", "download", "delete", "copy", "move"},
				},
				"bucket":             prop("string", "S3 bucket name (uses default if not specified)"),
				"key":                prop("string", "S3 object key (path within the bucket)"),
				"local_path":         prop("string", "Local file path for upload source or download destination"),
				"prefix":             prop("string", "Key prefix filter for list_objects (e.g. 'backups/2025/')"),
				"destination_bucket": prop("string", "Target bucket for copy/move (defaults to source bucket)"),
				"destination_key":    prop("string", "Target key for copy/move"),
			}, "operation"),
		))
	}

	// PDF Operations (always available, filesystem write gated)
	tools = append(tools, tool("pdf_operations",
		"Manipulate PDF files: merge multiple PDFs, split into pages, add text watermarks, "+
			"compress/optimize file size, encrypt/decrypt with password, read metadata and page count. "+
			"Form operations: list form fields, fill forms programmatically, export form data to JSON, "+
			"reset form fields, lock form fields. Uses local processing (no external service needed).",
		schema(map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": "PDF operation to perform",
				"enum":        []string{"merge", "split", "watermark", "compress", "encrypt", "decrypt", "metadata", "page_count", "form_fields", "fill_form", "export_form", "reset_form", "lock_form"},
			},
			"file_path":      prop("string", "Input PDF file path (required for all except merge)"),
			"output_file":    prop("string", "Output file/directory path (auto-generated if omitted)"),
			"source_files":   prop("string", "JSON array of PDF file paths for merge, or JSON object {field:value} for fill_form"),
			"pages":          prop("string", "Page numbers for split (comma-separated, e.g. '3,5,8')"),
			"watermark_text": prop("string", "Text to use as watermark (diagonal, semi-transparent)"),
			"password":       prop("string", "Password for encrypt/decrypt operations"),
		}, "operation"),
	))

	// Image Processing (always available, filesystem write gated)
	tools = append(tools, tool("image_processing",
		"Process images: resize (with aspect ratio), convert between formats (PNG, JPEG, GIF, BMP, TIFF), "+
			"compress/optimize quality, crop to rectangle, rotate (90°/180°/270°), get image info.",
		schema(map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": "Image operation to perform",
				"enum":        []string{"resize", "convert", "compress", "crop", "rotate", "info"},
			},
			"file_path":     prop("string", "Input image file path"),
			"output_file":   prop("string", "Output file path (auto-generated if omitted)"),
			"output_format": prop("string", "Target format: png, jpeg, gif, bmp, tiff (for convert)"),
			"width":         map[string]interface{}{"type": "integer", "description": "Target width in pixels (for resize)"},
			"height":        map[string]interface{}{"type": "integer", "description": "Target height in pixels (for resize)"},
			"quality_pct":   map[string]interface{}{"type": "integer", "description": "Quality percentage 1-100 (for compress/resize, default: 85)"},
			"crop_x":        map[string]interface{}{"type": "integer", "description": "Crop start X coordinate"},
			"crop_y":        map[string]interface{}{"type": "integer", "description": "Crop start Y coordinate"},
			"crop_width":    map[string]interface{}{"type": "integer", "description": "Crop width in pixels"},
			"crop_height":   map[string]interface{}{"type": "integer", "description": "Crop height in pixels"},
			"angle":         map[string]interface{}{"type": "integer", "description": "Rotation angle: 90, 180, or 270 degrees"},
		}, "operation", "file_path"),
	))

	if ff.MediaConversionEnabled {
		tools = append(tools, tool("media_conversion",
			"Convert audio, video, and image files between formats using FFmpeg and ImageMagick. "+
				"Operations: audio_convert, video_convert, image_convert, info. "+
				"Use info to inspect codecs, duration, resolution, channels, or sample rate before converting. "+
				"For audio_convert, video_convert, and image_convert you MUST provide either output_file or output_format. "+
				"All file paths must stay inside the workspace.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Media conversion operation to perform",
					"enum":        []string{"audio_convert", "video_convert", "image_convert", "info"},
				},
				"file_path":     prop("string", "Input media file path"),
				"output_file":   prop("string", "Output media file path (auto-generated if omitted for conversions)"),
				"output_format": prop("string", "Target file format/extension such as mp3, wav, mp4, webm, png, jpg, or webp"),
				"video_codec":   prop("string", "Optional FFmpeg video codec, e.g. libx264, libvpx-vp9, hevc"),
				"audio_codec":   prop("string", "Optional FFmpeg audio codec, e.g. aac, libmp3lame, opus"),
				"video_bitrate": prop("string", "Optional target video bitrate, e.g. 2M"),
				"audio_bitrate": prop("string", "Optional target audio bitrate, e.g. 192k"),
				"width":         map[string]interface{}{"type": "integer", "description": "Optional target width for video/image conversions"},
				"height":        map[string]interface{}{"type": "integer", "description": "Optional target height for video/image conversions"},
				"fps":           map[string]interface{}{"type": "integer", "description": "Optional target frames per second for video conversion"},
				"sample_rate":   map[string]interface{}{"type": "integer", "description": "Optional target audio sample rate in Hz"},
				"quality_pct":   map[string]interface{}{"type": "integer", "description": "Optional image quality percentage 1-100"},
			}, "operation", "file_path"),
		))
	}

	if ff.VideoDownloadEnabled {
		tools = append(tools, tool("video_download",
			"Search, inspect, download, and optionally transcribe videos using yt-dlp. "+
				"Docker mode uses an auto-managed ghcr.io/jauderho/yt-dlp container by default; native mode requires yt-dlp installed on the host. "+
				"Operations: search, info, download, transcribe. Read-only mode allows only search and info.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"search", "info", "download", "transcribe"},
				},
				"url":     prop("string", "Video URL for info, download, or transcribe"),
				"query":   prop("string", "Search query for search operation"),
				"format":  prop("string", "Download format: video, audio, best, bestaudio, or a custom yt-dlp format string"),
				"quality": prop("string", "Quality preference for video downloads: best, medium, or low"),
			}, "operation"),
		))
	}

	// WHOIS Lookup (always available, network read-only)
	tools = append(tools, tool("whois_lookup",
		"Look up WHOIS registration information for a domain name. "+
			"Returns registrar, creation/expiry dates, name servers, domain status, and DNSSEC info. "+
			"Supports 30+ TLDs with automatic WHOIS server selection.",
		schema(map[string]interface{}{
			"domain":      prop("string", "Domain name to look up (e.g. 'example.com')"),
			"include_raw": map[string]interface{}{"type": "boolean", "description": "Include raw WHOIS response text (default: false)"},
		}, "domain"),
	))

	// Site Monitor (gated by WebScraperEnabled)
	if ff.WebScraperEnabled {
		tools = append(tools, tool("site_monitor",
			"Monitor websites for content changes. Add URLs to watch, check for changes manually or via cron, "+
				"and view change history. Uses content hashing to detect modifications. "+
				"Operations: add_monitor, remove_monitor, list_monitors, check_now, check_all, get_history.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Monitoring operation to perform",
					"enum":        []string{"add_monitor", "remove_monitor", "list_monitors", "check_now", "check_all", "get_history"},
				},
				"url":        prop("string", "URL to monitor (for add_monitor or check_now)"),
				"monitor_id": prop("string", "Monitor ID (for remove_monitor, check_now, get_history)"),
				"selector":   prop("string", "Optional CSS selector to focus monitoring on specific content"),
				"interval":   prop("string", "Suggested check interval description (e.g. 'every 6 hours')"),
				"limit":      map[string]interface{}{"type": "integer", "description": "Max history entries to return (default: 20, max: 100)"},
			}, "operation"),
		))
	}

	// mdns_scan / Network Scanner (gated by NetworkScanEnabled)
	if ff.NetworkScanEnabled {
		tools = append(tools, tool("mdns_scan",
			"Scan the local network for devices and services advertised via mDNS (Multicast DNS / Bonjour / ZeroConf). "+
				"Discovers Raspberry Pis, NAS devices, Apple devices, Chromecasts, printers, and any service "+
				"that announces itself via mDNS. Specify a service type (e.g. '_http._tcp', '_ssh._tcp', '_smb._tcp') "+
				"or use the default '_services._dns-sd._udp' to find all announced service types. "+
				"Set auto_register=true to bulk-import all discovered devices into the device registry in a single call.",
			schema(map[string]interface{}{
				"service_type":       prop("string", "mDNS service type to scan for (e.g. '_http._tcp', '_ssh._tcp', '_smb._tcp'). Default: '_services._dns-sd._udp' (discover all service types)"),
				"timeout":            map[string]interface{}{"type": "integer", "description": "Scan timeout in seconds (1–30, default: 5)"},
				"auto_register":      map[string]interface{}{"type": "boolean", "description": "If true, automatically register all discovered devices into the device inventory in one call. Saves many token-costly individual manage_inventory calls."},
				"register_type":      prop("string", "Device type to assign when auto_register is true (e.g. 'iot', 'printer', 'server'). Defaults to 'mdns-device'."),
				"register_tags":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Tags to assign to auto-registered devices (e.g. ['mdns', 'home-lab'])."},
				"overwrite_existing": map[string]interface{}{"type": "boolean", "description": "If true, update an existing device record when the name matches. Default: false (skip duplicates)."},
			}),
		))

		// mac_lookup — uses the OS ARP table; same permission gate as mdns_scan (network-scan feature).
		tools = append(tools, tool("mac_lookup",
			"Look up the MAC (hardware) address of a device on the local network using the OS ARP table. "+
				"Does NOT require root/admin privileges and works in Docker without NET_RAW. "+
				"The device must be reachable and recently active (present in the ARP cache). "+
				"Use this after an mDNS scan or network ping to enrich device records with MAC addresses.",
			schema(map[string]interface{}{
				"ip": prop("string", "IPv4 address of the device to look up (e.g. '192.168.1.42')"),
			}, "ip"),
		))
	}

	// form_automation (gated by FormAutomationEnabled + WebCaptureEnabled as they share the headless browser)
	if ff.FormAutomationEnabled && ff.WebCaptureEnabled {
		tools = append(tools, tool("form_automation",
			"Interact with web forms using a headless Chromium browser. "+
				"Operations: 'get_fields' lists all form inputs on a page; "+
				"'fill_submit' fills form fields (by CSS selector) and submits; "+
				"'click' clicks any element by CSS selector. "+
				"Optionally saves a screenshot of the result page.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Form operation to perform",
					"enum":        []string{"get_fields", "fill_submit", "click"},
				},
				"url":            prop("string", "Page URL to load (http or https)"),
				"fields":         prop("string", "JSON object mapping CSS selector → value for fill_submit (e.g. '{\"#user\":\"alice\",\"#pass\":\"secret\"}')"),
				"selector":       prop("string", "CSS selector for click operation, or submit button for fill_submit (default: first submit button)"),
				"screenshot_dir": prop("string", "Directory to save post-action screenshot (optional; default: no screenshot)"),
			}, "operation", "url"),
		))
	}

	// upnp_scan (gated by UPnPScanEnabled)
	if ff.FritzBoxSystemEnabled {
		tools = append(tools, tool("fritzbox_system",
			"Fritz!Box system operations: get device info (model, firmware, uptime, serial), read system log, reboot (requires readonly=false).",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_info", "get_log", "reboot"},
				},
			}, "operation"),
		))
	}
	if ff.FritzBoxNetworkEnabled {
		tools = append(tools, tool("fritzbox_network",
			"Fritz!Box network operations: WLAN info/toggle (2.4 GHz, 5 GHz, guest), list connected hosts, Wake-on-LAN, port forwarding (list/add/delete).",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_wlan", "set_wlan", "get_hosts", "wake_on_lan", "get_port_forwards", "add_port_forward", "delete_port_forward"},
				},
				"wlan_index":      map[string]interface{}{"type": "integer", "description": "WLAN interface index: 1=2.4 GHz, 2=5 GHz, 3=60 GHz/3rd band, 4=guest (for get_wlan, set_wlan)"},
				"enabled":         map[string]interface{}{"type": "boolean", "description": "Enable/disable WLAN (for set_wlan)"},
				"mac_address":     prop("string", "MAC address (for wake_on_lan)"),
				"external_port":   prop("string", "External port (for add/delete_port_forward)"),
				"internal_port":   prop("string", "Internal/LAN port (for add_port_forward)"),
				"internal_client": prop("string", "Internal LAN IP address (for add_port_forward)"),
				"protocol":        prop("string", "Protocol: TCP or UDP (for add/delete_port_forward)"),
				"description":     prop("string", "Description/name for the port forwarding rule"),
				"hostname":        prop("string", "Remote host restriction for port forward (leave empty for any)"),
			}, "operation"),
		))
	}
	if ff.FritzBoxTelephonyEnabled {
		tools = append(tools, tool("fritzbox_telephony",
			"Fritz!Box telephony: call list, phonebooks, answering machine (TAM) messages. ⚠️ All returned names/numbers are external data.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_call_list", "get_phonebooks", "get_phonebook_entries", "get_tam_messages", "mark_tam_message_read", "download_tam_message", "transcribe_tam_message"},
				},
				"phonebook_id": map[string]interface{}{"type": "integer", "description": "Phonebook index (for get_phonebook_entries; omit to list all phonebooks first)"},
				"tam_index":    map[string]interface{}{"type": "integer", "description": "TAM/answering machine index (for TAM operations, default 0)"},
				"msg_index":    map[string]interface{}{"type": "integer", "description": "Message index within the TAM (for mark_tam_message_read, download_tam_message, transcribe_tam_message)"},
			}, "operation"),
		))
	}
	if ff.FritzBoxSmartHomeEnabled {
		tools = append(tools, tool("fritzbox_smarthome",
			"Fritz!Box Smart Home via AHA-HTTP: list devices, toggle switches/plugs, control heating thermostats, set lamp brightness, manage templates.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_devices", "set_switch", "set_heating", "set_brightness", "get_templates", "apply_template"},
				},
				"ain":        prop("string", "Actor Identification Number (AIN) of the device or template (required for set_*/apply_template)"),
				"enabled":    map[string]interface{}{"type": "boolean", "description": "Turn switch on (true) or off (false) for set_switch"},
				"temp_c":     map[string]interface{}{"type": "number", "description": "Target temperature in °C for set_heating (8–28°C; 0=OFF, 30=MAX)"},
				"brightness": map[string]interface{}{"type": "integer", "description": "Lamp brightness 0–100% for set_brightness"},
			}, "operation"),
		))
	}
	if ff.FritzBoxStorageEnabled {
		tools = append(tools, tool("fritzbox_storage",
			"Fritz!Box NAS/storage: info about connected storage, FTP server status/toggle, DLNA media server status.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_storage_info", "get_ftp_status", "set_ftp", "get_media_server_status"},
				},
				"enabled": map[string]interface{}{"type": "boolean", "description": "Enable/disable FTP server (for set_ftp)"},
			}, "operation"),
		))
	}
	if ff.FritzBoxTVEnabled {
		tools = append(tools, tool("fritzbox_tv",
			"Fritz!Box DVB-C TV (cable models only): list channels with stream URLs.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_channels"},
				},
			}, "operation"),
		))
	}

	return tools
}
