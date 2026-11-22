package agentmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/security"
)

// DownloadAttachment retrieves an attachment from its signed HTTPS URL without
// sending the AgentMail API key to the download host.
func (c *Client) DownloadAttachment(ctx context.Context, inboxID, messageID, attachmentID string, maxBytes int64) ([]byte, *Attachment, error) {
	if maxBytes <= 0 || maxBytes > 25<<20 {
		maxBytes = 25 << 20
	}
	meta, err := c.GetAttachment(ctx, inboxID, messageID, attachmentID)
	if err != nil {
		return nil, nil, err
	}
	raw := strings.TrimSpace(meta.DownloadURL)
	if raw == "" {
		return nil, nil, fmt.Errorf("attachment download URL is missing")
	}
	u, err := attachmentDownloadURL(raw, c.baseURL)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.attachmentHTTPClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("AgentMail attachment download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("AgentMail attachment download rejected")
	}
	data, err := readBounded(resp.Body, maxBytes)
	if err != nil {
		return nil, nil, err
	}
	return data, meta, nil
}

func newAttachmentHTTPClient() *http.Client {
	return attachmentRedirectPolicy(security.NewSSRFProtectedHTTPClient(30 * time.Second))
}

func attachmentRedirectPolicy(client *http.Client) *http.Client {
	copy := *client
	checkRedirect := copy.CheckRedirect
	copy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if _, err := attachmentDownloadURL(req.URL.String(), nil); err != nil {
			return err
		}
		if checkRedirect != nil {
			return checkRedirect(req, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 attachment redirects")
		}
		return nil
	}
	return &copy
}

func attachmentDownloadURL(raw string, base *url.URL) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid attachment URL")
	}
	if !u.IsAbs() && base != nil {
		u = base.ResolveReference(u)
	}
	if !strings.EqualFold(u.Scheme, "https") || u.Host == "" || u.User != nil || u.Fragment != "" || strings.ContainsAny(u.Path, "\r\n\x00") {
		return nil, fmt.Errorf("attachment URL must be HTTPS without credentials or fragments")
	}
	if err := security.ValidatePublicHTTPURL(u.String()); err != nil {
		return nil, fmt.Errorf("attachment URL is not public HTTPS: %w", err)
	}
	return u, nil
}

// MaxAttachmentCeilingMB is AgentMail's provider ceiling for outgoing attachments
// (the mailer adapter applies the same 25 MiB bound), so no configured limit can exceed it.
const MaxAttachmentCeilingMB = 25

// PrepareAttachments encodes bounded attachments. The trusted caller supplies
// the workspace-rooted opener, including its protected-system-path policy.
func PrepareAttachments(maxAttachmentMB int, inputs []AttachmentInput, openFile func(string) (*os.File, error)) ([]OutgoingAttachment, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	// The config only rejects negative values, so clamp before multiplying: an absurd
	// size would overflow int64 and turn the limit negative.
	if maxAttachmentMB > MaxAttachmentCeilingMB {
		maxAttachmentMB = MaxAttachmentCeilingMB
	}
	maxBytes := int64(maxAttachmentMB) * 1024 * 1024
	if maxBytes < 0 {
		return nil, fmt.Errorf("invalid attachment size limit")
	}

	out := make([]OutgoingAttachment, 0, len(inputs))
	for _, input := range inputs {
		if strings.TrimSpace(input.Base64) != "" {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.Base64))
			if err != nil {
				return nil, fmt.Errorf("decode attachment %q: %w", input.Filename, err)
			}
			if err := checkAttachmentSize(int64(len(decoded)), maxBytes, input.Filename); err != nil {
				return nil, err
			}
			filename := strings.TrimSpace(input.Filename)
			if filename == "" {
				return nil, fmt.Errorf("base64 attachment requires filename")
			}
			out = append(out, OutgoingAttachment{
				Filename:      filepath.Base(filename),
				ContentType:   strings.TrimSpace(input.ContentType),
				ContentBase64: strings.TrimSpace(input.Base64),
			})
			continue
		}

		path := strings.TrimSpace(input.Path)
		if path == "" {
			continue
		}
		if openFile == nil {
			return nil, fmt.Errorf("workspace attachment reader is not configured")
		}
		data, err := func() ([]byte, error) {
			file, err := openFile(path)
			if err != nil {
				return nil, err
			}
			defer file.Close()
			info, err := file.Stat()
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("attachment must be a regular file")
			}
			if err := checkAttachmentSize(info.Size(), maxBytes, path); err != nil {
				return nil, err
			}
			data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
			if err != nil {
				return nil, err
			}
			return data, checkAttachmentSize(int64(len(data)), maxBytes, path)
		}()
		if err != nil {
			return nil, fmt.Errorf("read attachment %q: %w", path, err)
		}
		filename := strings.TrimSpace(input.Filename)
		if filename == "" {
			filename = filepath.Base(path)
		}
		contentType := strings.TrimSpace(input.ContentType)
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(filename))
		}
		out = append(out, OutgoingAttachment{
			Filename:      filepath.Base(filename),
			ContentType:   contentType,
			ContentBase64: base64.StdEncoding.EncodeToString(data),
		})
	}
	return out, nil
}

func checkAttachmentSize(size, maxBytes int64, name string) error {
	if maxBytes >= 0 && size > maxBytes {
		return fmt.Errorf("attachment %q exceeds max size", name)
	}
	return nil
}
