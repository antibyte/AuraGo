package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/security"
)

const tregMediaLimit = 256 << 20
const tregUploadLimit = 32 << 20

type tregUpload struct {
	Field       string `json:"field"`
	Path        string `json:"path"`
	ContentType string `json:"content_type"`
}

// Only explicit artifact fields are fetched. Ordinary search-result links are
// never followed. Provider-native shapes are considered only for media catalog
// platforms or a descriptor-authorized final fetch.
func tregMediaPayload(data any, platform string) any {
	if media := tregPath(data, "media"); media != nil {
		return media
	}
	if platform != "image-gen" && platform != "video-gen" && platform != "voice-gen" {
		return nil
	}
	for _, path := range []string{"file.download_url", "data.audio", "data.image_urls", "image_urls", "images", "choices.0.message.images", "candidates.0.content.parts"} {
		if media := tregPath(data, path); media != nil {
			return media
		}
	}
	if media, ok := tregPath(data, "data").([]any); ok {
		return media
	}
	return nil
}

func (c *TregClient) resultMedia(ctx context.Context, value any) (MediaItem, error) {
	if err := ctx.Err(); err != nil {
		return MediaItem{}, err
	}
	if _, err := c.policy(); err != nil {
		return MediaItem{}, err
	}
	raw, _ := value.(string)
	encoded, contentType := "", ""
	if obj, ok := value.(map[string]any); ok {
		raw, _ = obj["url"].(string)
		encoded, _ = obj["b64_json"].(string)
		if nested, ok := obj["image_url"].(map[string]any); ok {
			raw, _ = nested["url"].(string)
		}
		for _, key := range []string{"inlineData", "inline_data"} {
			if inline, ok := obj[key].(map[string]any); ok {
				encoded, _ = inline["data"].(string)
				contentType, _ = inline["mimeType"].(string)
				if contentType == "" {
					contentType, _ = inline["mime_type"].(string)
				}
			}
		}
	}
	if strings.HasPrefix(raw, "data:") {
		header, payload, ok := strings.Cut(strings.TrimPrefix(raw, "data:"), ",")
		if !ok || !strings.HasSuffix(header, ";base64") {
			return MediaItem{}, fmt.Errorf("unsupported inline media encoding")
		}
		contentType, encoded = strings.TrimSuffix(header, ";base64"), payload
	}
	if encoded != "" {
		if len(encoded) > tregJSONLimit {
			return MediaItem{}, fmt.Errorf("inline media exceeds the JSON response limit")
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return MediaItem{}, fmt.Errorf("decode inline media: %w", err)
		}
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		if tregMediaType(contentType) == "" {
			return MediaItem{}, fmt.Errorf("unsupported inline media content type")
		}
		return c.storeMedia(bytes.NewReader(data), contentType)
	}
	if raw == "" {
		return MediaItem{}, fmt.Errorf("missing result media URL or inline image")
	}
	return c.downloadMedia(ctx, raw)
}

func (c *TregClient) encodeForm(ep TregEndpoint, p tregParameters, query url.Values) (url.Values, io.Reader, string, error) {
	form := url.Values{}
	for field, v := range p.Form {
		values, err := tregValues(v)
		if err != nil {
			return nil, nil, "", fmt.Errorf("form field %s: %w", field, err)
		}
		form[field] = values
	}
	if ep.Input.BodyType == "form" || ep.Input.BodyType == "urlencoded" || ep.Input.BodyType == "form-urlencoded" {
		if len(p.Uploads) != 0 {
			return nil, nil, "", fmt.Errorf("this endpoint does not accept file uploads")
		}
		return query, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", nil
	}
	if ep.Input.BodyType != "multipart" && ep.Input.BodyType != "multipart/form-data" {
		return nil, nil, "", fmt.Errorf("this endpoint does not declare form/multipart input")
	}
	if len(p.Uploads) > 20 {
		return nil, nil, "", fmt.Errorf("at most 20 uploads per call")
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for field, values := range form {
		if strings.ContainsAny(field, "\r\n") {
			return nil, nil, "", fmt.Errorf("invalid form field")
		}
		for _, value := range values {
			if err := w.WriteField(field, value); err != nil {
				return nil, nil, "", fmt.Errorf("encode form: %w", err)
			}
		}
	}
	for _, upload := range p.Uploads {
		if upload.Field == "" || strings.ContainsAny(upload.Field, "\r\n") {
			return nil, nil, "", fmt.Errorf("invalid upload field")
		}
		if c.WorkspaceDir == "" {
			return nil, nil, "", fmt.Errorf("upload workspace is unavailable")
		}
		resolved, err := secureResolve(c.WorkspaceDir, upload.Path)
		if err != nil {
			return nil, nil, "", fmt.Errorf("upload path: %w", err)
		}
		resolved, err = filepath.EvalSymlinks(resolved)
		if err != nil {
			return nil, nil, "", fmt.Errorf("resolve upload: %w", err)
		}
		_, root := filesystemRoots(c.WorkspaceDir)
		rel, err := filepath.Rel(root, resolved)
		if err != nil || !filepath.IsLocal(rel) {
			return nil, nil, "", fmt.Errorf("upload escapes the workspace")
		}
		if err := requireUnprotectedNotesPath(resolved, false); err != nil {
			return nil, nil, "", err
		}
		name := strings.ToLower(filepath.Base(resolved))
		if strings.HasPrefix(name, ".env") || name == "aurago_master.key" || name == "vault.bin" {
			return nil, nil, "", fmt.Errorf("protected upload path")
		}
		if c.ValidateUpload != nil {
			if err := c.ValidateUpload(resolved); err != nil {
				return nil, nil, "", err
			}
		}
		// Root.Open also rejects a symlink replacement that escapes the jail.
		r, err := os.OpenRoot(root)
		if err != nil {
			return nil, nil, "", fmt.Errorf("open upload root: %w", err)
		}
		f, err := r.Open(rel)
		_ = r.Close()
		if err != nil {
			return nil, nil, "", fmt.Errorf("open upload: %w", err)
		}
		info, statErr := f.Stat()
		if statErr != nil || !info.Mode().IsRegular() || info.Size() > tregUploadLimit {
			_ = f.Close()
			return nil, nil, "", fmt.Errorf("upload must be a regular file of at most 32 MiB")
		}
		typ := upload.ContentType
		if typ == "" {
			typ = mime.TypeByExtension(filepath.Ext(resolved))
		}
		if typ == "" {
			typ = "application/octet-stream"
		}
		if _, _, err := mime.ParseMediaType(typ); err != nil || strings.ContainsAny(typ, "\r\n") {
			_ = f.Close()
			return nil, nil, "", fmt.Errorf("invalid upload content type")
		}
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": upload.Field, "filename": filepath.Base(resolved)}))
		h.Set("Content-Type", typ)
		part, err := w.CreatePart(h)
		if err != nil {
			_ = f.Close()
			return nil, nil, "", fmt.Errorf("create upload part: %w", err)
		}
		n, err := io.Copy(part, io.LimitReader(f, tregUploadLimit+1))
		_ = f.Close()
		if err != nil || n > tregUploadLimit || buf.Len() > 64<<20 {
			return nil, nil, "", fmt.Errorf("upload failed or exceeds the 64 MiB total limit")
		}
	}
	if err := w.Close(); err != nil {
		return nil, nil, "", fmt.Errorf("finish multipart upload: %w", err)
	}
	return query, &buf, w.FormDataContentType(), nil
}

func tregMediaType(typ string) string {
	switch typ {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/avif":
		return "image"
	case "audio/mpeg", "audio/wav", "audio/x-wav", "audio/ogg", "audio/flac", "audio/mp4", "audio/webm":
		return "audio"
	case "video/mp4", "video/webm", "video/quicktime":
		return "video"
	case "application/pdf", "application/octet-stream":
		return "document"
	default:
		return ""
	}
}

func (c *TregClient) storeMedia(reader io.Reader, contentType string) (MediaItem, error) {
	var item MediaItem
	if c.DataDir == "" {
		return item, fmt.Errorf("media storage is unavailable")
	}
	kind := tregMediaType(contentType)
	if kind == "" {
		return item, fmt.Errorf("unsupported media content type")
	}
	extensions, _ := mime.ExtensionsByType(contentType)
	ext := ".bin"
	if len(extensions) != 0 {
		ext = extensions[0]
	}
	// Extensions are selected from a MIME allowlist, never a provider filename.
	rel := filepath.Join("treg_media", time.Now().UTC().Format("2006/01/02"))
	dir := filepath.Join(c.DataDir, rel)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return item, fmt.Errorf("create treg media directory: %w", err)
	}
	f, err := os.CreateTemp(dir, ".incoming-*")
	if err != nil {
		return item, fmt.Errorf("create treg media: %w", err)
	}
	temp := f.Name()
	defer os.Remove(temp)
	n, err := io.Copy(f, io.LimitReader(reader, tregMediaLimit+1))
	closeErr := f.Close()
	if err != nil {
		return item, fmt.Errorf("write treg media: %w", err)
	}
	if closeErr != nil {
		return item, fmt.Errorf("close treg media: %w", closeErr)
	}
	if n == 0 || n > tregMediaLimit {
		return item, fmt.Errorf("treg media is empty or exceeds 256 MiB")
	}
	filename := tregRandomID() + ext
	local := filepath.Join(dir, filename)
	if err := os.Rename(temp, local); err != nil {
		return item, fmt.Errorf("publish treg media: %w", err)
	}
	item = MediaItem{MediaType: kind, SourceTool: "treg_call", Provider: "treg", Filename: filename, FilePath: local, WebPath: "/files/" + filepath.ToSlash(filepath.Join(rel, filename)), FileSize: n, Format: strings.TrimPrefix(ext, ".")}
	// Unique generated paths remain valid even when identical media is registered twice.
	if c.MediaDB != nil {
		id, _, err := RegisterMedia(c.MediaDB, item)
		if err != nil {
			_ = os.Remove(local)
			return MediaItem{}, fmt.Errorf("register treg media: %w", err)
		}
		item.ID = id
	}
	return item, nil
}

func (c *TregClient) downloadMedia(ctx context.Context, rawURL string) (MediaItem, error) {
	if _, err := c.policy(); err != nil {
		return MediaItem{}, err
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" {
		return MediaItem{}, fmt.Errorf("invalid treg media URL")
	}
	client, err := security.NewStrictPublicHTTPClientForURL(rawURL, 90*time.Second)
	if err != nil {
		return MediaItem{}, fmt.Errorf("media destination: %w", err)
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return MediaItem{}, fmt.Errorf("build media request: %w", err)
	}
	// Intentionally no treg token, provider key or user-controlled headers here.
	resp, err := client.Do(req)
	if err != nil {
		return MediaItem{}, fmt.Errorf("download treg media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return MediaItem{}, fmt.Errorf("media download HTTP %d", resp.StatusCode)
	}
	typ, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	return c.storeMedia(resp.Body, typ)
}
