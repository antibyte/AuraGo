package desktop

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

type archiveSourceFile struct {
	name string
	data []byte
}

type archiveMember struct {
	Name       string
	Size       int64
	Compressed int64
	IsDir      bool
	Link       bool
	Special    bool
	ModTime    time.Time
	Read       func(budget int64) ([]byte, error)
}

func encodeZip(files []archiveSourceFile) ([]byte, error) {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, file := range files {
		output, err := writer.Create(file.name)
		if err != nil {
			return nil, err
		}
		if _, err := output.Write(file.data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeTar(files []archiveSourceFile, kind archiveKind) ([]byte, error) {
	var buf bytes.Buffer
	closer, err := tarCompressor(&buf, kind)
	if err != nil {
		return nil, err
	}
	writer := tar.NewWriter(closer)
	for _, file := range files {
		header := &tar.Header{
			Name:     file.name,
			Mode:     0o644,
			Size:     int64(len(file.data)),
			ModTime:  time.Unix(0, 0).UTC(),
			Format:   tar.FormatPAX,
			Typeflag: tar.TypeReg,
		}
		if err := writer.WriteHeader(header); err != nil {
			closer.Close()
			return nil, err
		}
		if _, err := writer.Write(file.data); err != nil {
			closer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		closer.Close()
		return nil, err
	}
	if err := closer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

func tarCompressor(dst io.Writer, kind archiveKind) (io.WriteCloser, error) {
	switch kind {
	case archiveTarGz:
		return gzip.NewWriter(dst), nil
	case archiveTarXz:
		return xz.NewWriter(dst)
	case archiveTarZst:
		return zstd.NewWriter(dst)
	default:
		return nopWriteCloser{dst}, nil
	}
}

func payloadName(filename string) string {
	base := filepath.Base(filename)
	lower := strings.ToLower(base)
	for _, suffix := range []string{".gz", ".bz2", ".xz", ".zst", ".lzma", ".lz"} {
		if strings.HasSuffix(lower, suffix) && len(base) > len(suffix) {
			stem := base[:len(base)-len(suffix)]
			if stem != "" && !strings.ContainsAny(stem, `/\`) {
				return stem
			}
		}
	}
	return "content"
}

func compressedLooksLikeTar(rs io.ReadSeeker, kind archiveKind) bool {
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return false
	}
	defer rs.Seek(0, io.SeekStart)
	reader, err := openNativeReader(io.LimitReader(rs, 1<<20), kind)
	if err != nil {
		return false
	}
	defer reader.Close()
	if named, ok := reader.(interface{ Name() string }); ok {
		if strings.HasSuffix(strings.ToLower(named.Name()), ".tar") {
			return true
		}
	}
	buf := make([]byte, 512)
	n, _ := io.ReadFull(reader, buf)
	return n >= 262 && bytes.Equal(buf[257:262], []byte("ustar"))
}

type namedReadCloser interface {
	io.ReadCloser
	Name() string
}

type gzipNamedReader struct{ *gzip.Reader }

func (g gzipNamedReader) Name() string { return g.Header.Name }

type zstdReadCloser struct{ *zstd.Decoder }

func (z zstdReadCloser) Close() error {
	z.Decoder.Close()
	return nil
}

func openNativeReader(r io.Reader, kind archiveKind) (io.ReadCloser, error) {
	switch kind {
	case archiveTar:
		return io.NopCloser(r), nil
	case archiveTarGz, archiveGz:
		reader, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		return gzipNamedReader{reader}, nil
	case archiveTarBz2, archiveBz2:
		return io.NopCloser(bzip2.NewReader(r)), nil
	case archiveTarXz, archiveXz:
		reader, err := xz.NewReader(r)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(reader), nil
	case archiveTarZst, archiveZst:
		reader, err := zstd.NewReader(r)
		if err != nil {
			return nil, err
		}
		return zstdReadCloser{reader}, nil
	default:
		return nil, errArchiveUnsupported
	}
}

func nativeMembers(ctx context.Context, rs io.ReadSeeker, kind archiveKind, filename string, keep bool) ([]archiveMember, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	reader, err := openNativeReader(rs, kind)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer reader.Close()
	if kind.singleFile() {
		return readSingleMember(reader, payloadName(filename), keep)
	}
	return readTarMembers(ctx, reader, keep)
}

func readSingleMember(r io.Reader, name string, keep bool) ([]archiveMember, error) {
	var data []byte
	var size int64
	if keep {
		var err error
		data, err = readLimited(r, archiveExpansionLimit)
		if err != nil {
			return nil, err
		}
		size = int64(len(data))
	} else {
		counted := &countReader{r: r}
		if _, err := io.Copy(io.Discard, io.LimitReader(counted, archiveExpansionLimit+1)); err != nil {
			return nil, err
		}
		if counted.n > archiveExpansionLimit {
			return nil, fmt.Errorf("archive expanded size exceeds limit")
		}
		size = counted.n
	}
	member := archiveMember{Name: name, Size: size}
	if keep {
		payload := data
		member.Read = func(int64) ([]byte, error) { return payload, nil }
	}
	return []archiveMember{member}, nil
}

type countReader struct {
	r io.Reader
	n int64
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

type priorMember struct {
	size    int64
	data    []byte
	dir     bool
	blocked bool
}

func archiveMemberKey(name string) string {
	name = strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "./")
	return strings.TrimSuffix(name, "/")
}

func readTarMembers(ctx context.Context, r io.Reader, keep bool) ([]archiveMember, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	reader := tar.NewReader(r)
	members := make([]archiveMember, 0)
	var used int64
	priors := map[string]priorMember{}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		header, err := reader.Next()
		if err == io.EOF {
			return members, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if len(members) >= ArchiveEntryLimit {
			return nil, ErrArchiveTooManyEntries
		}
		switch header.Typeflag {
		case tar.TypeXHeader, tar.TypeXGlobalHeader, tar.TypeGNULongName, tar.TypeGNULongLink:
			if err := discardEntry(reader, header.Size); err != nil {
				return nil, err
			}
			continue
		}
		member := archiveMember{
			Name:    strings.TrimPrefix(header.Name, "./"),
			Size:    header.Size,
			ModTime: header.ModTime,
		}
		var payload []byte
		switch header.Typeflag {
		case tar.TypeDir:
			member.IsDir = true
			member.Size = 0
			if err := discardEntry(reader, header.Size); err != nil {
				return nil, err
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || used > archiveExpansionLimit-header.Size {
				return nil, fmt.Errorf("archive expanded size exceeds limit")
			}
			if keep {
				data, err := readLimited(reader, header.Size)
				if err != nil {
					return nil, err
				}
				if int64(len(data)) != header.Size && header.Size > 0 {
					return nil, fmt.Errorf("read archive: short entry")
				}
				payload = data
				member.Read = func(int64) ([]byte, error) { return payload, nil }
				member.Size = int64(len(payload))
				used += member.Size
			} else if err := discardEntry(reader, header.Size); err != nil {
				return nil, err
			} else {
				used += header.Size
			}
		case tar.TypeLink:
			if err := discardEntry(reader, header.Size); err != nil {
				return nil, err
			}
			prior, ok := priors[archiveMemberKey(header.Linkname)]
			if !ok || prior.blocked || prior.dir {
				member.Link = true
				member.Size = 0
				break
			}
			if prior.size < 0 || used > archiveExpansionLimit-prior.size {
				return nil, fmt.Errorf("archive expanded size exceeds limit")
			}
			member.Size = prior.size
			if keep {
				if prior.data == nil {
					member.Link = true
					member.Size = 0
					break
				}
				payload = append([]byte(nil), prior.data...)
				member.Read = func(int64) ([]byte, error) { return payload, nil }
				member.Size = int64(len(payload))
				used += member.Size
			} else {
				used += prior.size
			}
		case tar.TypeSymlink:
			member.Link = true
			member.Size = 0
			if err := discardEntry(reader, header.Size); err != nil {
				return nil, err
			}
		default:
			member.Special = true
			member.Size = 0
			if err := discardEntry(reader, header.Size); err != nil {
				return nil, err
			}
		}
		members = append(members, member)
		if key := archiveMemberKey(member.Name); key != "" {
			priors[key] = priorMember{size: member.Size, data: payload, dir: member.IsDir, blocked: member.Link || member.Special}
		}
	}
}

func discardEntry(r io.Reader, size int64) error {
	if size < 0 || size > archiveExpansionLimit {
		return fmt.Errorf("archive expanded size exceeds limit")
	}
	n, err := io.Copy(io.Discard, io.LimitReader(r, size))
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("read archive: short entry")
	}
	return nil
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	if limit < 0 {
		return nil, fmt.Errorf("archive expanded size exceeds limit")
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("archive expanded size exceeds limit")
	}
	return data, nil
}

func (s *Service) extractNative(ctx context.Context, kind archiveKind, filename string, rs io.ReadSeeker, dst, source string, root *os.Root) error {
	members, err := nativeMembers(ctx, rs, kind, filename, true)
	if err != nil {
		return err
	}
	return s.publishMembers(ctx, kind, members, dst, source, root)
}

func readNativePreview(ctx context.Context, rs io.ReadSeeker, kind archiveKind, filename, want string, maxBytes int64) (ArchiveEntry, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if maxBytes < 0 {
		return ArchiveEntry{}, ErrArchiveEntryTooLarge
	}
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return ArchiveEntry{}, err
	}
	reader, err := openNativeReader(rs, kind)
	if err != nil {
		return ArchiveEntry{}, fmt.Errorf("open archive: %w", err)
	}
	defer reader.Close()
	if kind.singleFile() {
		if payloadName(filename) != want {
			return ArchiveEntry{}, ErrArchiveEntryNotFound
		}
		data, err := readPreviewBytes(reader, maxBytes)
		if err != nil {
			return ArchiveEntry{}, err
		}
		return previewEntry(want, data, time.Time{}), nil
	}
	tarReader := tar.NewReader(reader)
	var used int64
	var seen int
	remembered := map[string][]byte{}
	sizes := map[string]int64{}
	var rememberedBytes int64
	for {
		if err := ctx.Err(); err != nil {
			return ArchiveEntry{}, err
		}
		header, err := tarReader.Next()
		if err == io.EOF {
			return ArchiveEntry{}, ErrArchiveEntryNotFound
		}
		if err != nil {
			return ArchiveEntry{}, fmt.Errorf("read archive: %w", err)
		}
		switch header.Typeflag {
		case tar.TypeXHeader, tar.TypeXGlobalHeader, tar.TypeGNULongName, tar.TypeGNULongLink:
			if err := discardEntry(tarReader, header.Size); err != nil {
				return ArchiveEntry{}, err
			}
			continue
		}
		seen++
		if seen > ArchiveEntryLimit {
			return ArchiveEntry{}, ErrArchiveTooManyEntries
		}
		name := archiveMemberKey(header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := discardEntry(tarReader, header.Size); err != nil {
				return ArchiveEntry{}, err
			}
			if name == want {
				return ArchiveEntry{}, ErrArchiveEntryIsDirectory
			}
		case tar.TypeReg, tar.TypeRegA:
			sizes[name] = header.Size
			if name == want {
				if header.Size > maxBytes {
					return ArchiveEntry{}, ErrArchiveEntryTooLarge
				}
				data, err := readLimited(tarReader, header.Size)
				if err != nil {
					return ArchiveEntry{}, err
				}
				if int64(len(data)) != header.Size && header.Size > 0 {
					return ArchiveEntry{}, fmt.Errorf("read archive: short entry")
				}
				return previewEntry(want, data, header.ModTime), nil
			}
			if validArchivePath(name) && header.Size >= 0 && header.Size <= maxBytes && rememberedBytes <= maxBytes-header.Size && used <= archiveExpansionLimit-header.Size {
				data, err := readLimited(tarReader, header.Size)
				if err != nil {
					return ArchiveEntry{}, err
				}
				if int64(len(data)) != header.Size && header.Size > 0 {
					return ArchiveEntry{}, fmt.Errorf("read archive: short entry")
				}
				remembered[name] = data
				rememberedBytes += int64(len(data))
				used += int64(len(data))
				continue
			}
			if err := skipTarBudget(tarReader, header.Size, &used); err != nil {
				return ArchiveEntry{}, err
			}
		case tar.TypeLink:
			if err := discardEntry(tarReader, header.Size); err != nil {
				return ArchiveEntry{}, err
			}
			if name != want {
				continue
			}
			target := archiveMemberKey(header.Linkname)
			if size, ok := sizes[target]; ok && size > maxBytes {
				return ArchiveEntry{}, ErrArchiveEntryTooLarge
			}
			data, ok := remembered[target]
			if !ok || int64(len(data)) > maxBytes {
				return ArchiveEntry{}, ErrArchiveEntryNotPreviewable
			}
			return previewEntry(want, data, header.ModTime), nil
		default:
			if err := discardEntry(tarReader, header.Size); err != nil {
				return ArchiveEntry{}, err
			}
			if name == want {
				return ArchiveEntry{}, ErrArchiveEntryNotPreviewable
			}
		}
	}
}

func previewEntry(want string, data []byte, mod time.Time) ArchiveEntry {
	if mod.IsZero() {
		mod = time.Now().UTC()
	}
	return ArchiveEntry{
		Name:     filepath.Base(want),
		Size:     int64(len(data)),
		ModTime:  mod,
		MIMEType: MIMETypeForName(want),
		Data:     data,
	}
}

func readPreviewBytes(r io.Reader, maxBytes int64) ([]byte, error) {
	data, err := readLimited(r, maxBytes)
	if err != nil && strings.Contains(err.Error(), "exceeds limit") {
		return nil, ErrArchiveEntryTooLarge
	}
	return data, err
}

func skipTarBudget(r io.Reader, size int64, used *int64) error {
	if size < 0 || *used > archiveExpansionLimit-size {
		return fmt.Errorf("archive expanded size exceeds limit")
	}
	if err := discardEntry(r, size); err != nil {
		return err
	}
	*used += size
	return nil
}

func cleanMemberName(name string) (string, bool, error) {
	name = strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "./")
	name = strings.TrimSuffix(name, "/")
	if name == "" || name == "." {
		return "", true, nil
	}
	if !validArchivePath(name) {
		return "", false, fmt.Errorf("invalid archive entry")
	}
	return name, false, nil
}

func listingFromMembers(kind archiveKind, members []archiveMember) (ArchiveListing, error) {
	listing := ArchiveListing{Format: kind.String(), Entries: make([]ArchiveListEntry, 0, len(members))}
	for _, member := range members {
		if len(listing.Entries) >= ArchiveEntryLimit {
			return ArchiveListing{}, ErrArchiveTooManyEntries
		}
		if (member.Link || member.Special) && kind.filesystem() {
			listing.SkippedLinks++
		}
		listing.Entries = append(listing.Entries, ArchiveListEntry{
			Name:           member.Name,
			Size:           member.Size,
			CompressedSize: member.Compressed,
			IsDir:          member.IsDir,
			ModTime:        member.ModTime,
			Link:           member.Link || member.Special,
		})
	}
	return listing, nil
}
