package flows

import (
	"context"
	"fmt"
	"math"
	"mime"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Document and file node types.
const (
	TypePDFCreate = "doc.pdf_create"
	TypePDFRead   = "doc.pdf_read"
	TypeFileRead  = "file.read"
	TypeFileWrite = "file.write"
)

// Logical tool names that plan 1c's invoker and catalog env special-case.
const (
	PDFExtractorTool = "pdf_extractor"
	GotenbergTool    = "document_creator:gotenberg"
)

// maxUniqueNames is the highest number if_exists "unique" tries: "a (2).txt" up to
// "a (100).txt". Every candidate costs one tool call.
const maxUniqueNames = 100

// Choices of the enumerated parameters. The first entry is not necessarily the
// default; the definitions say which one is.
var (
	pdfFormats  = []string{"text", "markdown", "html"}
	paperSizes  = []string{"A4", "A3", "A5", "Letter", "Legal"}
	ifExistsOps = []string{"overwrite", "unique", "fail"}
)

// actionDef builds the common part of a curated action node.
func actionDef(typ, category, icon, tool string, env CatalogEnv) *NodeDef {
	key := strings.ReplaceAll(typ, ".", "_")
	return &NodeDef{
		Type: typ, Version: 1, Category: category, Icon: icon, Color: category, Tool: tool,
		LabelKey:         "easydrag.node." + key + ".label",
		DescriptionKey:   "easydrag.node." + key + ".description",
		SummaryKey:       "easydrag.node." + key + ".summary",
		AvailabilityFunc: availabilityOf(env, tool),
	}
}

// FileRef builds the flow representation of a file:
// {"$type":"file","path":…,"name":…,"mime":…,"size":…,"web_path":…}.
//
// A file reference is only a hint at a path, never proof of anything. Whoever can
// shape flow data (a webhook payload, a mail, a web page, a model answer) can write
// {"$type":"file","path":"/any/where"} by hand, and nothing checks "$type": FilePath
// accepts any object with a text "path". Such an object does not show that the file
// exists, that this flow made it, or that it may be read or written. The only
// protection is the sandbox the AuraGo filesystem, PDF and document tools enforce
// on their own (allowed directories); the nodes in this file never open a file
// themselves, they pass the path on. Do not base a security decision on "$type",
// and keep every parameter that takes a path flagged SensitiveSink, so that the
// lint warns when untrusted data flows into it.
func FileRef(filePath, name, mimeType, webPath string, size int64) map[string]any {
	if name == "" {
		name = filepath.Base(filePath)
	}
	if mimeType == "" {
		mimeType = mimeForName(name)
	}
	ref := map[string]any{"$type": "file", "path": filePath, "name": name, "mime": mimeType}
	if size > 0 {
		ref["size"] = float64(size)
	}
	if webPath != "" {
		ref["web_path"] = webPath
	}
	return ref
}

// FilePath accepts a file object or a path string and returns the trimmed path, or
// "" for anything else: only a text "path" counts, so a number, a nested object or
// a list in its place gives "" instead of a rendering of it. It never panics, and
// it does not limit the length (callers do, see filePathParam).
//
// The result is a path hint, not a verified file: see FileRef. "$type" is not
// looked at.
func FilePath(v any) string {
	switch x := v.(type) {
	case map[string]any:
		s, _ := x["path"].(string)
		return strings.TrimSpace(s)
	case string:
		return strings.TrimSpace(x)
	}
	return ""
}

// mimeOverrides are consulted before the system's table. The runtime image has no
// /etc/mime.types, and Go's built-in table then calls these common text formats
// application/octet-stream, which a flow would treat as binary.
var mimeOverrides = map[string]string{
	".md": "text/markdown", ".markdown": "text/markdown",
	".yaml": "application/yaml", ".yml": "application/yaml",
	".toml": "application/toml",
	".log":  "text/plain", ".ini": "text/plain",
}

// mimeForName guesses the media type from the extension of name, without any
// parameters ("text/plain", not "text/plain; charset=utf-8"). Unknown extensions
// are application/octet-stream.
func mimeForName(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if t, ok := mimeOverrides[ext]; ok {
		return t
	}
	if t := mime.TypeByExtension(ext); t != "" {
		if i := strings.Index(t, ";"); i >= 0 {
			t = t[:i]
		}
		return strings.TrimSpace(t)
	}
	return "application/octet-stream"
}

func registerDocNodes(reg *Registry, env CatalogEnv) error {
	for _, def := range []*NodeDef{pdfCreateDef(env), pdfReadDef(env), fileReadDef(env), fileWriteDef(env)} {
		if err := reg.Register(def); err != nil {
			return err
		}
	}
	return nil
}

// pdfCreateDef defines doc.pdf_create. Its output is made from the node's own
// parameters and the document tool's answer, so it is trusted. The content is not a
// sensitive sink: turning scraped or generated text into a PDF is the node's main
// job, and a lint warning for it would be noise. The output name is one: it picks
// which stored document gets replaced. (The document tool keeps only the base name,
// so the name cannot leave its directory.) Known limit: with format "html" the
// content is rendered by Gotenberg, which fetches whatever the HTML points to.
func pdfCreateDef(env CatalogEnv) *NodeDef {
	def := actionDef(TypePDFCreate, "documents", "file-type-pdf", "document_creator", env)
	def.PrimaryInput = "content"
	def.Effects = []Effect{EffectWritesFiles}
	def.Params = []ParamSpec{
		{Name: "title", Kind: ParamText, LabelKey: "easydrag.param.pdf_title", Templatable: true},
		{Name: "content", Kind: ParamTextarea, LabelKey: "easydrag.param.pdf_content", Required: true, Templatable: true},
		{Name: "format", Kind: ParamSegmented, LabelKey: "easydrag.param.pdf_format", Default: "text",
			Options: []Option{option("text", "pdf_format_text"), option("markdown", "pdf_format_markdown"), option("html", "pdf_format_html")}},
		{Name: "paper_size", Kind: ParamSelect, LabelKey: "easydrag.param.paper_size", Default: "A4",
			Options: []Option{{Value: "A4", Label: "A4"}, {Value: "A3", Label: "A3"}, {Value: "A5", Label: "A5"}, {Value: "Letter", Label: "Letter"}, {Value: "Legal", Label: "Legal"}}},
		{Name: "landscape", Kind: ParamBool, LabelKey: "easydrag.param.landscape", Default: false},
		{Name: "filename", Kind: ParamText, LabelKey: "easydrag.param.filename", Templatable: true, SensitiveSink: true},
	}
	def.OutputFields = []FieldSpec{{Name: "file", Type: "file", Primary: true}, {Name: "web_path", Type: "text"}}
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		if n == nil {
			return nil
		}
		issues := choiceIssue(n, "format", "text", pdfFormats)
		issues = append(issues, choiceIssue(n, "paper_size", "", paperSizes)...)
		if format, ok := choiceParam(n.Params["format"], "text", pdfFormats...); ok && (format == "markdown" || format == "html") &&
			availabilityOf(env, GotenbergTool)().State != AvailableState {
			issues = append(issues, paramIssue(n, IssueParamInvalid, SeverityError, "format", "Markdown and HTML need the Gotenberg document service"))
		}
		return issues
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		format, ok := choiceParam(in.Params["format"], "text", pdfFormats...)
		if !ok {
			return ExecResult{}, choiceError("format", pdfFormats)
		}
		paper, ok := choiceParam(in.Params["paper_size"], "", paperSizes...)
		if !ok {
			return ExecResult{}, choiceError("paper_size", paperSizes)
		}
		content, err := documentText("content", in.Params["content"])
		if err != nil {
			return ExecResult{}, err
		}
		title, err := documentText("title", in.Params["title"])
		if err != nil {
			return ExecResult{}, err
		}
		filename, err := fileNameParam(in.Params["filename"])
		if err != nil {
			return ExecResult{}, err
		}
		op := "create_pdf"
		switch format {
		case "markdown":
			op = "markdown_to_pdf"
		case "html":
			op = "html_to_pdf"
		}
		args := map[string]any{"operation": op, "content": content, "landscape": truthy(in.Params["landscape"])}
		setArg(args, "title", title)
		setArg(args, "paper_size", paper)
		setArg(args, "filename", strings.TrimSuffix(filename, ".pdf"))
		out, err := callTool(ctx, in, "document_creator", args)
		if err != nil {
			return ExecResult{}, err
		}
		if err := requireSuccess(out, "document service"); err != nil {
			return ExecResult{}, err
		}
		filePath := outString(out, "file_path")
		if filePath == "" {
			return ExecResult{}, NewNodeError("FLOW_TOOL_ERROR", "the document service did not return a file")
		}
		webPath := outString(out, "web_path")
		return ExecResult{Output: map[string]any{
			"file":     FileRef(filePath, outString(out, "filename"), "application/pdf", webPath, 0),
			"web_path": webPath,
		}}, nil
	}
	return def
}

// pdfReadDef defines doc.pdf_read. A PDF can come from anywhere (a mail
// attachment, a download), so its text is untrusted, and the file parameter is a
// sensitive sink: the PDF tool enforces its own sandbox, the node only passes the
// path on.
func pdfReadDef(env CatalogEnv) *NodeDef {
	def := actionDef(TypePDFRead, "documents", "file-search", PDFExtractorTool, env)
	def.PrimaryInput = "file"
	def.UntrustedOutput = true
	def.Params = []ParamSpec{{Name: "file", Kind: ParamFile, LabelKey: "easydrag.param.file", Required: true, Templatable: true, SensitiveSink: true}}
	def.OutputFields = []FieldSpec{{Name: "text", Type: "text", Primary: true}, {Name: "file", Type: "file"}}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		p, err := filePathParam(in.Params["file"], "choose a PDF file")
		if err != nil {
			return ExecResult{}, err
		}
		out, err := callTool(ctx, in, PDFExtractorTool, map[string]any{"filepath": p})
		if err != nil {
			return ExecResult{}, err
		}
		// The extractor answers {"status":"success","content":…}, summary mode included
		// (the summary is wrapped in the same envelope). Anything else is a refusal
		// that came as plain text, for instance "pdf_extractor is disabled in
		// settings", and must not become the document's text.
		if err := requireSuccess(out, "PDF reader"); err != nil {
			return ExecResult{}, err
		}
		text, ok := out["content"].(string)
		if !ok {
			return ExecResult{}, NewNodeError("FLOW_TOOL_ERROR", "the PDF reader returned no text")
		}
		return ExecResult{Output: map[string]any{"text": text, "file": FileRef(p, "", "application/pdf", "", 0)}}, nil
	}
	return def
}

// The filesystem tool returns at most 34816 bytes of a file (32 KiB plus a little
// padding for UTF-8). A longer file comes back cut, with "(truncated, file has N
// bytes total)" in the tool's message and the readTruncationMarker line at the end of
// the text, and with no flag of its own. Binary files are refused by the tool.
const readTruncationMarker = "[...truncated"

var (
	readTotalRe = regexp.MustCompile(`file has (\d+) bytes total`)
	readBytesRe = regexp.MustCompile(`^Read (\d+) bytes`)
)

// readInfo derives from the message of a successful read_file answer whether the
// text was cut and how big the file is. total is false when the message does not say.
// A full read says "Read N bytes", and then N is the size of the file. Only the start
// of the message is looked at; the tool's own message is short. data is the answer's
// data, which carries a "truncated" flag of its own in some tool modes.
func readInfo(out map[string]any, data any) (truncated bool, size float64, total bool) {
	msg := outString(out, "message")
	if len(msg) > 300 {
		msg = msg[:300]
	}
	truncated = strings.Contains(msg, "(truncated")
	re := readTotalRe
	if !truncated {
		re = readBytesRe
	}
	if m := re.FindStringSubmatch(msg); m != nil {
		if n, err := strconv.ParseFloat(m[1], 64); err == nil && !math.IsInf(n, 0) {
			size, total = n, true
		}
	}
	if obj, ok := data.(map[string]any); ok {
		if flag, _ := obj["truncated"].(bool); flag {
			truncated = true
		}
	}
	return truncated, size, total
}

// stripTruncationMarker removes the line the tool appends to a cut text
// ("\n\n[...truncated — use smart_file_read …]"), so the content is the file's own
// text. It only removes a short last paragraph of that shape, and the caller only asks
// when the tool said the text was cut.
func stripTruncationMarker(content string) string {
	i := strings.LastIndex(content, "\n\n"+readTruncationMarker)
	if i < 0 {
		return content
	}
	if tail := content[i+2:]; len(tail) > 300 || !strings.HasSuffix(tail, "...]") {
		return content
	}
	return content[:i]
}

// fileReadDef defines file.read. File content can come from anywhere (a download,
// a mail attachment, another user), so it is untrusted.
//
// The tool reads at most 34816 bytes (about 34 KB) of a file. The node reports
// what happened in two output fields: truncated is true when the text was cut, and
// total_size is the size of the file in bytes when the tool says so. A cut text has
// the tool's "[...truncated …]" marker line removed, so content is only the file's
// own text; a flow that needs more has to read the file in another way.
func fileReadDef(env CatalogEnv) *NodeDef {
	def := actionDef(TypeFileRead, "documents", "file-text", "filesystem", env)
	def.PrimaryInput = "path"
	def.UntrustedOutput = true
	def.Params = []ParamSpec{{Name: "path", Kind: ParamText, LabelKey: "easydrag.param.file_path", Required: true, Templatable: true, SensitiveSink: true}}
	def.OutputFields = []FieldSpec{
		{Name: "content", Type: "text", Primary: true}, {Name: "file", Type: "file"},
		{Name: "truncated", Type: "bool"}, {Name: "total_size", Type: "number"},
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		p, err := filePathParam(in.Params["path"], "enter a file path")
		if err != nil {
			return ExecResult{}, err
		}
		out, err := filesystemCall(ctx, in, map[string]any{"operation": "read_file", "file_path": p})
		if err != nil {
			return ExecResult{}, err
		}
		if err := requireSuccess(out, "file tool"); err != nil {
			return ExecResult{}, err
		}
		var content string
		ok := false
		switch data := out["data"].(type) {
		case string:
			content, ok = data, true
		case map[string]any:
			content, ok = data["content"].(string)
		}
		// A successful read always carries the text (an empty file gives ""). Reading
		// nothing without an error would let a flow carry on, and perhaps write an
		// empty file, as if the file were empty.
		if !ok {
			return ExecResult{}, NewNodeError("FLOW_TOOL_ERROR", "the file tool returned no content")
		}
		truncated, size, known := readInfo(out, out["data"])
		if truncated {
			content = stripTruncationMarker(content)
		}
		output := map[string]any{"content": content, "file": FileRef(p, "", "", "", 0), "truncated": truncated}
		if known {
			output["total_size"] = size
		}
		return ExecResult{Output: output}, nil
	}
	return def
}

// fileWriteDef defines file.write. Path and content are both sensitive sinks:
// untrusted data must not pick which file is written, nor become the content of a
// file that something else may run or read later. The filesystem tool enforces its
// own sandbox (allowed directories) and may be set read-only.
//
// if_exists "unique" and "fail" probe first and write second. These are two tool
// calls and the tool has no exclusive create, so a concurrent writer can still win
// the race between them; the modes are a convenience, not a lock.
func fileWriteDef(env CatalogEnv) *NodeDef {
	def := actionDef(TypeFileWrite, "documents", "file-pencil", "filesystem", env)
	def.PrimaryInput = "content"
	def.Effects = []Effect{EffectWritesFiles}
	def.Params = []ParamSpec{
		{Name: "path", Kind: ParamText, LabelKey: "easydrag.param.file_path", Required: true, Templatable: true, SensitiveSink: true},
		{Name: "content", Kind: ParamTextarea, LabelKey: "easydrag.param.file_content", Templatable: true, SensitiveSink: true},
		{Name: "if_exists", Kind: ParamSegmented, LabelKey: "easydrag.param.if_exists", Default: "overwrite",
			Options: []Option{option("overwrite", "if_exists_overwrite"), option("unique", "if_exists_unique"), option("fail", "if_exists_fail")}},
	}
	def.OutputFields = []FieldSpec{{Name: "file", Type: "file", Primary: true}}
	def.Validate = func(n *Node, _ ValidateContext) []Issue {
		if n == nil {
			return nil
		}
		return choiceIssue(n, "if_exists", "overwrite", ifExistsOps)
	}
	def.Execute = func(ctx context.Context, in ExecInput) (ExecResult, error) {
		target, err := filePathParam(in.Params["path"], "enter a file path")
		if err != nil {
			return ExecResult{}, err
		}
		content, err := documentText("content", in.Params["content"])
		if err != nil {
			return ExecResult{}, err
		}
		mode, ok := choiceParam(in.Params["if_exists"], "overwrite", ifExistsOps...)
		if !ok {
			// Not a silent overwrite: a mistyped "fail" must not destroy a file.
			return ExecResult{}, choiceError("if_exists", ifExistsOps)
		}
		if mode != "overwrite" {
			if target, err = freeTarget(ctx, in, target, mode); err != nil {
				return ExecResult{}, err
			}
		}
		// callTool fails on a cancelled context before it invokes the tool, so a run
		// that ends between the probe above and this write never writes.
		out, err := filesystemCall(ctx, in, map[string]any{"operation": "write_file", "file_path": target, "content": content})
		if err != nil {
			return ExecResult{}, err
		}
		// A refusal that came as plain text ("[PERMISSION DENIED] filesystem write
		// operations are disabled") is not a write.
		if err := requireSuccess(out, "file tool"); err != nil {
			return ExecResult{}, err
		}
		return ExecResult{Output: map[string]any{"file": FileRef(target, "", "", "", int64(len(content)))}}, nil
	}
	return def
}

// freeTarget applies if_exists "unique" or "fail" to target and returns the path to
// write: target itself when nothing is there, the first free "name (N).ext" for
// "unique" (N up to maxUniqueNames), and FLOW_FILE_EXISTS when the file exists under
// "fail" or when no number is free.
//
// FLOW_FILE_EXISTS is retried like any other error, because the engine's set of
// non-retryable codes is a contract this node does not change. A retry is not always
// harmless, though. When a write succeeded but its answer was lost (the attempt timed
// out in transit), the retry finds the file the first attempt made: "fail" then
// reports FLOW_FILE_EXISTS for it and "unique" writes a second copy under the next
// free name. A flow that sets Retry on file.write and cannot tolerate that should use
// "overwrite", which a re-run repeats safely.
func freeTarget(ctx context.Context, in ExecInput, target, mode string) (string, error) {
	exists, err := fileExists(ctx, in, target)
	if err != nil {
		return "", err
	}
	if !exists {
		return target, nil
	}
	if mode == "fail" {
		return "", NewNodeError("FLOW_FILE_EXISTS", "%s already exists", quoteForError(target))
	}
	for i := 2; i <= maxUniqueNames; i++ {
		candidate := numberedPath(target, i)
		taken, err := fileExists(ctx, in, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", NewNodeError("FLOW_FILE_EXISTS", "no free file name next to %s (tried up to %d)", quoteForError(target), maxUniqueNames)
}

// filesystemCall runs the filesystem tool through callTool. The tool's own refusals
// of a path (outside the allowed directories, cannot be resolved) and of a read-only
// target are final, so they become FLOW_TOOL_DENIED, which the engine does not
// retry, instead of the retried FLOW_TOOL_ERROR. A write refused before it starts (a
// path outside the allowed directories, writes switched off) carries no error code
// in the tool's answer and stays a plain, retried tool error.
func filesystemCall(ctx context.Context, in ExecInput, args map[string]any) (map[string]any, error) {
	out, err := callTool(ctx, in, "filesystem", args)
	if err == nil {
		return out, nil
	}
	if ne := asNodeError(err); ne.Code == "FLOW_TOOL_ERROR" {
		if data, ok := out["data"].(map[string]any); ok {
			switch data["error_code"] {
			case "path_resolution_error", "read_only_filesystem":
				return out, &NodeError{Code: "FLOW_TOOL_DENIED", Message: ne.Message}
			}
		}
	}
	return out, err
}

// Texts at the end of an OS error that say a path is not there, and texts that say
// the tool could not tell. Matching is done on the end of the tool's message: it
// reads "Failed to stat: stat <path>: <reason>", the path is the caller's data (a
// file may be named "no such file"), and the reason comes last.
var (
	statGoneTails    = []string{"no such file or directory", "does not exist", "cannot find the file specified", "cannot find the path specified"}
	statBlockedTails = []string{"permission denied", "access is denied", "not a directory", "input/output error", "too many levels of symbolic links", "file name too long"}
)

// maxStatTailBytes is how much of the end of a message statMissing looks at.
const maxStatTailBytes = 200

// statMissing decides from the output of a failed stat whether the path is simply
// not there. The filesystem tool reports every stat failure as error_code
// "io_error", the missing file included, so the code alone is not proof: the OS
// reason at the end of the tool's own message wins, in either direction. A message
// in the tool's own "Failed to …: <reason>" shape whose reason is not recognised is
// unknown and counts as not missing (fail closed, whatever the error code says). An
// io_error without such a message, nothing at all or a bare "stat failed", counts as
// missing (the tool's generic answer). Any other failure, a refused path included, is
// not "missing".
//
// It reads the parsed tool output, not the NodeError that callTool built: that
// message is cut to 300 runes, which can drop the reason of a long path.
func statMissing(out map[string]any) bool {
	for _, key := range []string{"message", "error", "text"} {
		msg, _ := out[key].(string)
		if len(msg) > maxStatTailBytes {
			msg = msg[len(msg)-maxStatTailBytes:]
		}
		tail := strings.ToLower(strings.TrimRight(strings.TrimSpace(msg), ". \t\r\n"))
		for _, s := range statGoneTails {
			if strings.HasSuffix(tail, s) {
				return true
			}
		}
		for _, s := range statBlockedTails {
			if strings.HasSuffix(tail, s) {
				return false
			}
		}
	}
	if msg := strings.TrimSpace(outString(out, "message")); len(msg) >= len(statFailedPrefix) && strings.EqualFold(msg[:len(statFailedPrefix)], statFailedPrefix) {
		return false
	}
	data, _ := out["data"].(map[string]any)
	return data["error_code"] == "io_error"
}

// statFailedPrefix starts every io error message of the filesystem tool ("Failed to
// stat: …", "Failed to open workspace root: …"): the reason follows the colon.
const statFailedPrefix = "Failed to "

// fileExists asks the filesystem tool. A stat that fails because the file is not
// there is a clean "no"; every other failure (denied, unavailable, the OS cannot
// tell, the tool is down, the context ended) is returned as it is, so a doubtful
// answer never lets "fail" or "unique" write over a file. An answer that does not
// say "success" is no "yes" either: a plain-text refusal must not read as "the file
// exists" (it would fail "fail" mode and burn maxUniqueNames calls in "unique").
func fileExists(ctx context.Context, in ExecInput, p string) (bool, error) {
	out, err := filesystemCall(ctx, in, map[string]any{"operation": "stat", "file_path": p})
	if err == nil {
		if err := requireSuccess(out, "file tool"); err != nil {
			return false, err
		}
		return true, nil
	}
	if asNodeError(err).Code == "FLOW_TOOL_ERROR" && statMissing(out) {
		return false, nil
	}
	return false, err
}

// numberedPath turns "notes/a.txt" into "notes/a (2).txt". The extension is looked
// for in the last element only, split at "/" or "\", and a dot file such as ".env"
// has none, so "dir.v2/file" becomes "dir.v2/file (2)" and ".env" ".env (2)".
func numberedPath(p string, n int) string {
	name := p[strings.LastIndexAny(p, `/\`)+1:]
	ext := path.Ext(name)
	if ext == name {
		ext = ""
	}
	return fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(p, ext), n, ext)
}
