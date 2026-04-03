# certificate_manager — SSL/TLS Certificate Tools

Analyze, generate, and manage X.509 TLS/SSL certificates.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `info` | Parse and display certificate details (expiry, SANs) | `file_path` |
| `check_remote`| Check the certificate of a remote server (HTTPS) | `hostname` (optional: `port` defaults to 443) |
| `generate_self_signed` | Generate a self-signed cert for testing | `domain`, `output_dir` |

## Key Behaviors

- **info**: Extracts validity dates, issuer, subject, and Subject Alternative Names from PEM encoded certificates.
- **check_remote**: Connects to an external host to retrieve and validate the live certificate.
- **generate_self_signed**: Creates a basic 2048-bit RSA self-signed cert (cert.pem + key.pem).

## Examples

```
# Check local cert file
certificate_manager(operation="info", file_path="/etc/ssl/certs/server.crt")

# Check when a website's cert expires
certificate_manager(operation="check_remote", hostname="google.com")

# Generate a cert for local dev
certificate_manager(operation="generate_self_signed", domain="localhost", output_dir="./certs")
```

## Tips
- Very useful for diagnosing "certificate expired" errors or mapping internal lab certificates.