package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/tools"
)

// The documents bridge.
//
// doc.pdf_create writes into the documents folder (tools.document_creator.output_dir) and
// returns its absolute path, but file.read (filesystem read_file) and doc.pdf_read (the
// pdf_extractor skill) read only inside the workspace jail. Attachments need no bridge:
// send_telegram and send_email open their files with tools.OpenOutgoingAttachment, which
// accepts the documents folder.
//
// For those two reads, a path that names a file of the documents folder (the absolute path
// document_creator returned, or the served form /files/documents/<name>) is opened with
// tools.OpenOutgoingAttachment, which applies the attachment jail (workspace and documents
// folder only, no protected AuraGo state, no symlink or hard-link escape), copied into
// <workspace>/.easydrag/<run>/doc-<random>/<name> (at most flowDocumentCopyLimit bytes),
// and the argument is rewritten to the copy. The copy exists only for the one dispatch:
// it is removed when the call returns. Run folders are removed once they are older than
// flowDocumentScratchRetention (the longest run plus an hour), which also clears what a
// crash left behind. .easydrag must be a plain directory (no symlink, no junction); folders
// are made, files written and old folders swept only through an os.Root of .easydrag
// itself (openFlowScratch), so nothing outside it is touched.
//
// A path that names the documents folder but cannot be read through the jail (it climbs
// out with "..", is a link that leads elsewhere, is missing or too large) is refused as
// denied; a copy that fails on the disk is a retried failure. Any other path goes to the
// tool unchanged and meets the tool's own jail.

const (
	// flowDocumentCopyLimit bounds the copy of one document.
	flowDocumentCopyLimit = 32 << 20
	// flowScratchDir is the scratch folder in the workspace.
	flowScratchDir = ".easydrag"
	// flowServedDocumentsPrefix is the URL path the server serves documents under.
	flowServedDocumentsPrefix = "/files/documents/"
)

// flowDocumentScratchRetention is how long a run's scratch folder may stay.
var flowDocumentScratchRetention = time.Duration(flows.MaxRunSecondsLimit)*time.Second + time.Hour

// flowDocumentArg names the argument of a read that the bridge may rewrite, or "".
func flowDocumentArg(tool string, args map[string]any) string {
	switch tool {
	case flows.PDFExtractorTool:
		return "filepath"
	case "filesystem":
		op, _ := args["operation"].(string)
		switch strings.ToLower(strings.TrimSpace(op)) {
		case "read_file", "read":
			if _, ok := args["file_path"].(string); ok {
				return "file_path"
			}
			if _, ok := args["path"].(string); ok {
				return "path"
			}
		}
	}
	return ""
}

// flowDocumentsDir is the documents folder as the server serves it.
func flowDocumentsDir(cfg *config.Config) string {
	dir := strings.TrimSpace(cfg.Tools.DocumentCreator.OutputDir)
	if dir == "" {
		dir = filepath.Join(cfg.Directories.DataDir, "documents")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	return abs
}

// flowWithin reports, without touching the filesystem, whether p lies inside dir.
func flowWithin(dir, p string) bool {
	if dir == "" {
		return false
	}
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// flowDocumentPath decides whether p names a file of the documents folder. It returns the
// local path to open and true for such a path, and an error for a served path that climbs
// out of the folder. A path inside the workspace is left to the tool.
func flowDocumentPath(cfg *config.Config, p string) (string, bool, error) {
	p = strings.TrimSpace(p)
	docs := flowDocumentsDir(cfg)
	if docs == "" {
		return "", false, nil
	}
	if strings.HasPrefix(p, flowServedDocumentsPrefix) {
		rel, err := url.PathUnescape(strings.TrimPrefix(p, flowServedDocumentsPrefix))
		if err != nil || rel == "" || strings.ContainsAny(rel, "\\\x00") {
			return "", true, errors.New("the document path is not valid")
		}
		parts := strings.Split(rel, "/")
		for _, part := range parts {
			if part == ".." || part == "" || part == "." {
				return "", true, errors.New("the document path leaves the documents folder")
			}
		}
		return filepath.Join(docs, filepath.FromSlash(path.Join(parts...))), true, nil
	}
	if !filepath.IsAbs(p) {
		return "", false, nil
	}
	clean := filepath.Clean(p)
	workspace, _ := filepath.Abs(strings.TrimSpace(cfg.Directories.WorkspaceDir))
	if !flowWithin(docs, clean) || cfg.Directories.WorkspaceDir != "" && flowWithin(workspace, clean) {
		return "", false, nil
	}
	return clean, true, nil
}

// flowScratchName makes a file or folder name safe: letters, digits, dot, dash and
// underscore, at most 120 bytes, never empty or a dot name.
func flowScratchName(name string, fallback string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 120 {
			break
		}
	}
	s := strings.Trim(b.String(), ".")
	if s == "" {
		return fallback
	}
	return s
}

// bridgeDocument rewrites a documents-folder path of a read into a scratch copy in the
// workspace (see the top of this file). It returns a cleanup for the copy, and a refusal
// when the path names the documents folder but cannot be read through the jail (denied,
// not retried) or the copy could not be made (failed, retried).
func bridgeDocument(cfg *config.Config, req flows.ToolRequest, args map[string]any) (cleanup func(), refusal *flows.ToolResponse) {
	none := func() {}
	refuse := func(status agent.ToolResultStatus, message string) (func(), *flows.ToolResponse) {
		r := flowToolRefusal(status, "the document cannot be read: "+message)
		return none, &r
	}
	key := flowDocumentArg(req.Tool, args)
	if key == "" {
		return none, nil
	}
	p, _ := args[key].(string)
	local, isDocument, err := flowDocumentPath(cfg, p)
	if !isDocument {
		return none, nil
	}
	if err != nil {
		return refuse(agent.ToolResultDenied, err.Error())
	}
	src, resolved, err := tools.OpenOutgoingAttachment(local, cfg)
	if err != nil {
		return refuse(agent.ToolResultDenied, flowBoundRunes(err.Error(), flowLogRunes))
	}
	defer src.Close()
	if info, err := src.Stat(); err != nil || !info.Mode().IsRegular() || info.Size() > flowDocumentCopyLimit {
		return refuse(agent.ToolResultDenied, fmt.Sprintf("it is not a regular file of at most %d MiB", flowDocumentCopyLimit>>20))
	}
	workspace, err := filepath.Abs(strings.TrimSpace(cfg.Directories.WorkspaceDir))
	if err != nil || strings.TrimSpace(cfg.Directories.WorkspaceDir) == "" {
		return refuse(agent.ToolResultNeedsSetup, "no workspace is configured")
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return refuse(agent.ToolResultFailed, "the workspace cannot be opened")
	}
	scratch, err := openFlowScratch(root)
	_ = root.Close()
	if err != nil {
		return refuse(agent.ToolResultDenied, err.Error())
	}
	sweepFlowScratch(scratch, time.Now())
	var suffix [6]byte
	_, _ = rand.Read(suffix[:])
	dir := filepath.Join(flowScratchName(req.RunID, "run"), "doc-"+hex.EncodeToString(suffix[:]))
	if err := scratch.MkdirAll(dir, 0o700); err != nil {
		_ = scratch.Close()
		return refuse(agent.ToolResultFailed, "the scratch folder cannot be made")
	}
	removeDir := func() {
		_ = scratch.RemoveAll(dir)
		_ = scratch.Close()
	}
	name := filepath.Join(dir, flowScratchName(filepath.Base(resolved), "document"))
	dst, err := scratch.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		removeDir()
		return refuse(agent.ToolResultFailed, "the copy cannot be written")
	}
	n, copyErr := io.Copy(dst, io.LimitReader(src, flowDocumentCopyLimit+1))
	closeErr := dst.Close()
	if copyErr != nil || closeErr != nil || n > flowDocumentCopyLimit {
		removeDir()
		return refuse(agent.ToolResultFailed, "the copy failed")
	}
	args[key] = filepath.Join(workspace, flowScratchDir, name)
	return removeDir, nil
}

// errFlowScratchNotPlain refuses a scratch folder that is not a plain directory.
var errFlowScratchNotPlain = errors.New("the scratch folder " + flowScratchDir + " in the workspace is not a plain folder")

// openFlowScratch opens <workspace>/.easydrag as a root of its own, making it when it is
// missing. It must be a plain directory: os.Root follows links that stay inside the
// workspace, so a symlink or junction at .easydrag pointing at "." or "projects" would let
// the sweep remove old workspace folders and put the copies elsewhere. The folder is
// checked with Lstat, then opened, and the opened folder must be the one Lstat saw.
func openFlowScratch(root *os.Root) (*os.Root, error) {
	if err := root.Mkdir(flowScratchDir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, errors.New("the scratch folder cannot be made")
	}
	before, err := root.Lstat(flowScratchDir)
	if err != nil || before.Mode().Type() != fs.ModeDir {
		return nil, errFlowScratchNotPlain
	}
	scratch, err := root.OpenRoot(flowScratchDir)
	if err != nil {
		return nil, errFlowScratchNotPlain
	}
	opened, err := scratch.Stat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(before, opened) {
		_ = scratch.Close()
		return nil, errFlowScratchNotPlain
	}
	return scratch, nil
}

// sweepFlowScratch removes run folders older than flowDocumentScratchRetention. It works
// inside the scratch root only and skips entries that are not plain directories
// (DirEntry.IsDir does not follow links).
func sweepFlowScratch(scratch *os.Root, now time.Time) {
	dir, err := scratch.Open(".")
	if err != nil {
		return
	}
	entries, err := dir.ReadDir(-1)
	_ = dir.Close()
	if err != nil {
		return
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !entry.IsDir() || now.Sub(info.ModTime()) <= flowDocumentScratchRetention {
			continue
		}
		_ = scratch.RemoveAll(entry.Name())
	}
}
