package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
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

	// Parse the -E pattern
	pattern, err := parseArgs(os.Args)
	if err != nil {
		log("main", "error", fmt.Sprintf("Argument parsing failed: %v", err))
		os.Exit(2)
	}

	// Determine input source: either a file or stdin
	var reader io.Reader
	if len(os.Args) > 3 {
		filename := os.Args[3]
		file, err := os.Open(filename)
		if err != nil {
			log("main", "error", fmt.Sprintf("Failed to open file %q: %v", filename, err))
			os.Exit(2)
		}
		defer file.Close()
		reader = file
		log("main", "debug", fmt.Sprintf("Reading from file: %s", filename))
	} else {
		reader = os.Stdin
		log("main", "debug", "Reading from stdin")
	}

	// Scan line by line
	scanner := bufio.NewScanner(reader)
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		log("main", "debug", fmt.Sprintf("Scanning line: %q", line))
		ok, err := matchLine([]byte(line), pattern)
		if err != nil {
			log("main", "error", fmt.Sprintf("Match error: %v", err))
			os.Exit(2)
		}
		if ok {
			fmt.Println(line)
			found = true
		}
	}
	if err := scanner.Err(); err != nil {
		log("main", "error", fmt.Sprintf("Error reading input: %v", err))
		os.Exit(2)
	}

	// Exit 0 if any lines matched, else 1
	if found {
		os.Exit(0)
	}
	os.Exit(1)
}

// parseArgs validates and returns the -E pattern.
func parseArgs(args []string) (string, error) {
	log("parseArgs", "debug", "Parsing arguments...")
	if len(args) < 3 || args[1] != "-E" {
		return "", fmt.Errorf("usage: mygrep -E <pattern> [file]")
	}
	log("parseArgs", "debug", fmt.Sprintf("Pattern received: %q", args[2]))
	return args[2], nil
}

// unescapePattern turns literal "\\d", "\\w", "\\1", etc. into "\d", "\w", "\1".
func unescapePattern(p string) string {
	return strings.ReplaceAll(p, `\\`, `\`)
}

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

	tryAt := func(start int) bool {
		results := matchNode(root, runes, start, emptyCaps)
		for _, r := range results {
			if !anchoredEnd || r.pos == len(runes) {
				log("matchLine", "debug", fmt.Sprintf("Matched at [%d:%d]", start, r.pos))
				return true
			}
		}
		return false
	}

	if anchoredStart {
		return tryAt(0), nil
	}
	for i := 0; i <= len(runes); i++ {
		if tryAt(i) {
			return true, nil
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
type captureNode struct {
	index int
	child node
}
type backRefNode struct{ index int }

//─── Parser ───────────────────────────────────────────────────────────────────

type parser struct {
	pattern    []rune
	pos        int
	groupCount int
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
		p.groupCount++
		idx := p.groupCount
		sub, err := p.parseAlternation()
		if err != nil {
			return nil, err
		}
		if p.pos >= len(p.pattern) || p.pattern[p.pos] != ')' {
			return nil, fmt.Errorf("unterminated group")
		}
		p.pos++
		return &captureNode{index: idx, child: sub}, nil

	case '.':
		p.pos++
		return &anyNode{}, nil

	case '\\':
		p.pos++
		if p.pos >= len(p.pattern) {
			return nil, fmt.Errorf("dangling escape")
		}
		esc := p.pattern[p.pos]
		// backrefs may be multiple digits
		if esc >= '1' && esc <= '9' {
			num := 0
			for p.pos < len(p.pattern) && p.pattern[p.pos] >= '0' && p.pattern[p.pos] <= '9' {
				num = num*10 + int(p.pattern[p.pos]-'0')
				p.pos++
			}
			return &backRefNode{index: num}, nil
		}
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

type matchRes struct {
	pos  int
	caps map[int][]rune
}

func matchNode(n node, runes []rune, pos int, caps map[int][]rune) []matchRes {
	switch x := n.(type) {
	case *literalNode:
		if pos < len(runes) && runes[pos] == x.char {
			return []matchRes{{pos + 1, caps}}
		}
		return nil

	case *digitNode:
		if pos < len(runes) && runes[pos] >= '0' && runes[pos] <= '9' {
			return []matchRes{{pos + 1, caps}}
		}
		return nil

	case *wordNode:
		if pos < len(runes) {
			c := runes[pos]
			if (c >= 'a' && c <= 'z') ||
				(c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') ||
				c == '_' {
				return []matchRes{{pos + 1, caps}}
			}
		}
		return nil

	case *anyNode:
		if pos < len(runes) {
			return []matchRes{{pos + 1, caps}}
		}
		return nil

	case *charClassNode:
		if pos < len(runes) {
			_, in := x.set[runes[pos]]
			if (x.negated && !in) || (!x.negated && in) {
				return []matchRes{{pos + 1, caps}}
			}
		}
		return nil

	case *sequenceNode:
		results := []matchRes{{pos, caps}}
		for _, child := range x.children {
			var next []matchRes
			for _, r := range results {
				next = append(next, matchNode(child, runes, r.pos, r.caps)...)
			}
			results = uniqueRes(next)
			if len(results) == 0 {
				break
			}
		}
		return results

	case *altNode:
		var all []matchRes
		for _, alt := range x.alternatives {
			all = append(all, matchNode(alt, runes, pos, caps)...)
		}
		return uniqueRes(all)

	case *repNode:
		return matchRep(x, runes, pos, caps, 0)

	case *captureNode:
		sub := matchNode(x.child, runes, pos, caps)
		var out []matchRes
		for _, r := range sub {
			// copy the caps map
			newCaps := make(map[int][]rune, len(r.caps))
			for k, v := range r.caps {
				newCaps[k] = v
			}
			// record this group’s substring
			newCaps[x.index] = append([]rune{}, runes[pos:r.pos]...)
			out = append(out, matchRes{r.pos, newCaps})
		}
		return uniqueRes(out)

	case *backRefNode:
		group, ok := caps[x.index]
		if !ok {
			return nil
		}
		if pos+len(group) > len(runes) {
			return nil
		}
		for i, cr := range group {
			if runes[pos+i] != cr {
				return nil
			}
		}
		return []matchRes{{pos + len(group), caps}}

	default:
		return nil
	}
}

func matchRep(r *repNode, runes []rune, pos int, caps map[int][]rune, count int) []matchRes {
	var results []matchRes
	// can stop if we've met the minimum
	if count >= r.min {
		results = append(results, matchRes{pos, caps})
	}
	// stop if max reached
	if r.max >= 0 && count == r.max {
		return uniqueRes(results)
	}
	// try one more
	next := matchNode(r.child, runes, pos, caps)
	for _, nr := range next {
		if nr.pos == pos {
			continue // avoid infinite loops
		}
		results = append(results, matchRep(r, runes, nr.pos, nr.caps, count+1)...)
	}
	return uniqueRes(results)
}

// uniqueRes deduplicates by (pos,caps) signature.
func uniqueRes(xs []matchRes) []matchRes {
	seen := make(map[string]bool)
	var out []matchRes
	for _, x := range xs {
		// build a signature: pos + sorted caps
		keys := make([]int, 0, len(x.caps))
		for k := range x.caps {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		sig := fmt.Sprintf("%d:", x.pos)
		for _, k := range keys {
			sig += fmt.Sprintf("%d=%s|", k, string(x.caps[k]))
		}
		if !seen[sig] {
			seen[sig] = true
			out = append(out, x)
		}
	}
	return out
}

// log prints to stderr according to the current logMode.
func log(funcName, level, message string) {
	if logLevels[level] >= logLevels[logMode] {
		fmt.Fprintf(os.Stderr, "[%s] [%s] %s\n",
			funcName, strings.ToUpper(level), message)
	}
}
