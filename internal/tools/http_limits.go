package tools

import (
	"fmt"
	"io"
)

const maxHTTPResponseSize int64 = 10 * 1024 * 1024

func readHTTPResponseBody(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = maxHTTPResponseSize
	}
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response body exceeds limit of %d bytes", limit)
	}
	return data, nil
}
