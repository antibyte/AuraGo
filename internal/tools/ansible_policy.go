package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func validateAnsibleLiteral(value interface{}, depth int) error {
	if depth > 64 {
		return fmt.Errorf("Ansible data nesting exceeds 64 levels")
	}
	switch v := value.(type) {
	case string:
		for _, token := range []string{"{{", "{%", "{#"} {
			if strings.Contains(v, token) {
				return fmt.Errorf("Ansible inputs must be literal; templates and lookups are not allowed")
			}
		}
	case map[string]interface{}:
		for key, item := range v {
			if err := validateAnsibleLiteral(key, depth+1); err != nil {
				return err
			}
			if err := validateAnsibleLiteral(item, depth+1); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, item := range v {
			if err := validateAnsibleLiteral(item, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateAnsibleModuleArgs(raw string) error {
	if err := validateAnsibleLiteral(raw, 0); err != nil {
		return err
	}
	if strings.HasPrefix(raw, "{") && strings.HasSuffix(raw, "}") {
		var arguments map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &arguments); err != nil {
			return fmt.Errorf("module arguments must be a valid JSON object")
		}
		return validateAnsibleLiteral(arguments, 0)
	}
	// Ansible decodes backslash escapes in key=value syntax.
	if strings.Contains(raw, "\\") {
		return fmt.Errorf("use JSON for escaped module arguments in read-only mode")
	}
	return nil
}

func validateAnsibleExtraVars(raw string) error {
	var variables map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &variables); err != nil || variables == nil {
		return fmt.Errorf("extra_vars must be a JSON object")
	}
	if err := validateAnsibleLiteral(variables, 0); err != nil {
		return err
	}
	for key := range variables {
		key = strings.ToLower(key)
		if strings.HasPrefix(key, "ansible_") || strings.HasPrefix(key, "_ansible_") {
			return fmt.Errorf("extra_vars cannot override Ansible connection or engine variables")
		}
		switch key {
		case "hostvars", "groups", "group_names", "inventory_hostname", "inventory_hostname_short", "inventory_file", "inventory_dir", "playbook_dir", "role_path", "vars", "omit":
			return fmt.Errorf("extra_vars cannot override Ansible connection or engine variables")
		}
	}
	return nil
}

func validateAnsibleHostPattern(pattern string) error {
	if pattern == "" || strings.HasPrefix(pattern, "-") || strings.HasPrefix(pattern, "@") || strings.IndexFunc(pattern, func(r rune) bool { return r < 32 }) >= 0 {
		return fmt.Errorf("invalid host pattern or limit")
	}
	return validateAnsibleLiteral(pattern, 0)
}

func ansibleDeclaredHosts(raw string) (map[string]bool, error) {
	var inventory map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &inventory); err != nil || inventory == nil {
		return nil, fmt.Errorf("invalid inventory response")
	}
	if err := validateAnsibleLiteral(inventory, 0); err != nil {
		return nil, err
	}
	hosts := make(map[string]bool)
	for name, rawGroup := range inventory {
		group, ok := rawGroup.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid inventory group")
		}
		if name == "_meta" {
			if rawVars, present := group["hostvars"]; present {
				hostvars, ok := rawVars.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("invalid inventory host variables")
				}
				for host := range hostvars {
					hosts[host] = true
				}
			}
		} else if rawHosts, present := group["hosts"]; present {
			entries, ok := rawHosts.([]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid inventory hosts")
			}
			for _, entry := range entries {
				host, ok := entry.(string)
				if !ok {
					return nil, fmt.Errorf("invalid inventory hosts")
				}
				hosts[host] = true
			}
		}
	}
	return hosts, nil
}

var ansibleHostListHeader = regexp.MustCompile(`^\s*hosts \(([0-9]+)\):\s*$`)

func validateAnsibleSelectedHosts(output string, declared map[string]bool) error {
	lines := strings.Split(output, "\n")
	selected := 0
	for index, line := range lines {
		match := ansibleHostListHeader.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		count, err := strconv.Atoi(match[1])
		if err != nil || count > len(lines)-index-1 {
			return fmt.Errorf("invalid host selection response")
		}
		for _, host := range lines[index+1 : index+1+count] {
			if !declared[strings.TrimSpace(host)] {
				return fmt.Errorf("host selection includes a host outside the static inventory")
			}
		}
		selected += count
	}
	if selected == 0 {
		return fmt.Errorf("host pattern selects no explicit inventory hosts")
	}
	return nil
}
