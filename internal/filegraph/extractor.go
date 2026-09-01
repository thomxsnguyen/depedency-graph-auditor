package filegraph

import (
	"fmt"
	"strings"
	"unicode"
)

type tokenKind uint8

const (
	tokenIdentifier tokenKind = iota
	tokenString
	tokenPunctuation
)

type sourceToken struct {
	kind  tokenKind
	value string
}

// ExtractImports returns relative module specifiers from supported
// JavaScript and TypeScript import forms.
func ExtractImports(source []byte) ([]string, error) {
	tokens, err := tokenize(source)
	if err != nil {
		return nil, err
	}

	imports := make([]string, 0)
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if token.kind != tokenIdentifier {
			continue
		}

		switch token.value {
		case "import":
			if specifier, ok := dynamicSpecifier(tokens, index); ok {
				imports = appendRelative(imports, specifier)
				continue
			}
			if index+1 < len(tokens) && tokens[index+1].kind == tokenString {
				imports = appendRelative(imports, tokens[index+1].value)
				continue
			}
			if specifier, ok := fromSpecifier(tokens, index+1); ok {
				imports = appendRelative(imports, specifier)
			}
		case "export":
			if specifier, ok := fromSpecifier(tokens, index+1); ok {
				imports = appendRelative(imports, specifier)
			}
		case "require":
			if index > 0 && tokens[index-1].kind == tokenPunctuation && tokens[index-1].value == "." {
				continue
			}
			if specifier, ok := dynamicSpecifier(tokens, index); ok {
				imports = appendRelative(imports, specifier)
			}
		}
	}
	return imports, nil
}

func appendRelative(imports []string, specifier string) []string {
	if strings.HasPrefix(specifier, "./") || strings.HasPrefix(specifier, "../") {
		return append(imports, specifier)
	}
	return imports
}

func dynamicSpecifier(tokens []sourceToken, index int) (string, bool) {
	if index+3 >= len(tokens) || tokens[index+1].value != "(" || tokens[index+2].kind != tokenString || tokens[index+3].value != ")" {
		return "", false
	}
	return tokens[index+2].value, true
}

func fromSpecifier(tokens []sourceToken, start int) (string, bool) {
	for index := start; index < len(tokens); index++ {
		if tokens[index].kind == tokenPunctuation && tokens[index].value == ";" {
			return "", false
		}
		if tokens[index].kind == tokenIdentifier && (tokens[index].value == "import" || tokens[index].value == "export") {
			return "", false
		}
		if tokens[index].kind == tokenIdentifier && tokens[index].value == "from" {
			if index+1 < len(tokens) && tokens[index+1].kind == tokenString {
				return tokens[index+1].value, true
			}
			return "", false
		}
	}
	return "", false
}

func tokenize(source []byte) ([]sourceToken, error) {
	tokens := make([]sourceToken, 0)
	for index := 0; index < len(source); {
		current := source[index]
		if unicode.IsSpace(rune(current)) {
			index++
			continue
		}
		if current == '/' && index+1 < len(source) && source[index+1] == '/' {
			index += 2
			for index < len(source) && source[index] != '\n' {
				index++
			}
			continue
		}
		if current == '/' && index+1 < len(source) && source[index+1] == '*' {
			end := index + 2
			for end+1 < len(source) && !(source[end] == '*' && source[end+1] == '/') {
				end++
			}
			if end+1 >= len(source) {
				return nil, fmt.Errorf("unterminated block comment")
			}
			index = end + 2
			continue
		}
		if current == '\'' || current == '"' {
			value, next, err := scanQuotedString(source, index, current)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, sourceToken{kind: tokenString, value: value})
			index = next
			continue
		}
		if current == '`' {
			next, err := skipTemplateLiteral(source, index)
			if err != nil {
				return nil, err
			}
			index = next
			continue
		}
		if isIdentifierStart(current) {
			end := index + 1
			for end < len(source) && isIdentifierPart(source[end]) {
				end++
			}
			tokens = append(tokens, sourceToken{kind: tokenIdentifier, value: string(source[index:end])})
			index = end
			continue
		}
		tokens = append(tokens, sourceToken{kind: tokenPunctuation, value: string(current)})
		index++
	}
	return tokens, nil
}

func scanQuotedString(source []byte, start int, quote byte) (string, int, error) {
	value := make([]byte, 0)
	for index := start + 1; index < len(source); index++ {
		switch source[index] {
		case quote:
			return string(value), index + 1, nil
		case '\n', '\r':
			return "", 0, fmt.Errorf("unterminated string literal")
		case '\\':
			if index+1 >= len(source) {
				return "", 0, fmt.Errorf("unterminated string escape")
			}
			index++
			value = append(value, source[index])
		default:
			value = append(value, source[index])
		}
	}
	return "", 0, fmt.Errorf("unterminated string literal")
}

func skipTemplateLiteral(source []byte, start int) (int, error) {
	for index := start + 1; index < len(source); index++ {
		if source[index] == '\\' {
			index++
			continue
		}
		if source[index] == '`' {
			return index + 1, nil
		}
	}
	return 0, fmt.Errorf("unterminated template literal")
}

func isIdentifierStart(value byte) bool {
	return value == '_' || value == '$' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func isIdentifierPart(value byte) bool {
	return isIdentifierStart(value) || value >= '0' && value <= '9'
}
