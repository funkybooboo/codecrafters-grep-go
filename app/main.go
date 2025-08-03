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
	kind       string // "literal", "digit", "word", "group", "negated_group"
	value      string
	quantifier string // "", "+" (for now)
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

	anchoredStart := false
	anchoredEnd := false

	if strings.HasPrefix(pattern, "^") {
		anchoredStart = true
		pattern = pattern[1:]
		log("matchLine", "debug", "Detected start anchor ^")
	}

	if strings.HasSuffix(pattern, "$") {
		anchoredEnd = true
		pattern = pattern[:len(pattern)-1]
		log("matchLine", "debug", "Detected end anchor $")
	}

	tokens, err := tokenizePattern(pattern)
	if err != nil {
		return false, err
	}
	log("matchLine", "debug", fmt.Sprintf("Tokenized into %d tokens", len(tokens)))

	ok := matchTokens(line, tokens, anchoredStart, anchoredEnd)
	log("matchLine", "debug", fmt.Sprintf("Final match result: %v", ok))
	return ok, nil
}

func unescapePattern(p string) string {
	return strings.ReplaceAll(p, `\\`, `\`)
}

func tokenizePattern(pat string) ([]token, error) {
	var tokens []token
	i := 0
	for i < len(pat) {
		var tok token

		switch {
		case pat[i] == '\\':
			if i+1 >= len(pat) {
				return nil, fmt.Errorf("dangling escape at end of pattern")
			}
			switch pat[i+1] {
			case 'd':
				tok = token{"digit", "", ""}
			case 'w':
				tok = token{"word", "", ""}
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
				tok = token{"negated_group", group[1:], ""}
			} else {
				tok = token{"group", group, ""}
			}
			i = j + 1

		default:
			r, size := utf8.DecodeRuneInString(pat[i:])
			tok = token{"literal", string(r), ""}
			i += size
		}

		if i < len(pat) && pat[i] == '+' {
			tok.quantifier = "+"
			i++
		}

		tokens = append(tokens, tok)
	}
	return tokens, nil
}

func matchTokens(input []byte, tokens []token, anchoredStart, anchoredEnd bool) bool {
	inputRunes := []rune(string(input))
	startPositions := []int{0}

	if !anchoredStart {
		startPositions = make([]int, len(inputRunes)+1)
		for i := range startPositions {
			startPositions[i] = i
		}
	}

	for _, start := range startPositions {
		pos := start
		tokIdx := 0
		match := true

		for tokIdx < len(tokens) {
			tok := tokens[tokIdx]
			startPos := pos
			count := 0

			for pos < len(inputRunes) && tokenMatchesRune(tok, inputRunes[pos]) {
				count++
				pos++
				if tok.quantifier != "+" {
					break
				}
			}

			if tok.quantifier == "+" && count == 0 {
				match = false
				break
			}
			if tok.quantifier == "" && count != 1 {
				match = false
				break
			}

			if tok.quantifier == "+" && tokIdx+1 < len(tokens) {
				for back := pos; back > startPos; back-- {
					if matchTokensFrom(inputRunes, back, tokens[tokIdx+1:], anchoredEnd) {
						return true
					}
				}
				match = false
				break
			}

			tokIdx++
		}

		if match {
			if anchoredEnd && pos != len(inputRunes) {
				continue
			}
			return true
		}
	}
	return false
}

func matchTokensFrom(inputRunes []rune, pos int, tokens []token, anchoredEnd bool) bool {
	tokIdx := 0
	for tokIdx < len(tokens) {
		if pos >= len(inputRunes) {
			return false
		}

		tok := tokens[tokIdx]
		startPos := pos
		count := 0

		for pos < len(inputRunes) && tokenMatchesRune(tok, inputRunes[pos]) {
			count++
			pos++
			if tok.quantifier != "+" {
				break
			}
		}

		if tok.quantifier == "+" && count == 0 {
			return false
		}
		if tok.quantifier == "" && count != 1 {
			return false
		}

		if tok.quantifier == "+" && tokIdx+1 < len(tokens) {
			for back := pos; back > startPos; back-- {
				if matchTokensFrom(inputRunes, back, tokens[tokIdx+1:], anchoredEnd) {
					return true
				}
			}
			return false
		}

		tokIdx++
	}

	if anchoredEnd {
		return pos == len(inputRunes)
	}
	return true
}

func tokenMatchesRune(tok token, c rune) bool {
	switch tok.kind {
	case "digit":
		return c >= '0' && c <= '9'
	case "word":
		return (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '_'
	case "literal":
		return string(c) == tok.value
	case "group":
		return strings.ContainsRune(tok.value, c)
	case "negated_group":
		return !strings.ContainsRune(tok.value, c)
	default:
		return false
	}
}

func log(funcName, level, message string) {
	if logLevels[level] >= logLevels[logMode] {
		_, _ = fmt.Fprintf(os.Stderr, "[%s] [%s] %s\n", funcName, strings.ToUpper(level), message)
	}
}
