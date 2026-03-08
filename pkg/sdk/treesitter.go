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

	// Query for high-level structural elements and their identifiers
	q, err := sitter.NewQuery([]byte(`
		(type_declaration (type_spec name: (type_identifier) @name)) @decl
		(method_declaration name: (field_identifier) @name) @decl
		(function_declaration name: (identifier) @name) @decl
	`), golang.GetLanguage())
	if err != nil {
		return "", fmt.Errorf("failed to compile query: %w", err)
	}

	qc := sitter.NewQueryCursor()
	qc.Exec(q, n)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("// SKELETON OF %s\n\n", path))

	// We use a map to deduplicate because @decl and @name trigger matches
	processedDecls := make(map[*sitter.Node]bool)

	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		var declNode *sitter.Node
		var nameNode *sitter.Node

		for _, c := range m.Captures {
			captureName := q.CaptureNameForId(c.Index)
			if captureName == "decl" {
				declNode = c.Node
			} else if captureName == "name" {
				nameNode = c.Node
			}
		}

		if declNode != nil && nameNode != nil {
			if processedDecls[declNode] {
				continue
			}
			processedDecls[declNode] = true

			startPos := nameNode.StartPoint()
			line := startPos.Row + 1
			col := startPos.Column + 1

			if declNode.Type() == "function_declaration" || declNode.Type() == "method_declaration" {
				sb.WriteString(fmt.Sprintf("// L%d:C%d\n", line, col))
				for i := 0; i < int(declNode.ChildCount()); i++ {
					child := declNode.Child(i)
					if child.Type() == "block" {
						// Skip the implementation body
						continue
					}
					sb.WriteString(child.Content(code) + " ")
				}
				sb.WriteString("{ /* ... */ }\n")
			} else if declNode.Type() == "type_declaration" {
				sb.WriteString(fmt.Sprintf("// L%d:C%d\n", line, col))
				sb.WriteString(declNode.Content(code) + "\n")
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
