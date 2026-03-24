## Tool: VirusTotal Scan

Scan a resource (URL, domain, IP address, or file hash) or a local file using the VirusTotal v3 API.
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
- `mode` (string, optional): For local files only. `auto` = hash lookup first, upload if unknown. `hash` = only calculate hashes and look them up. `upload` = force file upload.

Provide either `resource` or `file_path`.
