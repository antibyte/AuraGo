package tools

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CertificateManagerResult is the JSON response for certificate_manager operations.
type CertificateManagerResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ExecuteCertificateManager handles certificate inspection, remote checks, and local self-signed generation.
func ExecuteCertificateManager(operation, filePath, hostname string, port int, domain, outputDir string, days int, workspaceDir string) string {
	encode := func(r CertificateManagerResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	switch strings.TrimSpace(strings.ToLower(operation)) {
	case "info":
		return certificateInfo(filePath, workspaceDir, encode)
	case "check_remote":
		return certificateCheckRemote(hostname, port, encode)
	case "generate_self_signed":
		return certificateGenerateSelfSigned(domain, outputDir, days, workspaceDir, encode)
	default:
		return encode(CertificateManagerResult{Status: "error", Message: fmt.Sprintf("Unknown operation: %s", operation)})
	}
}

func certificateInfo(filePath, workspaceDir string, encode func(CertificateManagerResult) string) string {
	if strings.TrimSpace(filePath) == "" {
		return encode(CertificateManagerResult{Status: "error", Message: "'file_path' is required for info"})
	}
	resolved, err := secureResolve(workspaceDir, filePath)
	if err != nil {
		return encode(CertificateManagerResult{Status: "error", Message: err.Error()})
	}
	cert, err := readPEMCertificate(resolved)
	if err != nil {
		return encode(CertificateManagerResult{Status: "error", Message: err.Error()})
	}
	return encode(CertificateManagerResult{Status: "success", Data: certificateData(cert)})
}

func certificateCheckRemote(hostname string, port int, encode func(CertificateManagerResult) string) string {
	host := strings.TrimSpace(hostname)
	if host == "" {
		return encode(CertificateManagerResult{Status: "error", Message: "'hostname' is required for check_remote"})
	}
	if port <= 0 {
		port = 443
	}

	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 5 * time.Second},
		Config: &tls.Config{
			ServerName:         tlsServerName(host),
			InsecureSkipVerify: true, // We inspect the peer certificate, including private/LAN hosts.
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return encode(CertificateManagerResult{Status: "error", Message: fmt.Sprintf("Failed to connect: %v", err)})
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return encode(CertificateManagerResult{Status: "error", Message: "TLS connection was not established"})
	}
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return encode(CertificateManagerResult{Status: "error", Message: "Remote server did not present a certificate"})
	}

	data := certificateData(state.PeerCertificates[0])
	data["hostname"] = host
	data["port"] = port
	return encode(CertificateManagerResult{Status: "success", Data: data})
}

func certificateGenerateSelfSigned(domain, outputDir string, days int, workspaceDir string, encode func(CertificateManagerResult) string) string {
	if err := requireFilesystemWritePermission(); err != nil {
		return encode(CertificateManagerResult{Status: "error", Message: err.Error()})
	}

	name := strings.TrimSpace(domain)
	if name == "" {
		return encode(CertificateManagerResult{Status: "error", Message: "'domain' is required for generate_self_signed"})
	}
	if strings.TrimSpace(outputDir) == "" {
		return encode(CertificateManagerResult{Status: "error", Message: "'output_dir' is required for generate_self_signed"})
	}
	if days <= 0 {
		days = 365
	}

	resolvedDir, err := secureResolve(workspaceDir, outputDir)
	if err != nil {
		return encode(CertificateManagerResult{Status: "error", Message: err.Error()})
	}
	if err := os.MkdirAll(resolvedDir, 0755); err != nil {
		return encode(CertificateManagerResult{Status: "error", Message: fmt.Sprintf("Failed to create output directory: %v", err)})
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return encode(CertificateManagerResult{Status: "error", Message: fmt.Sprintf("Failed to generate key: %v", err)})
	}
	certDER, err := createSelfSignedCertificate(name, days, key)
	if err != nil {
		return encode(CertificateManagerResult{Status: "error", Message: err.Error()})
	}

	certPath := filepath.Join(resolvedDir, "cert.pem")
	keyPath := filepath.Join(resolvedDir, "key.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := writeFileAtomic(certPath, certPEM); err != nil {
		return encode(CertificateManagerResult{Status: "error", Message: fmt.Sprintf("Failed to write certificate: %v", err)})
	}
	if err := writeFileAtomic(keyPath, keyPEM); err != nil {
		return encode(CertificateManagerResult{Status: "error", Message: fmt.Sprintf("Failed to write private key: %v", err)})
	}
	if err := os.Chmod(keyPath, 0600); err != nil {
		return encode(CertificateManagerResult{Status: "error", Message: fmt.Sprintf("Failed to protect private key: %v", err)})
	}

	return encode(CertificateManagerResult{
		Status:  "success",
		Message: "Self-signed certificate generated",
		Data: map[string]interface{}{
			"cert_path": certPath,
			"key_path":  keyPath,
			"domain":    name,
			"days":      days,
		},
	})
}

func readPEMCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("file does not contain a PEM certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}
	return cert, nil
}

func createSelfSignedCertificate(name string, days int, key *rsa.PrivateKey) ([]byte, error) {
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial: %w", err)
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     now.AddDate(0, 0, days),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(name); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{name}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}
	return certDER, nil
}

func certificateData(cert *x509.Certificate) map[string]interface{} {
	ips := make([]string, 0, len(cert.IPAddresses))
	for _, ip := range cert.IPAddresses {
		ips = append(ips, ip.String())
	}
	return map[string]interface{}{
		"subject":       cert.Subject.String(),
		"issuer":        cert.Issuer.String(),
		"common_name":   cert.Subject.CommonName,
		"serial_number": cert.SerialNumber.String(),
		"not_before":    cert.NotBefore.Format(time.RFC3339),
		"not_after":     cert.NotAfter.Format(time.RFC3339),
		"dns_names":     cert.DNSNames,
		"ip_addresses":  ips,
		"is_ca":         cert.IsCA,
	}
}

func tlsServerName(host string) string {
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return ""
	}
	return strings.Trim(host, "[]")
}
