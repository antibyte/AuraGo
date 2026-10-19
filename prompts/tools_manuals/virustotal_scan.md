## Tool: VirusTotal Scan

Look up a resource (URL, domain, IP address, or file hash) or a local file using the VirusTotal v3 API.
Requires a configured VirusTotal API Key in the settings.

### WARNING
DO NOT use this tool on files containing personal data or sensitive information.
Files may be made available to security researchers and your submissions may be made public.

### Usage

```json
{"action": "virustotal_scan", "resource": "example.com"}
```

```json
{"action": "virustotal_scan", "file_path": "/path/to/file.exe", "mode": "auto"}
```

```json
{"action": "virustotal_scan", "file_path": "/path/to/file.exe", "mode": "hash"}
```

```json
{"action": "virustotal_scan", "file_path": "/path/to/file.exe", "mode": "upload"}
```

### Parameters
- `resource` (string, optional): The URL, domain, IP address, or file hash to scan.
- `file_path` (string, optional): Local file path to hash or upload to VirusTotal.
- `mode` (string, optional): For local files only. Omitted, `hash`, and legacy `auto` calculate hashes and look up existing reports without uploading file content. `upload` explicitly permits an upload after the canonical upload permission check.

Provide either `resource` or `file_path`.

### Behavior
- `resource` uses VirusTotal object/report endpoints:
  - file hash -> file report
  - domain -> domain report
  - IP -> IP report
  - URL -> existing URL report; an unknown URL is submitted only with explicit `mode: upload` and the upload grant
- `file_path` must resolve to a regular file inside the authorized workspace. The same rooted open file is hashed and, for an explicit upload, submitted. An unknown hash never causes an automatic upload.
