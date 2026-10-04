package tools

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Local execution sees a private, bounded snapshot. Absolute paths have exactly
// the same root restriction as relative names; executable inventory is rejected.
func stageAnsibleInputs(ctx context.Context, cfg AnsibleLocalConfig, name string, args []string) (dir string, result []string, err error) {
	if name != "ansible" && name != "ansible-inventory" && name != "ansible-playbook" {
		return "", nil, fmt.Errorf("unsupported Ansible executable")
	}
	if name == "ansible" && len(args) > 0 && strings.HasPrefix(args[0], "-") && args[0] != "--version" {
		return "", nil, fmt.Errorf("invalid host pattern")
	}
	dir, err = os.MkdirTemp(cfg.WorkDir, ".aurago-ansible-")
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dir)
		}
	}()
	if err = os.WriteFile(filepath.Join(dir, "_safe.cfg"), []byte("[defaults]\nhost_key_checking=True\n"), 0600); err != nil {
		return
	}
	result = append([]string(nil), args...)
	if name == "ansible-playbook" {
		if len(args) == 0 || cfg.PlaybooksDir == "" {
			err = fmt.Errorf("configured playbooks directory is required")
			return
		}
		var relative string
		relative, err = ansibleInputRelative(cfg.PlaybooksDir, args[0])
		if err != nil {
			return
		}
		if err = copyAnsibleTree(ctx, cfg.PlaybooksDir, filepath.Join(dir, "playbooks")); err != nil {
			return
		}
		result[0] = filepath.Join(dir, "playbooks", relative)
	}
	hasInventory := false
	for i := 0; i < len(result); i++ {
		if result[i] != "-i" {
			continue
		}
		hasInventory = true
		if i+1 >= len(result) {
			err = fmt.Errorf("inventory is missing")
			return
		}
		input := result[i+1]
		base := cfg.PlaybooksDir
		// A configured administrator default may live outside the playbook root.
		if input == cfg.DefaultInventory && filepath.IsAbs(input) {
			base = filepath.Dir(input)
		}
		var relative string
		relative, err = ansibleInputRelative(base, input)
		if err != nil {
			return
		}
		var root *os.Root
		root, err = os.OpenRoot(base)
		if err != nil {
			return
		}
		var data []byte
		func() {
			defer root.Close()
			var f *os.File
			f, err = root.Open(relative)
			if err != nil {
				return
			}
			defer f.Close()
			var info os.FileInfo
			info, err = f.Stat()
			if err != nil {
				return
			}
			if !info.Mode().IsRegular() || info.Mode()&0111 != 0 {
				err = fmt.Errorf("inventory must be a non-executable regular file")
				return
			}
			data, err = io.ReadAll(io.LimitReader(f, 1<<20+1))
		}()
		if err != nil {
			return
		}
		ext := strings.ToLower(filepath.Ext(relative))
		if len(data) > 1<<20 || strings.ContainsRune(string(data), 0) || strings.HasPrefix(string(data), "#!") || (ext != ".ini" && ext != ".yaml" && ext != ".yml" && ext != "") || strings.Contains(strings.ToLower(string(data)), "plugin:") {
			err = fmt.Errorf("only bounded static INI/YAML inventory is supported")
			return
		}
		if ext == "" {
			ext = ".ini"
		}
		result[i+1] = filepath.Join(dir, "_inventory"+ext)
		if err = os.WriteFile(result[i+1], data, 0600); err != nil {
			return
		}
		i++
	}
	if !hasInventory && !(name == "ansible" && len(args) == 1 && args[0] == "--version") {
		inventory := filepath.Join(dir, "_inventory.ini")
		if err = os.WriteFile(inventory, []byte("localhost ansible_connection=local\n"), 0600); err != nil {
			return
		}
		result = append(result, "-i", inventory)
	}
	return
}

func ansibleInputRelative(base, input string) (string, error) {
	if strings.TrimSpace(base) == "" {
		return "", fmt.Errorf("Ansible input root is not configured")
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(abs, path)
	}
	rel, err := filepath.Rel(abs, path)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return "", fmt.Errorf("Ansible input escapes its configured root")
	}
	if err := requireUnprotectedSystemPath(path, input); err != nil {
		return "", err
	}
	return rel, nil
}

func copyAnsibleTree(ctx context.Context, source, destination string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer root.Close()
	files, total := 0, int64(0)
	return fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".aurago-ansible-") {
				return fs.SkipDir
			}
			return os.MkdirAll(filepath.Join(destination, path), 0700)
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("playbook snapshots do not permit links")
		}
		if err := requireUnprotectedSystemPath(filepath.Join(source, path), path); err != nil {
			return err
		}
		files++
		if files > 512 {
			return fmt.Errorf("playbook snapshot exceeds 512 files")
		}
		f, err := root.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("playbook snapshot requires regular files")
		}
		data, err := io.ReadAll(io.LimitReader(f, (32<<20)-total+1))
		if err != nil {
			return err
		}
		total += int64(len(data))
		if total > 32<<20 {
			return fmt.Errorf("playbook snapshot exceeds 32 MiB")
		}
		return os.WriteFile(filepath.Join(destination, path), data, 0600)
	})
}
