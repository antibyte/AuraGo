package desktop

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	desktopSearchQueryMaxRunes = 256
	desktopSearchTraversalCap  = 20_000
	desktopSearchMatchCap      = 1_000
	desktopSearchPageDefault   = 50
	desktopSearchPageMax       = 100
	desktopSearchOffsetMax     = 1_000
	desktopSearchDepthCap      = 256
	desktopSearchReadBatch     = 256
)

type desktopSearchLimits struct {
	traversal int
	matches   int
	depth     int
	readBatch int
}

type desktopSearchDirectory struct {
	relative string
	depth    int
}

// SearchFilesPageOptions bounds one paged recursive search. A zero Limit uses
// the paged UI default; the legacy SearchFiles method retains its 1000-item cap.
type SearchFilesPageOptions struct {
	Query          string
	Type           string
	ModifiedAfter  *time.Time
	ModifiedBefore *time.Time
	MinSize        *int64
	MaxSize        *int64
	Offset         int
	Limit          int
}

// SearchFilesResult reports whether another page is available and whether the
// traversal or global match budget stopped the search early.
type SearchFilesResult struct {
	Files      []FileEntry `json:"files"`
	HasMore    bool        `json:"has_more"`
	Truncated  bool        `json:"truncated"`
	NextOffset int         `json:"next_offset"`
}

// SearchFilesPage searches below one rooted workspace directory, filtering
// before applying a deterministic path-ordered page.
func (s *Service) SearchFilesPage(ctx context.Context, rawPath string, options SearchFilesPageOptions) (SearchFilesResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return SearchFilesResult{}, err
	}

	query := strings.TrimSpace(options.Query)
	if utf8.RuneCountInString(query) > desktopSearchQueryMaxRunes {
		return SearchFilesResult{}, fmt.Errorf("desktop search query exceeds %d characters", desktopSearchQueryMaxRunes)
	}
	filterType := strings.ToLower(strings.TrimSpace(options.Type))
	if filterType == "" {
		filterType = "all"
	}
	switch filterType {
	case "all", "image", "audio", "video", "document", "archive", "folder":
	default:
		return SearchFilesResult{}, fmt.Errorf("invalid desktop search type")
	}
	if options.Offset < 0 || options.Offset > desktopSearchOffsetMax {
		return SearchFilesResult{}, fmt.Errorf("desktop search offset must be between 0 and %d", desktopSearchOffsetMax)
	}
	limit := options.Limit
	if limit == 0 {
		limit = desktopSearchPageDefault
	}
	if limit < 1 || limit > desktopSearchPageMax {
		return SearchFilesResult{}, fmt.Errorf("desktop search limit must be between 1 and %d", desktopSearchPageMax)
	}
	if options.ModifiedAfter != nil && options.ModifiedAfter.IsZero() || options.ModifiedBefore != nil && options.ModifiedBefore.IsZero() {
		return SearchFilesResult{}, fmt.Errorf("invalid desktop search modified date")
	}
	if options.ModifiedAfter != nil && options.ModifiedBefore != nil && options.ModifiedAfter.After(*options.ModifiedBefore) {
		return SearchFilesResult{}, fmt.Errorf("desktop search modified date range is reversed")
	}
	if options.MinSize != nil && *options.MinSize < 0 || options.MaxSize != nil && *options.MaxSize < 0 {
		return SearchFilesResult{}, fmt.Errorf("desktop search size cannot be negative")
	}
	if options.MinSize != nil && options.MaxSize != nil && *options.MinSize > *options.MaxSize {
		return SearchFilesResult{}, fmt.Errorf("desktop search size range is reversed")
	}

	dirPath, err := s.ResolvePath(rawPath)
	if err != nil {
		return SearchFilesResult{}, err
	}
	workspacePath, err := filepath.Abs(s.Config().WorkspaceDir)
	if err != nil {
		return SearchFilesResult{}, fmt.Errorf("resolve desktop search workspace: %w", err)
	}
	searchRelative, err := filepath.Rel(workspacePath, dirPath)
	if err != nil || !isWithinPath(workspacePath, dirPath) {
		return SearchFilesResult{}, fmt.Errorf("desktop path escapes workspace")
	}
	root, err := os.OpenRoot(workspacePath)
	if err != nil {
		return SearchFilesResult{}, fmt.Errorf("open desktop search workspace: %w", err)
	}
	defer root.Close()

	return searchFilesPageRooted(ctx, root, searchRelative, SearchFilesPageOptions{
		Query:          query,
		Type:           filterType,
		ModifiedAfter:  options.ModifiedAfter,
		ModifiedBefore: options.ModifiedBefore,
		MinSize:        options.MinSize,
		MaxSize:        options.MaxSize,
		Offset:         options.Offset,
		Limit:          limit,
	}, limit, desktopSearchLimits{
		traversal: desktopSearchTraversalCap,
		matches:   desktopSearchMatchCap,
		depth:     desktopSearchDepthCap,
		readBatch: desktopSearchReadBatch,
	})
}

func searchFilesPageRooted(ctx context.Context, root *os.Root, searchRelative string, options SearchFilesPageOptions, limit int, limits desktopSearchLimits) (SearchFilesResult, error) {
	needle := strings.ToLower(options.Query)
	matches := make([]FileEntry, 0, min(limit+1, limits.matches))
	directories := []desktopSearchDirectory{{relative: searchRelative}}
	visited := 0
	truncated := false
	stop := false
	for len(directories) > 0 && !stop {
		if err := ctx.Err(); err != nil {
			return SearchFilesResult{}, err
		}
		last := len(directories) - 1
		directory := directories[last]
		directories = directories[:last]
		directoryRoot, reader, err := openDesktopSearchDirectory(root, directory.relative)
		if err != nil {
			if directory.depth == 0 {
				return SearchFilesResult{}, fmt.Errorf("open desktop search directory: %w", err)
			}
			truncated = true
			continue
		}

		for {
			if err := ctx.Err(); err != nil {
				_ = reader.Close()
				_ = directoryRoot.Close()
				return SearchFilesResult{}, err
			}
			entries, readErr := reader.ReadDir(limits.readBatch)
			for _, entry := range entries {
				if err := ctx.Err(); err != nil {
					_ = reader.Close()
					_ = directoryRoot.Close()
					return SearchFilesResult{}, err
				}
				if visited >= limits.traversal {
					truncated = true
					stop = true
					break
				}
				visited++
				name := entry.Name()
				childRelative := filepath.Join(directory.relative, name)
				info, statErr := directoryRoot.Lstat(name)
				if statErr != nil {
					truncated = true
					continue
				}
				if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
					continue
				}
				if info.IsDir() {
					if directory.depth >= limits.depth {
						truncated = true
					} else {
						directories = append(directories, desktopSearchDirectory{relative: childRelative, depth: directory.depth + 1})
					}
				}
				if needle != "" && !strings.Contains(strings.ToLower(name), needle) {
					continue
				}
				if !desktopSearchTypeMatches(options.Type, name, info.IsDir()) ||
					options.ModifiedAfter != nil && info.ModTime().Before(*options.ModifiedAfter) ||
					options.ModifiedBefore != nil && info.ModTime().After(*options.ModifiedBefore) ||
					options.MinSize != nil && info.Size() < *options.MinSize ||
					options.MaxSize != nil && info.Size() > *options.MaxSize {
					continue
				}
				if len(matches) >= limits.matches {
					truncated = true
					stop = true
					break
				}

				itemType := "file"
				if info.IsDir() {
					itemType = "directory"
				}
				modTime := info.ModTime()
				matches = append(matches, FileEntry{
					Name:      name,
					Path:      filepath.ToSlash(childRelative),
					Type:      itemType,
					Size:      info.Size(),
					ModTime:   modTime,
					Modified:  modTime,
					MIMEType:  MIMETypeForName(name),
					MediaKind: desktopMediaKindForName(name),
					Mode:      info.Mode().String(),
					Created:   getCreationTime(info),
				})
			}
			if stop {
				break
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				if directory.depth == 0 {
					_ = reader.Close()
					_ = directoryRoot.Close()
					return SearchFilesResult{}, fmt.Errorf("read desktop search directory: %w", readErr)
				}
				truncated = true
				break
			}
		}
		if err := reader.Close(); err != nil {
			if directory.depth == 0 {
				_ = directoryRoot.Close()
				return SearchFilesResult{}, fmt.Errorf("close desktop search directory: %w", err)
			}
			truncated = true
		}
		if err := directoryRoot.Close(); err != nil {
			if directory.depth == 0 {
				return SearchFilesResult{}, fmt.Errorf("close desktop search directory root: %w", err)
			}
			truncated = true
		}
	}
	if err := ctx.Err(); err != nil {
		return SearchFilesResult{}, err
	}

	sort.Slice(matches, func(i, j int) bool {
		a, b := matches[i].Path, matches[j].Path
		if strings.EqualFold(a, b) {
			return a < b
		}
		return strings.ToLower(a) < strings.ToLower(b)
	})

	start := min(options.Offset, len(matches))
	end := min(start+limit, len(matches))
	page := append([]FileEntry{}, matches[start:end]...)
	return SearchFilesResult{
		Files:      page,
		HasMore:    end < len(matches),
		Truncated:  truncated,
		NextOffset: end,
	}, nil
}

func openDesktopSearchDirectory(root *os.Root, relative string) (*os.Root, *os.File, error) {
	current, err := root.OpenRoot(".")
	if err != nil {
		return nil, nil, err
	}
	ownedRoot := current
	cleaned := filepath.Clean(relative)
	if cleaned == "" {
		cleaned = "."
	}
	if filepath.IsAbs(cleaned) || filepath.VolumeName(cleaned) != "" {
		_ = ownedRoot.Close()
		return nil, nil, fmt.Errorf("desktop search directory is not relative")
	}
	if cleaned != "." {
		for _, component := range strings.Split(cleaned, string(os.PathSeparator)) {
			if component == "" || component == "." || component == ".." {
				if ownedRoot != nil {
					_ = ownedRoot.Close()
				}
				return nil, nil, fmt.Errorf("desktop search directory contains an invalid component")
			}
			before, err := current.Lstat(component)
			if err != nil {
				if ownedRoot != nil {
					_ = ownedRoot.Close()
				}
				return nil, nil, err
			}
			if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
				if ownedRoot != nil {
					_ = ownedRoot.Close()
				}
				return nil, nil, fmt.Errorf("desktop search refuses symlink or non-directory component")
			}
			next, err := current.OpenRoot(component)
			if err != nil {
				if ownedRoot != nil {
					_ = ownedRoot.Close()
				}
				return nil, nil, err
			}
			opened, statErr := next.Stat(".")
			after, lstatErr := current.Lstat(component)
			if statErr != nil || lstatErr != nil || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, opened) || !os.SameFile(after, opened) {
				_ = next.Close()
				if ownedRoot != nil {
					_ = ownedRoot.Close()
				}
				if statErr != nil {
					return nil, nil, statErr
				}
				if lstatErr != nil {
					return nil, nil, lstatErr
				}
				return nil, nil, fmt.Errorf("desktop search directory changed during open")
			}
			_ = ownedRoot.Close()
			current, ownedRoot = next, next
		}
	}
	directory, err := current.Open(".")
	if err != nil {
		_ = ownedRoot.Close()
		return nil, nil, err
	}
	opened, statErr := directory.Stat()
	rootInfo, rootStatErr := current.Stat(".")
	if statErr != nil || rootStatErr != nil || !os.SameFile(opened, rootInfo) {
		_ = directory.Close()
		_ = ownedRoot.Close()
		if statErr != nil {
			return nil, nil, statErr
		}
		if rootStatErr != nil {
			return nil, nil, rootStatErr
		}
		return nil, nil, fmt.Errorf("desktop search directory changed during open")
	}
	return ownedRoot, directory, nil
}

func desktopSearchTypeMatches(filterType, name string, isDir bool) bool {
	switch filterType {
	case "all":
		return true
	case "folder":
		return isDir
	case "archive":
		return !isDir && kindFromName(name) != archiveUnknown
	case "document":
		return !isDir && (desktopMediaKindForName(name) == "document" || strings.HasPrefix(MIMETypeForName(name), "text/"))
	default:
		return !isDir && desktopMediaKindForName(name) == filterType
	}
}
