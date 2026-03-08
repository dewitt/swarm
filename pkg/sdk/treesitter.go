package sdk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// generateGoSkeleton uses tree-sitter to parse a Go file and extract only
// the high-level structural declarations (types, methods, functions) without
// their internal implementation bodies, saving massive amounts of context tokens.
func generateGoSkeleton(path string) (string, error) {
	code, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())

	tree, err := parser.ParseCtx(context.Background(), nil, code)
	if err != nil {
		return "", fmt.Errorf("failed to parse file: %w", err)
	}

	n := tree.RootNode()

	// Query for high-level structural elements
	q, err := sitter.NewQuery([]byte(`
		(type_declaration) @type
		(method_declaration) @method
		(function_declaration) @func
	`), golang.GetLanguage())
	if err != nil {
		return "", fmt.Errorf("failed to compile query: %w", err)
	}

	qc := sitter.NewQueryCursor()
	qc.Exec(q, n)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("// SKELETON OF %s\n\n", path))

	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range m.Captures {
			node := c.Node
			
			if node.Type() == "function_declaration" || node.Type() == "method_declaration" {
				for i := 0; i < int(node.ChildCount()); i++ {
					child := node.Child(i)
					if child.Type() == "block" {
						// Skip the implementation body
						continue
					}
					sb.WriteString(child.Content(code) + " ")
				}
				sb.WriteString("{ /* ... */ }\n")
			} else if node.Type() == "type_declaration" {
				// We print the whole type declaration (structs, interfaces)
				// We could theoretically strip inner methods of interfaces, but structs are fine.
				sb.WriteString(node.Content(code) + "\n")
			}
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// getCodeSkeleton determines the language and extracts the skeleton.
func getCodeSkeleton(path string) (string, error) {
	ext := filepath.Ext(path)
	switch ext {
	case ".go":
		return generateGoSkeleton(path)
	default:
		return "", fmt.Errorf("tree-sitter skeleton extraction is currently only supported for .go files (got %s)", ext)
	}
}
