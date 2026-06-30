# Security Policy

We take the security and privacy of AuraGo seriously. As an agentic framework running directly inside homelabs with access to local services, containers, and secrets, ensuring a highly secure architecture is our primary concern. 

This document outlines how to report vulnerabilities and lists our supported security updates.

---

## 🛡️ Supported Versions

We currently provide security updates to the following versions of AuraGo:

| Version | Supported | Updates |
| --- | :---: | --- |
| **v2.x** (Current) |  | Mainline security fixes, critical dependency updates |
| **v1.x** | ❌ | End of Life (EOL). Please upgrade to v2.x |

---

## 📞 Reporting a Vulnerability

**Please DO NOT open a public GitHub issue to report a security vulnerability.** Public issues expose systems to exploits before a patch can be deployed.

To report a security vulnerability, please follow these steps:

1.  **Contact Us Privately**: Email your findings directly to **security@aurago.dev**.
2.  **Provide Details**: In your report, please include:
    *   A description of the vulnerability and its potential impact.
    *   Detailed, step-by-step instructions to reproduce the issue (proof-of-concept scripts, configurations, or screenshots).
    *   The version of AuraGo where you discovered the vulnerability.
    *   Any proposed suggestions for mitigating or fixing the vulnerability.
3.  **PGP Encryption (Optional)**: If you'd like to encrypt your report, please request our security team's PGP public key in your initial email.

---

## 🚀 Our Response Process

*   **Acknowledgement**: You will receive an automated response within **24 hours**, and a detailed human acknowledgement of your report within **72 hours**.
*   **Assessment**: Our security team will investigate the vulnerability to confirm the impact and coordinate with you on possible fixes.
*   **Patching**: Once a fix is verified, we will roll out a security patch in our next minor release (or immediate hotfix for high/critical issues).
*   **Disclosure**: We follow standard responsible disclosure. We ask that you do not publish details of the vulnerability until a patch has been released, allowing us to safeguard other self-hosters in the community.
*   **Credit**: Unless requested otherwise, we are happy to credit you in our release notes for finding the issue and helping secure the AuraGo community!
