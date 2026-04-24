package agent

import openai "github.com/sashabaranov/go-openai"

func buildCoreToolSchemas(ff ToolFeatureFlags, execSkillProps map[string]interface{}) []openai.Tool {
	return []openai.Tool{
		tool("list_skills",
			"List available pre-built skills and integrations that can be executed via execute_skill. Use this to discover capabilities like virustotal_scan, brave_search, pdf_extractor, wikipedia_search, or web_scraper.",
			schema(map[string]interface{}{}),
		),
		tool("execute_skill",
			"Run a pre-built registered skill (e.g. web_search, ddg_search, pdf_extractor, wikipedia_search, virustotal_scan). Use for external data retrieval.",
			schema(execSkillProps, "skill"),
		),
		tool("wikipedia_search",
			"Search Wikipedia and return the best matching article summary. "+
				"Use this for encyclopedic facts, biographies, places, historical topics, and definitions. "+
				"When Wikipedia summary mode is enabled, include search_query to request a focused summary.",
			schema(map[string]interface{}{
				"query":        prop("string", "The Wikipedia search term or page title to look up"),
				"language":     prop("string", "Optional Wikipedia language code such as de, en, fr, or ja"),
				"search_query": prop("string", "Optional focused question for summary mode, e.g. 'main subfields and recent breakthroughs'"),
			}, "query"),
		),
		tool("ddg_search",
			"Search the web with DuckDuckGo and return the top results. "+
				"When DDG summary mode is enabled, include search_query to request a focused synthesis of the results.",
			schema(map[string]interface{}{
				"query":        prop("string", "Search query to submit to DuckDuckGo"),
				"max_results":  map[string]interface{}{"type": "integer", "description": "Maximum number of results to return (default: 5)"},
				"search_query": prop("string", "Optional focused question for summary mode, e.g. 'most significant AI developments this week'"),
			}, "query"),
		),
		func() openai.Tool {
			if ff.AllowFilesystemWrite {
				return tool("filesystem",
					"Read, write, move, copy, delete files and directories, or list directory contents.",
					schema(map[string]interface{}{
						"operation": map[string]interface{}{
							"type":        "string",
							"description": "Operation to perform",
							"enum":        []string{"read_file", "write_file", "delete", "copy", "move", "list_dir", "create_dir", "stat", "copy_batch", "move_batch", "delete_batch", "create_dir_batch"},
						},
						"file_path":   prop("string", "Path to the file or directory"),
						"content":     prop("string", "Content to write (for write_file operations)"),
						"destination": prop("string", "Destination path (for copy/move operations)"),
						"items": map[string]interface{}{
							"type":        "array",
							"description": "Batch items for copy_batch, move_batch, delete_batch, or create_dir_batch. Each item needs file_path and optionally destination.",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"file_path":   prop("string", "Path to the file or directory"),
									"destination": prop("string", "Destination path for copy_batch or move_batch"),
								},
							},
						},
						"preview": prop("boolean", "If true, only return first 100 lines (for read_file)"),
						"limit":   prop("integer", "Maximum number of entries to return for list_dir (default: 500, max: 1000)"),
						"offset":  prop("integer", "Number of entries to skip for list_dir pagination (default: 0)"),
					}, "operation"),
				)
			}
			return tool("filesystem",
				"Read files and list directory contents (read-only — filesystem writes are disabled).",
				schema(map[string]interface{}{
					"operation": map[string]interface{}{
						"type":        "string",
						"description": "Read-only operation to perform",
						"enum":        []string{"read_file", "list_dir", "stat"},
					},
					"file_path": prop("string", "Path to the file or directory"),
					"preview":   prop("boolean", "If true, only return first 100 lines (for read_file)"),
					"limit":     prop("integer", "Maximum number of entries to return for list_dir (default: 500, max: 1000)"),
					"offset":    prop("integer", "Number of entries to skip for list_dir pagination (default: 0)"),
				}, "operation", "file_path"),
			)
		}(),
		tool("text_diff",
			"Compare two files or strings and return a unified diff.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"diff_files", "diff_strings"},
				},
				"file1": prop("string", "Path to the first file (for diff_files)"),
				"file2": prop("string", "Path to the second file (for diff_files)"),
				"text1": prop("string", "First text string (for diff_strings)"),
				"text2": prop("string", "Second text string (for diff_strings)"),
			}, "operation"),
		),
		tool("file_search",
			"Search for text patterns across files or find files by name. Supports regex patterns, glob filters, and recursive search.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Search operation to perform",
					"enum":        []string{"grep", "grep_recursive", "find"},
				},
				"pattern":     prop("string", "Search pattern (regex). Required for grep/grep_recursive."),
				"file_path":   prop("string", "File to search in (for grep)"),
				"glob":        prop("string", "File name glob pattern, e.g. '*.yaml', '*.go' (for grep_recursive and find)"),
				"output_mode": map[string]interface{}{"type": "string", "description": "Output format: 'content' (default, shows matching lines) or 'count' (just counts)", "enum": []string{"content", "count"}},
			}, "operation", "pattern"),
		),
		tool("file_reader_advanced",
			"Advanced file reading with line ranges, head/tail, line counting, and contextual search. Ideal for large files and log analysis.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Read operation to perform",
					"enum":        []string{"read_lines", "head", "tail", "count_lines", "search_context"},
				},
				"file_path":  prop("string", "Path to the file to read"),
				"start_line": prop("integer", "First line to read, 1-based (for read_lines)"),
				"end_line":   prop("integer", "Last line to read, 1-based inclusive (for read_lines)"),
				"line_count": prop("integer", "Number of lines for head/tail (default: 20) or context lines for search_context (default: 3)"),
				"pattern":    prop("string", "Search pattern (regex) for search_context"),
			}, "operation", "file_path"),
		),
		tool("smart_file_read",
			"Intelligently inspect large files without dumping them into the prompt. Analyze file metadata, take strategic samples, detect structure, or generate a focused summary.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Smart file read operation to perform",
					"enum":        []string{"analyze", "sample", "structure", "summarize"},
				},
				"file_path":  prop("string", "Path to the file to inspect"),
				"query":      prop("string", "Optional focus question for summarize, e.g. 'Find the root cause of the error spikes'."),
				"line_count": prop("integer", "Number of lines per sample section (default: 20; used by sample)."),
				"sampling_strategy": map[string]interface{}{
					"type":        "string",
					"description": "Sampling strategy for sample/summarize: head, tail, distributed, semantic (semantic currently falls back to distributed).",
					"enum":        []string{"head", "tail", "distributed", "semantic"},
				},
				"max_tokens": prop("integer", "Approximate token budget for sample/summarize output (default: 2500)."),
			}, "operation", "file_path"),
		),
		tool("system_metrics",
			"Retrieve current system resource usage: CPU, memory, disk, running processes, host info, temperatures, per-interface network stats, active connections, or per-disk I/O counters.",
			schema(map[string]interface{}{
				"target": map[string]interface{}{
					"type":        "string",
					"description": "Metrics category to retrieve",
					"enum":        []string{"all", "cpu", "memory", "disk", "processes", "host", "sensors", "network_detail", "connections", "disk_io"},
				},
			}),
		),
		tool("process_analyzer",
			"Analyze running OS processes. Find top CPU/memory consumers, search processes by name, "+
				"inspect a single process in detail, or view process trees (parent/child relationships). "+
				"Unlike process_management (which manages AuraGo background tasks), this tool queries ALL system processes.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Analysis operation to perform",
					"enum":        []string{"top_cpu", "top_memory", "find", "tree", "info"},
				},
				"name":  prop("string", "Process name to search for (required for find)"),
				"pid":   prop("integer", "Process ID (required for tree and info)"),
				"limit": map[string]interface{}{"type": "integer", "description": "Max results to return (1-100, default: 10)"},
			}, "operation"),
		),
		tool("process_management",
			"List, kill, or inspect running background processes managed by AuraGo.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list", "kill", "status"},
				},
				"pid":   prop("integer", "Process ID (for kill/status operations)"),
				"label": prop("string", "Process label (alternative to pid)"),
			}, "operation"),
		),
		tool("follow_up",
			"Schedule an autonomous background task for yourself to execute immediately after this response. "+
				"ONLY use this when you have all required information and will perform the work yourself. "+
				"⚠️ NEVER use follow_up to ask the user for input or relay a question — that creates an infinite loop. "+
				"If you are missing information needed to complete a task, respond DIRECTLY to the user with your question instead of using this tool.",
			schema(map[string]interface{}{
				"task_prompt":          prop("string", "Concrete, self-contained task the agent will perform autonomously. Must NOT be a question directed at the user."),
				"delay_seconds":        prop("integer", "Optional delay before the background task starts. Defaults to the configured follow-up delay."),
				"timeout_secs":         prop("integer", "Optional execution timeout for the background loopback request."),
				"notify_on_completion": prop("boolean", "If true, store a system notification when the task completes or fails."),
			}, "task_prompt"),
		),
		tool("wait_for_event",
			"Wait asynchronously for a concrete event, then continue autonomously in the background. "+
				"Use this for safe polling of AuraGo-managed processes, HTTP endpoints, or workspace files without blocking the current response.",
			schema(map[string]interface{}{
				"event_type":           map[string]interface{}{"type": "string", "enum": []string{"process_exited", "http_available", "file_changed"}, "description": "Which event to wait for."},
				"task_prompt":          prop("string", "Task to continue with once the event has completed."),
				"pid":                  prop("integer", "AuraGo background process ID for process_exited."),
				"url":                  prop("string", "HTTP URL to probe for http_available."),
				"host":                 prop("string", "Host to combine with port for http_available when url is omitted."),
				"port":                 prop("integer", "Optional port for host-based http_available checks."),
				"file_path":            prop("string", "Workspace file path to watch for file_changed."),
				"timeout_secs":         prop("integer", "Maximum time to wait before the task fails."),
				"interval_seconds":     prop("integer", "Polling interval in seconds."),
				"notify_on_completion": prop("boolean", "If true, store a system notification when the task completes or fails."),
			}, "event_type", "task_prompt"),
		),
		tool("analyze_image",
			"Analyze an image file using the Vision LLM. Describe content, read text (OCR), identify objects.",
			schema(map[string]interface{}{
				"file_path": prop("string", "Path to the image file (JPEG, PNG, GIF, WebP)"),
				"prompt":    prop("string", "Custom analysis prompt (default: general description)"),
			}, "file_path"),
		),
		tool("transcribe_audio",
			"Transcribe an audio file to text using the configured Speech-to-Text service.",
			schema(map[string]interface{}{
				"file_path": prop("string", "Path to the audio file (MP3, WAV, OGG, FLAC, M4A)"),
			}, "file_path"),
		),
		tool("send_image",
			"Send an image to the user. Shown inline with a click-to-zoom lightbox in the Web UI, as a native photo in Telegram, and as a file attachment in Discord. Provide a local workspace path or an image URL.",
			schema(map[string]interface{}{
				"path":    prop("string", "Local file path within the workspace (e.g. 'images/chart.png') or a full HTTPS URL to an image"),
				"caption": prop("string", "Optional caption or description shown with the image"),
			}, "path"),
		),
		tool("send_audio",
			"Send an audio file to the user. Shown with an inline audio player in the Web UI (play/pause, progress bar, speed control). Provide a local workspace path or a direct HTTPS URL to an audio file.",
			schema(map[string]interface{}{
				"path":  prop("string", "Local file path within the workspace (e.g. 'output.mp3') or a full HTTPS URL to an audio file (MP3, WAV, OGG, FLAC, M4A)"),
				"title": prop("string", "Optional title shown above the audio player"),
			}, "path"),
		),
		tool("send_video",
			"Send a video file to the user. Shown with an inline video player in the Web UI. Provide a local workspace path or a direct HTTPS URL to a browser-playable video file.",
			schema(map[string]interface{}{
				"path":  prop("string", "Local file path within the workspace (e.g. 'clips/demo.mp4') or a full HTTPS URL to a video file (MP4, WebM, MOV, OGV)"),
				"title": prop("string", "Optional title shown above the video player"),
			}, "path"),
		),
		tool("send_document",
			"Send a document to the user. Shown with Open and Download buttons in the Web UI. PDF files can be viewed inline in the browser. Provide a local workspace path or a direct HTTPS URL.",
			schema(map[string]interface{}{
				"path":  prop("string", "Local file path within the workspace or a full HTTPS URL to a document (PDF, DOCX, XLSX, PPTX, TXT, MD, CSV)"),
				"title": prop("string", "Optional title shown with the document card"),
			}, "path"),
		),
		tool("discover_tools",
			"Browse and search ALL available tools, including those hidden by adaptive filtering. "+
				"Use this when you need a tool that is not in your current tool list. "+
				"Operations: list_categories (browse by category), search (find tools by keyword), get_tool_info (get full parameter schema + guide for a specific tool so you can call it).",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list_categories", "search", "get_tool_info"},
				},
				"category":  prop("string", "Category to filter (for list_categories): system, files, network, media, smart_home, infrastructure, communication"),
				"query":     prop("string", "Search keyword (for search operation)"),
				"tool_name": prop("string", "Tool name to get full info for (for get_tool_info)"),
			}, "operation"),
		),
	}
}
