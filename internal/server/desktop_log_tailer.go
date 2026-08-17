package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	desktopLogMaxLineBytes   = 256 * 1024
	desktopLogTailPoll       = 200 * time.Millisecond
	desktopLogHeartbeatEvery = 20 * time.Second
	desktopLogMaxTailLines   = 2000
	desktopLogDefaultTail    = 500
	desktopLogSearchDefault  = 200
	desktopLogSearchMax      = 500
	desktopLogSearchContext  = 1
	desktopLogEstimateBytes  = 80
	desktopLogCountScanLimit = 8 << 20
)

// desktopLogRecord is a parsed (or best-effort) log line for the desktop viewer.
type desktopLogRecord struct {
	LineNo    int               `json:"line_no"`
	Time      string            `json:"time,omitempty"`
	Level     string            `json:"level,omitempty"`
	Msg       string            `json:"msg,omitempty"`
	Attrs     map[string]string `json:"attrs,omitempty"`
	Raw       string            `json:"raw"`
	File      string            `json:"file"`
	Offset    int64             `json:"offset,omitempty"`
	Truncated bool              `json:"truncated,omitempty"`
}

type desktopLogFileInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MTime    string `json:"mtime"`
	LinesEst int64  `json:"lines_est"`
}

type desktopLogTailResult struct {
	Records    []desktopLogRecord
	NewOffset  int64
	TotalLines int
	Truncated  bool
}

func parseSlogLine(raw string) (desktopLogRecord, bool) {
	rec := desktopLogRecord{Raw: raw, Attrs: map[string]string{}}
	if raw == "" {
		return rec, false
	}
	ok := false
	for i := 0; i < len(raw); {
		for i < len(raw) && (raw[i] == ' ' || raw[i] == '\t') {
			i++
		}
		if i >= len(raw) {
			break
		}
		eq := strings.IndexByte(raw[i:], '=')
		if eq < 1 {
			if rec.Msg == "" {
				rec.Msg = strings.TrimSpace(raw[i:])
			}
			break
		}
		key := raw[i : i+eq]
		i += eq + 1
		if key == "" {
			continue
		}
		var value string
		if i < len(raw) && raw[i] == '"' {
			value, i = parseSlogQuoted(raw, i+1)
		} else {
			end := i
			for end < len(raw) && raw[end] != ' ' && raw[end] != '\t' {
				end++
			}
			value = raw[i:end]
			i = end
		}
		switch key {
		case "time":
			rec.Time = value
			ok = true
		case "level":
			rec.Level = strings.ToUpper(strings.TrimSpace(value))
			ok = true
		case "msg":
			rec.Msg = value
			ok = true
		default:
			rec.Attrs[key] = value
		}
	}
	if len(rec.Attrs) == 0 {
		rec.Attrs = nil
	}
	return rec, ok
}

func parseSlogQuoted(raw string, start int) (string, int) {
	var b strings.Builder
	i := start
	for i < len(raw) {
		ch := raw[i]
		if ch == '"' {
			return b.String(), i + 1
		}
		if ch == '\\' && i+1 < len(raw) {
			i++
			switch raw[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte(raw[i])
			}
			i++
			continue
		}
		b.WriteByte(ch)
		i++
	}
	return b.String(), i
}

func sanitizeDesktopLogName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid file name")
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid file name")
	}
	if filepath.Base(name) != name {
		return "", fmt.Errorf("invalid file name")
	}
	return name, nil
}

func desktopLogDir(s *Server) string {
	if s == nil || s.Cfg == nil {
		return "./log"
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	if strings.TrimSpace(s.Cfg.Logging.LogDir) == "" {
		return "./log"
	}
	return s.Cfg.Logging.LogDir
}

func desktopVirtualDesktopReadOnly(s *Server) bool {
	if s == nil || s.Cfg == nil {
		return false
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return s.Cfg.VirtualDesktop.ReadOnly
}

func listDesktopLogFiles(logDir string) ([]desktopLogFileInfo, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []desktopLogFileInfo{}, nil
		}
		return nil, fmt.Errorf("read log dir: %w", err)
	}
	files := make([]desktopLogFileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, err := sanitizeDesktopLogName(name); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		files = append(files, desktopLogFileInfo{
			Name:     name,
			Size:     info.Size(),
			MTime:    info.ModTime().UTC().Format(time.RFC3339),
			LinesEst: estimateDesktopLogLines(filepath.Join(logDir, name), info.Size()),
		})
	}
	return files, nil
}

func estimateDesktopLogLines(path string, size int64) int64 {
	if size <= 0 {
		return 0
	}
	if size > desktopLogCountScanLimit {
		est := size / desktopLogEstimateBytes
		if est < 1 {
			return 1
		}
		return est
	}
	file, err := os.Open(path)
	if err != nil {
		est := size / desktopLogEstimateBytes
		if est < 1 {
			return 1
		}
		return est
	}
	defer file.Close()
	var count int64
	buf := make([]byte, 32*1024)
	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			count += int64(bytes.Count(buf[:n], []byte{'\n'}))
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			break
		}
	}
	if count == 0 && size > 0 {
		return 1
	}
	return count
}

func resolveDesktopLogFile(logDir, name string) (string, error) {
	clean, err := sanitizeDesktopLogName(name)
	if err != nil {
		return "", err
	}
	files, err := listDesktopLogFiles(logDir)
	if err != nil {
		return "", err
	}
	for _, file := range files {
		if file.Name == clean {
			return filepath.Join(logDir, clean), nil
		}
	}
	return "", fmt.Errorf("unknown log file")
}

func tailDesktopLog(path, fileName string, lines int, offset int64) (desktopLogTailResult, error) {
	if lines <= 0 {
		lines = desktopLogDefaultTail
	}
	if lines > desktopLogMaxTailLines {
		lines = desktopLogMaxTailLines
	}
	file, err := os.Open(path)
	if err != nil {
		return desktopLogTailResult{}, fmt.Errorf("open log: %w", err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return desktopLogTailResult{}, fmt.Errorf("stat log: %w", err)
	}
	size := stat.Size()
	result := desktopLogTailResult{}
	start := int64(0)
	if offset < 0 {
		start, err = findDesktopLogTailStart(file, size, lines)
		if err != nil {
			return desktopLogTailResult{}, err
		}
	} else {
		if offset > size {
			result.Truncated = true
			result.NewOffset = size
			result.TotalLines = int(estimateDesktopLogLines(path, size))
			return result, nil
		}
		start = offset
		if start > 0 {
			aligned, alignErr := alignDesktopLogOffset(file, start)
			if alignErr != nil {
				return desktopLogTailResult{}, alignErr
			}
			start = aligned
		}
	}
	lineNo, err := countDesktopLogNewlines(file, start)
	if err != nil {
		return desktopLogTailResult{}, err
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return desktopLogTailResult{}, fmt.Errorf("seek log: %w", err)
	}
	counter := &countingReader{r: file}
	scanner := bufio.NewScanner(counter)
	scanner.Buffer(make([]byte, 0, 64*1024), desktopLogMaxLineBytes)
	records := make([]desktopLogRecord, 0, lines)
	for len(records) < lines && scanner.Scan() {
		raw := scanner.Text()
		rec, _ := parseSlogLine(raw)
		lineNo++
		rec.LineNo = lineNo
		rec.File = fileName
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		if err == bufio.ErrTooLong {
			lineNo++
			raw := strings.ToValidUTF8(string(scanner.Bytes()), "\uFFFD")
			if len(raw) > desktopLogMaxLineBytes {
				raw = raw[:desktopLogMaxLineBytes]
			}
			rec, _ := parseSlogLine(raw)
			rec.LineNo = lineNo
			rec.File = fileName
			rec.Truncated = true
			records = append(records, rec)
		} else {
			return desktopLogTailResult{}, fmt.Errorf("read log: %w", err)
		}
	}
	result.Records = records
	result.NewOffset = start + counter.n
	if result.NewOffset > size {
		result.NewOffset = size
	}
	if offset < 0 {
		result.TotalLines = lineNo
	} else {
		result.TotalLines = int(estimateDesktopLogLines(path, size))
	}
	return result, nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func alignDesktopLogOffset(file *os.File, offset int64) (int64, error) {
	if offset <= 0 {
		return 0, nil
	}
	if _, err := file.Seek(offset-1, io.SeekStart); err != nil {
		return 0, fmt.Errorf("seek log: %w", err)
	}
	var prev [1]byte
	if _, err := io.ReadFull(file, prev[:]); err != nil {
		return 0, fmt.Errorf("align log: %w", err)
	}
	if prev[0] == '\n' {
		return offset, nil
	}
	buf := make([]byte, 4096)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			if idx := bytes.IndexByte(buf[:n], '\n'); idx >= 0 {
				pos, seekErr := file.Seek(0, io.SeekCurrent)
				if seekErr != nil {
					return 0, fmt.Errorf("seek log: %w", seekErr)
				}
				return pos - int64(n) + int64(idx) + 1, nil
			}
		}
		if err == io.EOF {
			pos, seekErr := file.Seek(0, io.SeekCurrent)
			if seekErr != nil {
				return 0, fmt.Errorf("seek log: %w", seekErr)
			}
			return pos, nil
		}
		if err != nil {
			return 0, fmt.Errorf("align log: %w", err)
		}
	}
}

func findDesktopLogTailStart(file *os.File, size int64, lines int) (int64, error) {
	if size <= 0 || lines <= 0 {
		return 0, nil
	}
	const chunk = 32 * 1024
	remaining := lines
	pos := size
	// Ignore a trailing newline so the last line is counted once.
	if size > 0 {
		if _, err := file.Seek(size-1, io.SeekStart); err != nil {
			return 0, fmt.Errorf("seek log: %w", err)
		}
		var last [1]byte
		if _, err := io.ReadFull(file, last[:]); err == nil && last[0] == '\n' {
			pos = size - 1
		}
	}
	buf := make([]byte, chunk)
	for pos > 0 && remaining > 0 {
		readSize := int64(chunk)
		if readSize > pos {
			readSize = pos
		}
		pos -= readSize
		if _, err := file.Seek(pos, io.SeekStart); err != nil {
			return 0, fmt.Errorf("seek log: %w", err)
		}
		n, err := io.ReadFull(file, buf[:readSize])
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return 0, fmt.Errorf("read log: %w", err)
		}
		for i := n - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				remaining--
				if remaining == 0 {
					return pos + int64(i) + 1, nil
				}
			}
		}
	}
	return 0, nil
}

func countDesktopLogNewlines(file *os.File, until int64) (int, error) {
	if until <= 0 {
		return 0, nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("seek log: %w", err)
	}
	var count int
	buf := make([]byte, 32*1024)
	var read int64
	for read < until {
		want := len(buf)
		if int64(want) > until-read {
			want = int(until - read)
		}
		n, err := file.Read(buf[:want])
		if n > 0 {
			count += bytes.Count(buf[:n], []byte{'\n'})
			read += int64(n)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("read log: %w", err)
		}
	}
	return count, nil
}

func streamDesktopLog(ctx context.Context, path, fileName string, since int64, emit func(evt string, payload interface{}) error) error {
	if since < 0 {
		since = 0
	}
	var carry []byte
	ticker := time.NewTicker(desktopLogTailPoll)
	defer ticker.Stop()
	heartbeat := time.NewTicker(desktopLogHeartbeatEvery)
	defer heartbeat.Stop()

	readNew := func() error {
		stat, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return emit("log_error", map[string]interface{}{"file": fileName, "error": "log file not available"})
			}
			return emit("log_error", map[string]interface{}{"file": fileName, "error": err.Error()})
		}
		size := stat.Size()
		if size < since {
			since = 0
			carry = nil
			if err := emit("log_truncated", map[string]interface{}{"file": fileName, "offset": int64(0)}); err != nil {
				return err
			}
		}
		if size <= since && len(carry) == 0 {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return emit("log_error", map[string]interface{}{"file": fileName, "error": err.Error()})
		}
		defer file.Close()
		if _, err := file.Seek(since, io.SeekStart); err != nil {
			return emit("log_error", map[string]interface{}{"file": fileName, "error": err.Error()})
		}
		lineNo, err := countDesktopLogNewlines(file, since)
		if err != nil {
			return emit("log_error", map[string]interface{}{"file": fileName, "error": err.Error()})
		}
		if _, err := file.Seek(since, io.SeekStart); err != nil {
			return emit("log_error", map[string]interface{}{"file": fileName, "error": err.Error()})
		}
		data, err := io.ReadAll(io.LimitReader(file, size-since+1))
		if err != nil {
			return emit("log_error", map[string]interface{}{"file": fileName, "error": err.Error()})
		}
		buf := append(carry, data...)
		carry = nil
		for len(buf) > 0 {
			idx := bytes.IndexByte(buf, '\n')
			if idx < 0 {
				if len(buf) > desktopLogMaxLineBytes {
					raw := strings.ToValidUTF8(string(buf[:desktopLogMaxLineBytes]), "\uFFFD")
					rec, _ := parseSlogLine(raw)
					rec.LineNo = lineNo + 1
					rec.File = fileName
					rec.Offset = size
					rec.Truncated = true
					if err := emit("log_line", rec); err != nil {
						return err
					}
					buf = nil
					lineNo++
					break
				}
				carry = append([]byte(nil), buf...)
				break
			}
			raw := strings.TrimRight(string(buf[:idx]), "\r")
			buf = buf[idx+1:]
			rec, _ := parseSlogLine(raw)
			lineNo++
			rec.LineNo = lineNo
			rec.File = fileName
			rec.Offset = size
			if err := emit("log_line", rec); err != nil {
				return err
			}
		}
		since = size
		return nil
	}

	if err := readNew(); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-heartbeat.C:
			if err := emit("log_heartbeat", map[string]interface{}{"file": fileName, "offset": since}); err != nil {
				return err
			}
		case <-ticker.C:
			if err := readNew(); err != nil {
				return err
			}
		}
	}
}

func searchDesktopLog(path, fileName, query string, levels map[string]struct{}, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = desktopLogSearchDefault
	}
	if limit > desktopLogSearchMax {
		limit = desktopLogSearchMax
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), desktopLogMaxLineBytes)
	var window []desktopLogRecord
	matches := make([]map[string]interface{}, 0)
	lineNo := 0
	queryLower := strings.ToLower(query)
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		rec, _ := parseSlogLine(raw)
		rec.LineNo = lineNo
		rec.File = fileName
		window = append(window, rec)
		if len(window) > desktopLogSearchContext*2+1 {
			window = window[1:]
		}
		if !desktopLogRecordMatches(rec, queryLower, levels) {
			continue
		}
		before := []desktopLogRecord{}
		after := []desktopLogRecord{}
		// Context after the match is filled on subsequent iterations by
		// rewriting the last match; seed before now.
		idx := len(window) - 1
		if idx > 0 {
			start := idx - desktopLogSearchContext
			if start < 0 {
				start = 0
			}
			before = append(before, window[start:idx]...)
		}
		matches = append(matches, map[string]interface{}{
			"line_no": rec.LineNo,
			"record":  rec,
			"before":  before,
			"after":   after,
		})
		if len(matches) >= limit {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan log: %w", err)
	}
	// Fill after-context from a second lightweight pass only for collected matches.
	if len(matches) > 0 {
		wanted := map[int]int{}
		for i, match := range matches {
			if line, ok := match["line_no"].(int); ok {
				wanted[line] = i
			}
		}
		if err := fillDesktopLogSearchAfter(path, fileName, wanted, matches); err != nil {
			return nil, err
		}
	}
	return matches, nil
}

func fillDesktopLogSearchAfter(path, fileName string, wanted map[int]int, matches []map[string]interface{}) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), desktopLogMaxLineBytes)
	lineNo := 0
	pending := map[int]int{}
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		if idx, ok := wanted[lineNo-1]; ok && lineNo-1 > 0 {
			pending[idx] = desktopLogSearchContext
		}
		for idx, left := range pending {
			if left <= 0 {
				continue
			}
			rec, _ := parseSlogLine(raw)
			rec.LineNo = lineNo
			rec.File = fileName
			after, _ := matches[idx]["after"].([]desktopLogRecord)
			matches[idx]["after"] = append(after, rec)
			pending[idx] = left - 1
			if pending[idx] <= 0 {
				delete(pending, idx)
			}
		}
	}
	return scanner.Err()
}

func desktopLogRecordMatches(rec desktopLogRecord, queryLower string, levels map[string]struct{}) bool {
	if len(levels) > 0 {
		level := strings.ToUpper(strings.TrimSpace(rec.Level))
		if level == "" {
			level = "INFO"
		}
		if _, ok := levels[level]; !ok {
			return false
		}
	}
	if queryLower == "" {
		return true
	}
	if strings.Contains(strings.ToLower(rec.Raw), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(rec.Msg), queryLower) {
		return true
	}
	for key, value := range rec.Attrs {
		if strings.Contains(strings.ToLower(key), queryLower) || strings.Contains(strings.ToLower(value), queryLower) {
			return true
		}
	}
	return false
}
