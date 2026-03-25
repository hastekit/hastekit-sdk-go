package codesplitters

import (
	"fmt"
	"strings"
	"unicode"

	treesitter "github.com/tree-sitter/go-tree-sitter"
	treesittergo "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// goQuery is the S-expression query to find top-level Go declarations.
const goQuery = `
(function_declaration name: (identifier) @name) @decl
(method_declaration receiver: (parameter_list) @receiver name: (field_identifier) @name) @decl
(type_declaration (type_spec name: (type_identifier) @name)) @decl
`

// GoParser implements LanguageParser for Go source code using tree-sitter.
type GoParser struct{}

// Compile-time check.
var _ LanguageParser = (*GoParser)(nil)

// NewGoParser creates a new Go language parser.
func NewGoParser() *GoParser {
	return &GoParser{}
}

func (p *GoParser) Language() string     { return "go" }
func (p *GoParser) Extensions() []string { return []string{".go"} }

// Parse parses Go source code and returns code chunks for each top-level
// function, method, and type declaration.
func (p *GoParser) Parse(source []byte) ([]CodeChunk, error) {
	parser := treesitter.NewParser()
	defer parser.Close()

	lang := treesitter.NewLanguage(treesittergo.Language())
	if err := parser.SetLanguage(lang); err != nil {
		return nil, fmt.Errorf("set language: %w", err)
	}

	tree := parser.Parse(source, nil)
	if tree == nil {
		return nil, fmt.Errorf("parse returned nil tree")
	}
	defer tree.Close()

	root := tree.RootNode()

	// Extract package name from the tree (single pass).
	pkgName := extractGoPackage(root, source)

	// Run the query to find declarations.
	query, err := treesitter.NewQuery(lang, goQuery)
	if err != nil {
		return nil, fmt.Errorf("compile query: %w", err)
	}
	defer query.Close()

	cursor := treesitter.NewQueryCursor()
	defer cursor.Close()

	var chunks []CodeChunk
	matches := cursor.Matches(query, root, source)
	for match := matches.Next(); match != nil; match = matches.Next() {
		declNode := nodeForCapture(match, query, "decl")
		nameNode := nodeForCapture(match, query, "name")
		if declNode == nil || nameNode == nil {
			continue
		}

		name := nameNode.Utf8Text(source)
		kind := normalizeGoKind(declNode.Kind())

		chunk := CodeChunk{
			Kind:       kind,
			Name:       name,
			Package:    pkgName,
			Exported:   name != "" && unicode.IsUpper(rune(name[0])),
			DocComment: precedingDocComment(declNode, source),
			StartLine:  declNode.StartPosition().Row + 1,
			EndLine:    declNode.EndPosition().Row + 1,
			Code:       declNode.Utf8Text(source),
			Snippet:    goSnippet(declNode, source, kind),
		}

		if kind == "function" || kind == "method" {
			chunk.Signature = goSignature(declNode, source)
			if kind == "method" {
				if recNode := nodeForCapture(match, query, "receiver"); recNode != nil {
					chunk.Receiver = recNode.Utf8Text(source)
				}
			}
		}

		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// extractGoPackage finds the package name from the AST root node.
func extractGoPackage(root *treesitter.Node, source []byte) string {
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() == "package_clause" {
			for j := uint(0); j < child.ChildCount(); j++ {
				pkgChild := child.Child(j)
				if pkgChild.Kind() == "package_identifier" {
					return pkgChild.Utf8Text(source)
				}
			}
		}
	}
	return ""
}

// normalizeGoKind maps tree-sitter node kinds to our canonical kind strings.
func normalizeGoKind(tsKind string) string {
	switch tsKind {
	case "function_declaration":
		return "function"
	case "method_declaration":
		return "method"
	case "type_declaration":
		return "type"
	default:
		return tsKind
	}
}

// goSignature returns the parameter list and return type for a function/method node.
func goSignature(decl *treesitter.Node, source []byte) string {
	params := decl.ChildByFieldName("parameters")
	if params == nil {
		return "()"
	}
	sig := params.Utf8Text(source)
	result := decl.ChildByFieldName("result")
	if result != nil {
		sig += " " + result.Utf8Text(source)
	}
	return sig
}

// goSnippet returns a short declaration snippet (no body for funcs, first line for types).
func goSnippet(decl *treesitter.Node, source []byte, kind string) string {
	if kind == "type" {
		text := decl.Utf8Text(source)
		for i, c := range text {
			if c == '\n' || c == '{' {
				return text[:i]
			}
		}
		return text
	}
	// For func/method: everything up to and including the return type (no body).
	params := decl.ChildByFieldName("parameters")
	if params == nil {
		return ""
	}
	start := decl.StartByte()
	end := params.EndByte()
	resultNode := decl.ChildByFieldName("result")
	if resultNode != nil {
		end = resultNode.EndByte()
	}
	return string(source[start:end])
}

// nodeForCapture returns the first node in the match for the given capture name, or nil.
func nodeForCapture(match *treesitter.QueryMatch, query *treesitter.Query, name string) *treesitter.Node {
	idx, ok := query.CaptureIndexForName(name)
	if !ok {
		return nil
	}
	for _, c := range match.Captures {
		if c.Index == uint32(idx) {
			return &c.Node
		}
	}
	return nil
}

// precedingDocComment returns the text of the comment node immediately before decl, if any.
func precedingDocComment(decl *treesitter.Node, source []byte) string {
	prev := decl.PrevSibling()
	if prev == nil {
		return ""
	}
	if prev.IsExtra() || prev.Kind() == "comment" {
		return prev.Utf8Text(source)
	}
	return ""
}

// IsGeneratedGoFile checks if Go source content indicates a generated file.
func IsGeneratedGoFile(content []byte) bool {
	checkLen := 1024
	if len(content) < checkLen {
		checkLen = len(content)
	}
	header := strings.ToLower(string(content[:checkLen]))
	return strings.Contains(header, "code generated") ||
		strings.Contains(header, "do not edit") ||
		strings.Contains(header, "auto-generated") ||
		strings.Contains(header, "automatically generated")
}
