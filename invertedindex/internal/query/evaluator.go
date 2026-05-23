package query

import (
	"github.com/pstpn/iidx/internal/index"
)

type Evaluator struct {
	idx *index.InvertedIndex
}

func NewEvaluator(idx *index.InvertedIndex) *Evaluator {
	return &Evaluator{idx: idx}
}

func (e *Evaluator) Evaluate(node *Node) *index.PostingList {
	if node == nil {
		return index.NewPostingList(nil, 0)
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
		return index.NewPostingList(nil, 0)
	}
}

func (e *Evaluator) evaluateTerm(node *Node) *index.PostingList {
	pl := e.idx.GetPostings(node.Term)
	if pl == nil {
		return index.NewPostingList(nil, 0)
	}
	return pl
}

func (e *Evaluator) evaluateAnd(node *Node) *index.PostingList {
	left := e.Evaluate(node.Children[0])
	right := e.Evaluate(node.Children[1])
	return index.Intersect(left, right)
}

func (e *Evaluator) evaluateOr(node *Node) *index.PostingList {
	left := e.Evaluate(node.Children[0])
	right := e.Evaluate(node.Children[1])
	return index.Union(left, right)
}

func (e *Evaluator) evaluateNot(node *Node) *index.PostingList {
	left := e.Evaluate(node.Children[0])
	right := e.Evaluate(node.Children[1])
	return index.Difference(left, right)
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
