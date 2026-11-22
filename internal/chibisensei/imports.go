package chibisensei

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Workspace is the desktop workspace as the domain sees it; internal/server implements it over desktop.Service.
// The package never opens a user path itself.
type Workspace interface {
	ReadFile(ctx context.Context, path string, maxBytes int64) ([]byte, error)                     // ErrFileTooLarge over maxBytes
	WriteFile(ctx context.Context, path string, r io.Reader, maxBytes int64, overwrite bool) error // ErrFileExists
}

var ErrFileExists = errors.New("chibisensei: file exists")
var ErrFileTooLarge = errors.New("chibisensei: file too large")

// Mapping tells the import how to read a file.
type Mapping struct {
	Shape      string `json:"shape"`
	System     string `json:"system"`
	User       string `json:"user"`
	Assistant  string `json:"assistant"`
	Text       string `json:"text"`
	Label      string `json:"label"`
	SystemText string `json:"system_text"`
}

// ImportPreview is the answer of the preview route.
type ImportPreview struct {
	Format  string     `json:"format"`
	Columns []string   `json:"columns"`
	Mapping Mapping    `json:"mapping"`
	Sample  []RowInput `json:"sample"`
	Issues  []Issue    `json:"issues"`
}

// ImportRow is one parsed row with its 1-based line (paragraph number for text files).
type ImportRow struct {
	Line  int
	Input RowInput
}

// ImportParams are the parameters of the import_file job.
type ImportParams struct {
	WorkspacePath string   `json:"workspace_path"`
	Format        string   `json:"format"`
	Mapping       *Mapping `json:"mapping"`
}

const (
	importChunk         = 500
	importParagraphMax  = 2000
	importPreviewRows   = 5
	importPreviewLines  = 200
	importPreviewIssues = 20
)

func importIssue(field, key, msg string) *ValidationError {
	return &ValidationError{Issues: []Issue{issue(field, key, msg)}}
}

// DetectFormat keeps a requested jsonl|csv|text (text needs a .txt/.md/.markdown extension); auto picks by extension.
func DetectFormat(wsPath, requested string) (string, error) {
	ext := strings.ToLower(path.Ext(wsPath))
	switch requested {
	case "jsonl", "csv":
		return requested, nil
	case "text":
		if hasTextExtension(wsPath) {
			return "text", nil
		}
	case "", "auto":
		switch ext {
		case ".jsonl", ".json", ".ndjson":
			return "jsonl", nil
		case ".csv", ".tsv":
			return "csv", nil
		}
	}
	return "", importIssue("format", "import.unsupported_file", "This file type is not supported.")
}

// DetectJSONLShape inspects the first object of a JSONL file; "" means unknown.
func DetectJSONLShape(obj map[string]json.RawMessage) string {
	has := func(k string) bool { _, ok := obj[k]; return ok }
	switch {
	case has("messages"):
		return "messages"
	case has("conversations"):
		return "conversations"
	case has("instruction") && has("output"):
		return "instruction"
	case has("prompt") && has("completion"):
		return "prompt_completion"
	case has("text") && has("label"):
		return "text_label"
	case has("text"):
		return "text"
	}
	return ""
}

// AutoMapCSV matches lower-cased trimmed headers to the roles of a row.
func AutoMapCSV(header []string) Mapping {
	names := make([]string, len(header))
	have := map[string]bool{}
	for i, h := range header {
		names[i] = strings.ToLower(strings.TrimSpace(h))
		have[names[i]] = true
	}
	pick := func(cands ...string) string {
		for _, c := range cands {
			if have[c] {
				return c
			}
		}
		return ""
	}
	m := Mapping{
		User:      pick("question", "prompt", "input", "instruction", "user", "query"),
		Assistant: pick("answer", "response", "output", "completion", "assistant", "reply"),
		System:    pick("system", "system_prompt"),
		Text:      pick("text", "content", "sentence", "document"),
		Label:     pick("label", "category", "class", "target"),
	}
	switch {
	case m.User != "" && m.Assistant != "":
		m.Shape = "columns"
	case m.Text != "" && m.Label != "":
		m.Shape = "text_label"
	case m.Text != "":
		m.Shape = "text"
	}
	return m
}

// ImportStats counts what ParseImport saw. Issues keeps at most 200 (20 in a preview); Truncated reports dropped ones.
type ImportStats struct {
	Valid, Invalid int
	Truncated      bool
	Issues         []Issue
}

type parseOpts struct{ limit, maxLine, maxIssues int }

// ParseImport streams rows to emit until it returns false or limit valid rows were emitted (limit <= 0: no limit).
func ParseImport(data []byte, format string, m Mapping, recipe string, limit int, emit func(ImportRow) bool) (ImportStats, Mapping, []string, error) {
	return parseImport(data, format, m, recipe, parseOpts{limit: limit, maxIssues: maxResultIssues}, emit)
}

// PreviewImport parses the first rows for the preview; the issues come from the first 200 lines (at most 20 kept).
// A mapping problem is reported as an issue with the detected columns, not as an error.
func PreviewImport(data []byte, format string, m *Mapping, recipe string) (ImportPreview, error) {
	var mm Mapping
	if m != nil {
		mm = *m
	}
	sample := []RowInput{}
	stats, rm, cols, err := parseImport(data, format, mm, recipe,
		parseOpts{maxLine: importPreviewLines, maxIssues: importPreviewIssues}, func(r ImportRow) bool {
			if len(sample) < importPreviewRows {
				sample = append(sample, r.Input)
			}
			return true
		})
	pv := ImportPreview{Format: format, Columns: cols, Mapping: rm, Sample: sample, Issues: stats.Issues}
	if pv.Columns == nil {
		pv.Columns = []string{}
	}
	if pv.Issues == nil {
		pv.Issues = []Issue{}
	}
	if err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			pv.Issues = append(pv.Issues, ve.Issues...)
			return pv, nil
		}
		return ImportPreview{}, err
	}
	return pv, nil
}

type importState struct {
	opts   parseOpts
	recipe string
	sys    string
	stats  ImportStats
	emit   func(ImportRow) bool
	stop   bool
}

func (s *importState) problem(line int, key, msg string) {
	s.stats.Invalid++
	if len(s.stats.Issues) >= s.opts.maxIssues {
		s.stats.Truncated = true
		return
	}
	is := issue("", key, msg)
	is.Line = line
	s.stats.Issues = append(s.stats.Issues, is)
}

// row validates one built row and hands it to emit.
func (s *importState) row(line int, in RowInput) {
	if len(in.Messages) > 0 && strings.TrimSpace(s.sys) != "" && in.Messages[0].Role != "system" {
		in.Messages = append([]Message{{Role: "system", Content: s.sys}}, in.Messages...)
	}
	if is := ValidateRowInput(in); len(is) > 0 {
		s.problem(line, "import.invalid_row", is[0].Message)
		return
	}
	kind := rowKindChat
	if in.Text != nil {
		kind = rowKindText
	}
	if probs := StructureProblems(kind, in.Messages, s.recipe); len(probs) > 0 {
		s.problem(line, "import.invalid_row", probs[0])
		return
	}
	s.stats.Valid++
	if !s.emit(ImportRow{Line: line, Input: in}) || (s.opts.limit > 0 && s.stats.Valid >= s.opts.limit) {
		s.stop = true
	}
}

func nz(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

// fillMapping sets the default names of a shape.
func fillMapping(m Mapping) Mapping {
	switch m.Shape {
	case "prompt_completion":
		m.User, m.Assistant = nz(m.User, "prompt"), nz(m.Assistant, "completion")
	case "instruction":
		m.User, m.Assistant, m.System = nz(m.User, "instruction"), nz(m.Assistant, "output"), nz(m.System, "system")
	case "text_label":
		m.Text, m.Label = nz(m.Text, "text"), nz(m.Label, "label")
	case "text":
		m.Text = nz(m.Text, "text")
	}
	return m
}

func validShape(sh string, csvFile bool) bool {
	switch sh {
	case "prompt_completion", "instruction", "text_label", "text", "columns":
		return true
	case "messages", "conversations":
		return !csvFile
	}
	return false
}

// fromFields builds a row of a flat shape; key != "" reports a problem.
func (s *importState) fromFields(m Mapping, get func(string) (string, bool)) (RowInput, string, string) {
	free := s.recipe == RecipeFree
	switch m.Shape {
	case "text":
		t, _ := get(m.Text)
		if !free {
			return RowInput{}, "import.text_needs_free", "Text rows are only allowed in the free recipe."
		}
		return RowInput{Text: &t}, "", ""
	case "text_label":
		t, _ := get(m.Text)
		if free {
			return RowInput{Text: &t}, "", ""
		}
		l, _ := get(m.Label)
		return RowInput{Messages: []Message{{Role: "user", Content: t}, {Role: "assistant", Content: l}}}, "", ""
	}
	user, _ := get(m.User)
	asst, _ := get(m.Assistant)
	if m.Shape == "instruction" {
		if in, _ := get("input"); strings.TrimSpace(in) != "" {
			user += "\n\n" + in
		}
	}
	if strings.TrimSpace(user) == "" {
		return RowInput{}, "import.invalid_row", "The question is empty."
	}
	var msgs []Message
	if m.System != "" {
		if sys, _ := get(m.System); strings.TrimSpace(sys) != "" {
			msgs = append(msgs, Message{Role: "system", Content: sys})
		}
	}
	msgs = append(msgs, Message{Role: "user", Content: user}, Message{Role: "assistant", Content: asst})
	return RowInput{Messages: msgs}, "", ""
}

func rawToString(raw json.RawMessage) string {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || string(t) == "null" {
		return ""
	}
	if t[0] == '"' {
		var s string
		if json.Unmarshal(t, &s) == nil {
			return s
		}
	}
	if c := t[0]; c == '-' || (c >= '0' && c <= '9') {
		if i, err := strconv.ParseInt(string(t), 10, 64); err == nil {
			return strconv.FormatInt(i, 10)
		}
		if f, err := strconv.ParseFloat(string(t), 64); err == nil {
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
	}
	return string(t)
}

func rawText(raw json.RawMessage) (string, bool) {
	t := bytes.TrimSpace(raw)
	if len(t) == 0 || string(t) == "null" {
		return "", true
	}
	if t[0] != '"' {
		return "", false
	}
	var s string
	err := json.Unmarshal(t, &s)
	return s, err == nil
}

func normRole(role string, conv bool) string {
	r := strings.ToLower(strings.TrimSpace(role))
	if conv {
		switch r {
		case "system":
			return "system"
		case "human", "user":
			return "user"
		case "gpt", "assistant", "bot", "model", "chatgpt":
			return "assistant"
		}
		return ""
	}
	switch r {
	case "system", "user", "assistant":
		return r
	}
	return ""
}

// fromChat builds a row of the messages or conversations shape.
func fromChat(obj map[string]json.RawMessage, conv bool) (RowInput, string, string) {
	field, roleKey, textKey := "messages", "role", "content"
	if conv {
		field, roleKey, textKey = "conversations", "from", "value"
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(obj[field], &items); err != nil {
		return RowInput{}, "import.bad_json", "The " + field + " field is not a list of messages."
	}
	msgs := make([]Message, 0, len(items))
	for _, it := range items {
		role := normRole(rawToString(it[roleKey]), conv)
		if role == "" {
			return RowInput{}, "import.bad_role", "A message has an unknown role."
		}
		content, ok := rawText(it[textKey])
		if !ok {
			return RowInput{}, "import.bad_json", "A message has no text content."
		}
		msgs = append(msgs, Message{Role: role, Content: content})
	}
	return RowInput{Messages: msgs}, "", ""
}

func parseImport(data []byte, format string, m Mapping, recipe string, opts parseOpts, emit func(ImportRow) bool) (ImportStats, Mapping, []string, error) {
	if opts.maxIssues <= 0 {
		opts.maxIssues = maxResultIssues
	}
	st := &importState{opts: opts, recipe: recipe, sys: m.SystemText, emit: emit}
	data = bytes.TrimPrefix(data, utf8BOM)
	var cols []string
	var err error
	switch format {
	case "jsonl":
		m, cols, err = st.parseJSONL(data, m)
	case "csv":
		m, cols, err = st.parseCSV(data, m)
	case "text":
		if recipe != RecipeFree {
			return st.stats, m, []string{}, importIssue("format", "import.text_needs_free", "Text files can only be imported into the free recipe.")
		}
		m = Mapping{Shape: "text", SystemText: m.SystemText}
		st.parseText(data)
	default:
		return st.stats, m, []string{}, importIssue("format", "import.unsupported_file", "This file type is not supported.")
	}
	if cols == nil {
		cols = []string{}
	}
	return st.stats, m, cols, err
}

func (s *importState) parseJSONL(data []byte, m Mapping) (Mapping, []string, error) {
	if m.Shape != "" && !validShape(m.Shape, false) {
		return m, nil, importIssue("mapping.shape", "import.bad_mapping", "Unknown mapping shape.")
	}
	var cols []string
	first := true
	line, pos := 0, 0
	for pos < len(data) && !s.stop {
		line++
		if s.opts.maxLine > 0 && line > s.opts.maxLine {
			break
		}
		var raw []byte
		if i := bytes.IndexByte(data[pos:], '\n'); i < 0 {
			raw, pos = data[pos:], len(data)
		} else {
			raw, pos = data[pos:pos+i], pos+i+1
		}
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 {
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
			s.problem(line, "import.bad_json", "The line is not a valid JSON object.")
			continue
		}
		if first {
			first = false
			for k := range obj {
				cols = append(cols, k)
			}
			sort.Strings(cols)
			if m.Shape == "" {
				m.Shape = DetectJSONLShape(obj)
			}
			m = fillMapping(m)
		}
		var in RowInput
		var key, msg string
		switch m.Shape {
		case "":
			key, msg = "import.unknown_shape", "The line has an unknown structure."
		case "messages":
			in, key, msg = fromChat(obj, false)
		case "conversations":
			in, key, msg = fromChat(obj, true)
		default:
			in, key, msg = s.fromFields(m, func(k string) (string, bool) {
				raw, ok := obj[k]
				return rawToString(raw), ok
			})
		}
		if key != "" {
			s.problem(line, key, msg)
			continue
		}
		s.row(line, in)
	}
	return m, cols, nil
}

func detectDelimiter(data []byte) rune {
	first := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		first = data[:i]
	}
	best, bestN := ',', bytes.Count(first, []byte(","))
	for _, d := range []rune{';', '\t'} {
		if n := bytes.Count(first, []byte(string(d))); n > bestN {
			best, bestN = d, n
		}
	}
	return best
}

func (s *importState) parseCSV(data []byte, m Mapping) (Mapping, []string, error) {
	if m.Shape != "" && !validShape(m.Shape, true) {
		return m, nil, importIssue("mapping.shape", "import.bad_mapping", "Unknown mapping shape.")
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = detectDelimiter(data)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return m, nil, importIssue("file", "import.empty", "The file is empty.")
	}
	if err != nil {
		return m, nil, importIssue("file", "import.bad_csv", "The file is not valid CSV.")
	}
	cols := make([]string, len(header))
	idx := map[string]int{}
	for i, h := range header {
		cols[i] = strings.ToLower(strings.TrimSpace(h))
		if _, dup := idx[cols[i]]; !dup {
			idx[cols[i]] = i
		}
	}
	if m.Shape == "" {
		keep := m.SystemText
		m = AutoMapCSV(header)
		m.SystemText = keep
	}
	if m.Shape == "" {
		return m, cols, importIssue("mapping", "import.no_mapping", "The columns could not be matched; choose a mapping.")
	}
	m = fillMapping(m)
	// the referenced columns must exist; a system column is optional
	type need struct{ field, name string }
	var req []need
	switch m.Shape {
	case "columns", "prompt_completion", "instruction":
		req = []need{{"mapping.user", m.User}, {"mapping.assistant", m.Assistant}}
	case "text":
		req = []need{{"mapping.text", m.Text}}
	case "text_label":
		req = []need{{"mapping.text", m.Text}, {"mapping.label", m.Label}}
	}
	for _, c := range req {
		if _, ok := idx[strings.ToLower(strings.TrimSpace(c.name))]; !ok || c.name == "" {
			return m, cols, importIssue(c.field, "import.unknown_column", "The mapped column does not exist.")
		}
	}
	for !s.stop {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var pe *csv.ParseError
			if errors.As(err, &pe) {
				if s.opts.maxLine > 0 && pe.StartLine > s.opts.maxLine {
					break
				}
				s.problem(pe.StartLine, "import.bad_csv", "The line is not valid CSV.")
				continue
			}
			return m, cols, err
		}
		line, _ := r.FieldPos(0)
		if s.opts.maxLine > 0 && line > s.opts.maxLine {
			break
		}
		in, key, msg := s.fromFields(m, func(k string) (string, bool) {
			i, ok := idx[strings.ToLower(strings.TrimSpace(k))]
			if !ok {
				return "", false
			}
			if i >= len(rec) {
				return "", true
			}
			return rec[i], true
		})
		if key != "" {
			s.problem(line, key, msg)
			continue
		}
		s.row(line, in)
	}
	return m, cols, nil
}

func (s *importState) parseText(data []byte) {
	n := 0
	eachParagraph(data, importParagraphMax, func(p string) bool {
		n++
		if s.opts.maxLine > 0 && n > s.opts.maxLine {
			return false
		}
		t := p
		s.row(n, RowInput{Text: &t})
		return !s.stop
	})
}

// SplitParagraphs splits on blank lines, trims, drops empty paragraphs and cuts one over max runes at the last
// whitespace before max (a hard cut when there is none).
func SplitParagraphs(text string, max int) []string {
	out := []string{}
	eachParagraph([]byte(text), max, func(p string) bool { out = append(out, p); return true })
	return out
}

func eachParagraph(data []byte, max int, fn func(string) bool) {
	var buf bytes.Buffer
	flush := func() bool {
		s := strings.TrimSpace(buf.String())
		buf.Reset()
		if s == "" {
			return true
		}
		if !utf8.ValidString(s) {
			s = strings.ToValidUTF8(s, string(utf8.RuneError))
		}
		return cutParagraph(s, max, fn)
	}
	pos := 0
	for pos < len(data) {
		var line []byte
		if i := bytes.IndexByte(data[pos:], '\n'); i < 0 {
			line, pos = data[pos:], len(data)
		} else {
			line, pos = data[pos:pos+i], pos+i+1
		}
		line = bytes.TrimRight(line, "\r")
		if len(bytes.TrimSpace(line)) == 0 {
			if !flush() {
				return
			}
			continue
		}
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.Write(line)
	}
	flush()
}

func cutParagraph(s string, max int, fn func(string) bool) bool {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return fn(s)
	}
	r := []rune(s)
	for len(r) > max {
		cut := max
		for i := max; i > 0; i-- {
			if unicode.IsSpace(r[i]) {
				cut = i
				break
			}
		}
		if part := strings.TrimSpace(string(r[:cut])); part != "" && !fn(part) {
			return false
		}
		r = r[cut:]
		for len(r) > 0 && unicode.IsSpace(r[0]) {
			r = r[1:]
		}
	}
	if len(r) > 0 {
		return fn(strings.TrimSpace(string(r)))
	}
	return true
}

func jobErrorFrom(err error) error {
	var ve *ValidationError
	if errors.As(err, &ve) && len(ve.Issues) > 0 {
		code := "invalid_import"
		if strings.HasSuffix(ve.Issues[0].MessageKey, "unsupported_file") {
			code = "unsupported_file"
		}
		return &JobError{Code: code, Message: ve.Issues[0].Message}
	}
	return err
}

// RunImport is the import_file job body: it reads the file through the workspace and adds the rows in chunks of 500.
func RunImport(ctx context.Context, st *Store, ws Workspace, projectID string, p ImportParams, maxFileBytes int64, importMaxRows, datasetMaxRows int, report func(Progress)) (JobResult, error) {
	proj, err := st.GetProject(ctx, projectID)
	if err != nil {
		return JobResult{}, err
	}
	format, err := DetectFormat(p.WorkspacePath, p.Format)
	if err != nil {
		return JobResult{}, jobErrorFrom(err)
	}
	data, err := ws.ReadFile(ctx, p.WorkspacePath, maxFileBytes)
	switch {
	case errors.Is(err, ErrFileTooLarge):
		return JobResult{}, &JobError{Code: "file_too_large", Message: "The file is too large."}
	case errors.Is(err, os.ErrNotExist):
		return JobResult{}, &JobError{Code: "file_not_found", Message: "The file was not found."}
	case err != nil:
		return JobResult{}, err
	}
	var m Mapping
	if p.Mapping != nil {
		m = *p.Mapping
	}

	batchID := uuid.NewString()
	var res JobResult
	var addIssues []Issue
	addSkipped, seen, submitted := 0, 0, 0
	stop := false
	var failure error
	chunk := make([]RowInput, 0, importChunk)
	lines := make([]int, 0, importChunk)
	flush := func() error {
		if len(chunk) == 0 {
			return nil
		}
		ar, err := st.AddRows(ctx, projectID, chunk, AddOptions{Source: "file", BatchID: batchID, BatchKind: "import",
			MaxRows: datasetMaxRows, Partial: true, Strict: true})
		if err != nil {
			return err
		}
		res.RowsAdded += ar.Added
		addSkipped += ar.Skipped
		for _, is := range ar.Issues {
			if i, ok := rowIndex(is.Field); ok && i < len(lines) {
				is.Line = lines[i]
			}
			addIssues = append(addIssues, is)
		}
		if ar.LimitReached {
			stop = true
		}
		submitted += len(chunk)
		chunk, lines = chunk[:0], lines[:0]
		report(Progress{Done: submitted, Total: seen, Phase: "importing"})
		return nil
	}

	stats, _, _, perr := ParseImport(data, format, m, proj.Recipe, importMaxRows, func(r ImportRow) bool {
		if ctx.Err() != nil {
			return false
		}
		in := r.Input
		in.SourceRef = p.WorkspacePath + "#L" + strconv.Itoa(r.Line)
		chunk, lines = append(chunk, in), append(lines, r.Line)
		seen++
		if len(chunk) >= importChunk {
			if err := flush(); err != nil {
				failure = err
				return false
			}
		}
		return !stop
	})
	data = nil
	if perr == nil && failure == nil && ctx.Err() == nil {
		failure = flush()
	}
	res.RowsSkipped = stats.Invalid + addSkipped
	res.Issues = append(stats.Issues, addIssues...)
	if len(res.Issues) > maxResultIssues {
		res.Issues = res.Issues[:maxResultIssues]
	}
	if res.RowsAdded > 0 {
		res.BatchID = batchID
	}
	switch {
	case ctx.Err() != nil:
		return res, ctx.Err()
	case perr != nil:
		return res, jobErrorFrom(perr)
	case failure != nil:
		return res, failure
	}
	return res, nil
}

// rowIndex reads the index out of a "rows[i]" or "rows[i].field" issue field.
func rowIndex(field string) (int, bool) {
	rest, ok := strings.CutPrefix(field, "rows[")
	if !ok {
		return 0, false
	}
	end := strings.IndexByte(rest, ']')
	if end < 0 {
		return 0, false
	}
	n, err := strconv.Atoi(rest[:end])
	return n, err == nil
}
