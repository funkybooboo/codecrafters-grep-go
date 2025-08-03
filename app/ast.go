package main

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
