package tools

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

func homepageDocumentIntegrity(data []byte) (string, error) {
	if len(bytes.TrimSpace(data)) == 0 || !utf8.Valid(data) {
		return "", fmt.Errorf("document is empty or is not valid UTF-8")
	}
	if bytes.Contains(data, []byte("[output truncated due to size limit]")) {
		return "", fmt.Errorf("document contains a tool output truncation marker")
	}
	z := html.NewTokenizer(bytes.NewReader(data))
	var title strings.Builder
	htmlOpen, htmlClose, articles := 0, 0, 0
	inTitle := false
	for {
		kind := z.Next()
		if kind == html.ErrorToken {
			if err := z.Err(); err != io.EOF {
				return "", fmt.Errorf("read document structure: %w", err)
			}
			break
		}
		token := z.Token()
		if htmlClose > 0 && ((kind == html.TextToken && strings.TrimSpace(token.Data) != "") || kind == html.StartTagToken || kind == html.SelfClosingTagToken) {
			return "", fmt.Errorf("document has content after its closing HTML tag")
		}
		switch kind {
		case html.StartTagToken, html.SelfClosingTagToken:
			switch token.Data {
			case "html":
				htmlOpen++
			case "article":
				articles++
			case "title":
				inTitle = true
			}
		case html.EndTagToken:
			switch token.Data {
			case "html":
				htmlClose++
			case "article":
				articles--
				if articles < 0 {
					return "", fmt.Errorf("document has an unmatched closing article tag")
				}
			case "title":
				inTitle = false
			}
		case html.TextToken:
			if inTitle {
				title.WriteString(token.Data)
			}
		}
	}
	if articles != 0 || htmlOpen > 1 || htmlClose > 1 || htmlOpen != htmlClose {
		return "", fmt.Errorf("document is incomplete: html tags %d/%d, unclosed articles %d", htmlOpen, htmlClose, articles)
	}
	return strings.Join(strings.Fields(title.String()), " "), nil
}

func homepageReadDocument(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxHomepageWriteFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxHomepageWriteFileSize {
		return nil, fmt.Errorf("document exceeds the %d-byte verification limit", maxHomepageWriteFileSize)
	}
	return data, nil
}

func homepageStaticIndexHash(directory string) (string, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return "", err
	}
	defer root.Close()
	info, err := root.Lstat("index.html")
	if err != nil {
		return "", fmt.Errorf("static publication requires index.html: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("index.html must be a regular file, not a symbolic link")
	}
	file, err := root.Open("index.html")
	if err != nil {
		return "", err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) || info.Size() != opened.Size() || !info.ModTime().Equal(opened.ModTime()) {
		return "", fmt.Errorf("index.html changed while opening")
	}
	data, err := homepageReadDocument(file)
	if err != nil {
		return "", err
	}
	after, err := file.Stat()
	if err != nil {
		return "", err
	}
	if opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) || int64(len(data)) != opened.Size() {
		return "", fmt.Errorf("index.html changed while verifying")
	}
	if _, err := homepageDocumentIntegrity(data); err != nil {
		return "", fmt.Errorf("index.html failed integrity checks: %w", err)
	}
	return homepageDocumentHash(data), nil
}

func homepageDocumentHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
