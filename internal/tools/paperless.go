package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"aurago/internal/security"
)

// PaperlessConfig holds connection parameters for a Paperless-ngx instance.
type PaperlessConfig struct {
	Context  context.Context `json:"-"` // Request owner; nil retains the standalone API lifetime.
	URL      string          // Base URL, e.g. https://paperless.example.com
	APIToken string          // API authentication token
	ReadOnly bool            // true = block upload/update/delete
}

// paperlessHTTPClient is a shared HTTP client for Paperless-ngx calls.
var paperlessHTTPClient = &http.Client{Timeout: 60 * time.Second, CheckRedirect: security.SameOriginRedirect}

// ── Internal helpers ─────────────────────────────────────────────────

// paperlessAPI builds a full API URL from the base URL and a path.
func paperlessAPI(cfg PaperlessConfig, path string) string {
	base := strings.TrimRight(cfg.URL, "/")
	return base + "/api/" + strings.TrimLeft(path, "/")
}

// paperlessRequest performs an authenticated HTTP request against the Paperless-ngx API.
func paperlessRequest(cfg PaperlessConfig, method, endpoint string, body io.Reader, contentType string) (*http.Response, error) {
	if err := security.ValidateHTTPBaseURL(cfg.URL); err != nil {
		return nil, err
	}
	security.RegisterSensitive(cfg.APIToken)
	reqURL := paperlessAPI(cfg, endpoint)
	req, err := newToolHTTPRequest(cfg.Context, method, reqURL, body)
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

	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read paperless response body: %w", err)
	}
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

func paperlessReadOnlyError(cfg PaperlessConfig) string {
	if !cfg.ReadOnly {
		return ""
	}
	return paperlessEncode(FSResult{Status: "error", Message: "Paperless-ngx is in read-only mode. Disable paperless_ngx.readonly to allow changes."})
}

// wrapExternal wraps untrusted content in <external_data> tags to prevent prompt injection.
func wrapExternal(s string) string {
	return security.IsolateExternalData(s)
}

const (
	paperlessLookupPageSize       = 200
	paperlessMaxLookupPages       = 50
	paperlessMaxSearchResults     = 50
	paperlessDefaultSearchResults = 25
	paperlessTextLimit            = 8000
	paperlessDisplayLookupPages   = 1
)

type paperlessNamed struct {
	ID   int
	Name string
}

type paperlessCatalog struct {
	byName    map[string]int
	byID      map[int]string
	items     []paperlessNamed
	count     int
	truncated bool
}

// paperlessLoadCatalog reads one API collection.
// wanted stops the walk once every non-numeric name is present.
// maxPages outside 1..paperlessMaxLookupPages uses the package cap.
// A rejected or repeated next link keeps the rows already read and marks the catalog truncated.
// A page that fails to load returns that error. Requested names stop the walk before the next page is read.
func paperlessLoadCatalog(cfg PaperlessConfig, resource string, wanted []string, maxPages int) (paperlessCatalog, error) {
	resource = strings.Trim(resource, "/")
	cat := paperlessCatalog{
		byName: map[string]int{},
		byID:   map[int]string{},
	}
	if resource == "" {
		return cat, fmt.Errorf("paperless catalog resource is empty")
	}
	if maxPages < 1 || maxPages > paperlessMaxLookupPages {
		maxPages = paperlessMaxLookupPages
	}
	base, err := url.Parse(cfg.URL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return cat, fmt.Errorf("paperless URL is not an absolute HTTP(S) URL")
	}
	endpoint := resource + "/?page_size=" + strconv.Itoa(paperlessLookupPageSize)
	seen := map[string]struct{}{}
	for page := 0; page < maxPages; page++ {
		if _, dup := seen[endpoint]; dup {
			paperlessFinishCatalog(&cat, true)
			return cat, nil
		}
		seen[endpoint] = struct{}{}
		result, err := paperlessJSON(cfg, endpoint)
		if err != nil {
			return cat, err
		}
		if count, ok := result["count"].(float64); ok && int(count) > cat.count {
			cat.count = int(count)
		}
		results, _ := result["results"].([]interface{})
		for _, row := range results {
			obj, ok := row.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := obj["name"].(string)
			id, ok := paperlessJSONID(obj["id"])
			if !ok || name == "" {
				continue
			}
			if _, exists := cat.byID[id]; exists {
				continue
			}
			cat.byName[strings.ToLower(name)] = id
			cat.byID[id] = name
			cat.items = append(cat.items, paperlessNamed{ID: id, Name: name})
		}
		if paperlessCatalogHasNames(cat, wanted) {
			paperlessFinishCatalog(&cat, false)
			return cat, nil
		}
		nextRaw, _ := result["next"].(string)
		nextRaw = strings.TrimSpace(nextRaw)
		if nextRaw == "" {
			paperlessFinishCatalog(&cat, false)
			return cat, nil
		}
		nextEndpoint, err := paperlessNextEndpoint(base, resource, nextRaw)
		if err != nil {
			paperlessFinishCatalog(&cat, true)
			return cat, nil
		}
		if page == maxPages-1 {
			paperlessFinishCatalog(&cat, true)
			return cat, nil
		}
		endpoint = nextEndpoint
	}
	paperlessFinishCatalog(&cat, true)
	return cat, nil
}

func paperlessCatalogHasNames(cat paperlessCatalog, wanted []string) bool {
	if len(wanted) == 0 {
		return false
	}
	for _, name := range wanted {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, numeric := paperlessNumericID(name); numeric {
			continue
		}
		if _, ok := cat.byName[strings.ToLower(name)]; !ok {
			return false
		}
	}
	return true
}

func paperlessFinishCatalog(cat *paperlessCatalog, hitCap bool) {
	if cat.count < len(cat.items) {
		cat.count = len(cat.items)
	}
	cat.truncated = hitCap || cat.count > len(cat.items)
}

func paperlessNextEndpoint(base *url.URL, resource, nextRaw string) (string, error) {
	parsed, err := url.Parse(nextRaw)
	if err != nil || parsed.Opaque != "" {
		return "", fmt.Errorf("paperless next URL is invalid")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("paperless next URL must not contain credentials")
	}
	if parsed.Fragment != "" {
		return "", fmt.Errorf("paperless next URL must not contain a fragment")
	}
	absolute := parsed
	if !parsed.IsAbs() {
		absolute = base.ResolveReference(parsed)
	}
	if absolute.User != nil || !security.SameHTTPOrigin(base, absolute) {
		return "", fmt.Errorf("paperless next URL left the configured origin")
	}
	if paperlessPathEscapes(absolute.EscapedPath(), absolute.Path) {
		return "", fmt.Errorf("paperless next URL is not an API path")
	}
	prefix := strings.TrimRight(base.EscapedPath(), "/") + "/api/"
	escapedPath := absolute.EscapedPath()
	if !strings.HasPrefix(escapedPath, prefix) {
		return "", fmt.Errorf("paperless next URL is not an API path")
	}
	decodedPrefix := strings.TrimRight(base.Path, "/") + "/api/"
	cleaned := path.Clean(absolute.Path)
	if !strings.HasPrefix(cleaned, decodedPrefix) && cleaned != strings.TrimRight(decodedPrefix, "/") {
		return "", fmt.Errorf("paperless next URL is not an API path")
	}
	rest := strings.TrimPrefix(escapedPath, prefix)
	if rest == "" || strings.Contains(rest, "..") || strings.HasPrefix(rest, "/") {
		return "", fmt.Errorf("paperless next URL is not an API path")
	}
	if !strings.HasPrefix(rest, resource+"/") && !strings.HasPrefix(rest, resource+"?") {
		return "", fmt.Errorf("paperless next URL is not an API path")
	}
	if absolute.RawQuery != "" {
		rest += "?" + absolute.RawQuery
	}
	return rest, nil
}

func paperlessPathEscapes(escaped, decoded string) bool {
	if strings.Contains(decoded, "..") || strings.Contains(escaped, "..") {
		return true
	}
	once, err := url.PathUnescape(escaped)
	if err != nil || strings.Contains(once, "..") {
		return true
	}
	twice, err := url.PathUnescape(once)
	return err != nil || strings.Contains(twice, "..")
}

func paperlessJSONID(raw interface{}) (int, bool) {
	switch number := raw.(type) {
	case float64:
		if number <= 0 || number != math.Trunc(number) || number > float64(math.MaxInt) {
			return 0, false
		}
		return int(number), true
	case json.Number:
		whole, err := number.Int64()
		if err != nil || whole <= 0 || whole > int64(math.MaxInt) {
			return 0, false
		}
		return int(whole), true
	default:
		return 0, false
	}
}

func splitPaperlessNames(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func paperlessNamesNeedCatalog(parts []string) bool {
	for _, part := range parts {
		if _, ok := paperlessNumericID(part); !ok {
			return true
		}
	}
	return false
}

func paperlessNumericID(raw string) (int, bool) {
	if !validPaperlessDocumentID(raw) {
		return 0, false
	}
	number, err := strconv.ParseInt(raw, 10, 63)
	if err != nil || number <= 0 || number > int64(math.MaxInt) {
		return 0, false
	}
	return int(number), true
}

func paperlessResolveField(cfg PaperlessConfig, resource, label, raw string, many bool) ([]int, []string, error) {
	var parts []string
	if many {
		parts = splitPaperlessNames(raw)
	} else if trimmed := strings.TrimSpace(raw); trimmed != "" {
		parts = []string{trimmed}
	}
	if len(parts) == 0 {
		return nil, nil, nil
	}
	var cat paperlessCatalog
	loaded := false
	if paperlessNamesNeedCatalog(parts) {
		loadedCat, err := paperlessLoadCatalog(cfg, resource, parts, paperlessMaxLookupPages)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load %s: %w", label, err)
		}
		cat = loadedCat
		loaded = true
	}
	return paperlessResolveExact(cat, loaded, label, parts)
}

func paperlessResolveExact(cat paperlessCatalog, loaded bool, label string, parts []string) ([]int, []string, error) {
	var ids []int
	var names []string
	var missing []string
	for _, part := range parts {
		if id, ok := paperlessNumericID(part); ok {
			name := part
			if loaded {
				if known, found := cat.byID[id]; found && known != "" {
					name = known
				}
			}
			ids = append(ids, id)
			names = append(names, name)
			continue
		}
		if !loaded {
			return nil, nil, fmt.Errorf("%s catalog was not loaded", label)
		}
		id, ok := cat.byName[strings.ToLower(part)]
		if !ok {
			missing = append(missing, part)
			continue
		}
		name := cat.byID[id]
		if name == "" {
			name = part
		}
		ids = append(ids, id)
		names = append(names, name)
	}
	if len(missing) > 0 {
		joined := strings.Join(missing, ", ")
		if cat.truncated {
			return nil, nil, fmt.Errorf("%s catalog is incomplete and %s was not found", label, joined)
		}
		return nil, nil, fmt.Errorf("%s not found: %s", label, joined)
	}
	return ids, names, nil
}

func paperlessSearchLimit(limit int) int {
	if limit <= 0 {
		return paperlessDefaultSearchResults
	}
	if limit > paperlessMaxSearchResults {
		return paperlessMaxSearchResults
	}
	return limit
}

func paperlessClipText(content string, limit int) (string, int, int, bool) {
	runes := []rune(content)
	original := len(runes)
	if limit < 0 {
		limit = 0
	}
	if original <= limit {
		return content, original, original, false
	}
	return string(runes[:limit]), limit, original, true
}

func paperlessTaskID(body []byte) (string, bool) {
	var asString string
	if err := json.Unmarshal(body, &asString); err == nil {
		if id := strings.TrimSpace(asString); id != "" && len(id) <= 80 && !strings.ContainsAny(id, " \n\r\t<>") {
			return id, true
		}
	}
	var asObject map[string]interface{}
	if err := json.Unmarshal(body, &asObject); err == nil {
		if id, ok := asObject["task_id"].(string); ok {
			id = strings.TrimSpace(id)
			if id != "" && len(id) <= 80 && !strings.ContainsAny(id, " \n\r\t<>") {
				return id, true
			}
		}
	}
	raw := strings.TrimSpace(string(body))
	if raw != "" && len(raw) <= 80 && !strings.ContainsAny(raw, " \n\r\t<>\"") && !strings.HasPrefix(raw, "{") && !strings.HasPrefix(raw, "[") {
		return raw, true
	}
	return "", false
}

func paperlessTagIDs(raw interface{}) []int {
	list, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var ids []int
	seen := map[int]struct{}{}
	for _, item := range list {
		id, ok := paperlessJSONID(item)
		if !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func paperlessDocumentTagIDs(raw interface{}) ([]int, bool) {
	list, ok := raw.([]interface{})
	if !ok {
		return nil, false
	}
	ids := paperlessTagIDs(raw)
	if len(list) > 0 && len(ids) == 0 {
		return nil, false
	}
	return ids, true
}

func paperlessRetainKnownTagIDs(current []int, cat paperlessCatalog) []int {
	kept := make([]int, 0, len(current))
	for _, id := range current {
		if _, ok := cat.byID[id]; ok {
			kept = append(kept, id)
		}
	}
	return kept
}

func paperlessUnionIDs(current, extra []int) (union []int, added []int) {
	seen := map[int]struct{}{}
	for _, id := range current {
		if id <= 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		union = append(union, id)
	}
	for _, id := range extra {
		if id <= 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		union = append(union, id)
		added = append(added, id)
	}
	return union, added
}

// ── Public operations ────────────────────────────────────────────────

// PaperlessTestConnection tests connectivity to a Paperless-ngx instance.
func PaperlessTestConnection(cfg PaperlessConfig) string {
	if cfg.URL == "" {
		return paperlessEncode(FSResult{Status: "error", Message: "Paperless-ngx URL is not configured"})
	}
	if cfg.APIToken == "" {
		return paperlessEncode(FSResult{Status: "error", Message: "Paperless-ngx API token is not configured"})
	}
	// Hit a lightweight endpoint to verify URL + token
	result, err := paperlessJSON(cfg, "tags/?page_size=1")
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Connection failed: %v", err)})
	}
	count, _ := result["count"].(float64)
	return paperlessEncode(FSResult{
		Status:  "success",
		Message: fmt.Sprintf("Connection successful (%d tags found)", int(count)),
	})
}

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
	var tagLookup map[int]string
	if parts := splitPaperlessNames(tags); len(parts) > 0 {
		var cat paperlessCatalog
		loaded := false
		if paperlessNamesNeedCatalog(parts) {
			loadedCat, err := paperlessLoadCatalog(cfg, "tags", parts, paperlessMaxLookupPages)
			if err != nil {
				return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("failed to load tag: %v", err)})
			}
			cat = loadedCat
			loaded = true
			tagLookup = cat.byID
		}
		ids, _, err := paperlessResolveExact(cat, loaded, "tag", parts)
		if err != nil {
			return paperlessEncode(FSResult{Status: "error", Message: err.Error()})
		}
		for _, id := range ids {
			params.Add("tags__id__all", strconv.Itoa(id))
		}
	}
	if correspondent != "" {
		ids, _, err := paperlessResolveField(cfg, "correspondents", "correspondent", correspondent, false)
		if err != nil {
			return paperlessEncode(FSResult{Status: "error", Message: err.Error()})
		}
		if len(ids) == 1 {
			params.Set("correspondent__id", strconv.Itoa(ids[0]))
		}
	}
	if documentType != "" {
		ids, _, err := paperlessResolveField(cfg, "document_types", "document type", documentType, false)
		if err != nil {
			return paperlessEncode(FSResult{Status: "error", Message: err.Error()})
		}
		if len(ids) == 1 {
			params.Set("document_type__id", strconv.Itoa(ids[0]))
		}
	}
	params.Set("page_size", strconv.Itoa(paperlessSearchLimit(limit)))
	params.Set("ordering", "-created")

	endpoint := "documents/?" + params.Encode()
	result, err := paperlessJSON(cfg, endpoint)
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Search failed: %v", err)})
	}

	count, _ := result["count"].(float64)
	results, _ := result["results"].([]interface{})

	// Tag names are display-only. One page is enough; other IDs stay numeric.
	if tagLookup == nil {
		if cat, err := paperlessLoadCatalog(cfg, "tags", nil, paperlessDisplayLookupPages); err == nil {
			tagLookup = cat.byID
		}
	}

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
				if tagID, ok := paperlessJSONID(t); ok {
					if name, found := tagLookup[tagID]; found {
						d.Tags = append(d.Tags, name)
						continue
					}
					d.Tags = append(d.Tags, strconv.Itoa(tagID))
					continue
				}
				d.Tags = append(d.Tags, fmt.Sprintf("%v", t))
			}
		}
		docs = append(docs, d)
	}

	total := int(count)
	if total < len(docs) {
		total = len(docs)
	}
	message := fmt.Sprintf("Found %d documents", total)
	if len(docs) < total {
		message = fmt.Sprintf("Showing %d of %d documents", len(docs), total)
	}
	return paperlessEncode(FSResult{
		Status:  "success",
		Message: message,
		Data:    docs,
	})
}

// PaperlessGet retrieves metadata for a single document by ID.
func PaperlessGet(cfg PaperlessConfig, documentID string) string {
	if !validPaperlessDocumentID(documentID) {
		return paperlessEncode(FSResult{Status: "error", Message: "'document_id' must be a positive decimal integer for get"})
	}

	endpoint := "documents/" + documentID + "/"
	doc, err := paperlessJSON(cfg, endpoint)
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to get document: %v", err)})
	}

	title, _ := doc["title"].(string)
	if title != "" {
		doc["title"] = wrapExternal(title)
	}
	message := ""
	if content, ok := doc["content"].(string); ok {
		clipped, returned, original, truncatedText := paperlessClipText(content, paperlessTextLimit)
		doc["content"] = wrapExternal(clipped)
		if truncatedText {
			message = fmt.Sprintf("Document: %s (returned %d of %d characters)", wrapExternal(title), returned, original)
		}
	}

	return paperlessEncode(FSResult{Status: "success", Message: message, Data: doc})
}

// PaperlessDownload retrieves the text content of a document by ID.
func PaperlessDownload(cfg PaperlessConfig, documentID string) string {
	if !validPaperlessDocumentID(documentID) {
		return paperlessEncode(FSResult{Status: "error", Message: "'document_id' must be a positive decimal integer for download"})
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

	text, returned, original, truncatedText := paperlessClipText(content, paperlessTextLimit)
	message := fmt.Sprintf("Document: %s (%d characters)", wrapExternal(title), original)
	if truncatedText {
		message = fmt.Sprintf("Document: %s (returned %d of %d characters)", wrapExternal(title), returned, original)
	}

	return paperlessEncode(FSResult{
		Status:  "success",
		Message: message,
		Data:    wrapExternal(text),
	})
}

// PaperlessUpload uploads a new document to Paperless-ngx.
func PaperlessUpload(cfg PaperlessConfig, title, content, tags, correspondent, documentType string) string {
	if msg := paperlessReadOnlyError(cfg); msg != "" {
		return msg
	}
	if content == "" {
		return paperlessEncode(FSResult{Status: "error", Message: "'content' is required for upload"})
	}

	tagIDs, _, err := paperlessResolveField(cfg, "tags", "tag", tags, true)
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: err.Error()})
	}
	correspondentIDs, _, err := paperlessResolveField(cfg, "correspondents", "correspondent", correspondent, false)
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: err.Error()})
	}
	documentTypeIDs, _, err := paperlessResolveField(cfg, "document_types", "document type", documentType, false)
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: err.Error()})
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writeField := func(key, value string) error {
		return writer.WriteField(key, value)
	}
	if title != "" {
		if err := writeField("title", title); err != nil {
			return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to create form: %v", err)})
		}
	}
	for _, id := range tagIDs {
		if err := writeField("tags", strconv.Itoa(id)); err != nil {
			return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to create form: %v", err)})
		}
	}
	if len(correspondentIDs) == 1 {
		if err := writeField("correspondent", strconv.Itoa(correspondentIDs[0])); err != nil {
			return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to create form: %v", err)})
		}
	}
	if len(documentTypeIDs) == 1 {
		if err := writeField("document_type", strconv.Itoa(documentTypeIDs[0])); err != nil {
			return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to create form: %v", err)})
		}
	}

	// Text-only upload. Paperless consumes the file asynchronously.
	part, err := writer.CreateFormFile("document", sanitizeFilename(title)+".txt")
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to create form: %v", err)})
	}
	if _, err := part.Write([]byte(content)); err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to create form: %v", err)})
	}
	if err := writer.Close(); err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to create form: %v", err)})
	}

	resp, err := paperlessRequest(cfg, "POST", "documents/post_document/", body, writer.FormDataContentType())
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Upload failed: %v", err)})
	}
	defer resp.Body.Close()

	respBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Upload response read failed: %v", err)})
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		data := map[string]interface{}{}
		if taskID, ok := paperlessTaskID(respBody); ok {
			data["task_id"] = taskID
		}
		return paperlessEncode(FSResult{
			Status:  "success",
			Message: "Document accepted. Processing is still running.",
			Data:    data,
		})
	}

	return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Upload returned HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 500))})
}

// PaperlessUpdate updates metadata on an existing document (PATCH).
// The Paperless-ngx API expects integer IDs for correspondent, document_type,
// and tags. Names are resolved to IDs via the list endpoints automatically.
func PaperlessUpdate(cfg PaperlessConfig, documentID, title, tags, correspondent, documentType string) string {
	if msg := paperlessReadOnlyError(cfg); msg != "" {
		return msg
	}
	if !validPaperlessDocumentID(documentID) {
		return paperlessEncode(FSResult{Status: "error", Message: "'document_id' must be a positive decimal integer for update"})
	}

	patch := make(map[string]interface{})
	if title != "" {
		patch["title"] = title
	}

	if correspondent != "" {
		ids, _, err := paperlessResolveField(cfg, "correspondents", "correspondent", correspondent, false)
		if err != nil {
			return paperlessEncode(FSResult{Status: "error", Message: err.Error()})
		}
		if len(ids) == 1 {
			patch["correspondent"] = ids[0]
		}
	}

	if documentType != "" {
		ids, _, err := paperlessResolveField(cfg, "document_types", "document type", documentType, false)
		if err != nil {
			return paperlessEncode(FSResult{Status: "error", Message: err.Error()})
		}
		if len(ids) == 1 {
			patch["document_type"] = ids[0]
		}
	}

	tagNote := ""
	if parts := splitPaperlessNames(tags); len(parts) > 0 {
		var cat paperlessCatalog
		loaded := false
		if paperlessNamesNeedCatalog(parts) {
			loadedCat, err := paperlessLoadCatalog(cfg, "tags", parts, paperlessMaxLookupPages)
			if err != nil {
				return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("failed to load tag: %v", err)})
			}
			cat = loadedCat
			loaded = true
		}
		ids, names, err := paperlessResolveExact(cat, loaded, "tag", parts)
		if err != nil {
			return paperlessEncode(FSResult{Status: "error", Message: err.Error()})
		}
		doc, err := paperlessJSON(cfg, "documents/"+documentID+"/")
		if err != nil {
			return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to get document: %v", err)})
		}
		current, tagsOK := paperlessDocumentTagIDs(doc["tags"])
		if !tagsOK {
			return paperlessEncode(FSResult{Status: "error", Message: "Failed to read existing tags, so no tags were changed."})
		}
		if loaded && !cat.truncated {
			current = paperlessRetainKnownTagIDs(current, cat)
		}
		union, added := paperlessUnionIDs(current, ids)
		if len(added) == 0 {
			tagNote = " Existing tags were kept."
		} else {
			patch["tags"] = union
			nameByID := make(map[int]string, len(ids))
			for i, id := range ids {
				nameByID[id] = names[i]
			}
			addedNames := make([]string, 0, len(added))
			for _, id := range added {
				if name := nameByID[id]; name != "" {
					addedNames = append(addedNames, name)
				} else {
					addedNames = append(addedNames, strconv.Itoa(id))
				}
			}
			tagNote = fmt.Sprintf(" Added tags: %s. Existing tags were kept.", strings.Join(addedNames, ", "))
		}
	}

	if len(patch) == 0 {
		if tagNote != "" {
			return paperlessEncode(FSResult{Status: "success", Message: fmt.Sprintf("Document %s updated successfully.%s", documentID, tagNote)})
		}
		return paperlessEncode(FSResult{Status: "error", Message: "At least one field to update must be provided (title, tags, correspondent, document_type)"})
	}

	patchJSON, _ := json.Marshal(patch)
	endpoint := "documents/" + documentID + "/"
	resp, err := paperlessRequest(cfg, "PATCH", endpoint, bytes.NewReader(patchJSON), "application/json")
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Update failed: %v", err)})
	}
	defer resp.Body.Close()

	respBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("PATCH response read failed: %v", err)})
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return paperlessEncode(FSResult{Status: "success", Message: fmt.Sprintf("Document %s updated successfully.%s", documentID, tagNote)})
	}

	return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("PATCH returned HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 500))})
}

// PaperlessDelete deletes a document by ID.
func PaperlessDelete(cfg PaperlessConfig, documentID string) string {
	if msg := paperlessReadOnlyError(cfg); msg != "" {
		return msg
	}
	if !validPaperlessDocumentID(documentID) {
		return paperlessEncode(FSResult{Status: "error", Message: "'document_id' must be a positive decimal integer for delete"})
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

	respBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("DELETE response read failed: %v", err)})
	}
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
	cat, err := paperlessLoadCatalog(cfg, strings.Trim(endpoint, "/"), nil, paperlessMaxLookupPages)
	if err != nil {
		return paperlessEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to list %s: %v", label, err)})
	}

	type item struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	items := make([]item, 0, len(cat.items))
	for _, row := range cat.items {
		items = append(items, item{
			ID:   row.ID,
			Name: wrapExternal(row.Name),
		})
	}
	message := fmt.Sprintf("Found %d %s", len(items), label)
	if cat.truncated {
		total := cat.count
		if total < len(items) {
			total = len(items)
		}
		message = fmt.Sprintf("Showing %d of %d %s", len(items), total, label)
	}

	return paperlessEncode(FSResult{
		Status:  "success",
		Message: message,
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

func validPaperlessDocumentID(id string) bool {
	if id == "" || id[0] < '1' || id[0] > '9' {
		return false
	}
	for _, ch := range id {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	_, err := strconv.ParseUint(id, 10, 63)
	return err == nil
}
