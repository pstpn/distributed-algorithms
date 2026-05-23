package query

import (
	"testing"
)

func TestParseTerm(t *testing.T) {
	node, err := ParseQuery("hello")
	if err != nil {
		t.Fatalf("parse term: %v", err)
	}
	if node.Type != nodeTerm {
		t.Errorf("node type: got %v, want NodeTerm", node.Type)
	}
	if node.Term != "hello" {
		t.Errorf("term: got %q, want %q", node.Term, "hello")
	}
}

func TestParseAnd(t *testing.T) {
	node, err := ParseQuery("hello AND world")
	if err != nil {
		t.Fatalf("parse AND: %v", err)
	}
	if node.Type != nodeAnd {
		t.Errorf("node type: got %v, want NodeAnd", node.Type)
	}
	if len(node.Children) != 2 {
		t.Fatalf("children count: got %d, want 2", len(node.Children))
	}
	if node.Children[0].Term != "hello" {
		t.Errorf("left child: got %q, want %q", node.Children[0].Term, "hello")
	}
	if node.Children[1].Term != "world" {
		t.Errorf("right child: got %q, want %q", node.Children[1].Term, "world")
	}
}

func TestParseOr(t *testing.T) {
	node, err := ParseQuery("cat OR dog")
	if err != nil {
		t.Fatalf("parse OR: %v", err)
	}
	if node.Type != nodeOr {
		t.Errorf("node type: got %v, want NodeOr", node.Type)
	}
}

func TestParseNot(t *testing.T) {
	node, err := ParseQuery("cat NOT dog")
	if err != nil {
		t.Fatalf("parse NOT: %v", err)
	}
	if node.Type != nodeNot {
		t.Errorf("node type: got %v, want NodeNot", node.Type)
	}
}

func TestParseAdj(t *testing.T) {
	node, err := ParseQuery("hello ADJ world")
	if err != nil {
		t.Fatalf("parse ADJ: %v", err)
	}
	if node.Type != nodeAdj {
		t.Errorf("node type: got %v, want NodeAdj", node.Type)
	}
}

func TestParseNear(t *testing.T) {
	node, err := ParseQuery("cat NEAR/5 dog")
	if err != nil {
		t.Fatalf("parse NEAR: %v", err)
	}
	if node.Type != nodeNear {
		t.Errorf("node type: got %v, want NodeNear", node.Type)
	}
	if node.Distance != 5 {
		t.Errorf("distance: got %d, want 5", node.Distance)
	}
}

func TestParseComplexQuery(t *testing.T) {
	node, err := ParseQuery("(cat OR dog) AND pet")
	if err != nil {
		t.Fatalf("parse complex: %v", err)
	}
	if node.Type != nodeAnd {
		t.Errorf("root type: got %v, want NodeAnd", node.Type)
	}
	if node.Children[0].Type != nodeOr {
		t.Errorf("left child type: got %v, want NodeOr", node.Children[0].Type)
	}
	if node.Children[1].Type != nodeTerm {
		t.Errorf("right child type: got %v, want NodeTerm", node.Children[1].Type)
	}
}

func TestCollectTerms(t *testing.T) {
	node, err := ParseQuery("hello AND world")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	terms := node.CollectTerms()
	if len(terms) != 2 {
		t.Fatalf("terms count: got %d, want 2", len(terms))
	}
	if terms[0] != "hello" || terms[1] != "world" {
		t.Errorf("terms: got %v, want [hello world]", terms)
	}
}

func TestParseCaseInsensitive(t *testing.T) {
	node, err := ParseQuery("Hello AND World")
	if err != nil {
		t.Fatalf("parse case insensitive: %v", err)
	}
	if node.Children[0].Term != "hello" {
		t.Errorf("left term: got %q, want %q", node.Children[0].Term, "hello")
	}
	if node.Children[1].Term != "world" {
		t.Errorf("right term: got %q, want %q", node.Children[1].Term, "world")
	}
}
