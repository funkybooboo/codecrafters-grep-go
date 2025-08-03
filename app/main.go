package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

var logMode = "debug" // Change to "info" or higher to reduce verbosity

var logLevels = map[string]int{
	"debug": 0,
	"info":  1,
	"warn":  2,
	"error": 3,
}

type token struct {
	kind  string // "literal", "digit", "word", "group", "negated_group"
	value string
}

func main() {
	log("main", "debug", "Starting program...")

	pattern, err := parseArgs(os.Args)
	if err != nil {
		log("main", "error", fmt.Sprintf("Argument parsing failed: %v", err))
		os.Exit(2)
	}

	line, err := io.ReadAll(os.Stdin)
	if err != nil {
		log("main", "error", fmt.Sprintf("Failed to read stdin: %v", err))
		os.Exit(2)
	}
	log("main", "debug", fmt.Sprintf("Read input: %q", line))

	ok, err := matchLine(line, pattern)
	if err != nil {
		log("main", "error", fmt.Sprintf("Match error: %v", err))
		os.Exit(2)
	}

	if !ok {
		log("main", "info", "Pattern not matched, exiting with code 1")
		os.Exit(1)
	}

	log("main", "info", "Pattern matched, exiting with code 0")
}

func parseArgs(args []string) (string, error) {
	log("parseArgs", "debug", "Parsing arguments...")
	if len(args) < 3 || args[1] != "-E" {
		return "", fmt.Errorf("usage: mygrep -E <pattern>")
	}
	log("parseArgs", "debug", fmt.Sprintf("Pattern received: %q", args[2]))
	return args[2], nil
}

func matchLine(line []byte, pattern string) (bool, error) {
	log("matchLine", "debug", fmt.Sprintf("Raw pattern: %q", pattern))

	pattern = unescapePattern(pattern)
	log("matchLine", "debug", fmt.Sprintf("Unescaped pattern: %q", pattern))

	anchored := false
	if strings.HasPrefix(pattern, "^") {
		anchored = true
		pattern = pattern[1:] // strip the ^
		log("matchLine", "debug", "Detected start anchor ^")
	}

	tokens, err := tokenizePattern(pattern)
	if err != nil {
		return false, err
	}
	log("matchLine", "debug", fmt.Sprintf("Tokenized into %d tokens", len(tokens)))

	ok := matchTokens(line, tokens, anchored)
	log("matchLine", "debug", fmt.Sprintf("Final match result: %v", ok))
	return ok, nil
}

func unescapePattern(p string) string {
	// Replace \\ with \
	return strings.ReplaceAll(p, `\\`, `\`)
}

func tokenizePattern(pat string) ([]token, error) {
	var tokens []token
	i := 0
	for i < len(pat) {
		switch {
		case pat[i] == '\\':
			if i+1 >= len(pat) {
				return nil, fmt.Errorf("dangling escape at end of pattern")
			}
			switch pat[i+1] {
			case 'd':
				tokens = append(tokens, token{"digit", ""})
			case 'w':
				tokens = append(tokens, token{"word", ""})
			default:
				return nil, fmt.Errorf("unsupported escape sequence: \\%c", pat[i+1])
			}
			i += 2

		case pat[i] == '[':
			j := i + 1
			for j < len(pat) && pat[j] != ']' {
				j++
			}
			if j >= len(pat) {
				return nil, fmt.Errorf("unterminated character group")
			}
			group := pat[i+1 : j]
			if strings.HasPrefix(group, "^") {
				tokens = append(tokens, token{"negated_group", group[1:]})
			} else {
				tokens = append(tokens, token{"group", group})
			}
			i = j + 1

		default:
			r, size := utf8.DecodeRuneInString(pat[i:])
			tokens = append(tokens, token{"literal", string(r)})
			i += size
		}
	}
	return tokens, nil
}

func matchTokens(input []byte, tokens []token, anchored bool) bool {
	inputRunes := []rune(string(input))

	startIndexes := []int{0}
	if !anchored {
		startIndexes = make([]int, len(inputRunes)-len(tokens)+1)
		for i := range startIndexes {
			startIndexes[i] = i
		}
	}

	for _, i := range startIndexes {
		if i+len(tokens) > len(inputRunes) {
			continue
		}

		match := true
		for j, tok := range tokens {
			c := inputRunes[i+j]
			switch tok.kind {
			case "digit":
				if c < '0' || c > '9' {
					match = false
				}
			case "word":
				if !(c >= 'a' && c <= 'z') &&
					!(c >= 'A' && c <= 'Z') &&
					!(c >= '0' && c <= '9') &&
					c != '_' {
					match = false
				}
			case "literal":
				if string(c) != tok.value {
					match = false
				}
			case "group":
				if !strings.ContainsRune(tok.value, c) {
					match = false
				}
			case "negated_group":
				if strings.ContainsRune(tok.value, c) {
					match = false
				}
			default:
				match = false
			}
			if !match {
				break
			}
		}

		if match {
			return true
		}
	}
	return false
}

func log(funcName, level, message string) {
	if logLevels[level] >= logLevels[logMode] {
		_, _ = fmt.Fprintf(os.Stderr, "[%s] [%s] %s\n", funcName, strings.ToUpper(level), message)
	}
}
