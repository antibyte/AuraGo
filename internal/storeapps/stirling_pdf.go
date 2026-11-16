package storeapps

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
)

var pdfOperations = map[string]string{"ocr": "/api/v1/misc/ocr-pdf", "split": "/api/v1/general/split-pages", "merge": "/api/v1/general/merge-pdfs", "cert_sign": "/api/v1/security/cert-sign"}
var pdfPages = regexp.MustCompile(`^(all|[1-9][0-9]*(?:\s*[-,]\s*[1-9][0-9]*)*)$`)
var ocrLanguage = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_/-]{1,63}$`)

type PDFInput struct{ Data []byte }
type PDFRequest struct {
	Operation string
	Languages []string
	Pages     string
}

func NormalizeCertificateFingerprint(value string) (string, error) {
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), ":", ""))
	raw, err := hex.DecodeString(value)
	if err != nil || len(raw) != sha256.Size {
		return "", errors.New("Enter the SHA-256 fingerprint of the approved server certificate")
	}
	return value, nil
}

func (c Connection) stirlingCertificate(ctx context.Context) ([]byte, error) {
	expected, err := NormalizeCertificateFingerprint(c.CertificateFingerprint)
	if err != nil {
		return nil, err
	}
	data, err := c.exchange(ctx, "GET", "/api/v1/admin/server-certificate/certificate", nil, "", "X-API-KEY", c.Credential, false, 64<<10)
	if err != nil {
		return nil, errors.New("Cannot read Stirling's public server certificate; enable server signing and use an authorized API key")
	}
	cert, err := x509.ParseCertificate(data)
	if err != nil {
		return nil, errors.New("Stirling returned an invalid server certificate")
	}
	sum := sha256.Sum256(cert.Raw)
	if hex.EncodeToString(sum[:]) != expected {
		return nil, errors.New("Server certificate changed: signing is blocked until an administrator verifies its fingerprint")
	}
	if time.Now().Before(cert.NotBefore) || time.Now().After(cert.NotAfter) {
		return nil, errors.New("The server certificate is not currently valid")
	}
	return cert.Raw, nil
}

func (c Connection) stirlingLanguages(ctx context.Context) ([]string, error) {
	var data struct {
		Languages []string `json:"languages"`
	}
	err := c.jsonRequest(ctx, "GET", "/api/v1/ui-data/ocr-pdf", nil, false, "X-API-KEY", c.Credential, &data)
	return data.Languages, err
}

func (c Connection) StirlingProbe(ctx context.Context, process, sign bool) error {
	raw, err := c.exchange(ctx, "GET", "/v1/api-docs", nil, "", "X-API-KEY", c.Credential, false, 8<<20)
	if err != nil {
		return err
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if json.Unmarshal(raw, &spec) != nil || len(spec.Paths) == 0 {
		return errors.New("Store update required: Stirling API description is unavailable")
	}
	for op, endpoint := range pdfOperations {
		if ((op == "cert_sign" && sign) || (op != "cert_sign" && process)) && spec.Paths[endpoint]["post"] == nil {
			return fmt.Errorf("Store update required: Stirling lacks %s", op)
		}
	}
	if _, err = c.stirlingLanguages(ctx); err != nil {
		return errors.New("Cannot read installed OCR languages; check the Stirling API key and version")
	}
	if sign {
		_, err = c.stirlingCertificate(ctx)
	}
	return err
}

func (c Connection) StirlingProcess(ctx context.Context, request PDFRequest, inputs []PDFInput, maxBytes int64) ([]byte, string, error) {
	endpoint, ok := pdfOperations[request.Operation]
	if !ok || len(inputs) < 1 || len(inputs) > 20 || (request.Operation != "merge" && len(inputs) != 1) || (request.Operation == "merge" && len(inputs) < 2) {
		return nil, "", errors.New("Select one PDF, or 2-20 PDFs to merge")
	}
	var total int64
	for _, input := range inputs {
		total += int64(len(input.Data))
		if !bytes.HasPrefix(input.Data, []byte("%PDF-")) || total > maxBytes {
			return nil, "", errors.New("Input is not a PDF or exceeds the total file-size limit")
		}
	}
	fields := map[string]string{}
	var certificate []byte
	switch request.Operation {
	case "ocr":
		if len(request.Languages) < 1 || len(request.Languages) > 8 {
			return nil, "", errors.New("Select 1-8 installed OCR languages")
		}
		languages, err := c.stirlingLanguages(ctx)
		if err != nil {
			return nil, "", err
		}
		for _, language := range request.Languages {
			if !ocrLanguage.MatchString(language) || !slices.Contains(languages, language) {
				return nil, "", fmt.Errorf("OCR language package is not installed: %s", language)
			}
		}
		fields["ocrType"] = "skip-text"
		fields["ocrRenderType"] = "hocr"
		fields["sidecar"] = "false"
	case "split":
		if len(request.Pages) > 1024 || !pdfPages.MatchString(request.Pages) {
			return nil, "", errors.New("Split pages must be 'all' or positive page numbers/ranges")
		}
		fields["pageNumbers"] = request.Pages
	case "merge":
		fields["sortType"] = "orderProvided"
		fields["removeCertSign"] = "false"
	case "cert_sign":
		var err error
		certificate, err = c.stirlingCertificate(ctx)
		if err != nil {
			return nil, "", err
		}
		fields["certType"] = "SERVER"
		fields["showSignature"] = "false"
		fields["showLogo"] = "false"
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, "", err
		}
	}
	for _, language := range request.Languages {
		if request.Operation == "ocr" {
			if err := writer.WriteField("languages", language); err != nil {
				return nil, "", err
			}
		}
	}
	for i, input := range inputs {
		part, err := writer.CreateFormFile("fileInput", fmt.Sprintf("input-%02d.pdf", i+1))
		if err != nil {
			return nil, "", err
		}
		if _, err = part.Write(input.Data); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	data, err := c.exchange(ctx, "POST", endpoint, &body, writer.FormDataContentType(), "X-API-KEY", c.Credential, true, maxBytes)
	if err != nil {
		return nil, "", err
	}
	if request.Operation == "cert_sign" {
		current, e := c.stirlingCertificate(ctx)
		if e != nil || !bytes.Equal(certificate, current) {
			return nil, "", errors.New("Server certificate changed during signing; result was withheld")
		}
	}
	extension := ".pdf"
	if request.Operation == "split" && bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		if err = validatePDFArchive(data, maxBytes); err != nil {
			return nil, "", err
		}
		extension = ".zip"
	} else if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil, "", errors.New("Stirling did not return a complete PDF result")
	}
	return data, extension, nil
}

func validatePDFArchive(data []byte, maxBytes int64) error {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(z.File) == 0 || len(z.File) > 1000 {
		return errors.New("Invalid or oversized PDF split archive")
	}
	var total uint64
	seen := map[string]bool{}
	for _, f := range z.File {
		name := f.Name
		if maxBytes <= 0 || f.UncompressedSize64 > uint64(maxBytes)-total {
			return errors.New("Unsafe or oversized PDF split archive")
		}
		total += f.UncompressedSize64
		if name == "" || path.Clean(name) != name || strings.ContainsAny(name, "\\:") || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || path.Ext(name) != ".pdf" || !f.Mode().IsRegular() || seen[name] || total > uint64(maxBytes) {
			return errors.New("Unsafe or oversized PDF split archive")
		}
		seen[name] = true
	}
	return nil
}
