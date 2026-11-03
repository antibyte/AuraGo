package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const defaultJSONBodyLimit int64 = 10 << 20

type jsonBodyLimitKey struct{}

func (s *Server) jsonBodyLimit() int64 {
	if s != nil {
		if cfg := s.ConfigSnapshot(); cfg != nil && cfg.Server.MaxBodyBytes > 0 {
			return cfg.Server.MaxBodyBytes
		}
	}
	return defaultJSONBodyLimit
}

func withJSONBodyLimit(ctx context.Context, limit int64) context.Context {
	return context.WithValue(ctx, jsonBodyLimitKey{}, limit)
}

// decodeJSONRequest is for ordinary JSON API bodies. Upload/stream decoders
// retain their explicit budgets. An existing smaller MaxBytesReader still wins.
// Reading through the end rejects extra documents and over-limit whitespace.
func decodeJSONRequest(w http.ResponseWriter, r *http.Request, dst any, owner ...*Server) error {
	limit, _ := r.Context().Value(jsonBodyLimitKey{}).(int64)
	if len(owner) > 0 {
		limit = owner[0].jsonBodyLimit()
	}
	if limit <= 0 {
		limit = defaultJSONBodyLimit
	}
	if r.Body == nil {
		return io.EOF
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return io.EOF
	}
	return json.Unmarshal(data, dst)
}

func decodeOptionalJSONRequest(w http.ResponseWriter, r *http.Request, dst any, owner ...*Server) error {
	err := decodeJSONRequest(w, r, dst, owner...)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func jsonDecodeStatus(err error) int {
	var oversized *http.MaxBytesError
	if errors.As(err, &oversized) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}
