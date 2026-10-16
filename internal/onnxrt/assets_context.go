package onnxrt

import (
	"context"
	"io"
)

type assetContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *assetContextReader) Read(data []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	if len(data) > 1<<20 {
		data = data[:1<<20]
	}
	return reader.reader.Read(data)
}
