package services

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"strings"
)

const (
	maxDocumentArchiveEntries           = 4096
	maxDocumentArchiveMemberBytes int64 = 8 << 20
	maxDocumentArchiveTotalBytes  int64 = 64 << 20
)

var (
	// ErrDocumentArchiveTooLarge marks archive expansion limits that callers may
	// translate to a size-specific protocol error.
	ErrDocumentArchiveTooLarge = errors.New("document archive extraction limit exceeded")
	ErrDocumentArchiveUnsafe   = errors.New("document archive contains an unsafe entry")
)

// ValidateDocumentArchive validates the ZIP container shared by Office and ODF
// ingestion. Only members read by the text extractors count toward the expansion
// budget, so media-heavy documents remain supported.
func ValidateDocumentArchive(path, requiredEntry, requiredMimetype string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open document archive: %w", err)
	}
	defer reader.Close()

	if len(reader.File) > maxDocumentArchiveEntries {
		return fmt.Errorf("%w: archive contains more than %d entries", ErrDocumentArchiveTooLarge, maxDocumentArchiveEntries)
	}

	requiredEntry = cleanArchiveMemberName(requiredEntry)
	foundRequired := false
	foundMimetype := requiredMimetype == ""
	var relevantBytes uint64
	for _, file := range reader.File {
		name, err := validateDocumentArchiveMember(file)
		if err != nil {
			return err
		}
		if name == requiredEntry {
			foundRequired = true
		}
		if !documentArchiveMemberRelevant(name, requiredEntry) {
			continue
		}
		size := file.UncompressedSize64
		if size > uint64(maxDocumentArchiveMemberBytes) {
			return fmt.Errorf("%w: archive member %q exceeds %d bytes", ErrDocumentArchiveTooLarge, name, maxDocumentArchiveMemberBytes)
		}
		if relevantBytes > uint64(maxDocumentArchiveTotalBytes)-size {
			return fmt.Errorf("%w: relevant archive members exceed %d bytes", ErrDocumentArchiveTooLarge, maxDocumentArchiveTotalBytes)
		}
		relevantBytes += size

		if name == "mimetype" && requiredMimetype != "" {
			raw, readErr := readDocumentArchiveMember(file, newDocumentArchiveBudget())
			if readErr != nil {
				return readErr
			}
			foundMimetype = strings.TrimSpace(string(raw)) == requiredMimetype
		}
	}
	if !foundRequired || !foundMimetype {
		return fmt.Errorf("required document archive entries are missing")
	}
	return nil
}

type documentArchiveBudget struct {
	used int64
}

func newDocumentArchiveBudget() *documentArchiveBudget {
	return &documentArchiveBudget{}
}

func readDocumentArchiveMember(file *zip.File, budget *documentArchiveBudget) ([]byte, error) {
	if file == nil || budget == nil {
		return nil, fmt.Errorf("document archive member is unavailable")
	}
	name, err := validateDocumentArchiveMember(file)
	if err != nil {
		return nil, err
	}
	if file.UncompressedSize64 > uint64(maxDocumentArchiveMemberBytes) {
		return nil, fmt.Errorf("%w: archive member %q exceeds %d bytes", ErrDocumentArchiveTooLarge, name, maxDocumentArchiveMemberBytes)
	}
	remaining := maxDocumentArchiveTotalBytes - budget.used
	if remaining <= 0 {
		return nil, fmt.Errorf("%w: relevant archive members exceed %d bytes", ErrDocumentArchiveTooLarge, maxDocumentArchiveTotalBytes)
	}
	limit := maxDocumentArchiveMemberBytes
	if remaining < limit {
		limit = remaining
	}

	content, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open document archive member %q: %w", name, err)
	}
	defer content.Close()
	raw, err := io.ReadAll(io.LimitReader(content, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read document archive member %q: %w", name, err)
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("%w: relevant archive members exceed extraction budget", ErrDocumentArchiveTooLarge)
	}
	budget.used += int64(len(raw))
	return raw, nil
}

func validateDocumentArchiveMember(file *zip.File) (string, error) {
	if file == nil {
		return "", fmt.Errorf("%w: missing archive entry", ErrDocumentArchiveUnsafe)
	}
	if file.Flags&0x1 != 0 {
		return "", fmt.Errorf("%w: encrypted archive entry", ErrDocumentArchiveUnsafe)
	}
	if file.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%w: linked archive entry", ErrDocumentArchiveUnsafe)
	}
	if strings.Contains(file.Name, `\`) || strings.ContainsRune(file.Name, '\x00') ||
		pathpkg.IsAbs(file.Name) || hasDocumentArchiveDrivePrefix(file.Name) {
		return "", fmt.Errorf("%w: invalid archive entry path", ErrDocumentArchiveUnsafe)
	}
	for _, segment := range strings.Split(file.Name, "/") {
		if segment == ".." {
			return "", fmt.Errorf("%w: traversing archive entry", ErrDocumentArchiveUnsafe)
		}
	}
	name := cleanArchiveMemberName(file.Name)
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("%w: invalid archive entry path", ErrDocumentArchiveUnsafe)
	}
	return name, nil
}

func hasDocumentArchiveDrivePrefix(name string) bool {
	if len(name) < 2 || name[1] != ':' {
		return false
	}
	first := name[0]
	return (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')
}

func cleanArchiveMemberName(name string) string {
	return pathpkg.Clean(strings.TrimSpace(strings.ReplaceAll(name, `\`, "/")))
}

func documentArchiveMemberRelevant(name, requiredEntry string) bool {
	switch requiredEntry {
	case "word/document.xml":
		return name == requiredEntry
	case "xl/workbook.xml":
		return name == requiredEntry ||
			name == "xl/sharedStrings.xml" ||
			(strings.HasPrefix(name, "xl/worksheets/sheet") && strings.HasSuffix(name, ".xml"))
	case "ppt/presentation.xml":
		return name == requiredEntry ||
			(strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml"))
	case "content.xml":
		return name == requiredEntry || name == "mimetype"
	default:
		return name == requiredEntry
	}
}
