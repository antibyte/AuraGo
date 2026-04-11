package tools

import (
	"aurago/internal/security"
	"encoding/json"
	"fmt"
	"strings"
)

// CertificateManagerResult is the JSON response for certificate_manager operations.
type CertificateManagerResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ExecuteCertificateManager handles operations generate_self_signed, check_expiry, inspect, renew.
func ExecuteCertificateManager(operation, certName string, vault *security.Vault) string {
	encode := func(r CertificateManagerResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if certName == "" || !strings.HasPrefix(certName, "agent_cert_") {
		return encode(CertificateManagerResult{Status: "error", Message: "certName must begin with 'agent_cert_'"})
	}

	switch operation {
	case "generate_self_signed":
		// Stub implementation
		secureKey := certName + "_key"
		secureCert := certName + "_crt"
		err := vault.WriteSecret(secureKey, "stub-private-key")
		if err != nil {
			return encode(CertificateManagerResult{Status: "error", Message: err.Error()})
		}
		vault.WriteSecret(secureCert, "stub-public-cert")
		return encode(CertificateManagerResult{Status: "success", Message: "Certificate generated and securely stored via stub."})
	case "check_expiry":
		return encode(CertificateManagerResult{Status: "success", Message: "Expiry check is a stub. Valid for 365 days."})
	case "inspect":
		certData, err := vault.ReadSecret(certName + "_crt")
		if err != nil {
			return encode(CertificateManagerResult{Status: "error", Message: "Certificate not found"})
		}
		return encode(CertificateManagerResult{Status: "success", Data: certData})
	case "renew":
		return encode(CertificateManagerResult{Status: "success", Message: "Certificate renew is a stub."})
	default:
		return encode(CertificateManagerResult{Status: "error", Message: fmt.Sprintf("Unknown operation: %s", operation)})
	}
}
