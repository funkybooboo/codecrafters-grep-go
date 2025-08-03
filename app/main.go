package main

import (
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

// parseArgs validates and returns the -E pattern.
func parseArgs(args []string) (string, error) {
	log("parseArgs", "debug", "Parsing arguments...")
	if len(args) < 3 || args[1] != "-E" {
		return "", fmt.Errorf("usage: mygrep -E <pattern>")
	}
	log("parseArgs", "debug", fmt.Sprintf("Pattern received: %q", args[2]))
	return args[2], nil
}

// unescapePattern turns "\\d" → "\d", "\\w" → "\w", etc.
func unescapePattern(p string) string {
	return strings.ReplaceAll(p, `\\`, `\`)
}

// matchLine applies anchors, parses the pattern into an AST, then
// backtracks to see if it matches the input.
func matchLine(input []byte, pattern string) (bool, error) {
	log("matchLine", "debug", fmt.Sprintf("Raw pattern: %q", pattern))
	pattern = unescapePattern(pattern)
	log("matchLine", "debug", fmt.Sprintf("Unescaped pattern: %q", pattern))

	// handle ^ and $ anchors
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

	// parse into AST
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

	// Attempt match
	if anchoredStart {
		ends := matchNode(root, runes, 0)
		for _, e := range ends {
			if !anchoredEnd || e == len(runes) {
				log("matchLine", "debug", fmt.Sprintf("Matched at [0:%d]", e))
				return true, nil
			}
		}
		return false, nil
	}

	// unanchored start: try every offset
	for start := 0; start <= len(runes); start++ {
		ends := matchNode(root, runes, start)
		for _, e := range ends {
			if !anchoredEnd || e == len(runes) {
				log("matchLine", "debug", fmt.Sprintf("Matched at [%d:%d]", start, e))
				return true, nil
			}
		}
	}
	return false, nil
}

//─── AST Node Types ────────────────────────────────────────────────────────────

type node interface{}

type literalNode struct{ char rune }
type digitNode struct{}
type wordNode struct{}
type anyNode struct{}
type charClassNode struct {
	set     map[rune]bool
	negated bool
}
type sequenceNode struct{ children []node }
type altNode struct{ alternatives []node }
type repNode struct {
	child    node
	min, max int // max<0 means “infinite”
}

//─── Parser ────────────────────────────────────────────────────────────────────

type parser struct {
	pattern []rune
	pos     int
}

func newParser(p string) *parser {
	return &parser{pattern: []rune(p), pos: 0}
}

func (p *parser) parse() (node, error) {
	return p.parseAlternation()
}

func (p *parser) parseAlternation() (node, error) {
	first, err := p.parseConcatenation()
	if err != nil {
		return nil, err
	}
	alts := []node{first}
	for p.pos < len(p.pattern) && p.pattern[p.pos] == '|' {
		p.pos++
		next, err := p.parseConcatenation()
		if err != nil {
			return nil, err
		}
		alts = append(alts, next)
	}
	if len(alts) == 1 {
		return first, nil
	}
	return &altNode{alternatives: alts}, nil
}

func (p *parser) parseConcatenation() (node, error) {
	var parts []node
	for p.pos < len(p.pattern) {
		ch := p.pattern[p.pos]
		if ch == ')' || ch == '|' {
			break
		}
		n, err := p.parseRepetition()
		if err != nil {
			return nil, err
		}
		parts = append(parts, n)
	}
	if len(parts) == 0 {
		return &sequenceNode{children: nil}, nil
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	return &sequenceNode{children: parts}, nil
}

func (p *parser) parseRepetition() (node, error) {
	atom, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.pattern) {
		switch p.pattern[p.pos] {
		case '+':
			p.pos++
			return &repNode{child: atom, min: 1, max: -1}, nil
		case '?':
			p.pos++
			return &repNode{child: atom, min: 0, max: 1}, nil
		}
	}
	return atom, nil
}

func (p *parser) parseAtom() (node, error) {
	if p.pos >= len(p.pattern) {
		return nil, fmt.Errorf("unexpected end of pattern")
	}
	ch := p.pattern[p.pos]
	switch ch {
	case '(':
		p.pos++
		sub, err := p.parseAlternation()
		if err != nil {
			return nil, err
		}
		if p.pos >= len(p.pattern) || p.pattern[p.pos] != ')' {
			return nil, fmt.Errorf("unterminated group")
		}
		p.pos++
		return sub, nil
	case '.':
		p.pos++
		return &anyNode{}, nil
	case '\\':
		p.pos++
		if p.pos >= len(p.pattern) {
			return nil, fmt.Errorf("dangling escape")
		}
		esc := p.pattern[p.pos]
		p.pos++
		switch esc {
		case 'd':
			return &digitNode{}, nil
		case 'w':
			return &wordNode{}, nil
		default:
			return nil, fmt.Errorf("unsupported escape: \\%c", esc)
		}
	case '[':
		p.pos++
		neg := false
		if p.pos < len(p.pattern) && p.pattern[p.pos] == '^' {
			neg = true
			p.pos++
		}
		set := make(map[rune]bool)
		for p.pos < len(p.pattern) && p.pattern[p.pos] != ']' {
			set[p.pattern[p.pos]] = true
			p.pos++
		}
		if p.pos >= len(p.pattern) || p.pattern[p.pos] != ']' {
			return nil, fmt.Errorf("unterminated character class")
		}
		p.pos++
		return &charClassNode{set: set, negated: neg}, nil
	default:
		p.pos++
		return &literalNode{char: ch}, nil
	}
}

//─── Matcher ───────────────────────────────────────────────────────────────────

func matchNode(n node, runes []rune, pos int) []int {
	switch x := n.(type) {
	case *literalNode:
		if pos < len(runes) && runes[pos] == x.char {
			return []int{pos + 1}
		}
		return nil
	case *digitNode:
		if pos < len(runes) && runes[pos] >= '0' && runes[pos] <= '9' {
			return []int{pos + 1}
		}
		return nil
	case *wordNode:
		if pos < len(runes) {
			c := runes[pos]
			if (c >= 'a' && c <= 'z') ||
				(c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') ||
				c == '_' {
				return []int{pos + 1}
			}
		}
		return nil
	case *anyNode:
		if pos < len(runes) {
			return []int{pos + 1}
		}
		return nil
	case *charClassNode:
		if pos < len(runes) {
			_, in := x.set[runes[pos]]
			if x.negated {
				if !in {
					return []int{pos + 1}
				}
			} else {
				if in {
					return []int{pos + 1}
				}
			}
		}
		return nil
	case *sequenceNode:
		positions := []int{pos}
		for _, child := range x.children {
			var next []int
			for _, p := range positions {
				next = append(next, matchNode(child, runes, p)...)
			}
			positions = unique(next)
			if len(positions) == 0 {
				break
			}
		}
		return positions
	case *altNode:
		var all []int
		for _, alt := range x.alternatives {
			all = append(all, matchNode(alt, runes, pos)...)
		}
		return unique(all)
	case *repNode:
		return matchRep(x, runes, pos, 0)
	default:
		return nil
	}
}

func matchRep(r *repNode, runes []rune, pos, count int) []int {
	var results []int
	// if we've satisfied the minimum, allow stopping here
	if count >= r.min {
		results = append(results, pos)
	}
	// if we've hit the maximum (and max>=0), stop
	if r.max >= 0 && count == r.max {
		return unique(results)
	}
	// otherwise, try one more repetition
	next := matchNode(r.child, runes, pos)
	for _, np := range next {
		if np == pos {
			// safety: never loop infinitely on empty matches
			continue
		}
		results = append(results, matchRep(r, runes, np, count+1)...)
	}
	return unique(results)
}

// unique removes duplicate ints
func unique(xs []int) []int {
	seen := make(map[int]bool)
	var out []int
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// log prints to stderr according to the current logMode
func log(funcName, level, message string) {
	if logLevels[level] >= logLevels[logMode] {
		_, _ = fmt.Fprintf(os.Stderr, "[%s] [%s] %s\n",
			funcName, strings.ToUpper(level), message)
	}
}
