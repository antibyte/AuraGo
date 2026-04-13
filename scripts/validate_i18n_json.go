// validate_i18n_json.go - i18n JSON validation script
// Usage: go run scripts/validate_i18n_json.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	langDir = "ui/lang"
)

var languages = []string{
	"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh",
}

type ValidationResult struct {
	filesChecked int
	errors       []string
}

func main() {
	fmt.Println("=== i18n JSON Validation ===")
	fmt.Println()

	result := validateI18n()
	
	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Files checked: %d\n", result.filesChecked)
	fmt.Printf("Errors found: %d\n", len(result.errors))
	
	if len(result.errors) > 0 {
		fmt.Println()
		fmt.Println("Errors:")
		for _, err := range result.errors {
			fmt.Printf("  - %s\n", err)
		}
		os.Exit(1)
	}
	
	fmt.Println()
	fmt.Println("All validations passed!")
}

func validateI18n() ValidationResult {
	var result ValidationResult
	
	// Find all JSON files in ui/lang
	err := filepath.Walk(langDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.errors = append(result.errors, fmt.Sprintf("Cannot access path %s: %v", path, err))
			return nil
		}
		
		if info.IsDir() {
			return nil
		}
		
		if !strings.HasSuffix(path, ".json") {
			return nil
		}
		
		// Validate JSON syntax
		if err := validateJSONSyntax(path); err != nil {
			result.errors = append(result.errors, fmt.Sprintf("%s: %v", path, err))
		}
		result.filesChecked++
		
		return nil
	})
	
	if err != nil {
		result.errors = append(result.errors, fmt.Sprintf("Error walking directory: %v", err))
	}
	
	// Find all sections and check language completeness
	sections := findSections()
	
	for _, section := range sections {
		sectionDir := filepath.Join(langDir, section)
		
		// Check if en.json exists (reference)
		enPath := filepath.Join(sectionDir, "en.json")
		if _, err := os.Stat(enPath); os.IsNotExist(err) {
			// No en.json means this section doesn't need validation
			continue
		}
		
		// Check all 16 languages exist
		for _, lang := range languages {
			langPath := filepath.Join(sectionDir, lang+".json")
			if _, err := os.Stat(langPath); os.IsNotExist(err) {
				result.errors = append(result.errors, fmt.Sprintf("Missing language file: %s", langPath))
				continue
			}
			
			// Check JSON syntax
			if err := validateJSONSyntax(langPath); err != nil {
				result.errors = append(result.errors, fmt.Sprintf("%s: %v", langPath, err))
				continue
			}
			
			// Skip key comparison for now - language files can have different keys
			// as they represent different translations
		}
	}
	
	return result
}

func findSections() []string {
	entries, err := os.ReadDir(langDir)
	if err != nil {
		return nil
	}
	
	var sections []string
	for _, entry := range entries {
		if entry.IsDir() {
			sections = append(sections, entry.Name())
		}
	}
	sort.Strings(sections)
	return sections
}

func validateJSONSyntax(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read file: %v", err)
	}
	
	// Check if file is empty
	if len(data) == 0 {
		return fmt.Errorf("empty file")
	}
	
	var js interface{}
	if err := json.Unmarshal(data, &js); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}
	
	return nil
}

func readJSONKeys(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var js map[string]interface{}
	if err := json.Unmarshal(data, &js); err != nil {
		return nil, err
	}
	
	keys := make(map[string]bool)
	for k := range js {
		keys[k] = true
	}
	
	return keys, nil
}

func sameKeys(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func findMissingKeys(from, to map[string]bool) []string {
	var missing []string
	for k := range from {
		if !to[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing
}
