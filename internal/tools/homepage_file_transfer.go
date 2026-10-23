package tools

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"aurago/internal/security"
)

// Archives avoid DockerExec's output truncation and the shell's argument limit.
func homepageReadDockerFile(cfg DockerConfig, relPath string) (string, error) {
	ctx, cancel := context.WithTimeout(nativeHTTPContext(cfg.Context), time.Minute)
	defer cancel()
	archive, err := DockerGetArchiveContext(ctx, cfg, homepageContainerName, "/workspace/"+relPath, maxHomepageWriteFileSize+64*1024)
	if err != nil {
		return "", err
	}
	reader := tar.NewReader(bytes.NewReader(archive))
	header, err := reader.Next()
	if err != nil {
		return "", fmt.Errorf("read file archive header: %w", err)
	}
	if header.Name != path.Base(relPath) || !header.FileInfo().Mode().IsRegular() || header.Size > maxHomepageWriteFileSize {
		return "", fmt.Errorf("expected one regular file of at most %d bytes", maxHomepageWriteFileSize)
	}
	content, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read file archive contents: %w", err)
	}
	if _, err := reader.Next(); err != io.EOF {
		return "", fmt.Errorf("invalid file archive: expected end of archive, got %v", err)
	}
	return string(content), nil
}

// homepageDockerExecFailure turns a DockerExec result into an error when the
// command did not succeed. A non-zero exit code or an error status aborts the
// caller. The container message is scrubbed before it leaves the process.
func homepageDockerExecFailure(result string) error {
	status := ""
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resp); err == nil {
		status, _ = resp["status"].(string)
	}
	exitCode, _ := homepageDockerExecResult(result)
	if status != "error" && exitCode == 0 {
		return nil
	}
	message := homepageDockerExecErrorMessage(result)
	if message == "" {
		message = fmt.Sprintf("docker exec failed with exit code %d", exitCode)
	}
	return errors.New(security.Scrub(message))
}

func homepageWriteDockerFile(cfg DockerConfig, relPath, content string) error {
	if len(content) > maxHomepageWriteFileSize {
		return fmt.Errorf("content too large: %d bytes exceeds maximum of %d bytes", len(content), maxHomepageWriteFileSize)
	}
	ctx, cancel := context.WithTimeout(nativeHTTPContext(cfg.Context), time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := "/workspace/" + path.Dir(relPath)
	execResult := homepageDockerExecFunc(cfg, homepageContainerName, "mkdir -p "+shellSingleQuote(dir), "")
	if err := homepageDockerExecFailure(execResult); err != nil {
		return fmt.Errorf("create file directory: %w", err)
	}
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	if err := writer.WriteHeader(&tar.Header{Name: path.Base(relPath), Mode: 0644, Size: int64(len(content))}); err != nil {
		return fmt.Errorf("create file archive header: %w", err)
	}
	if _, err := io.Copy(writer, strings.NewReader(content)); err != nil {
		return fmt.Errorf("create file archive contents: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close file archive: %w", err)
	}
	endpoint := "/containers/" + homepageContainerName + "/archive?path=" + url.QueryEscape(dir) + "&noOverwriteDirNonDir=true"
	data, code, err := DockerRequestBytesContext(ctx, cfg, http.MethodPut, endpoint, archive.Bytes(), "application/x-tar")
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("Docker archive upload failed: HTTP %d: %s", code, dockerBodyMessage(code, data))
	}
	return nil
}
