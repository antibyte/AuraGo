package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CodeAnalyzer provides basic structural parsing for source files
type CodeAnalyzer struct{}

func NewCodeAnalyzer() *CodeAnalyzer {
	return &CodeAnalyzer{}
}

// StructureItem represents a parsed element (Function, Class, Type etc.)
type StructureItem struct {
	Type     string
	Name     string
	Line     int
	Language string
}

// ExtractStructure parses a file to extract its major structural components using regex.
func (c *CodeAnalyzer) ExtractStructure(filePath string) ([]StructureItem, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(filePath))
	var items []StructureItem

	// Define language-specific regexes
	var funcRegex, classRegex *regexp.Regexp
	switch ext {
	case ".go":
		funcRegex = regexp.MustCompile(`^func\s+(?:\([^)]+\)\s+)?([A-Za-z0-9_]+)\s*\(`)
		classRegex = regexp.MustCompile(`^type\s+([A-Za-z0-9_]+)\s+(?:struct|interface)`)
	case ".py":
		funcRegex = regexp.MustCompile(`^\s*def\s+([A-Za-z0-9_]+)\s*\(`)
		classRegex = regexp.MustCompile(`^\s*class\s+([A-Za-z0-9_]+)\s*[:\(]`)
	case ".js", ".ts", ".jsx", ".tsx":
		funcRegex = regexp.MustCompile(`(?:function\s+([A-Za-z0-9_]+)\s*\()|(?:(?:const|let|var)\s+([A-Za-z0-9_]+)\s*=\s*(?:async\s*)?(?:function|\([^)]*\)\s*=>))`)
		classRegex = regexp.MustCompile(`^\s*class\s+([A-Za-z0-9_]+)\s*(?:extends|implements|\{)`)
	default:
		return nil, fmt.Errorf("unsupported file extension for structure extraction: %s", ext)
	}

	scanner := bufio.NewScanner(file)
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()

		if classRegex != nil {
			matches := classRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				name := matches[1]
				if name == "" && len(matches) > 2 {
					name = matches[2]
				}
				if name != "" {
					items = append(items, StructureItem{Type: "Class/Type", Name: name, Line: lineNum, Language: ext})
				}
				lineNum++
				continue
			}
		}

		if funcRegex != nil {
			matches := funcRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				name := matches[1]
				if name == "" && len(matches) > 2 {
					name = matches[2] // for JS arrow functions
				}
				if name != "" {
					items = append(items, StructureItem{Type: "Function", Name: name, Line: lineNum, Language: ext})
				}
			}
		}
		lineNum++
	}

	return items, scanner.Err()
}

// SymbolSearch searches for definitions of a given symbol via simple regex matching.
// Useful for broad but fast discovery across files.
func (c *CodeAnalyzer) SymbolSearch(dirOrFile string, symbol string) ([]string, error) {
	info, err := os.Stat(dirOrFile)
	if err != nil {
		return nil, fmt.Errorf("stat failed: %w", err)
	}

	var results []string
	// Basic regex for finding definitions (func/class/var) containing the symbol
	pattern := fmt.Sprintf(`(?:func|def|class|type|const|let|var|function)\s+(?:\([^)]+\)\s+)?%s\b`, regexp.QuoteMeta(symbol))
	regex := regexp.MustCompile(pattern)

	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".go" && ext != ".py" && ext != ".js" && ext != ".ts" && ext != ".jsx" && ext != ".tsx" {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 1
		for scanner.Scan() {
			if regex.MatchString(scanner.Text()) {
				results = append(results, fmt.Sprintf("%s:%d", path, lineNum))
			}
			lineNum++
		}
		return nil
	}

	if info.IsDir() {
		err = filepath.Walk(dirOrFile, walkFunc)
	} else {
		err = walkFunc(dirOrFile, info, nil)
	}

	return results, err
}
