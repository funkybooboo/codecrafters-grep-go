package main

import (
	"fmt"
)

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
