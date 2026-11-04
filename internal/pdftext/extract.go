package pdftext

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	pdf "aurago/internal/pdftext/parser"
)

const MaxInputBytes = pdf.MaxInputBytes

var ErrLimit = pdf.ErrLimit
var ErrNoText = errors.New("PDF contains no extractable text (possibly scanned/image-based)")

// Extract reads a bounded file handle; it never returns partial text on failure.
func Extract(ctx context.Context, input io.ReaderAt, size int64) (text string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	defer func() {
		if cause := recover(); cause != nil {
			text = ""
			if e, ok := cause.(error); ok {
				err = fmt.Errorf("parse PDF: %w", e)
			} else {
				err = errors.New("malformed PDF")
			}
		}
	}()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if size > MaxInputBytes {
		return "", fmt.Errorf("%w: input bytes", ErrLimit)
	}
	if size < 0 || input == nil {
		return "", errors.New("invalid PDF input")
	}
	reader, err := pdf.NewReaderContext(ctx, io.NewSectionReader(input, 0, size), size)
	if err != nil {
		return "", fmt.Errorf("open PDF: %w", err)
	}
	pages := reader.NumPage()
	var result strings.Builder
	for i := 1; i <= pages; i++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		content, err := page.GetPlainText(nil)
		if terminal := reader.Err(); terminal != nil {
			return "", terminal
		}
		if err != nil {
			return "", fmt.Errorf("read PDF page: %w", err)
		}
		if len(content)+1 > pdf.MaxTextBytes-result.Len() {
			return "", fmt.Errorf("%w: aggregate extracted text", ErrLimit)
		}
		result.WriteString(content)
		result.WriteByte('\n')
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	text = strings.TrimSpace(result.String())
	if text == "" {
		return "", ErrNoText
	}
	return text, nil
}
