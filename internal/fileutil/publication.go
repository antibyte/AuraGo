package fileutil

import (
	"context"
	"os"
	"path/filepath"
)

type publicationGateKey struct{}

// WithPublicationGate lets an owner serialize a bounded local commit against
// revocation. The gate must invoke commit at most once and must not call another
// gated operation. It grants no path or filesystem permission.
func WithPublicationGate(ctx context.Context, gate func(commit func() error) error) context.Context {
	return context.WithValue(ctx, publicationGateKey{}, gate)
}

// PublishContext checks cancellation at the commit boundary, under the optional
// owner gate. Do not put network I/O or other gated operations in commit.
func PublishContext(ctx context.Context, commit func() error) error {
	checked := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return commit()
	}
	if gate, ok := ctx.Value(publicationGateKey{}).(func(func() error) error); ok {
		return gate(checked)
	}
	return checked()
}

// WriteFileContext publishes a complete synced file. The caller owns path
// authorization and destination serialization, as with RenameContext.
func WriteFileContext(ctx context.Context, path string, data []byte, mode os.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".aurago-publish-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = f.Chmod(mode); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return RenameContext(ctx, f.Name(), path)
}
