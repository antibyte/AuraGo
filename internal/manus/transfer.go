package manus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// File is the safe metadata Manus returns for an uploaded file.
type File struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

type uploadTicket struct {
	OK        bool   `json:"ok"`
	RequestID string `json:"request_id"`
	File      File   `json:"file"`
	UploadURL string `json:"upload_url"`
	ExpiresAt int64  `json:"upload_expires_at"`
}

// UploadLocalFile creates a Manus file record and completes its presigned PUT.
func (c *Client) UploadLocalFile(ctx context.Context, local LocalFile) (File, error) {
	if local.handle == nil {
		return File{}, fmt.Errorf("Manus upload file is not backed by a validated workspace handle")
	}
	defer local.handle.Close()
	var ticket uploadTicket
	if err := c.doJSON(ctx, http.MethodPost, "/v2/file.upload", nil, map[string]string{"filename": local.Filename}, &ticket); err != nil {
		return File{}, err
	}
	remoteURL, err := ValidateRemoteFileURL(ticket.UploadURL)
	if err != nil {
		return File{}, err
	}
	if err := validatePublicResolution(ctx, remoteURL); err != nil {
		return File{}, err
	}
	if _, err := local.handle.Seek(0, io.SeekStart); err != nil {
		return File{}, fmt.Errorf("reset Manus upload file: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, remoteURL.String(), local.handle)
	if err != nil {
		return File{}, fmt.Errorf("create Manus presigned upload request: %w", err)
	}
	req.ContentLength = local.Size
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := c.fileHTTPClient.Do(req)
	if err != nil {
		return File{}, fmt.Errorf("upload file to Manus storage: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return File{}, fmt.Errorf("Manus storage upload returned HTTP %d", resp.StatusCode)
	}
	return ticket.File, nil
}

// DownloadAttachment retrieves one tracked-task attachment into bounded memory.
// The runtime is solely responsible for writing the bytes into the workspace.
func (c *Client) DownloadAttachment(ctx context.Context, attachment TaskAttachment, maxBytes int64) ([]byte, error) {
	remoteURL, err := ValidateRemoteFileURL(attachment.URL)
	if err != nil {
		return nil, err
	}
	if err := validatePublicResolution(ctx, remoteURL); err != nil {
		return nil, err
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("Manus download size limit must be positive")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Manus attachment request: %w", err)
	}
	resp, err := c.fileHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download Manus attachment: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Manus attachment download returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return nil, fmt.Errorf("Manus attachment exceeds the configured size limit")
	}
	payload, err := readBounded(resp.Body, maxBytes)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func createUniqueAttachment(root *os.Root, dir, name string) (string, *os.File, error) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for index := 0; index < 1000; index++ {
		candidateName := name
		if index > 0 {
			candidateName = stem + "-" + strconv.Itoa(index) + ext
		}
		candidate := filepath.Join(dir, candidateName)
		file, err := root.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return candidate, file, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", nil, fmt.Errorf("create Manus attachment: %w", err)
		}
	}
	return "", nil, fmt.Errorf("could not allocate a unique Manus attachment filename")
}

func cloneSecureFileClient(source *http.Client) *http.Client {
	clone := *source
	previous := clone.CheckRedirect
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("Manus file redirect limit exceeded")
		}
		remoteURL, err := ValidateRemoteFileURL(req.URL.String())
		if err != nil {
			return err
		}
		if err := validatePublicResolution(req.Context(), remoteURL); err != nil {
			return err
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
	return &clone
}

func validatePublicResolution(ctx context.Context, remoteURL *url.URL) error {
	if ip := net.ParseIP(remoteURL.Hostname()); ip != nil {
		if !isPublicIP(ip) {
			return fmt.Errorf("Manus file host is private or local")
		}
		return nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, remoteURL.Hostname())
	if err != nil {
		return fmt.Errorf("resolve Manus file host: %w", err)
	}
	if len(addresses) == 0 {
		return fmt.Errorf("Manus file host has no addresses")
	}
	for _, address := range addresses {
		if !isPublicIP(address.IP) {
			return fmt.Errorf("Manus file host resolves to a private or local address")
		}
	}
	return nil
}
