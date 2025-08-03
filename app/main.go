package main

import (
	"bytes"
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
	log("matchLine", "debug", fmt.Sprintf("Matching pattern: %q", pattern))

	switch pattern {
	case `\d`:
		log("matchLine", "debug", "Pattern is \\d — matching any digit")
		for _, b := range line {
			if b >= '0' && b <= '9' {
				log("matchLine", "debug", fmt.Sprintf("Found digit: %q", b))
				return true, nil
			}
		}
		log("matchLine", "debug", "No digit found")
		return false, nil

	case `\w`:
		log("matchLine", "debug", "Pattern is \\w — matching alphanumeric or underscore")
		for _, b := range line {
			if (b >= 'a' && b <= 'z') ||
				(b >= 'A' && b <= 'Z') ||
				(b >= '0' && b <= '9') ||
				b == '_' {
				log("matchLine", "debug", fmt.Sprintf("Found alphanumeric/underscore: %q", b))
				return true, nil
			}
		}
		log("matchLine", "debug", "No alphanumeric/underscore character found")
		return false, nil
	}

	// Handle positive character group like [abc]
	if strings.HasPrefix(pattern, "[") && strings.HasSuffix(pattern, "]") {
		log("matchLine", "debug", "Pattern is a character group")
		if len(pattern) <= 2 {
			return false, fmt.Errorf("empty character group: %q", pattern)
		}
		charGroup := pattern[1 : len(pattern)-1] // remove brackets
		log("matchLine", "debug", fmt.Sprintf("Character group contents: %q", charGroup))
		ok := bytes.ContainsAny(line, charGroup)
		log("matchLine", "debug", fmt.Sprintf("Group match result: %v", ok))
		return ok, nil
	}

	// Default: literal character match (single rune)
	if utf8.RuneCountInString(pattern) != 1 {
		return false, fmt.Errorf("unsupported pattern: %q", pattern)
	}

	log("matchLine", "debug", fmt.Sprintf("Pattern is literal: %q", pattern))
	ok := bytes.ContainsAny(line, pattern)
	log("matchLine", "debug", fmt.Sprintf("Match result: %v", ok))
	return ok, nil
}

func log(funcName, level, message string) {
	if logLevels[level] >= logLevels[logMode] {
		_, _ = fmt.Fprintf(os.Stderr, "[%s] [%s] %s\n", funcName, strings.ToUpper(level), message)
	}
}
