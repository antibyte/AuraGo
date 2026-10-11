package onnxrt

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
)

// wheelMemberBasePattern keeps flattened member names portable file names:
// no leading dot, no trailing dot or space (Windows strips them), no ":"
// (NTFS alternate data streams) and no spaces.
var wheelMemberBasePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*[A-Za-z0-9]$`)

// validateWheelMembers requires clean, relative, forward-slash member paths
// whose base names are portable and unique ignoring case, because members are
// flattened on extraction (also on case-insensitive file systems) next to the
// runtime's .complete.json marker.
func validateWheelMembers(id string, members []string) error {
	if len(members) == 0 {
		return fmt.Errorf("wheel asset %s lists no members", id)
	}
	bases := make(map[string]bool, len(members))
	for _, member := range members {
		if member == "" || strings.Contains(member, `\`) || path.IsAbs(member) || path.Clean(member) != member ||
			member == ".." || strings.HasPrefix(member, "../") {
			return fmt.Errorf("wheel asset %s has invalid member %q", id, member)
		}
		base := path.Base(member)
		if base == ".complete.json" {
			return fmt.Errorf("wheel asset %s member %q collides with the runtime marker", id, member)
		}
		if !wheelMemberBasePattern.MatchString(base) {
			return fmt.Errorf("wheel asset %s member %q has a base name that must match %s", id, member, wheelMemberBasePattern)
		}
		folded := strings.ToLower(base)
		if bases[folded] {
			return fmt.Errorf("wheel asset %s has duplicate member name %q", id, base)
		}
		bases[folded] = true
	}
	return nil
}

// extractWheelMembers copies the listed members of a wheel (a zip file) into
// target, flattened to their base names. Every member must exist exactly once
// as a regular file; everything else in the wheel is ignored.
func extractWheelMembers(archivePath, target string, members []string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open wheel: %w", err)
	}
	defer reader.Close()
	wanted := make(map[string]bool, len(members))
	for _, member := range members {
		wanted[member] = true
	}
	found := make(map[string]bool, len(members))
	for _, entry := range reader.File {
		name := strings.ReplaceAll(entry.Name, `\`, "/")
		if !wanted[name] {
			continue
		}
		if found[name] {
			return fmt.Errorf("wheel member %q appears more than once", name)
		}
		if entry.Mode()&os.ModeSymlink != 0 || !entry.Mode().IsRegular() {
			return fmt.Errorf("wheel member %q is not a regular file", name)
		}
		destination, err := secureArchiveDestination(target, path.Base(name))
		if err != nil {
			return err
		}
		if err := writeZipEntry(entry, destination, sanitizedArchiveMode(entry.Mode(), name)); err != nil {
			return fmt.Errorf("extract wheel member %q: %w", name, err)
		}
		found[name] = true
	}
	var missing []string
	for _, member := range members {
		if !found[member] {
			missing = append(missing, member)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("wheel is missing %s", strings.Join(missing, ", "))
	}
	return nil
}

func writeZipEntry(entry *zip.File, destination string, mode os.FileMode) error {
	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, source); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}
