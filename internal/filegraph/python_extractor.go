package filegraph

import (
	"fmt"
	"strings"
)

// PythonImport is one module referenced by a Python import statement.
type PythonImport struct {
	Module string
	Level  int
	Names  []string
}

// String reconstructs the meaningful module portion for diagnostics.
func (imported PythonImport) String() string {
	module := strings.Repeat(".", imported.Level) + imported.Module
	if imported.Module == "" && len(imported.Names) > 0 {
		return module + strings.Join(imported.Names, ",")
	}
	return module
}

type pythonTokenKind uint8

const (
	pythonIdentifier pythonTokenKind = iota
	pythonPunctuation
	pythonNewline
)

type pythonToken struct {
	kind  pythonTokenKind
	value string
}

// ExtractPythonImports parses ordinary import and from-import statements
// without executing or importing the source file.
func ExtractPythonImports(source []byte) ([]PythonImport, error) {
	tokens, err := tokenizePython(source)
	if err != nil {
		return nil, err
	}

	imports := make([]PythonImport, 0)
	statementStart := true
	for index := 0; index < len(tokens); {
		token := tokens[index]
		if token.kind == pythonNewline || token.value == ";" {
			statementStart = true
			index++
			continue
		}
		if !statementStart || token.kind != pythonIdentifier {
			statementStart = false
			index++
			continue
		}

		switch token.value {
		case "import":
			parsed, next, err := parsePythonImport(tokens, index+1)
			if err != nil {
				return nil, err
			}
			imports = append(imports, parsed...)
			index = next
		case "from":
			parsed, next, err := parsePythonFromImport(tokens, index+1)
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

func parsePythonImport(tokens []pythonToken, start int) ([]PythonImport, int, error) {
	imports := make([]PythonImport, 0, 1)
	index := start
	for {
		module, next := parseDottedPythonName(tokens, index)
		if module == "" {
			return nil, index, fmt.Errorf("invalid Python import statement")
		}
		imports = append(imports, PythonImport{Module: module})
		index = next
		if index < len(tokens) && tokens[index].kind == pythonIdentifier && tokens[index].value == "as" {
			index++
			if index >= len(tokens) || tokens[index].kind != pythonIdentifier {
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

func parsePythonFromImport(tokens []pythonToken, start int) (PythonImport, int, error) {
	index := start
	level := 0
	for index < len(tokens) && tokens[index].value == "." {
		level++
		index++
	}
	module := ""
	if index < len(tokens) && !(tokens[index].kind == pythonIdentifier && tokens[index].value == "import") {
		var next int
		module, next = parseDottedPythonName(tokens, index)
		index = next
	}
	if level == 0 && module == "" {
		return PythonImport{}, index, fmt.Errorf("invalid Python from-import module")
	}
	if index >= len(tokens) || tokens[index].kind != pythonIdentifier || tokens[index].value != "import" {
		return PythonImport{}, index, fmt.Errorf("invalid Python from-import statement")
	}
	index++
	if index < len(tokens) && tokens[index].value == "(" {
		index++
	}

	names := make([]string, 0, 1)
	for index < len(tokens) && tokens[index].kind != pythonNewline && tokens[index].value != ";" && tokens[index].value != ")" {
		if tokens[index].kind == pythonIdentifier || tokens[index].value == "*" {
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
		return PythonImport{}, index, fmt.Errorf("invalid Python from-import names")
	}
	return PythonImport{Module: module, Level: level, Names: names}, index, nil
}

func parseDottedPythonName(tokens []pythonToken, start int) (string, int) {
	if start >= len(tokens) || tokens[start].kind != pythonIdentifier {
		return "", start
	}
	parts := []string{tokens[start].value}
	index := start + 1
	for index+1 < len(tokens) && tokens[index].value == "." && tokens[index+1].kind == pythonIdentifier {
		parts = append(parts, tokens[index+1].value)
		index += 2
	}
	return strings.Join(parts, "."), index
}

func tokenizePython(source []byte) ([]pythonToken, error) {
	tokens := make([]pythonToken, 0)
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
				tokens = append(tokens, pythonToken{kind: pythonNewline, value: "\n"})
			}
			index++
		case current == '\'' || current == '"':
			next, newlines, err := skipPythonString(source, index, current)
			if err != nil {
				return nil, err
			}
			if bracketDepth == 0 {
				for range newlines {
					tokens = append(tokens, pythonToken{kind: pythonNewline, value: "\n"})
				}
			}
			index = next
		case isPythonIdentifierStart(current):
			end := index + 1
			for end < len(source) && isPythonIdentifierPart(source[end]) {
				end++
			}
			tokens = append(tokens, pythonToken{kind: pythonIdentifier, value: string(source[index:end])})
			index = end
		default:
			value := string(current)
			tokens = append(tokens, pythonToken{kind: pythonPunctuation, value: value})
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

func skipPythonString(source []byte, start int, quote byte) (int, int, error) {
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

func isPythonIdentifierStart(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func isPythonIdentifierPart(value byte) bool {
	return isPythonIdentifierStart(value) || value >= '0' && value <= '9'
}
