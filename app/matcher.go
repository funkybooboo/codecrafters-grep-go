package main

import (
	"fmt"
	"sort"
)

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
			newCaps := make(map[int][]rune, len(r.caps))
			for k, v := range r.caps {
				newCaps[k] = v
			}
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
	if count >= r.min {
		results = append(results, matchRes{pos, caps})
	}
	if r.max >= 0 && count == r.max {
		return uniqueRes(results)
	}
	next := matchNode(r.child, runes, pos, caps)
	for _, nr := range next {
		if nr.pos == pos {
			continue
		}
		results = append(results, matchRep(r, runes, nr.pos, nr.caps, count+1)...)
	}
	return uniqueRes(results)
}

func uniqueRes(xs []matchRes) []matchRes {
	seen := make(map[string]bool)
	var out []matchRes
	for _, x := range xs {
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
