package desktop

import (
	"fmt"
	"os"
	"path"
	"strings"
)

func preparePetFiles(files map[string][]byte) (map[string][]byte, error) {
	cleaned := make(map[string][]byte, len(files))
	seen := make(map[string]bool, len(files))
	for name, data := range files {
		clean, ok := cleanPetRelPath(name)
		key := strings.ToLower(clean)
		if !ok || seen[key] {
			return nil, fmt.Errorf("invalid or duplicate pet file path %q", name)
		}
		seen[key] = true
		cleaned[clean] = data
	}
	for name := range seen {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if seen[parent] {
				return nil, fmt.Errorf("pet file conflicts with a directory")
			}
		}
	}
	return cleaned, nil
}

func preflightPetWrite(root *os.Root, name string) error {
	parts := strings.Split(name, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (i < len(parts)-1 && !info.IsDir()) || (i == len(parts)-1 && !info.Mode().IsRegular()) {
			return errPetEntryLinked
		}
	}
	return nil
}

// Pin each real directory before creating descendants; a swapped link cannot redirect writes.
func openPetWriteDirectory(root *os.Root, rel string) (*os.Root, error) {
	current, err := root.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(rel, "/") {
		if part == "." {
			continue
		}
		info, err := current.Lstat(part)
		if os.IsNotExist(err) {
			err = current.Mkdir(part, 0700)
			if err == nil || os.IsExist(err) {
				info, err = current.Lstat(part)
			}
		}
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			err = errPetEntryLinked
		}
		if err != nil {
			current.Close()
			return nil, err
		}
		next, err := current.OpenRoot(part)
		current.Close()
		if err != nil {
			return nil, err
		}
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(info, opened) {
			next.Close()
			return nil, errPetEntryLinked
		}
		current = next
	}
	return current, nil
}
