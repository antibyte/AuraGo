package tools

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"aurago/internal/config"
	"aurago/internal/gog"
	"aurago/internal/security"
)

func ExecuteGog(ctx context.Context, cfg *config.Config, vault *security.Vault, req gog.Request) gog.Result {
	op, ok := gog.Lookup(req.Service, req.Operation)
	if !ok {
		return gog.Failure("unknown_operation")
	}
	if _, err := gog.Authorize(cfg, "agent", req, op.Grant); err != nil {
		return gog.Failure(err.Error())
	}
	input, _ := req.Args["input_path"].(string)
	output, _ := req.Args["output_path"].(string)
	delete(req.Args, "input_path")
	delete(req.Args, "output_path")
	var files gog.Files
	var downloaded bytes.Buffer
	if op.Input {
		f, _, err := OpenOutgoingAttachment(input, cfg)
		if err != nil {
			return gog.Failure("file_access_denied")
		}
		defer f.Close()
		files.Input = f
		files.Name = filepath.Base(input)
	}
	var target string
	if op.Output {
		perms, _ := EffectiveRuntimePermissions(ctx)
		if !perms.AllowFilesystemWrite {
			return gog.Failure("filesystem_write_disabled")
		}
		var err error
		target, err = secureResolve(cfg.Directories.WorkspaceDir, output)
		if err != nil || output == "" {
			return gog.Failure("file_access_denied")
		}
		if err = requireUnprotectedNotesPath(target, true); err != nil {
			return gog.Failure("file_access_denied")
		}
		if _, err = rootedToolLstat(target); !os.IsNotExist(err) {
			return gog.Failure("destination_exists")
		}
		files.Output = &downloaded
	}
	svc := NewGogService(cfg, vault, nil)
	result := svc.Execute(ctx, "agent", req, &files)
	if result.Status == "ok" && op.Output {
		perms, _ := EffectiveRuntimePermissions(ctx)
		if !perms.AllowFilesystemWrite || ctx.Err() != nil {
			return gog.Failure("authorization_changed")
		}
		if _, e := gog.Authorize(svc.Config(), "agent", req, op.Grant); e != nil {
			return gog.Failure("authorization_changed")
		}
		e := publishGogFile(ctx, target, downloaded.Bytes())
		if e != nil {
			return gog.Failure("file_write_failed")
		}
		result.Data, _ = json.Marshal(map[string]any{"output_path": output, "bytes": downloaded.Len()})
	}
	return result
}

// Link publishes a complete new file atomically and cannot replace a racing writer.
func publishGogFile(ctx context.Context, target string, data []byte) error {
	root, rel, e := rootedToolPath(target)
	if e != nil {
		return e
	}
	defer root.Close()
	temp := filepath.Join(filepath.Dir(rel), ".gog-"+rand.Text())
	f, e := root.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer root.Remove(temp)
	_, e = f.Write(data)
	if e == nil {
		e = f.Sync()
	}
	closed := f.Close()
	if e != nil {
		return e
	}
	if closed != nil {
		return closed
	}
	p, _ := EffectiveRuntimePermissions(ctx)
	if !p.AllowFilesystemWrite {
		return fmt.Errorf("filesystem_write_disabled")
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	return root.Link(temp, rel)
}
