package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

var logMode = "debug" // Change to "info" or higher to reduce verbosity
var logLevels = map[string]int{
	"debug": 0,
	"info":  1,
	"warn":  2,
	"error": 3,
}

// parseArgs handles [-r] -E <pattern> [paths...]
func parseArgs(args []string) (recursive bool, pattern string, paths []string, err error) {
	i := 1
	if i < len(args) && args[i] == "-r" {
		recursive = true
		i++
	}
	if i >= len(args) || args[i] != "-E" {
		return false, "", nil, fmt.Errorf("usage: mygrep [-r] -E <pattern> [paths...]")
	}
	i++
	if i >= len(args) {
		return false, "", nil, fmt.Errorf("usage: mygrep [-r] -E <pattern> [paths...]")
	}
	pattern = args[i]
	i++
	paths = args[i:]
	return
}

// scanAndPrint reads from reader line by line, applies pattern,
// prints matching lines (with optional filename prefix), and
// returns true if any lines matched.
func scanAndPrint(prefix string, reader io.Reader, pattern string, addPrefix bool) bool {
	scanner := bufio.NewScanner(reader)
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		log("scanAndPrint", "debug", fmt.Sprintf("Scanning line: %q", line))
		ok, err := matchLine([]byte(line), pattern)
		if err != nil {
			log("scanAndPrint", "error", fmt.Sprintf("Match error: %v", err))
			os.Exit(2)
		}
		if ok {
			if addPrefix {
				fmt.Printf("%s:%s\n", prefix, line)
			} else {
				fmt.Println(line)
			}
			found = true
		}
	}
	if err := scanner.Err(); err != nil {
		log("scanAndPrint", "error", fmt.Sprintf("Error reading %s: %v", prefix, err))
		os.Exit(2)
	}
	return found
}

func unescapePattern(p string) string {
	return strings.ReplaceAll(p, `\\`, `\`)
}

func log(funcName, level, message string) {
	if logLevels[level] >= logLevels[logMode] {
		fmt.Fprintf(os.Stderr, "[%s] [%s] %s\n",
			funcName, strings.ToUpper(level), message)
	}
}
