# certificate_manager — SSL/TLS Certificate Tools

Analyze, generate, and manage X.509 TLS/SSL certificates.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `info` | Parse and display certificate details (expiry, SANs) from a workspace-resolved PEM file | `file_path` |
| `check_remote` | Check the TLS peer certificate of a remote HTTPS/TLS service | `hostname` (optional: `port` defaults to 443) |
| `generate_self_signed` | Generate a self-signed cert for testing under the workspace | `domain`, `output_dir` (optional: `days`, default 365) |

## Key Behaviors

- **info**: Extracts validity dates, issuer, subject, serial number, DNS names, and IP SANs from PEM encoded certificates.
- **check_remote**: Opens a short-timeout TLS connection and inspects the peer certificate. It does not follow HTTP redirects and requires `agent.allow_network_requests`.
- **generate_self_signed**: Creates a 2048-bit RSA self-signed cert (`cert.pem` + `key.pem`) in `output_dir`. The private key is written with mode `0600` and the operation requires filesystem-write permission.

## Examples

```
# Check local cert file
certificate_manager(operation="info", file_path="certs/server.crt")

# Check when a website's cert expires
certificate_manager(operation="check_remote", hostname="google.com")

# Check a home-lab service on a custom port
certificate_manager(operation="check_remote", hostname="nas.local", port=8443)

# Generate a cert for local dev
certificate_manager(operation="generate_self_signed", domain="localhost", output_dir="certs", days=30)
```

## Tips
- Very useful for diagnosing "certificate expired" errors or mapping internal lab certificates.
- Use workspace-relative paths for `file_path` and `output_dir`; absolute paths outside the workspace are rejected.
