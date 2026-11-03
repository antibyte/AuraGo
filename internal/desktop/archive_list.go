package desktop

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// ArchiveListEntry is one member returned by the desktop archive browser.
type ArchiveListEntry struct {
	Name           string
	Size           int64
	CompressedSize int64
	IsDir          bool
	ModTime        time.Time
	Link           bool
}

// ArchiveListing is a bounded view of one workspace archive.
type ArchiveListing struct {
	Format       string
	Entries      []ArchiveListEntry
	SkippedLinks int
}

// ListArchive reads archive members without writing them into the workspace.
func (s *Service) ListArchive(ctx context.Context, rawPath string) (ArchiveListing, error) {
	if err := s.ensureReady(ctx); err != nil {
		return ArchiveListing{}, err
	}
	src, err := s.resolveWorkspacePathNoSymlinks(rawPath, false)
	if err != nil {
		return ArchiveListing{}, err
	}
	root, err := os.OpenRoot(s.Config().WorkspaceDir)
	if err != nil {
		return ArchiveListing{}, err
	}
	defer root.Close()
	rel, err := filepath.Rel(s.Config().WorkspaceDir, src)
	if err != nil {
		return ArchiveListing{}, err
	}
	input, err := root.Open(rel)
	if err != nil {
		return ArchiveListing{}, err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return ArchiveListing{}, err
	}
	if !info.Mode().IsRegular() {
		return ArchiveListing{}, fmt.Errorf("archive is not a regular file")
	}
	kind, err := detectArchiveKind(filepath.Base(src), io.NewSectionReader(input, 0, info.Size()))
	if err != nil {
		return ArchiveListing{}, err
	}
	switch {
	case kind == archiveZip:
		return listZip(input, info.Size())
	case kind == archiveUnknown:
		return ArchiveListing{}, errArchiveUnsupported
	case kind.native():
		members, err := nativeMembers(ctx, io.NewSectionReader(input, 0, info.Size()), kind, filepath.Base(src), false)
		if err != nil {
			return ArchiveListing{}, err
		}
		return listingFromMembers(kind, members)
	default:
		input.Close()
		return s.listHost(ctx, kind, src)
	}
}

func listZip(readerAt io.ReaderAt, size int64) (ArchiveListing, error) {
	reader, err := zip.NewReader(readerAt, size)
	if err != nil {
		return ArchiveListing{}, fmt.Errorf("failed to open zip file: %w", err)
	}
	if len(reader.File) > ArchiveEntryLimit {
		return ArchiveListing{}, ErrArchiveTooManyEntries
	}
	listing := ArchiveListing{Format: archiveZip.String(), Entries: make([]ArchiveListEntry, 0, len(reader.File))}
	for _, file := range reader.File {
		listing.Entries = append(listing.Entries, ArchiveListEntry{
			Name:           file.Name,
			Size:           int64(file.UncompressedSize64),
			CompressedSize: int64(file.CompressedSize64),
			IsDir:          file.FileInfo().IsDir(),
			ModTime:        file.Modified,
		})
	}
	return listing, nil
}
