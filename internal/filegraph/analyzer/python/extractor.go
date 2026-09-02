package python

import (
	"fmt"
	"strings"
)

// Import is one module referenced by a Python import statement.
type Import struct {
	Module string
	Level  int
	Names  []string
}

// String reconstructs the meaningful module portion for diagnostics.
func (imported Import) String() string {
	module := strings.Repeat(".", imported.Level) + imported.Module
	if imported.Module == "" && len(imported.Names) > 0 {
		return module + strings.Join(imported.Names, ",")
	}
	return module
}

type tokenKind uint8

const (
	identifier tokenKind = iota
	punctuation
	newline
)

type token struct {
	kind  tokenKind
	value string
}

// ExtractImports parses ordinary import and from-import statements without
// executing or importing the source file.
func ExtractImports(source []byte) ([]Import, error) {
	tokens, err := tokenize(source)
	if err != nil {
		return nil, err
	}

	imports := make([]Import, 0)
	statementStart := true
	for index := 0; index < len(tokens); {
		current := tokens[index]
		if current.kind == newline || current.value == ";" {
			statementStart = true
			index++
			continue
		}
		if !statementStart || current.kind != identifier {
			statementStart = false
			index++
			continue
		}

		switch current.value {
		case "import":
			parsed, next, err := parseImport(tokens, index+1)
			if err != nil {
				return nil, err
			}
			imports = append(imports, parsed...)
			index = next
		case "from":
			parsed, next, err := parseFromImport(tokens, index+1)
			if err != nil {
				return nil, err
			}
			imports = append(imports, parsed)
			index = next
		default:
			statementStart = false
			index++
		}
	}
	return imports, nil
}

func parseImport(tokens []token, start int) ([]Import, int, error) {
	imports := make([]Import, 0, 1)
	index := start
	for {
		module, next := parseDottedName(tokens, index)
		if module == "" {
			return nil, index, fmt.Errorf("invalid Python import statement")
		}
		imports = append(imports, Import{Module: module})
		index = next
		if index < len(tokens) && tokens[index].kind == identifier && tokens[index].value == "as" {
			index++
			if index >= len(tokens) || tokens[index].kind != identifier {
				return nil, index, fmt.Errorf("invalid Python import alias")
			}
			index++
		}
		if index >= len(tokens) || tokens[index].value != "," {
			return imports, index, nil
		}
		index++
	}
}

func parseFromImport(tokens []token, start int) (Import, int, error) {
	index := start
	level := 0
	for index < len(tokens) && tokens[index].value == "." {
		level++
		index++
	}
	module := ""
	if index < len(tokens) && !(tokens[index].kind == identifier && tokens[index].value == "import") {
		var next int
		module, next = parseDottedName(tokens, index)
		index = next
	}
	if level == 0 && module == "" {
		return Import{}, index, fmt.Errorf("invalid Python from-import module")
	}
	if index >= len(tokens) || tokens[index].kind != identifier || tokens[index].value != "import" {
		return Import{}, index, fmt.Errorf("invalid Python from-import statement")
	}
	index++
	if index < len(tokens) && tokens[index].value == "(" {
		index++
	}

	names := make([]string, 0, 1)
	for index < len(tokens) && tokens[index].kind != newline && tokens[index].value != ";" && tokens[index].value != ")" {
		if tokens[index].kind == identifier || tokens[index].value == "*" {
			name := tokens[index].value
			if name == "as" {
				index += 2
				continue
			}
			names = append(names, name)
		}
		index++
	}
	if len(names) == 0 {
		return Import{}, index, fmt.Errorf("invalid Python from-import names")
	}
	return Import{Module: module, Level: level, Names: names}, index, nil
}

func parseDottedName(tokens []token, start int) (string, int) {
	if start >= len(tokens) || tokens[start].kind != identifier {
		return "", start
	}
	parts := []string{tokens[start].value}
	index := start + 1
	for index+1 < len(tokens) && tokens[index].value == "." && tokens[index+1].kind == identifier {
		parts = append(parts, tokens[index+1].value)
		index += 2
	}
	return strings.Join(parts, "."), index
}

func tokenize(source []byte) ([]token, error) {
	tokens := make([]token, 0)
	bracketDepth := 0
	for index := 0; index < len(source); {
		current := source[index]
		switch {
		case current == ' ' || current == '\t' || current == '\r':
			index++
		case current == '#':
			for index < len(source) && source[index] != '\n' {
				index++
			}
		case current == '\\' && index+1 < len(source) && source[index+1] == '\n':
			index += 2
		case current == '\n':
			if bracketDepth == 0 {
				tokens = append(tokens, token{kind: newline, value: "\n"})
			}
			index++
		case current == '\'' || current == '"':
			next, newlines, err := skipString(source, index, current)
			if err != nil {
				return nil, err
			}
			if bracketDepth == 0 {
				for range newlines {
					tokens = append(tokens, token{kind: newline, value: "\n"})
				}
			}
			index = next
		case isIdentifierStart(current):
			end := index + 1
			for end < len(source) && isIdentifierPart(source[end]) {
				end++
			}
			tokens = append(tokens, token{kind: identifier, value: string(source[index:end])})
			index = end
		default:
			value := string(current)
			tokens = append(tokens, token{kind: punctuation, value: value})
			if current == '(' || current == '[' || current == '{' {
				bracketDepth++
			} else if current == ')' || current == ']' || current == '}' {
				if bracketDepth > 0 {
					bracketDepth--
				}
			}
			index++
		}
	}
	return tokens, nil
}

func skipString(source []byte, start int, quote byte) (int, int, error) {
	triple := start+2 < len(source) && source[start+1] == quote && source[start+2] == quote
	index := start + 1
	if triple {
		index = start + 3
	}
	newlines := 0
	for index < len(source) {
		if source[index] == '\\' {
			index += 2
			continue
		}
		if source[index] == '\n' {
			newlines++
			if !triple {
				return 0, 0, fmt.Errorf("unterminated Python string literal")
			}
		}
		if triple && index+2 < len(source) && source[index] == quote && source[index+1] == quote && source[index+2] == quote {
			return index + 3, newlines, nil
		}
		if !triple && source[index] == quote {
			return index + 1, newlines, nil
		}
		index++
	}
	return 0, 0, fmt.Errorf("unterminated Python string literal")
}

func isIdentifierStart(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func isIdentifierPart(value byte) bool {
	return isIdentifierStart(value) || value >= '0' && value <= '9'
}
