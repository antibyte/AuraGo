package services

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

const (
	maxIndexedFileBytes    int64 = 32 << 20
	maxIndexedContentBytes       = 500 << 10
	maxArchiveMemberBytes  int64 = 8 << 20
)

func shouldSkipIndexingFile(info os.FileInfo) (bool, string) {
	if info == nil {
		return true, "file metadata unavailable"
	}
	if info.Size() > maxIndexedFileBytes {
		return true, fmt.Sprintf("file exceeds %d byte indexing limit", maxIndexedFileBytes)
	}
	return false, ""
}

func readIndexedTextFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	data, err := readLimitedAll(file, maxIndexedFileBytes)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func readLimitedAll(r io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("content exceeds %d byte limit", maxBytes)
	}
	return data, nil
}

func limitIndexedContent(content string) (string, bool) {
	if len(content) <= maxIndexedContentBytes {
		return content, false
	}
	return truncateIndexedContentUTF8(content, maxIndexedContentBytes), true
}

func truncateIndexedContentUTF8(content string, maxBytes int) string {
	if len(content) <= maxBytes {
		return content
	}
	for maxBytes > 0 && !utf8.ValidString(content[:maxBytes]) {
		maxBytes--
	}
	return content[:maxBytes]
}
