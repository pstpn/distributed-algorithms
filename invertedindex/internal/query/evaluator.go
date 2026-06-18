package query

import (
	"github.com/pstpn/iidx/internal/index"
)

type Evaluator struct {
	idx          *index.InvertedIndex
	TermPostings map[string]*index.PostingList
}

func NewEvaluator(idx *index.InvertedIndex) *Evaluator {
	return &Evaluator{
		idx:          idx,
		TermPostings: make(map[string]*index.PostingList),
	}
}

func (e *Evaluator) Evaluate(node *Node) *index.PostingList {
	if node == nil {
		return index.NewPostingList(nil)
	}

	switch node.Type {
	case nodeTerm:
		return e.evaluateTerm(node)
	case nodeAnd:
		return e.evaluateAnd(node)
	case nodeOr:
		return e.evaluateOr(node)
	case nodeNot:
		return e.evaluateNot(node)
	case nodeAdj:
		return e.evaluateAdj(node)
	case nodeNear:
		return e.evaluateNear(node)
	default:
		return index.NewPostingList(nil)
	}
}

func (e *Evaluator) evaluateTerm(node *Node) *index.PostingList {
	if pl, ok := e.TermPostings[node.Term]; ok {
		return pl
	}
	pl := e.idx.GetPostings(node.Term)
	if pl == nil {
		return index.NewPostingList(nil)
	}
	e.TermPostings[node.Term] = pl
	return pl
}

func (e *Evaluator) evaluateAnd(node *Node) *index.PostingList {
	leftNode := node.Children[0]
	rightNode := node.Children[1]

	if rightNode.Type == nodeNot {
		left := e.Evaluate(leftNode)
		right := e.Evaluate(rightNode.Children[0])
		return index.Difference(left, right)
	}

	left := e.Evaluate(leftNode)
	right := e.Evaluate(rightNode)
	return index.Intersect(left, right)
}

func (e *Evaluator) evaluateOr(node *Node) *index.PostingList {
	left := e.Evaluate(node.Children[0])
	right := e.Evaluate(node.Children[1])
	return index.Union(left, right)
}

func (e *Evaluator) evaluateNot(node *Node) *index.PostingList {
	return index.NewPostingList(nil)
}

func (e *Evaluator) evaluateAdj(node *Node) *index.PostingList {
	left := e.Evaluate(node.Children[0])
	right := e.Evaluate(node.Children[1])
	return index.Adjacent(left, right)
}

func (e *Evaluator) evaluateNear(node *Node) *index.PostingList {
	left := e.Evaluate(node.Children[0])
	right := e.Evaluate(node.Children[1])
	return index.Near(left, right, node.Distance)
}
