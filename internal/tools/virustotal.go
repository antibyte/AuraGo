package tools

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var virustotalHTTPClient = &http.Client{Timeout: 30 * time.Second}
var virustotalBaseURL = "https://www.virustotal.com/api/v3"
var virustotalHashPattern = regexp.MustCompile(`(?i)^[a-f0-9]{32}$|^[a-f0-9]{40}$|^[a-f0-9]{64}$`)
var virustotalDomainPattern = regexp.MustCompile(`(?i)^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

type VirusTotalOptions struct {
	Resource string
	FilePath string
	Mode     string
}

type virustotalHashes struct {
	MD5    string `json:"md5"`
	SHA1   string `json:"sha1"`
	SHA256 string `json:"sha256"`
}

type virustotalAPIError struct {
	StatusCode int
	Body       string
}

func (e *virustotalAPIError) Error() string {
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("VirusTotal HTTP Error %d", e.StatusCode)
	}
	return fmt.Sprintf("VirusTotal HTTP Error %d: %s", e.StatusCode, e.Body)
}

func (e *virustotalAPIError) NotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// ExecuteVirusTotalScan performs a VirusTotal lookup for a URL/domain/IP/hash resource.
func ExecuteVirusTotalScan(apiKey string, resource string) string {
	return ExecuteVirusTotalScanWithOptions(apiKey, VirusTotalOptions{Resource: resource})
}

// ExecuteVirusTotalScanWithOptions supports resource lookups, local file hashing,
// and optional file uploads to VirusTotal.
func ExecuteVirusTotalScanWithOptions(apiKey string, opts VirusTotalOptions) string {
	if apiKey == "" {
		return formatError("VirusTotal API Key is missing. Please configure it in settings.")
	}

	resource := strings.TrimSpace(opts.Resource)
	filePath := strings.TrimSpace(opts.FilePath)
	mode := normalizeVirusTotalMode(opts.Mode)

	if resource == "" && filePath == "" {
		return formatError("Either resource or file_path is required")
	}
	if resource != "" && filePath != "" {
		return formatError("Provide either resource or file_path, not both")
	}

	if filePath != "" {
		out, err := executeVirusTotalFileFlow(apiKey, filePath, mode)
		if err != nil {
			return formatError(err.Error())
		}
		return out
	}

	out, err := virustotalLookupResource(apiKey, resource)
	if err != nil {
		return formatError(err.Error())
	}
	return out
}

func normalizeVirusTotalMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "auto":
		return "auto"
	case "hash":
		return "hash"
	case "upload":
		return "upload"
	default:
		return "auto"
	}
}

func executeVirusTotalFileFlow(apiKey, filePath, mode string) (string, error) {
	hashes, size, err := computeVirusTotalFileHashes(filePath)
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"status": "success",
		"input": map[string]interface{}{
			"file_path": filepath.Clean(filePath),
			"mode":      mode,
			"size":      size,
		},
		"hashes": hashes,
	}

	lookupJSON, lookupPayload, err := virustotalLookupFileHash(apiKey, hashes.SHA256)
	if err == nil {
		result["lookup"] = lookupPayload
		result["used"] = "hash_lookup"
		return marshalVirusTotalResult(result), nil
	}

	var apiErr *virustotalAPIError
	if !strings.EqualFold(mode, "upload") && (!isVirusTotalNotFound(err, &apiErr) || mode == "hash") {
		if isVirusTotalNotFound(err, &apiErr) {
			result["lookup"] = map[string]interface{}{
				"status":  "not_found",
				"message": "No existing VirusTotal record was found for the file hash.",
			}
			result["used"] = "hash_lookup"
			return marshalVirusTotalResult(result), nil
		}
		return "", fmt.Errorf("VirusTotal hash lookup failed: %w", err)
	}

	result["lookup"] = map[string]interface{}{
		"status":  "not_found",
		"message": "No existing VirusTotal record was found for the file hash. Uploading file instead.",
	}

	uploadPayload, err := virustotalUploadFile(apiKey, filePath, size)
	if err != nil {
		return "", fmt.Errorf("VirusTotal file upload failed: %w", err)
	}

	result["upload"] = uploadPayload
	result["used"] = "file_upload"
	result["upload_hint"] = "VirusTotal may still be analyzing the file. Re-run a hash lookup with the SHA-256 if you need final verdict details."

	// If upload response already includes a detailed data object, expose it directly.
	if lookupJSON != "" {
		result["lookup_raw"] = lookupJSON
	}
	return marshalVirusTotalResult(result), nil
}

func isVirusTotalNotFound(err error, target **virustotalAPIError) bool {
	var apiErr *virustotalAPIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if target != nil {
		*target = apiErr
	}
	return apiErr.NotFound()
}

func virustotalLookupResource(apiKey, resource string) (string, error) {
	resource = strings.TrimSpace(resource)
	resourceType := classifyVirusTotalResource(resource)
	if resourceType == "" {
		return "", fmt.Errorf("unsupported VirusTotal resource type: provide a URL, domain, IP address, or file hash")
	}

	result := map[string]interface{}{
		"status": "success",
		"input": map[string]interface{}{
			"resource":      resource,
			"resource_type": resourceType,
		},
	}

	switch resourceType {
	case "file_hash":
		payload, err := virustotalLookupJSON(apiKey, fmt.Sprintf("%s/files/%s", virustotalBaseURL, url.PathEscape(strings.ToLower(resource))))
		if err != nil {
			return "", fmt.Errorf("VirusTotal file lookup failed: %w", err)
		}
		result["used"] = "file_report"
		result["result"] = payload
	case "domain":
		payload, err := virustotalLookupJSON(apiKey, fmt.Sprintf("%s/domains/%s", virustotalBaseURL, url.PathEscape(strings.ToLower(resource))))
		if err != nil {
			return "", fmt.Errorf("VirusTotal domain lookup failed: %w", err)
		}
		result["used"] = "domain_report"
		result["result"] = payload
	case "ip_address":
		payload, err := virustotalLookupJSON(apiKey, fmt.Sprintf("%s/ip_addresses/%s", virustotalBaseURL, url.PathEscape(resource)))
		if err != nil {
			return "", fmt.Errorf("VirusTotal IP lookup failed: %w", err)
		}
		result["used"] = "ip_report"
		result["result"] = payload
	case "url":
		urlID := base64.RawURLEncoding.EncodeToString([]byte(resource))
		payload, err := virustotalLookupJSON(apiKey, fmt.Sprintf("%s/urls/%s", virustotalBaseURL, url.PathEscape(urlID)))
		if err == nil {
			result["used"] = "url_report"
			result["result"] = payload
			return marshalVirusTotalResult(result), nil
		}

		var apiErr *virustotalAPIError
		if !isVirusTotalNotFound(err, &apiErr) {
			return "", fmt.Errorf("VirusTotal URL lookup failed: %w", err)
		}

		analysis, analysisErr := virustotalSubmitURL(apiKey, resource)
		if analysisErr != nil {
			return "", fmt.Errorf("VirusTotal URL submission failed: %w", analysisErr)
		}
		result["used"] = "url_submission"
		result["submission"] = analysis

		if analysisID := virustotalAnalysisID(analysis); analysisID != "" {
			if analysisPayload, reportPayload, pollErr := virustotalPollAnalysisAndFetchURL(apiKey, analysisID, urlID); pollErr == nil {
				result["analysis"] = analysisPayload
				if reportPayload != nil {
					result["result"] = reportPayload
					result["used"] = "url_report_after_submission"
				}
			}
		}
	default:
		return "", fmt.Errorf("unsupported VirusTotal resource type: %s", resourceType)
	}

	return marshalVirusTotalResult(result), nil
}

func classifyVirusTotalResource(resource string) string {
	resource = strings.TrimSpace(resource)
	if resource == "" {
		return ""
	}
	if virustotalHashPattern.MatchString(resource) {
		return "file_hash"
	}
	if parsed, err := url.Parse(resource); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return "url"
	}
	if net.ParseIP(resource) != nil {
		return "ip_address"
	}
	if virustotalDomainPattern.MatchString(strings.ToLower(resource)) {
		return "domain"
	}
	return ""
}

func virustotalLookupJSON(apiKey, endpoint string) (map[string]interface{}, error) {
	bodyBytes, err := virustotalDoRequest(apiKey, http.MethodGet, endpoint, nil, "application/json")
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse VirusTotal response JSON")
	}
	return result, nil
}

func virustotalSubmitURL(apiKey, resource string) (map[string]interface{}, error) {
	form := url.Values{}
	form.Set("url", resource)

	bodyBytes, err := virustotalDoRequest(apiKey, http.MethodPost, fmt.Sprintf("%s/urls", virustotalBaseURL), strings.NewReader(form.Encode()), "application/x-www-form-urlencoded")
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse VirusTotal URL submission JSON")
	}
	return result, nil
}

func virustotalAnalysisID(payload map[string]interface{}) string {
	data, _ := payload["data"].(map[string]interface{})
	id, _ := data["id"].(string)
	return id
}

func virustotalPollAnalysisAndFetchURL(apiKey, analysisID, urlID string) (map[string]interface{}, map[string]interface{}, error) {
	for attempt := 0; attempt < 3; attempt++ {
		analysis, err := virustotalLookupJSON(apiKey, fmt.Sprintf("%s/analyses/%s", virustotalBaseURL, url.PathEscape(analysisID)))
		if err != nil {
			return nil, nil, err
		}
		if virustotalAnalysisCompleted(analysis) {
			report, err := virustotalLookupJSON(apiKey, fmt.Sprintf("%s/urls/%s", virustotalBaseURL, url.PathEscape(urlID)))
			if err != nil {
				return analysis, nil, err
			}
			return analysis, report, nil
		}
		time.Sleep(2 * time.Second)
	}
	analysis, err := virustotalLookupJSON(apiKey, fmt.Sprintf("%s/analyses/%s", virustotalBaseURL, url.PathEscape(analysisID)))
	if err != nil {
		return nil, nil, err
	}
	return analysis, nil, nil
}

func virustotalAnalysisCompleted(payload map[string]interface{}) bool {
	data, _ := payload["data"].(map[string]interface{})
	attrs, _ := data["attributes"].(map[string]interface{})
	status, _ := attrs["status"].(string)
	return strings.EqualFold(status, "completed")
}

func virustotalLookupFileHash(apiKey, sha256Hash string) (string, map[string]interface{}, error) {
	endpoint := fmt.Sprintf("%s/files/%s", virustotalBaseURL, url.PathEscape(sha256Hash))
	bodyBytes, err := virustotalDoRequest(apiKey, http.MethodGet, endpoint, nil, "application/json")
	if err != nil {
		return "", nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", nil, fmt.Errorf("failed to parse VirusTotal file lookup JSON")
	}
	return string(bodyBytes), result, nil
}

func virustotalUploadFile(apiKey, filePath string, size int64) (map[string]interface{}, error) {
	uploadURL := fmt.Sprintf("%s/files", virustotalBaseURL)
	if size > 32*1024*1024 {
		customURL, err := virustotalGetLargeUploadURL(apiKey)
		if err != nil {
			return nil, err
		}
		uploadURL = customURL
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for upload: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create multipart form: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to attach file to upload: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize upload body: %w", err)
	}

	bodyBytes, err := virustotalDoRequest(apiKey, http.MethodPost, uploadURL, &body, writer.FormDataContentType())
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse VirusTotal upload response JSON")
	}
	return result, nil
}

func virustotalGetLargeUploadURL(apiKey string) (string, error) {
	endpoint := fmt.Sprintf("%s/files/upload_url", virustotalBaseURL)
	bodyBytes, err := virustotalDoRequest(apiKey, http.MethodGet, endpoint, nil, "application/json")
	if err != nil {
		return "", err
	}

	var payload struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return "", fmt.Errorf("failed to parse VirusTotal large upload URL response")
	}
	if strings.TrimSpace(payload.Data) == "" {
		return "", fmt.Errorf("VirusTotal did not return an upload URL for large file upload")
	}
	return payload.Data, nil
}

func virustotalDoRequest(apiKey, method, endpoint string, body io.Reader, contentType string) ([]byte, error) {
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create VirusTotal request: %w", err)
	}
	req.Header.Set("x-apikey", apiKey)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := virustotalHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VirusTotal request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, readErr := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read VirusTotal response: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &virustotalAPIError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(bodyBytes)),
		}
	}

	return bodyBytes, nil
}

func computeVirusTotalFileHashes(filePath string) (virustotalHashes, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return virustotalHashes{}, 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return virustotalHashes{}, 0, fmt.Errorf("failed to stat file: %w", err)
	}
	if info.IsDir() {
		return virustotalHashes{}, 0, fmt.Errorf("file_path must point to a file, not a directory")
	}

	md5Hash := md5.New()
	sha1Hash := sha1.New()
	sha256Hash := sha256.New()

	writer := io.MultiWriter(md5Hash, sha1Hash, sha256Hash)
	if _, err := io.Copy(writer, file); err != nil {
		return virustotalHashes{}, 0, fmt.Errorf("failed to read file for hashing: %w", err)
	}

	return virustotalHashes{
		MD5:    hex.EncodeToString(md5Hash.Sum(nil)),
		SHA1:   hex.EncodeToString(sha1Hash.Sum(nil)),
		SHA256: hex.EncodeToString(sha256Hash.Sum(nil)),
	}, info.Size(), nil
}

func marshalVirusTotalResult(result map[string]interface{}) string {
	b, _ := json.MarshalIndent(result, "", "  ")
	return string(b)
}
