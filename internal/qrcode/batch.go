package qrcode

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/text/unicode/norm"
)

type csvQRRow struct {
	row      int
	spec     QRSpec
	filename string
}

var csvFields = map[string]bool{}

func init() {
	for _, s := range strings.Fields("type text url filename caption font version first_name last_name formatted_name middle_name prefix suffix organization title phone phone_work phone_private phone_mobile email note birthday street city region postal_code country") {
		csvFields[s] = true
	}
}
func rowFailure(row int, err error) RowError {
	var v *ValidationError
	if errors.As(err, &v) {
		return RowError{Row: row, Field: v.Field, Code: v.Code, Message: v.Message}
	}
	if errors.Is(err, ErrVerification) {
		return RowError{Row: row, Code: "verification", Message: "QR verification failed; choose a safer style or a larger size"}
	}
	return RowError{Row: row, Code: "invalid", Message: "row could not be rendered"}
}
func parseCSVRows(ctx context.Context, data []byte, base QRSpec, opts BatchOptions) ([]csvQRRow, BatchResult, error) {
	result := BatchResult{}
	if err := ctx.Err(); err != nil {
		return nil, result, err
	}
	if len(data) == 0 || len(data) > MaxCSVBytes || !utf8.Valid(data) {
		return nil, result, invalid("csv", "limit", "CSV must be valid UTF-8 and contain 1 byte to 5 MiB")
	}
	r := csv.NewReader(contextReader{ctx, bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))})
	r.FieldsPerRecord = -1
	header, e := r.Read()
	if e != nil || len(header) == 0 || len(header) > 40 {
		return nil, result, invalid("csv", "header", "CSV requires a supported header with at most 40 columns")
	}
	seen := map[string]bool{}
	for i, key := range header {
		key = strings.ToLower(strings.TrimSpace(key))
		header[i] = key
		if !csvFields[key] || seen[key] {
			return nil, result, invalid("csv", "header", "CSV contains an unsupported or duplicate column")
		}
		seen[key] = true
	}
	kind := opts.ContentType
	if kind == "" {
		kind = base.Content.Type
	}
	if kind == "" {
		kind = "text"
	}
	if !oneOf(kind, "text", "url", "vcard") {
		return nil, result, invalid("csv.content_type", "option", "CSV content type must be text, url or vcard")
	}
	rows := []csvQRRow{}
	names := map[string]bool{}
	for {
		if e := ctx.Err(); e != nil {
			return nil, result, e
		}
		record, e := r.Read()
		if e == io.EOF {
			break
		}
		if result.Total >= MaxBatchRows {
			return nil, result, invalid("csv", "rows", "CSV exceeds 500 rows")
		}
		result.Total++
		line := result.Total + 1
		if e != nil {
			result.Errors = append(result.Errors, RowError{Row: line, Code: "csv_syntax", Message: "CSV row contains malformed quoting or fields"})
			return rows, result, nil
		}
		if len(record) != len(header) {
			result.Errors = append(result.Errors, RowError{Row: line, Code: "columns", Message: "CSV row has a different column count from the header"})
			continue
		}
		values := map[string]string{}
		for i, value := range record {
			if len(value) > MaxPayloadBytes {
				result.Errors = append(result.Errors, RowError{Row: line, Field: header[i], Code: "limit", Message: "CSV field exceeds the payload limit"})
			}
			values[header[i]] = value
		}
		rowKind := kind
		if values["type"] != "" {
			if opts.ContentType != "" && values["type"] != kind {
				result.Errors = append(result.Errors, RowError{Row: line, Field: "type", Code: "type", Message: "row type conflicts with the selected batch type"})
				continue
			}
			rowKind = values["type"]
		}
		s := base
		s.Content = Content{Type: rowKind}
		switch rowKind {
		case "text":
			s.Content.Text = values["text"]
		case "url":
			s.Content.URL = values["url"]
		case "vcard":
			v := VCardContent{Version: values["version"], FirstName: values["first_name"], LastName: values["last_name"], FormattedName: values["formatted_name"], MiddleName: values["middle_name"], Prefix: values["prefix"], Suffix: values["suffix"], Organization: values["organization"], Title: values["title"], Phone: values["phone"], PhoneWork: values["phone_work"], PhonePrivate: values["phone_private"], PhoneMobile: values["phone_mobile"], Email: values["email"], URL: values["url"], Note: values["note"], Birthday: values["birthday"]}
			a := AddressContent{Street: values["street"], City: values["city"], Region: values["region"], PostalCode: values["postal_code"], Country: values["country"]}
			if a != (AddressContent{}) {
				v.Address = &a
			}
			s.Content.VCard = &v
		default:
			result.Errors = append(result.Errors, RowError{Row: line, Field: "type", Code: "type", Message: "row type must be text, url or vcard"})
			continue
		}
		if _, ok := seen["caption"]; ok {
			if s.Frame == nil {
				s.Frame = &Frame{}
			} else {
				f := *s.Frame
				s.Frame = &f
			}
			s.Frame.Text = values["caption"]
		}
		if _, ok := seen["font"]; ok && values["font"] != "" {
			if s.Frame == nil {
				s.Frame = &Frame{}
			} else {
				f := *s.Frame
				s.Frame = &f
			}
			s.Frame.Font = values["font"]
		}
		if _, e := Payload(s.Content); e != nil {
			result.Errors = append(result.Errors, rowFailure(line, e))
			continue
		}
		name := values["filename"]
		if name == "" {
			name = fmt.Sprintf("qr-%03d", result.Total)
		}
		ext := s.Output.Format
		if ext == "" {
			ext = "png"
		}
		switch ext {
		case "jpeg":
			ext = "jpg"
		case "markdown":
			ext = "md"
		case "ascii", "unicode", "unicode_half":
			ext = "txt"
		}
		name, e = safeArchiveName(name, ext)
		if e != nil {
			result.Errors = append(result.Errors, rowFailure(line, e))
			continue
		}
		key := strings.ToLower(norm.NFC.String(name))
		if names[key] {
			result.Errors = append(result.Errors, RowError{Row: line, Field: "filename", Code: "collision", Message: "CSV filenames must be unique, including case differences"})
			continue
		}
		names[key] = true
		rows = append(rows, csvQRRow{line, s, name})
	}
	if result.Total == 0 {
		return rows, result, invalid("csv", "rows", "CSV contains no data rows")
	}
	return rows, result, nil
}
func safeArchiveName(name, ext string) (string, error) {
	if !oneOf(ext, "png", "jpg", "svg", "txt", "md") {
		return "", invalid("filename", "format", "unsupported archive output format")
	}
	name = norm.NFC.String(strings.TrimSpace(name))
	if len(name) == 0 || len(name) > 160 || strings.Contains(name, "..") || strings.ContainsAny(name, "/\\<>:\"|?*") || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return "", invalid("filename", "filename", "use a simple filename without paths, traversal or reserved characters")
	}
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", invalid("filename", "filename", "filename contains a control character")
		}
	}
	stem := strings.TrimSuffix(name, path.Ext(name))
	if strings.EqualFold(path.Ext(name), "."+ext) {
		name = stem
	}
	device := strings.ToUpper(strings.TrimRight(strings.SplitN(name, ".", 2)[0], " ."))
	if oneOf(device, "CON", "PRN", "AUX", "NUL") || len(device) > 3 && (strings.HasPrefix(device, "COM") || strings.HasPrefix(device, "LPT")) && oneOf(device[3:], "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³") {
		return "", invalid("filename", "device_name", "reserved device names are not allowed")
	}
	return name + "." + ext, nil
}

// ValidateBatch renders every row without creating an archive. Only the first
// six valid rows include a bounded visual preview; row errors remain complete.
func ValidateBatch(ctx context.Context, data []byte, base QRSpec, opts BatchOptions, renderOptions ...RenderOptions) (BatchResult, error) {
	return runBatch(ctx, data, base, opts, true, renderOptions...)
}
func Batch(ctx context.Context, data []byte, base QRSpec, opts BatchOptions, renderOptions ...RenderOptions) (BatchResult, error) {
	return runBatch(ctx, data, base, opts, false, renderOptions...)
}
func runBatch(ctx context.Context, data []byte, base QRSpec, opts BatchOptions, dryRun bool, renderOptions ...RenderOptions) (BatchResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	rows, result, err := parseCSVRows(ctx, data, base, opts)
	if err != nil {
		return result, err
	}
	buf := &limitedBuffer{limit: MaxBatchBytes}
	var archive *zip.Writer
	if !dryRun {
		archive = zip.NewWriter(buf)
	}
	totalBytes, previewBytes := 0, 0
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return BatchResult{}, err
		}
		rendered, err := Render(ctx, row.spec, renderOptions...)
		if err != nil {
			if ctx.Err() != nil {
				return BatchResult{}, ctx.Err()
			}
			result.Errors = append(result.Errors, rowFailure(row.row, err))
			continue
		}
		totalBytes += len(rendered.Data)
		if totalBytes > MaxBatchBytes {
			return BatchResult{}, invalid("csv", "output_limit", "combined batch output exceeds 128 MiB")
		}
		entry := BatchEntry{Row: row.row, Filename: row.filename, Metadata: rendered.Metadata}
		if dryRun && len(result.Entries) < 6 && strings.HasPrefix(rendered.Metadata.MIME, "image/") {
			var im image.Image
			if rendered.Metadata.Extension == "svg" {
				im, err = rasterizeSVG(ctx, rendered.Data, true)
			} else {
				im, _, err = decodeRaster(ctx, rendered.Data)
			}
			if err == nil {
				bounds := im.Bounds()
				w, h := bounds.Dx(), bounds.Dy()
				if w > 160 || h > 160 {
					if w >= h {
						h = max(1, h*160/w)
						w = 160
					} else {
						w = max(1, w*160/h)
						h = 160
					}
				}
				thumb := image.NewNRGBA(image.Rect(0, 0, w, h))
				xdraw.ApproxBiLinear.Scale(thumb, thumb.Bounds(), im, bounds, draw.Src, nil)
				raw, e := encodePNG(thumb, 256<<10)
				if e == nil && previewBytes+len(raw) <= 1<<20 {
					entry.Preview = &ImageResource{MIME: "image/png", Data: raw}
					previewBytes += len(raw)
				}
			}
		}
		result.Entries = append(result.Entries, entry)
		if archive != nil {
			header := &zip.FileHeader{Name: row.filename, Method: zip.Deflate}
			header.SetMode(0600)
			file, e := archive.CreateHeader(header)
			if e != nil {
				return BatchResult{}, invalid("csv", "archive", "archive output exceeds its limit")
			}
			if _, e = file.Write(rendered.Data); e != nil {
				return BatchResult{}, invalid("csv", "archive", "archive output exceeds its limit")
			}
		}
	}
	if archive != nil {
		if err := archive.Close(); err != nil {
			return BatchResult{}, invalid("csv", "archive", "archive output exceeds its limit")
		}
	}
	if err := ctx.Err(); err != nil {
		return BatchResult{}, err
	}
	if len(result.Errors) > 0 {
		if dryRun {
			return result, nil
		}
		return result, ErrBatchInvalid
	}
	if !dryRun {
		result.Data = buf.Bytes()
	}
	return result, nil
}
