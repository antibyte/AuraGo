package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/security"
)

// PaperlessConfig holds connection parameters for a Paperless-ngx instance.
type PaperlessConfig struct {
	URL      string // Base URL, e.g. https://paperless.example.com
	APIToken string // API authentication token
}

// paperlessHTTPClient is a shared HTTP client for Paperless-ngx calls.
var paperlessHTTPClient = &http.Client{Timeout: 60 * time.Second}

// ── Internal helpers ─────────────────────────────────────────────────

// paperlessAPI builds a full API URL from the base URL and a path.
func paperlessAPI(cfg PaperlessConfig, path string) string {
	base := strings.TrimRight(cfg.URL, "/")
	return base + "/api/" + strings.TrimLeft(path, "/")
}

// paperlessRequest performs an authenticated HTTP request against the Paperless-ngx API.
func paperlessRequest(cfg PaperlessConfig, method, endpoint string, body io.Reader, contentType string) (*http.Response, error) {
	reqURL := paperlessAPI(cfg, endpoint)
	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+cfg.APIToken)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return paperlessHTTPClient.Do(req)
}

// paperlessJSON performs a GET request and decodes the JSON response body.
func paperlessJSON(cfg PaperlessConfig, endpoint string) (map[string]interface{}, error) {
	resp, err := paperlessRequest(cfg, "GET", endpoint, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(data), 500))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return result, nil
}

// paperlessEncode is a convenience wrapper that marshals a value to JSON.
func paperlessEncode(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// wrapExternal wraps untrusted content in <external_data> tags to prevent prompt injection.
func wrapExternal(s string) string {
	return security.IsolateExternalData(s)
}

// ── Public operations ────────────────────────────────────────────────

// PaperlessSearch performs a full-text document search.
// query is the search string; tags, correspondent, documentType are optional filters.
func PaperlessSearch(cfg PaperlessConfig, query, tags, correspondent, documentType string, limit int) string {
	if cfg.URL == "" || cfg.APIToken == "" {
		return paperlessEncode(FSResult{Status: "error", Message: "Paperless-ngx URL and API token must be configured."})
	}

	params := url.Values{}
	if query != "" {
		params.Set("query", query)
	}
	if tags != "" {
		params.Set("tags__name__icontains", tags)
	}
	if correspondent != "" {
		params.Set("correspondent__name__icontains", correspondent)
	}
	if documentType != "" {
		params.Set("document_type__name__icontains", documentType)
	}
	if limit > 0 {
		params.Set("page_size", strconv.Itoa(limit))
	} else {
		params.Set("page_size", "25")
	}
	params.Set("ordering", "-created")

	endpoint := "documents/?" + params.Encode()
	result, err := paperlessJSON(cfg, endpoint)
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Search failed: %v", err)})
	}

	count, _ := result["count"].(float64)
	results, _ := result["results"].([]interface{})

	type docSummary struct {
		ID            interface{} `json:"id"`
		Title         string      `json:"title"`
		Correspondent string      `json:"correspondent_name,omitempty"`
		DocumentType  string      `json:"document_type_name,omitempty"`
		Created       string      `json:"created,omitempty"`
		Added         string      `json:"added,omitempty"`
		Tags          []string    `json:"tags,omitempty"`
	}

	var docs []docSummary
	for _, r := range results {
		doc, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		d := docSummary{
			ID:    doc["id"],
			Title: wrapExternal(fmt.Sprintf("%v", doc["title"])),
		}
		if v, ok := doc["correspondent_name"]; ok && v != nil {
			d.Correspondent = fmt.Sprintf("%v", v)
		}
		if v, ok := doc["document_type_name"]; ok && v != nil {
			d.DocumentType = fmt.Sprintf("%v", v)
		}
		if v, ok := doc["created"]; ok && v != nil {
			d.Created = fmt.Sprintf("%v", v)
		}
		if v, ok := doc["added"]; ok && v != nil {
			d.Added = fmt.Sprintf("%v", v)
		}
		if tagList, ok := doc["tags"].([]interface{}); ok {
			for _, t := range tagList {
				d.Tags = append(d.Tags, fmt.Sprintf("%v", t))
			}
		}
		docs = append(docs, d)
	}

	return paperlessEncode(FSResult{
		Status:  "success",
		Message: fmt.Sprintf("Found %d documents (showing %d)", int(count), len(docs)),
		Data:    docs,
	})
}

// PaperlessGet retrieves metadata for a single document by ID.
func PaperlessGet(cfg PaperlessConfig, documentID string) string {
	if documentID == "" {
		return paperlessEncode(FSResult{Status: "error", Message: "'document_id' is required for get"})
	}

	endpoint := "documents/" + documentID + "/"
	doc, err := paperlessJSON(cfg, endpoint)
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to get document: %v", err)})
	}

	// Wrap untrusted text fields
	if title, ok := doc["title"].(string); ok {
		doc["title"] = wrapExternal(title)
	}
	if content, ok := doc["content"].(string); ok {
		doc["content"] = wrapExternal(truncate(content, 8000))
	}

	return paperlessEncode(FSResult{Status: "success", Data: doc})
}

// PaperlessDownload retrieves the text content of a document by ID.
func PaperlessDownload(cfg PaperlessConfig, documentID string) string {
	if documentID == "" {
		return paperlessEncode(FSResult{Status: "error", Message: "'document_id' is required for download"})
	}

	// First get document metadata for the title
	endpoint := "documents/" + documentID + "/"
	doc, err := paperlessJSON(cfg, endpoint)
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to get document: %v", err)})
	}

	title, _ := doc["title"].(string)
	content, _ := doc["content"].(string)

	if content == "" {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Document %s has no text content", documentID)})
	}

	text := truncate(content, 8000)

	return paperlessEncode(FSResult{
		Status:  "success",
		Message: fmt.Sprintf("Document: %s (%s bytes)", wrapExternal(title), strconv.Itoa(len(content))),
		Data:    wrapExternal(text),
	})
}

// PaperlessUpload uploads a new document to Paperless-ngx.
func PaperlessUpload(cfg PaperlessConfig, title, content, tags, correspondent, documentType string) string {
	if content == "" {
		return paperlessEncode(FSResult{Status: "error", Message: "'content' is required for upload"})
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if title != "" {
		writer.WriteField("title", title)
	}
	if tags != "" {
		// Paperless expects tags as individual tag names
		for _, tag := range strings.Split(tags, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				writer.WriteField("tags", tag)
			}
		}
	}
	if correspondent != "" {
		writer.WriteField("correspondent", correspondent)
	}
	if documentType != "" {
		writer.WriteField("document_type", documentType)
	}

	// Create a file part for the document content
	part, err := writer.CreateFormFile("document", sanitizeFilename(title)+".txt")
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to create form: %v", err)})
	}
	part.Write([]byte(content))
	writer.Close()

	resp, err := paperlessRequest(cfg, "POST", "documents/post_document/", body, writer.FormDataContentType())
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Upload failed: %v", err)})
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return paperlessEncode(FSResult{Status: "success", Message: "Document uploaded successfully. It will appear after Paperless-ngx finishes processing."})
	}

	return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Upload returned HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 500))})
}

// PaperlessUpdate updates metadata on an existing document (PATCH).
func PaperlessUpdate(cfg PaperlessConfig, documentID, title, tags, correspondent, documentType string) string {
	if documentID == "" {
		return paperlessEncode(FSResult{Status: "error", Message: "'document_id' is required for update"})
	}

	patch := make(map[string]interface{})
	if title != "" {
		patch["title"] = title
	}
	if correspondent != "" {
		patch["correspondent_name"] = correspondent
	}
	if documentType != "" {
		patch["document_type_name"] = documentType
	}
	// Tags are passed as comma-separated names; Paperless expects tag IDs,
	// but the API also supports tag names via the correspondent endpoint.
	// We use the __name field approach for simplicity.
	if tags != "" {
		var tagNames []string
		for _, t := range strings.Split(tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagNames = append(tagNames, t)
			}
		}
		patch["tags_names"] = tagNames
	}

	if len(patch) == 0 {
		return paperlessEncode(FSResult{Status: "error", Message: "At least one field to update must be provided (title, tags, correspondent, document_type)"})
	}

	patchJSON, _ := json.Marshal(patch)
	endpoint := "documents/" + documentID + "/"
	resp, err := paperlessRequest(cfg, "PATCH", endpoint, bytes.NewReader(patchJSON), "application/json")
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Update failed: %v", err)})
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return paperlessEncode(FSResult{Status: "success", Message: fmt.Sprintf("Document %s updated successfully", documentID)})
	}

	return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("PATCH returned HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 500))})
}

// PaperlessDelete deletes a document by ID.
func PaperlessDelete(cfg PaperlessConfig, documentID string) string {
	if documentID == "" {
		return paperlessEncode(FSResult{Status: "error", Message: "'document_id' is required for delete"})
	}

	endpoint := "documents/" + documentID + "/"
	resp, err := paperlessRequest(cfg, "DELETE", endpoint, nil, "")
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Delete failed: %v", err)})
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		return paperlessEncode(FSResult{Status: "success", Message: fmt.Sprintf("Document %s deleted", documentID)})
	}
	if resp.StatusCode == 404 {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Document %s not found", documentID)})
	}

	respBody, _ := io.ReadAll(resp.Body)
	return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("DELETE returned HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 500))})
}

// PaperlessListTags returns all tags defined in Paperless-ngx.
func PaperlessListTags(cfg PaperlessConfig) string {
	return paperlessListEndpoint(cfg, "tags/", "tags")
}

// PaperlessListCorrespondents returns all correspondents defined in Paperless-ngx.
func PaperlessListCorrespondents(cfg PaperlessConfig) string {
	return paperlessListEndpoint(cfg, "correspondents/", "correspondents")
}

// PaperlessListDocumentTypes returns all document types defined in Paperless-ngx.
func PaperlessListDocumentTypes(cfg PaperlessConfig) string {
	return paperlessListEndpoint(cfg, "document_types/", "document types")
}

// paperlessListEndpoint is a generic helper for listing tags/correspondents/document_types.
func paperlessListEndpoint(cfg PaperlessConfig, endpoint, label string) string {
	result, err := paperlessJSON(cfg, endpoint+"?page_size=200")
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to list %s: %v", label, err)})
	}

	results, _ := result["results"].([]interface{})
	count, _ := result["count"].(float64)

	type item struct {
		ID   interface{} `json:"id"`
		Name string      `json:"name"`
	}
	var items []item
	for _, r := range results {
		obj, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		items = append(items, item{
			ID:   obj["id"],
			Name: wrapExternal(fmt.Sprintf("%v", obj["name"])),
		})
	}

	return paperlessEncode(FSResult{
		Status:  "success",
		Message: fmt.Sprintf("Found %d %s", int(count), label),
		Data:    items,
	})
}

// sanitizeFilename produces a safe filename from a title string.
func sanitizeFilename(title string) string {
	if title == "" {
		return "document"
	}
	// Replace problematic characters
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	name := replacer.Replace(title)
	if len(name) > 100 {
		name = name[:100]
	}
	return name
}
