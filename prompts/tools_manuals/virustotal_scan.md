## Tool: VirusTotal Scan

Scan a resource (URL, domain, IP address, or file hash) using the VirusTotal v3 API. 
Requires a configured VirusTotal API Key in the settings.

### WARNING
DO NOT use this tool on files containing personal data or sensitive information.
Files may be made available to security researchers and your submissions may be made public.

### Usage

```json
{"action": "virustotal_scan", "resource": "example.com"}
```

### Parameters
- `resource` (string, required): The URL, domain, IP address, or file hash to scan.
