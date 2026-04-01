package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// GWorkspaceClient holds auth state for Google Workspace API calls.
type GWorkspaceClient struct {
	AccessToken  string
	RefreshToken string
	TokenExpiry  time.Time
	ClientID     string
	ClientSecret string
	Vault        *security.Vault
}

var gwHTTPClient = &http.Client{Timeout: 30 * time.Second}

// NewGWorkspaceClient builds a client from config + vault.
func NewGWorkspaceClient(cfg config.Config, vault *security.Vault) (*GWorkspaceClient, error) {
	gw := cfg.GoogleWorkspace
	if gw.AccessToken == "" {
		return nil, fmt.Errorf("no Google Workspace access token — connect via Settings > Google Workspace")
	}

	clientSecret := gw.ClientSecret
	if clientSecret == "" {
		s, _ := vault.ReadSecret("google_workspace_client_secret")
		clientSecret = s
	}

	var expiry time.Time
	if gw.TokenExpiry != "" {
		expiry, _ = time.Parse(time.RFC3339, gw.TokenExpiry)
	}

	return &GWorkspaceClient{
		AccessToken:  gw.AccessToken,
		RefreshToken: gw.RefreshToken,
		TokenExpiry:  expiry,
		ClientID:     gw.ClientID,
		ClientSecret: clientSecret,
		Vault:        vault,
	}, nil
}

// refreshIfNeeded refreshes the access token if it has expired.
func (c *GWorkspaceClient) refreshIfNeeded() error {
	if c.RefreshToken == "" {
		return nil // Cannot refresh without refresh token
	}
	if !c.TokenExpiry.IsZero() && time.Now().Before(c.TokenExpiry.Add(-60*time.Second)) {
		return nil // Still valid
	}

	form := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"refresh_token": {c.RefreshToken},
		"grant_type":    {"refresh_token"},
	}

	resp, err := gwHTTPClient.Post("https://oauth2.googleapis.com/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return fmt.Errorf("failed to read token response: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("token refresh failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	c.AccessToken = tok.AccessToken
	c.TokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)

	// Persist updated token to vault
	if c.Vault != nil {
		tokenData, _ := json.Marshal(map[string]string{
			"access_token":  c.AccessToken,
			"refresh_token": c.RefreshToken,
			"token_expiry":  c.TokenExpiry.Format(time.RFC3339),
		})
		_ = c.Vault.WriteSecret("oauth_google_workspace", string(tokenData))
	}

	return nil
}

// request makes an authenticated HTTP request to a Google API.
func (c *GWorkspaceClient) request(method, rawURL string, body interface{}) ([]byte, int, error) {
	if err := c.refreshIfNeeded(); err != nil {
		return nil, 0, err
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, rawURL, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := gwHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}
	return respBody, resp.StatusCode, nil
}

func gwErrJSON(format string, args ...interface{}) string {
	msg := fmt.Sprintf(format, args...)
	out, _ := json.Marshal(map[string]string{"status": "error", "message": msg})
	return string(out)
}

func gwWrapExternal(data interface{}) string {
	out, _ := json.Marshal(data)
	return security.IsolateExternalData(string(out))
}

// ── Gmail ────────────────────────────────────────────────────────────────

func (c *GWorkspaceClient) GmailList(query string, maxResults int) string {
	if maxResults <= 0 {
		maxResults = 10
	}
	u := fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages?maxResults=%d", maxResults)
	if query != "" {
		u += "&q=" + url.QueryEscape(query)
	}

	data, status, err := c.request("GET", u, nil)
	if err != nil {
		return gwErrJSON("Gmail list failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Gmail API error (HTTP %d): %s", status, string(data))
	}

	var result struct {
		Messages []struct {
			ID       string `json:"id"`
			ThreadID string `json:"threadId"`
		} `json:"messages"`
		ResultSizeEstimate int `json:"resultSizeEstimate"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return gwErrJSON("Failed to parse Gmail list: %v", err)
	}

	// Fetch snippet for each message
	type msgSummary struct {
		ID      string `json:"id"`
		From    string `json:"from"`
		Subject string `json:"subject"`
		Snippet string `json:"snippet"`
		Date    string `json:"date"`
	}
	summaries := make([]msgSummary, 0, len(result.Messages))
	for _, m := range result.Messages {
		detail, dStatus, dErr := c.request("GET", fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages/%s?format=metadata&metadataHeaders=From&metadataHeaders=Subject&metadataHeaders=Date", m.ID), nil)
		if dErr != nil || dStatus != 200 {
			continue
		}
		var msg struct {
			ID      string `json:"id"`
			Snippet string `json:"snippet"`
			Payload struct {
				Headers []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"headers"`
			} `json:"payload"`
		}
		if json.Unmarshal(detail, &msg) != nil {
			continue
		}
		s := msgSummary{ID: msg.ID, Snippet: msg.Snippet}
		for _, h := range msg.Payload.Headers {
			switch h.Name {
			case "From":
				s.From = h.Value
			case "Subject":
				s.Subject = h.Value
			case "Date":
				s.Date = h.Value
			}
		}
		summaries = append(summaries, s)
	}

	return gwWrapExternal(map[string]interface{}{
		"status":   "ok",
		"count":    len(summaries),
		"messages": summaries,
	})
}

func (c *GWorkspaceClient) GmailRead(messageID string) string {
	if messageID == "" {
		return gwErrJSON("message_id is required")
	}
	u := fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages/%s", messageID)

	data, status, err := c.request("GET", u, nil)
	if err != nil {
		return gwErrJSON("Gmail read failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Gmail API error (HTTP %d): %s", status, string(data))
	}

	var msg struct {
		ID      string   `json:"id"`
		Snippet string   `json:"snippet"`
		Labels  []string `json:"labelIds"`
		Payload struct {
			Headers []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
			Body struct {
				Data string `json:"data"`
			} `json:"body"`
			Parts []struct {
				MimeType string `json:"mimeType"`
				Body     struct {
					Data string `json:"data"`
				} `json:"body"`
			} `json:"parts"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return gwErrJSON("Failed to parse message: %v", err)
	}

	// Extract headers
	headers := make(map[string]string)
	for _, h := range msg.Payload.Headers {
		switch h.Name {
		case "From", "To", "Subject", "Date", "Cc":
			headers[h.Name] = h.Value
		}
	}

	// Extract body
	bodyText := ""
	if msg.Payload.Body.Data != "" {
		if decoded, err := base64.URLEncoding.DecodeString(msg.Payload.Body.Data); err == nil {
			bodyText = string(decoded)
		}
	}
	if bodyText == "" {
		for _, part := range msg.Payload.Parts {
			if part.MimeType == "text/plain" && part.Body.Data != "" {
				if decoded, err := base64.URLEncoding.DecodeString(part.Body.Data); err == nil {
					bodyText = string(decoded)
					break
				}
			}
		}
	}

	return gwWrapExternal(map[string]interface{}{
		"status":  "ok",
		"id":      msg.ID,
		"labels":  msg.Labels,
		"headers": headers,
		"body":    bodyText,
		"snippet": msg.Snippet,
	})
}

func (c *GWorkspaceClient) GmailSend(to, subject, body string) string {
	if to == "" {
		return gwErrJSON("'to' is required")
	}
	raw := fmt.Sprintf("From: me\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s", to, subject, body)
	encoded := base64.URLEncoding.EncodeToString([]byte(raw))

	payload := map[string]string{"raw": encoded}
	data, status, err := c.request("POST", "https://gmail.googleapis.com/gmail/v1/users/me/messages/send", payload)
	if err != nil {
		return gwErrJSON("Gmail send failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Gmail send error (HTTP %d): %s", status, string(data))
	}

	var result struct {
		ID       string   `json:"id"`
		LabelIDs []string `json:"labelIds"`
	}
	json.Unmarshal(data, &result)
	out, _ := json.Marshal(map[string]interface{}{
		"status":     "ok",
		"message_id": result.ID,
	})
	return string(out)
}

func (c *GWorkspaceClient) GmailModifyLabels(messageID string, addLabels, removeLabels []string) string {
	if messageID == "" {
		return gwErrJSON("message_id is required")
	}

	payload := map[string]interface{}{
		"addLabelIds":    addLabels,
		"removeLabelIds": removeLabels,
	}
	u := fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages/%s/modify", messageID)
	data, status, err := c.request("POST", u, payload)
	if err != nil {
		return gwErrJSON("Gmail modify labels failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Gmail API error (HTTP %d): %s", status, string(data))
	}

	out, _ := json.Marshal(map[string]string{"status": "ok", "message_id": messageID})
	return string(out)
}

// ── Calendar ─────────────────────────────────────────────────────────────

func (c *GWorkspaceClient) CalendarList(query string, maxResults int) string {
	if maxResults <= 0 {
		maxResults = 10
	}
	now := time.Now().Format(time.RFC3339)
	u := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/primary/events?maxResults=%d&timeMin=%s&singleEvents=true&orderBy=startTime", maxResults, url.QueryEscape(now))
	if query != "" {
		u += "&q=" + url.QueryEscape(query)
	}

	data, status, err := c.request("GET", u, nil)
	if err != nil {
		return gwErrJSON("Calendar list failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Calendar API error (HTTP %d): %s", status, string(data))
	}

	var result struct {
		Items []struct {
			ID          string `json:"id"`
			Summary     string `json:"summary"`
			Description string `json:"description"`
			Start       struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"start"`
			End struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"end"`
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return gwErrJSON("Failed to parse calendar events: %v", err)
	}

	type eventSummary struct {
		ID          string `json:"id"`
		Summary     string `json:"summary"`
		Description string `json:"description"`
		Start       string `json:"start"`
		End         string `json:"end"`
		Status      string `json:"status"`
	}
	events := make([]eventSummary, 0, len(result.Items))
	for _, item := range result.Items {
		start := item.Start.DateTime
		if start == "" {
			start = item.Start.Date
		}
		end := item.End.DateTime
		if end == "" {
			end = item.End.Date
		}
		events = append(events, eventSummary{
			ID: item.ID, Summary: item.Summary, Description: item.Description,
			Start: start, End: end, Status: item.Status,
		})
	}

	return gwWrapExternal(map[string]interface{}{
		"status": "ok",
		"count":  len(events),
		"events": events,
	})
}

func (c *GWorkspaceClient) CalendarCreate(title, description, startTime, endTime string) string {
	if title == "" {
		return gwErrJSON("'title' is required")
	}
	if startTime == "" || endTime == "" {
		return gwErrJSON("'start_time' and 'end_time' are required (RFC3339 format)")
	}

	event := map[string]interface{}{
		"summary":     title,
		"description": description,
		"start":       map[string]string{"dateTime": startTime},
		"end":         map[string]string{"dateTime": endTime},
	}

	data, status, err := c.request("POST", "https://www.googleapis.com/calendar/v3/calendars/primary/events", event)
	if err != nil {
		return gwErrJSON("Calendar create failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Calendar API error (HTTP %d): %s", status, string(data))
	}

	var created struct {
		ID      string `json:"id"`
		HTMLink string `json:"htmlLink"`
	}
	json.Unmarshal(data, &created)
	out, _ := json.Marshal(map[string]interface{}{
		"status":   "ok",
		"event_id": created.ID,
		"link":     created.HTMLink,
	})
	return string(out)
}

func (c *GWorkspaceClient) CalendarUpdate(eventID, title, description, startTime, endTime string) string {
	if eventID == "" {
		return gwErrJSON("'event_id' is required")
	}

	event := make(map[string]interface{})
	if title != "" {
		event["summary"] = title
	}
	if description != "" {
		event["description"] = description
	}
	if startTime != "" {
		event["start"] = map[string]string{"dateTime": startTime}
	}
	if endTime != "" {
		event["end"] = map[string]string{"dateTime": endTime}
	}

	u := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/primary/events/%s", eventID)
	data, status, err := c.request("PATCH", u, event)
	if err != nil {
		return gwErrJSON("Calendar update failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Calendar API error (HTTP %d): %s", status, string(data))
	}

	out, _ := json.Marshal(map[string]string{"status": "ok", "event_id": eventID})
	return string(out)
}

// ── Drive ────────────────────────────────────────────────────────────────

func (c *GWorkspaceClient) DriveSearch(query string, maxResults int) string {
	if maxResults <= 0 {
		maxResults = 10
	}
	u := fmt.Sprintf("https://www.googleapis.com/drive/v3/files?pageSize=%d&fields=files(id,name,mimeType,modifiedTime,size)", maxResults)
	if query != "" {
		u += "&q=" + url.QueryEscape(query)
	}

	data, status, err := c.request("GET", u, nil)
	if err != nil {
		return gwErrJSON("Drive search failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Drive API error (HTTP %d): %s", status, string(data))
	}

	var result struct {
		Files []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			MimeType     string `json:"mimeType"`
			ModifiedTime string `json:"modifiedTime"`
			Size         string `json:"size"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return gwErrJSON("Failed to parse Drive results: %v", err)
	}

	return gwWrapExternal(map[string]interface{}{
		"status": "ok",
		"count":  len(result.Files),
		"files":  result.Files,
	})
}

func (c *GWorkspaceClient) DriveGetContent(fileID string) string {
	if fileID == "" {
		return gwErrJSON("'file_id' is required")
	}

	// First get file metadata to check mime type
	metaURL := fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s?fields=id,name,mimeType", fileID)
	metaData, metaStatus, err := c.request("GET", metaURL, nil)
	if err != nil {
		return gwErrJSON("Drive get metadata failed: %v", err)
	}
	if metaStatus != 200 {
		return gwErrJSON("Drive API error (HTTP %d): %s", metaStatus, string(metaData))
	}

	var meta struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		MimeType string `json:"mimeType"`
	}
	json.Unmarshal(metaData, &meta)

	// For Google Docs/Sheets/Slides, use export; for regular files, use download
	var contentURL string
	switch {
	case strings.Contains(meta.MimeType, "google-apps.document"):
		contentURL = fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s/export?mimeType=text/plain", fileID)
	case strings.Contains(meta.MimeType, "google-apps.spreadsheet"):
		contentURL = fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s/export?mimeType=text/csv", fileID)
	case strings.Contains(meta.MimeType, "google-apps.presentation"):
		contentURL = fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s/export?mimeType=text/plain", fileID)
	default:
		contentURL = fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s?alt=media", fileID)
	}

	data, status, err := c.request("GET", contentURL, nil)
	if err != nil {
		return gwErrJSON("Drive get content failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Drive API error (HTTP %d): %s", status, string(data))
	}

	return gwWrapExternal(map[string]interface{}{
		"status":    "ok",
		"file_id":   meta.ID,
		"file_name": meta.Name,
		"mime_type": meta.MimeType,
		"content":   string(data),
	})
}

// ── Docs ─────────────────────────────────────────────────────────────────

func (c *GWorkspaceClient) DocsGet(documentID string) string {
	if documentID == "" {
		return gwErrJSON("'document_id' is required")
	}
	u := fmt.Sprintf("https://docs.googleapis.com/v1/documents/%s", documentID)

	data, status, err := c.request("GET", u, nil)
	if err != nil {
		return gwErrJSON("Docs get failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Docs API error (HTTP %d): %s", status, string(data))
	}

	// Extract text content from the document body
	var doc struct {
		Title      string `json:"title"`
		DocumentID string `json:"documentId"`
		Body       struct {
			Content []struct {
				Paragraph struct {
					Elements []struct {
						TextRun struct {
							Content string `json:"content"`
						} `json:"textRun"`
					} `json:"elements"`
				} `json:"paragraph"`
			} `json:"content"`
		} `json:"body"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return gwErrJSON("Failed to parse document: %v", err)
	}

	var text strings.Builder
	for _, block := range doc.Body.Content {
		for _, el := range block.Paragraph.Elements {
			text.WriteString(el.TextRun.Content)
		}
	}

	return gwWrapExternal(map[string]interface{}{
		"status":      "ok",
		"document_id": doc.DocumentID,
		"title":       doc.Title,
		"content":     text.String(),
	})
}

func (c *GWorkspaceClient) DocsCreate(title, body string) string {
	if title == "" {
		return gwErrJSON("'title' is required")
	}

	// Create blank document
	payload := map[string]string{"title": title}
	data, status, err := c.request("POST", "https://docs.googleapis.com/v1/documents", payload)
	if err != nil {
		return gwErrJSON("Docs create failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Docs API error (HTTP %d): %s", status, string(data))
	}

	var created struct {
		DocumentID string `json:"documentId"`
		Title      string `json:"title"`
	}
	json.Unmarshal(data, &created)

	// If body content provided, insert it
	if body != "" {
		updatePayload := map[string]interface{}{
			"requests": []map[string]interface{}{
				{
					"insertText": map[string]interface{}{
						"location": map[string]int{"index": 1},
						"text":     body,
					},
				},
			},
		}
		u := fmt.Sprintf("https://docs.googleapis.com/v1/documents/%s:batchUpdate", created.DocumentID)
		c.request("POST", u, updatePayload)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":      "ok",
		"document_id": created.DocumentID,
		"title":       created.Title,
	})
	return string(out)
}

func (c *GWorkspaceClient) DocsUpdate(documentID, body string) string {
	if documentID == "" {
		return gwErrJSON("'document_id' is required")
	}
	if body == "" {
		return gwErrJSON("'body' is required")
	}

	// Get current document length to replace all content
	metaURL := fmt.Sprintf("https://docs.googleapis.com/v1/documents/%s", documentID)
	metaData, metaStatus, err := c.request("GET", metaURL, nil)
	if err != nil || metaStatus != 200 {
		return gwErrJSON("Failed to get document metadata: %v", err)
	}

	var doc struct {
		Body struct {
			Content []struct {
				EndIndex int `json:"endIndex"`
			} `json:"content"`
		} `json:"body"`
	}
	json.Unmarshal(metaData, &doc)

	// Find the last content end index
	endIdx := 1
	for _, block := range doc.Body.Content {
		if block.EndIndex > endIdx {
			endIdx = block.EndIndex
		}
	}

	requests := []map[string]interface{}{}
	// Delete existing content (if any beyond the newline)
	if endIdx > 2 {
		requests = append(requests, map[string]interface{}{
			"deleteContentRange": map[string]interface{}{
				"range": map[string]int{
					"startIndex": 1,
					"endIndex":   endIdx - 1,
				},
			},
		})
	}
	// Insert new content
	requests = append(requests, map[string]interface{}{
		"insertText": map[string]interface{}{
			"location": map[string]int{"index": 1},
			"text":     body,
		},
	})

	updatePayload := map[string]interface{}{"requests": requests}
	u := fmt.Sprintf("https://docs.googleapis.com/v1/documents/%s:batchUpdate", documentID)
	data, status, err := c.request("POST", u, updatePayload)
	if err != nil {
		return gwErrJSON("Docs update failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Docs API error (HTTP %d): %s", status, string(data))
	}

	out, _ := json.Marshal(map[string]string{"status": "ok", "document_id": documentID})
	return string(out)
}

// ── Sheets ───────────────────────────────────────────────────────────────

func (c *GWorkspaceClient) SheetsGet(spreadsheetID, cellRange string) string {
	if spreadsheetID == "" {
		return gwErrJSON("'document_id' is required (spreadsheet ID)")
	}
	if cellRange == "" {
		cellRange = "Sheet1"
	}

	u := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s", spreadsheetID, url.PathEscape(cellRange))
	data, status, err := c.request("GET", u, nil)
	if err != nil {
		return gwErrJSON("Sheets get failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Sheets API error (HTTP %d): %s", status, string(data))
	}

	var result struct {
		Range  string          `json:"range"`
		Values [][]interface{} `json:"values"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return gwErrJSON("Failed to parse sheet data: %v", err)
	}

	return gwWrapExternal(map[string]interface{}{
		"status": "ok",
		"range":  result.Range,
		"rows":   len(result.Values),
		"values": result.Values,
	})
}

func (c *GWorkspaceClient) SheetsUpdate(spreadsheetID, cellRange string, values [][]interface{}) string {
	if spreadsheetID == "" {
		return gwErrJSON("'document_id' is required (spreadsheet ID)")
	}
	if cellRange == "" {
		return gwErrJSON("'range' is required (A1 notation)")
	}
	if len(values) == 0 {
		return gwErrJSON("'values' is required (2D array)")
	}

	payload := map[string]interface{}{
		"range":          cellRange,
		"majorDimension": "ROWS",
		"values":         values,
	}

	u := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s?valueInputOption=USER_ENTERED",
		spreadsheetID, url.PathEscape(cellRange))
	data, status, err := c.request("PUT", u, payload)
	if err != nil {
		return gwErrJSON("Sheets update failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Sheets API error (HTTP %d): %s", status, string(data))
	}

	var result struct {
		UpdatedRange string `json:"updatedRange"`
		UpdatedRows  int    `json:"updatedRows"`
		UpdatedCells int    `json:"updatedCells"`
	}
	json.Unmarshal(data, &result)
	out, _ := json.Marshal(map[string]interface{}{
		"status":        "ok",
		"updated_range": result.UpdatedRange,
		"updated_rows":  result.UpdatedRows,
		"updated_cells": result.UpdatedCells,
	})
	return string(out)
}

func (c *GWorkspaceClient) SheetsCreate(title string) string {
	if title == "" {
		return gwErrJSON("'title' is required")
	}

	payload := map[string]interface{}{
		"properties": map[string]string{"title": title},
	}

	data, status, err := c.request("POST", "https://sheets.googleapis.com/v4/spreadsheets", payload)
	if err != nil {
		return gwErrJSON("Sheets create failed: %v", err)
	}
	if status != 200 {
		return gwErrJSON("Sheets API error (HTTP %d): %s", status, string(data))
	}

	var created struct {
		SpreadsheetID  string `json:"spreadsheetId"`
		SpreadsheetURL string `json:"spreadsheetUrl"`
		Properties     struct {
			Title string `json:"title"`
		} `json:"properties"`
	}
	json.Unmarshal(data, &created)
	out, _ := json.Marshal(map[string]interface{}{
		"status":         "ok",
		"spreadsheet_id": created.SpreadsheetID,
		"url":            created.SpreadsheetURL,
		"title":          created.Properties.Title,
	})
	return string(out)
}

// ── Dispatch ─────────────────────────────────────────────────────────────

// ExecuteGoogleWorkspace is the main entry point called from agent dispatch.
// It enforces ReadOnly mode and per-scope access checks before routing to the
// appropriate API method.
func ExecuteGoogleWorkspace(cfg config.Config, vault *security.Vault, operation string, params map[string]interface{}) string {
	gw := cfg.GoogleWorkspace

	// Scope + readonly enforcement
	writeOps := map[string]bool{
		"gmail_send": true, "gmail_modify_labels": true,
		"calendar_create": true, "calendar_update": true,
		"docs_create": true, "docs_update": true,
		"sheets_update": true, "sheets_create": true,
	}
	if gw.ReadOnly && writeOps[operation] {
		return gwErrJSON("Google Workspace is in read-only mode. Operation '%s' is blocked.", operation)
	}

	// Per-scope access checks
	scopeMap := map[string]bool{
		"gmail_list":          gw.Gmail,
		"gmail_read":          gw.Gmail,
		"gmail_send":          gw.GmailSend,
		"gmail_modify_labels": gw.Gmail,
		"calendar_list":       gw.Calendar,
		"calendar_create":     gw.CalendarWrite,
		"calendar_update":     gw.CalendarWrite,
		"drive_search":        gw.Drive,
		"drive_get_content":   gw.Drive,
		"docs_get":            gw.Docs,
		"docs_create":         gw.DocsWrite,
		"docs_update":         gw.DocsWrite,
		"sheets_get":          gw.Sheets,
		"sheets_update":       gw.SheetsWrite,
		"sheets_create":       gw.SheetsWrite,
	}
	if allowed, exists := scopeMap[operation]; exists && !allowed {
		return gwErrJSON("Google Workspace scope for operation '%s' is disabled in configuration.", operation)
	}

	client, err := NewGWorkspaceClient(cfg, vault)
	if err != nil {
		return gwErrJSON("Failed to initialize Google Workspace client: %v", err)
	}

	// Helper to extract params
	str := func(key string) string {
		if v, ok := params[key].(string); ok {
			return v
		}
		return ""
	}
	intVal := func(key string) int {
		switch v := params[key].(type) {
		case float64:
			return int(v)
		case int:
			return v
		case string:
			n, _ := strconv.Atoi(v)
			return n
		}
		return 0
	}
	strSlice := func(key string) []string {
		if arr, ok := params[key].([]interface{}); ok {
			result := make([]string, 0, len(arr))
			for _, v := range arr {
				if s, ok := v.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
		return nil
	}

	switch operation {
	// Gmail
	case "gmail_list":
		return client.GmailList(str("query"), intVal("max_results"))
	case "gmail_read":
		return client.GmailRead(str("message_id"))
	case "gmail_send":
		return client.GmailSend(str("to"), str("subject"), str("body"))
	case "gmail_modify_labels":
		return client.GmailModifyLabels(str("message_id"), strSlice("add_labels"), strSlice("remove_labels"))

	// Calendar
	case "calendar_list":
		return client.CalendarList(str("query"), intVal("max_results"))
	case "calendar_create":
		return client.CalendarCreate(str("title"), str("description"), str("start_time"), str("end_time"))
	case "calendar_update":
		return client.CalendarUpdate(str("event_id"), str("title"), str("description"), str("start_time"), str("end_time"))

	// Drive
	case "drive_search":
		return client.DriveSearch(str("query"), intVal("max_results"))
	case "drive_get_content":
		return client.DriveGetContent(str("file_id"))

	// Docs
	case "docs_get":
		return client.DocsGet(str("document_id"))
	case "docs_create":
		return client.DocsCreate(str("title"), str("body"))
	case "docs_update":
		return client.DocsUpdate(str("document_id"), str("body"))

	// Sheets
	case "sheets_get":
		return client.SheetsGet(str("document_id"), str("range"))
	case "sheets_update":
		// Extract values from params
		var vals [][]interface{}
		if v, ok := params["values"].([]interface{}); ok {
			for _, row := range v {
				if r, ok := row.([]interface{}); ok {
					vals = append(vals, r)
				}
			}
		}
		return client.SheetsUpdate(str("document_id"), str("range"), vals)
	case "sheets_create":
		return client.SheetsCreate(str("title"))

	default:
		return gwErrJSON("Unknown Google Workspace operation: '%s'", operation)
	}
}
