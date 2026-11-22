package chibisensei

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
)

// ExportParams are the parameters of the export job.
type ExportParams struct {
	WorkspacePath string `json:"workspace_path"`
	Which         string `json:"which"`
	Overwrite     bool   `json:"overwrite"`
}

const maxExportBytes = 2 << 30

type exportChat struct {
	Messages []Message `json:"messages"`
}

type exportText struct {
	Text string `json:"text"`
}

// ExportLine is {"messages":[...]} or {"text":"..."} followed by a newline, without HTML escaping.
func ExportLine(r Row) []byte {
	var v any
	if r.Kind == rowKindText && r.Text != nil {
		v = exportText{Text: *r.Text}
	} else {
		msgs := r.Messages
		if msgs == nil {
			msgs = []Message{}
		}
		v = exportChat{Messages: msgs}
	}
	return []byte(encodeJSON(v) + "\n")
}

// RunExport is the export job body: the canonical JSONL of all, train, val or eval rows, streamed to the workspace.
func RunExport(ctx context.Context, st *Store, ws Workspace, projectID string, p ExportParams, report func(Progress)) (JobResult, error) {
	if !strings.HasSuffix(strings.ToLower(p.WorkspacePath), ".jsonl") {
		return JobResult{}, &ValidationError{Issues: []Issue{issue("workspace_path", "export.bad_extension", "The export file must end with .jsonl.")}}
	}
	switch p.Which {
	case "all", "train", "val", "eval":
	default:
		return JobResult{}, &ValidationError{Issues: []Issue{issue("which", "export.bad_which", "Unknown export selection.")}}
	}
	srows, proj, err := st.LoadSplitRows(ctx, projectID)
	if err != nil {
		return JobResult{}, err
	}
	sp := ComputeSplit(srows, proj.SplitSeed, proj.TaskKind == "classify")
	want := func(id int64) bool {
		a, ok := sp.Assign[id]
		if !ok || a == "exclude" {
			return false
		}
		return p.Which == "all" || a == p.Which
	}
	total := 0
	for _, r := range srows {
		if want(r.ID) {
			total++
		}
	}
	report(Progress{Done: 0, Total: total, Phase: "exporting"})

	pr, pw := io.Pipe()
	errCh := make(chan error, 1)
	go func() {
		werr := ws.WriteFile(ctx, p.WorkspacePath, pr, maxExportBytes, p.Overwrite)
		if werr == nil {
			werr = io.ErrClosedPipe // the writer must not outlive a finished upload
		}
		pr.CloseWithError(werr)
		if errors.Is(werr, io.ErrClosedPipe) {
			werr = nil
		}
		errCh <- werr
	}()

	bw := bufio.NewWriterSize(pw, 64*1024)
	written := 0
	eachErr := st.EachRow(ctx, projectID, func(r Row) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !want(r.ID) {
			return nil
		}
		if _, err := bw.Write(ExportLine(r)); err != nil {
			return err
		}
		written++
		if written%200 == 0 {
			report(Progress{Done: written, Total: total, Phase: "exporting"})
		}
		return nil
	})
	if eachErr == nil {
		eachErr = bw.Flush()
	}
	if eachErr == nil {
		// the pipe is still open, so a failed check keeps the writer from publishing a file the split disagrees with
		if now, err := st.GetProject(ctx, projectID); err != nil {
			eachErr = err
		} else if now.DataRev != proj.DataRev {
			eachErr = ErrDataChanged
		}
	}
	if eachErr != nil {
		pw.CloseWithError(eachErr)
	} else {
		pw.Close()
	}
	werr := <-errCh
	res := JobResult{Rows: written, Path: p.WorkspacePath}
	if err := ctx.Err(); err != nil {
		return res, err
	}
	if errors.Is(eachErr, ErrDataChanged) {
		return res, &JobError{Code: "data_changed", Message: "The rows changed during the export."}
	}
	if werr != nil {
		switch {
		case errors.Is(werr, ErrFileExists):
			return res, &JobError{Code: "file_exists", Message: "The file already exists."}
		case errors.Is(werr, ErrFileTooLarge):
			return res, &JobError{Code: "file_too_large", Message: "The export is too large."}
		}
		return res, werr
	}
	if eachErr != nil {
		return res, eachErr
	}
	report(Progress{Done: written, Total: total, Phase: "exporting"})
	return res, nil
}
