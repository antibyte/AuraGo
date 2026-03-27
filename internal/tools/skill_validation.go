package tools

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	maxSkillNameLength        = 64
	maxSkillCodeBytes         = 1 << 20
	maxSkillDescriptionLength = 2048
	maxSkillCategoryLength    = 48
	maxSkillTags              = 12
	maxSkillTagLength         = 32
)

var (
	skillNamePattern       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,63}$`)
	skillCategoryPattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,47}$`)
	skillTagPattern        = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	skillDependencyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	skillReservedNames     = map[string]struct{}{
		"con": {}, "prn": {}, "aux": {}, "nul": {},
		"com1": {}, "com2": {}, "com3": {}, "com4": {}, "com5": {}, "com6": {}, "com7": {}, "com8": {}, "com9": {},
		"lpt1": {}, "lpt2": {}, "lpt3": {}, "lpt4": {}, "lpt5": {}, "lpt6": {}, "lpt7": {}, "lpt8": {}, "lpt9": {},
	}
)

func validateSkillName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("skill name is required")
	}
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("skill name must be valid UTF-8")
	}
	if !isASCII(name) {
		return "", fmt.Errorf("skill name must use ASCII letters, numbers, underscores, or hyphens")
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", fmt.Errorf("skill name must not contain path separators or '..'")
	}
	if len(name) > maxSkillNameLength {
		return "", fmt.Errorf("skill name is too long (max %d characters)", maxSkillNameLength)
	}
	if !skillNamePattern.MatchString(name) {
		return "", fmt.Errorf("skill name must start with a letter or underscore and only contain letters, numbers, underscores, or hyphens")
	}
	if _, reserved := skillReservedNames[strings.ToLower(name)]; reserved {
		return "", fmt.Errorf("skill name '%s' is reserved on Windows", name)
	}
	return name, nil
}

func validateSkillCode(code string) error {
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("skill code is required")
	}
	if len(code) > maxSkillCodeBytes {
		return fmt.Errorf("skill code too large (max %d bytes)", maxSkillCodeBytes)
	}
	return nil
}

func normalizeSkillDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if len(description) > maxSkillDescriptionLength {
		return "", fmt.Errorf("skill description too long (max %d characters)", maxSkillDescriptionLength)
	}
	return description, nil
}

func normalizeSkillCategory(category string) (string, error) {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		return "", nil
	}
	if len(category) > maxSkillCategoryLength {
		return "", fmt.Errorf("skill category too long (max %d characters)", maxSkillCategoryLength)
	}
	if !skillCategoryPattern.MatchString(category) {
		return "", fmt.Errorf("skill category must start with a letter and only contain lowercase letters, numbers, underscores, or hyphens")
	}
	return category, nil
}

func normalizeSkillTags(tags []string) ([]string, error) {
	if len(tags) == 0 {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, min(len(tags), maxSkillTags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		if len(tag) > maxSkillTagLength {
			return nil, fmt.Errorf("skill tag '%s' is too long (max %d characters)", tag, maxSkillTagLength)
		}
		if !skillTagPattern.MatchString(tag) {
			return nil, fmt.Errorf("skill tag '%s' may only contain lowercase letters, numbers, underscores, or hyphens", tag)
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
		if len(out) >= maxSkillTags {
			break
		}
	}
	slices.Sort(out)
	return out, nil
}

func normalizeSkillDependencies(dependencies []string) ([]string, error) {
	if len(dependencies) == 0 {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(dependencies))
	out := make([]string, 0, len(dependencies))
	for _, dep := range dependencies {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		if !skillDependencyPattern.MatchString(dep) {
			return nil, fmt.Errorf("invalid dependency name '%s'", dep)
		}
		if _, ok := seen[dep]; ok {
			continue
		}
		seen[dep] = struct{}{}
		out = append(out, dep)
	}
	slices.Sort(out)
	return out, nil
}

func normalizeSkillTemplateInputs(description, baseURL string, dependencies, vaultKeys []string) (string, string, []string, []string, error) {
	var err error
	description, err = normalizeSkillDescription(description)
	if err != nil {
		return "", "", nil, nil, err
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL != "" {
		parsed, parseErr := url.Parse(baseURL)
		if parseErr != nil || parsed.Scheme == "" || parsed.Host == "" {
			return "", "", nil, nil, fmt.Errorf("base_url must be an absolute URL")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", "", nil, nil, fmt.Errorf("base_url must use http or https")
		}
	}
	dependencies, err = normalizeSkillDependencies(dependencies)
	if err != nil {
		return "", "", nil, nil, err
	}
	vaultKeys = normalizeVaultKeyList(vaultKeys)
	return description, baseURL, dependencies, vaultKeys, nil
}

func normalizeVaultKeyList(keys []string) []string {
	if len(keys) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}

func writeFileExclusive(path string, data []byte, perm os.FileMode) error {
	fd, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	defer fd.Close()
	if _, err := fd.Write(data); err != nil {
		return err
	}
	return fd.Sync()
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}
