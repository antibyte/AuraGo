package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func hasSnakeCase(name string) bool {
	if strings.Contains(name, "_") {
		// Allow blank identifier
		if name == "_" {
			return false
		}
		// Allow ALL_CAPS constants
		if strings.ToUpper(name) == name {
			return false
		}
		// It has an underscore and is not ALL_CAPS
		return true
	}
	return false
}

func main() {
	dir := `C:\Users\Andi\Documents\repo\AuraGo\internal\agent`
	fset := token.NewFileSet()

	violations := 0

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip test files
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		node, parseErr := parser.ParseFile(fset, path, nil, parser.AllErrors|parser.ParseComments)
		if parseErr != nil {
			fmt.Printf("Parse error in %s: %v\n", path, parseErr)
			return nil
		}

		// Inspect all AST nodes
		ast.Inspect(node, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.FuncDecl:
				// Check function/method name
				if hasSnakeCase(v.Name.Name) && !v.Name.IsExported() {
					pos := fset.Position(v.Pos())
					fmt.Printf("FUNC_NAME: %s:%d: func %s\n", pos.Filename, pos.Line, v.Name.Name)
					violations++
				}

				// Check function parameters
				if v.Type.Params != nil {
					for _, field := range v.Type.Params.List {
						for _, name := range field.Names {
							if hasSnakeCase(name.Name) {
								pos := fset.Position(name.Pos())
								fmt.Printf("FUNC_PARAM: %s:%d: param %s\n", pos.Filename, pos.Line, name.Name)
								violations++
							}
						}
					}
				}

				// Check function results
				if v.Type.Results != nil {
					for _, field := range v.Type.Results.List {
						for _, name := range field.Names {
							if hasSnakeCase(name.Name) {
								pos := fset.Position(name.Pos())
								fmt.Printf("FUNC_RESULT: %s:%d: result %s\n", pos.Filename, pos.Line, name.Name)
								violations++
							}
						}
					}
				}

				// Check receiver names
				if v.Recv != nil {
					for _, field := range v.Recv.List {
						for _, name := range field.Names {
							if hasSnakeCase(name.Name) && name.Name != "_" {
								pos := fset.Position(name.Pos())
								fmt.Printf("RECV_NAME: %s:%d: receiver %s\n", pos.Filename, pos.Line, name.Name)
								violations++
							}
						}
					}
				}

			case *ast.TypeSpec:
				// Check type name
				if hasSnakeCase(v.Name.Name) && !v.Name.IsExported() {
					pos := fset.Position(v.Pos())
					fmt.Printf("TYPE_NAME: %s:%d: type %s\n", pos.Filename, pos.Line, v.Name.Name)
					violations++
				}

				// Check struct fields
				if structType, ok := v.Type.(*ast.StructType); ok {
					for _, field := range structType.Fields.List {
						for _, name := range field.Names {
							if hasSnakeCase(name.Name) {
								pos := fset.Position(name.Pos())
								fmt.Printf("STRUCT_FIELD: %s:%d: field %s\n", pos.Filename, pos.Line, name.Name)
								violations++
							}
						}
					}
				}

				// Check interface methods
				if ifaceType, ok := v.Type.(*ast.InterfaceType); ok {
					for _, method := range ifaceType.Methods.List {
						for _, name := range method.Names {
							if hasSnakeCase(name.Name) {
								pos := fset.Position(name.Pos())
								fmt.Printf("IFACE_METHOD: %s:%d: method %s\n", pos.Filename, pos.Line, name.Name)
								violations++
							}
						}
					}
				}

			case *ast.ValueSpec:
				// Check var/const names
				for _, name := range v.Names {
					if hasSnakeCase(name.Name) {
						// Check if it's a constant that's ALL CAPS (already filtered by hasSnakeCase)
						pos := fset.Position(name.Pos())
						kind := "VAR"
						if v.Gen != nil && v.Gen.Tok == token.CONST {
							kind = "CONST"
						}
						fmt.Printf("%s_DECL: %s:%d: %s %s\n", kind, pos.Filename, pos.Line, kind, name.Name)
						violations++
					}
				}

			case *ast.AssignStmt:
				// Check short variable declarations (:=)
				if v.Tok == token.DEFINE {
					for _, expr := range v.Lhs {
						if ident, ok := expr.(*ast.Ident); ok {
							if hasSnakeCase(ident.Name) {
								pos := fset.Position(ident.Pos())
								fmt.Printf("SHORT_VAR: %s:%d: %s :=\n", pos.Filename, pos.Line, ident.Name)
								violations++
							}
						}
					}
				}

			case *ast.RangeStmt:
				// Check range variables
				if v.Key != nil {
					if ident, ok := v.Key.(*ast.Ident); ok {
						if hasSnakeCase(ident.Name) {
							pos := fset.Position(ident.Pos())
							fmt.Printf("RANGE_VAR: %s:%d: %s := range\n", pos.Filename, pos.Line, ident.Name)
							violations++
						}
					}
				}
				if v.Value != nil {
					if ident, ok := v.Value.(*ast.Ident); ok {
						if hasSnakeCase(ident.Name) {
							pos := fset.Position(ident.Pos())
							fmt.Printf("RANGE_VAR: %s:%d: %s := range\n", pos.Filename, pos.Line, ident.Name)
							violations++
						}
					}
				}

			case *ast.ForStmt:
				// Check for-loop init variables
				if init, ok := v.Init.(*ast.AssignStmt); ok && init.Tok == token.DEFINE {
					for _, expr := range init.Lhs {
						if ident, ok := expr.(*ast.Ident); ok {
							if hasSnakeCase(ident.Name) {
								pos := fset.Position(ident.Pos())
								fmt.Printf("FOR_VAR: %s:%d: %s\n", pos.Filename, pos.Line, ident.Name)
								violations++
							}
						}
					}
				}
			}
			return true
		})

		return nil
	})

	if err != nil {
		fmt.Printf("Walk error: %v\n", err)
	}

	fmt.Printf("\n=== Total violations found: %d ===\n", violations)
}
