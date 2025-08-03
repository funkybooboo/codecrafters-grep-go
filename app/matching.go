package main

import (
	"fmt"
	"strings"
)

// matchLine applies ^/$ anchors, parses into an AST with nested capture‐groups/backrefs,
// then backtracks to see if the input matches.
func matchLine(input []byte, pattern string) (bool, error) {
	log("matchLine", "debug", fmt.Sprintf("Raw pattern: %q", pattern))

	pattern = unescapePattern(pattern)
	log("matchLine", "debug", fmt.Sprintf("Unescaped pattern: %q", pattern))

	// handle anchors
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

	// parse into AST (with nested capture numbering)
	p := newParser(pattern)
	root, err := p.parse()
	if err != nil {
		return false, err
	}
	if p.pos < len(p.pattern) {
		return false, fmt.Errorf("unexpected character at position %d", p.pos)
	}

	// convert input to runes
	runes := []rune(string(input))
	emptyCaps := make(map[int][]rune)

	// try to match starting at a given offset
	tryMatch := func(start int) bool {
		results := matchNode(root, runes, start, emptyCaps)
		for _, r := range results {
			if !anchoredEnd || r.pos == len(runes) {
				log("matchLine", "debug", fmt.Sprintf("Matched at [%d:%d]", start, r.pos))
				return true
			}
		}
		return false
	}

	// if anchored at start, only try offset 0
	if anchoredStart {
		return tryMatch(0), nil
	}
	// otherwise try every offset
	for i := 0; i <= len(runes); i++ {
		if tryMatch(i) {
			return true, nil
		}
	}
	return false, nil
}
