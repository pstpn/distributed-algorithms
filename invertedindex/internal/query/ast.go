package query

import "fmt"

type nodeType int

const (
	nodeTerm nodeType = iota
	nodeAnd
	nodeOr
	nodeNot
	nodeAdj
	nodeNear
)

type Node struct {
	Type     nodeType
	Term     string
	Distance int
	Children []*Node
}

func newTermNode(term string) *Node {
	return &Node{
		Type: nodeTerm,
		Term: term,
	}
}

func newAndNode(left, right *Node) *Node {
	return &Node{
		Type:     nodeAnd,
		Children: []*Node{left, right},
	}
}

func newOrNode(left, right *Node) *Node {
	return &Node{
		Type:     nodeOr,
		Children: []*Node{left, right},
	}
}

func newNotNode(left, right *Node) *Node {
	return &Node{
		Type:     nodeNot,
		Children: []*Node{left, right},
	}
}

func newAdjNode(left, right *Node) *Node {
	return &Node{
		Type:     nodeAdj,
		Children: []*Node{left, right},
	}
}

func newNearNode(left, right *Node, distance int) *Node {
	return &Node{
		Type:     nodeNear,
		Distance: distance,
		Children: []*Node{left, right},
	}
}

func (n *Node) String() string {
	switch n.Type {
	case nodeTerm:
		return n.Term
	case nodeAnd:
		return "(" + n.Children[0].String() + " AND " + n.Children[1].String() + ")"
	case nodeOr:
		return "(" + n.Children[0].String() + " OR " + n.Children[1].String() + ")"
	case nodeNot:
		return "(" + n.Children[0].String() + " NOT " + n.Children[1].String() + ")"
	case nodeAdj:
		return "(" + n.Children[0].String() + " ADJ " + n.Children[1].String() + ")"
	case nodeNear:
		return "(" + n.Children[0].String() + " NEAR/" + fmt.Sprintf("%d", n.Distance) + " " + n.Children[1].String() + ")"
	default:
		return "<unknown>"
	}
}

func (n *Node) CollectTerms() []string {
	switch n.Type {
	case nodeTerm:
		return []string{n.Term}
	default:
		var terms []string
		for _, child := range n.Children {
			terms = append(terms, child.CollectTerms()...)
		}
		return terms
	}
}
