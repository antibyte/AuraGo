package outputcompress

import (
	"fmt"
	"strings"
)

func compressPing(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	lines := strings.Split(result, "\n")
	if len(lines) <= 4 {
		return result // Short output, keep as-is
	}

	var sb strings.Builder
	var hostLine, statsLine, rttLine string
	var errorLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		// PING header line
		if strings.HasPrefix(lower, "ping ") || strings.HasPrefix(lower, "ping6 ") {
			hostLine = trimmed
			continue
		}

		// Statistics separator
		if strings.HasPrefix(trimmed, "--- ") && strings.HasSuffix(trimmed, " ---") {
			continue
		}

		// Statistics lines
		if strings.Contains(lower, "packets transmitted") ||
			strings.Contains(lower, "packet loss") {
			statsLine = trimmed
			continue
		}

		// RTT summary line
		if strings.Contains(lower, "round-trip") || strings.Contains(lower, "rtt ") {
			rttLine = trimmed
			continue
		}

		// Error / unreachable / timeout lines
		if strings.Contains(lower, "request timeout") ||
			strings.Contains(lower, "destination host unreachable") ||
			strings.Contains(lower, "100% packet loss") ||
			strings.Contains(lower, "network is unreachable") ||
			strings.Contains(lower, "name or service not known") ||
			strings.Contains(lower, "temporary failure in name resolution") ||
			strings.Contains(lower, "could not resolve") ||
			strings.Contains(lower, "unknown host") {
			errorLines = append(errorLines, trimmed)
			continue
		}
	}

	// Build compact output
	if hostLine != "" {
		sb.WriteString(hostLine + "\n")
	}
	if statsLine != "" {
		sb.WriteString(statsLine + "\n")
	}
	if rttLine != "" {
		sb.WriteString(rttLine + "\n")
	}
	for _, el := range errorLines {
		sb.WriteString(el + "\n")
	}

	compressed := sb.String()
	if compressed == "" {
		return result
	}
	return compressed
}

// compressDig compresses dig output by keeping Question, Answer,
// and Query time sections. Removes Authority and Additional sections
// unless they contain error indicators.
func compressDig(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	lines := strings.Split(result, "\n")
	if len(lines) <= 10 {
		return result // Short output, keep as-is
	}

	var sb strings.Builder
	inSection := ""
	skipping := false
	answerCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Section headers
		if strings.HasPrefix(trimmed, ";; ") {
			sectionName := strings.ToLower(trimmed)
			switch {
			case strings.Contains(sectionName, "question section"):
				inSection = "question"
				skipping = false
			case strings.Contains(sectionName, "answer section"):
				inSection = "answer"
				skipping = false
			case strings.Contains(sectionName, "authority section"):
				inSection = "authority"
				skipping = true // Skip authority by default
			case strings.Contains(sectionName, "additional section"):
				inSection = "additional"
				skipping = true // Skip additional by default
			default:
				inSection = ""
				skipping = false
			}
			sb.WriteString(trimmed + "\n")
			continue
		}

		// Skip authority/additional sections
		if skipping {
			// But keep error indicators
			lower := strings.ToLower(trimmed)
			if strings.Contains(lower, "servfail") || strings.Contains(lower, "nxdomain") ||
				strings.Contains(lower, "refused") || strings.Contains(lower, "error") ||
				strings.Contains(lower, "status:") {
				sb.WriteString(trimmed + "\n")
			}
			continue
		}

		// Count answer lines
		if inSection == "answer" && trimmed != "" && !strings.HasPrefix(trimmed, ";;") {
			answerCount++
		}

		// Always keep query time, server, and msg size
		if strings.HasPrefix(trimmed, ";;") {
			sb.WriteString(trimmed + "\n")
			continue
		}

		sb.WriteString(trimmed + "\n")
	}

	// If we had many answer lines, add a note
	if answerCount > 20 {
		sb.WriteString(fmt.Sprintf("  [%d total answer records]\n", answerCount))
	}

	compressed := sb.String()
	// Only return if actually shorter
	if len(compressed) < len(result) {
		return compressed
	}
	return result
}

// compressDNS compresses nslookup and host command output.
// These commands produce relatively compact output already,
// so we mainly deduplicate and strip verbose headers.
func compressDNS(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	lines := strings.Split(result, "\n")
	if len(lines) <= 5 {
		return result
	}

	var sb strings.Builder
	var answerLines []string
	var serverLines []string
	skippedHeaders := 0
	serverSectionDone := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Server info lines (only first section before blank line)
		if !serverSectionDone {
			if strings.HasPrefix(trimmed, "Server:") || strings.HasPrefix(trimmed, "Address:") {
				serverLines = append(serverLines, trimmed)
				continue
			}
		}

		// First blank line marks end of server section
		if trimmed == "" && len(serverLines) > 0 {
			serverSectionDone = true
			continue
		}

		// Skip non-authoritative header
		if strings.Contains(trimmed, "Non-authoritative") ||
			strings.Contains(trimmed, "authoritative") {
			skippedHeaders++
			continue
		}

		// Skip empty lines between sections
		if trimmed == "" {
			continue
		}

		answerLines = append(answerLines, trimmed)
	}

	for _, sl := range serverLines {
		sb.WriteString(sl + "\n")
	}
	for _, a := range answerLines {
		sb.WriteString(a + "\n")
	}

	compressed := sb.String()
	if compressed == "" {
		return result
	}
	return compressed
}

// compressCurl compresses curl/wget output by detecting content type
// and applying appropriate strategies.
func compressCurl(output string) string {
	result := StripANSI(output)
	result = CollapseWhitespace(result)

	// Detect output type
	trimmed := strings.TrimSpace(result)

	// JSON response
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return compressCurlJSON(result)
	}

	// HTML response
	if strings.HasPrefix(trimmed, "<!DOCTYPE") || strings.HasPrefix(trimmed, "<html") ||
		strings.HasPrefix(trimmed, "<HTML") {
		return compressCurlHTML(result)
	}

	// HTTP headers + body (verbose mode)
	if strings.Contains(trimmed, "HTTP/") {
		return compressCurlVerbose(result)
	}

	// For everything else, apply generic compression
	return compressGeneric(result)
}

// compressCurlJSON compresses JSON curl output.
func compressCurlJSON(output string) string {
	result := compactJSON(output)
	result = DeduplicateLines(result)

	lines := strings.Split(result, "\n")
	if len(lines) > 100 {
		result = TailFocus(result, 20, 50, 5)
	}
	return result
}

// compressCurlHTML compresses HTML curl output.
func compressCurlHTML(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= 20 {
		return output
	}

	// For HTML, keep first few lines (doctype, title) and last lines
	var sb strings.Builder
	titleFound := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		// Keep title
		if strings.Contains(lower, "<title>") {
			sb.WriteString(trimmed + "\n")
			titleFound = true
			continue
		}

		// Keep error indicators
		if strings.Contains(lower, "error") || strings.Contains(lower, "404") ||
			strings.Contains(lower, "403") || strings.Contains(lower, "500") ||
			strings.Contains(lower, "denied") || strings.Contains(lower, "forbidden") {
			sb.WriteString(trimmed + "\n")
		}
	}

	if !titleFound && sb.Len() == 0 {
		return TailFocus(output, 5, 10, 5)
	}

	sb.WriteString(fmt.Sprintf("  [%d-line HTML response]\n", len(lines)))
	return sb.String()
}

// compressCurlVerbose compresses verbose curl output (with headers).
func compressCurlVerbose(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= 15 {
		return output
	}

	var sb strings.Builder
	var requestHeaders, responseHeaders []string
	var bodyLines []string
	var statusCode string
	inRequest := false
	inResponse := false
	inBody := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect request/response sections
		if strings.HasPrefix(trimmed, "> ") {
			if strings.Contains(trimmed, "HTTP/") {
				// This is actually a response status in some curl versions
			}
			inRequest = true
			inResponse = false
			inBody = false
			requestHeaders = append(requestHeaders, trimmed)
			continue
		}
		if strings.HasPrefix(trimmed, "< ") {
			inRequest = false
			inResponse = true
			inBody = false
			if strings.Contains(trimmed, "HTTP/") {
				statusCode = trimmed
			}
			responseHeaders = append(responseHeaders, trimmed)
			continue
		}

		// Blank line after response headers = body start
		if trimmed == "" && inResponse {
			inResponse = false
			inBody = true
			continue
		}

		if inBody || (!inRequest && !inResponse && trimmed != "") {
			bodyLines = append(bodyLines, trimmed)
		}
	}

	// Build compact output
	if statusCode != "" {
		sb.WriteString(statusCode + "\n")
	}

	// Keep important response headers (content-type, location, etc.)
	for _, h := range responseHeaders {
		lower := strings.ToLower(h)
		if strings.Contains(lower, "content-type:") ||
			strings.Contains(lower, "location:") ||
			strings.Contains(lower, "content-length:") ||
			strings.Contains(lower, "server:") ||
			strings.Contains(lower, "set-cookie:") {
			sb.WriteString(h + "\n")
		}
	}

	// Body
	if len(bodyLines) > 0 {
		sb.WriteString("\n")
		bodyText := strings.Join(bodyLines, "\n")
		// If body is JSON, compact it
		if strings.HasPrefix(strings.TrimSpace(bodyText), "{") ||
			strings.HasPrefix(strings.TrimSpace(bodyText), "[") {
			bodyText = compactJSON(bodyText)
		}
		bodyLines2 := strings.Split(bodyText, "\n")
		if len(bodyLines2) > 50 {
			sb.WriteString(TailFocus(bodyText, 10, 20, 5))
		} else {
			sb.WriteString(bodyText)
		}
	}

	compressed := sb.String()
	if compressed == "" {
		return output
	}
	return compressed
}

// ─── V7: File / Log viewing compressors ──────────────────────────────────────

// isLogContent detects if output looks like log data by checking for
// timestamp patterns or log level markers.
