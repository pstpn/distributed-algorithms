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
	node, err := ParseQuery("cat AND NOT dog")
	if err != nil {
		t.Fatalf("parse NOT: %v", err)
	}
	if node.Type != nodeAnd {
		t.Fatalf("root type: got %v, want NodeAnd", node.Type)
	}
	if len(node.Children) != 2 {
		t.Fatalf("children count: got %d, want 2", len(node.Children))
	}
	if node.Children[0].Type != nodeTerm {
		t.Errorf("left child type: got %v, want NodeTerm", node.Children[0].Type)
	}
	if node.Children[1].Type != nodeNot {
		t.Errorf("right child type: got %v, want NodeNot", node.Children[1].Type)
	}
	if len(node.Children[1].Children) != 1 {
		t.Fatalf("NOT children count: got %d, want 1", len(node.Children[1].Children))
	}
	if node.Children[1].Children[0].Term != "dog" {
		t.Errorf("NOT child term: got %q, want %q", node.Children[1].Children[0].Term, "dog")
	}
}

func TestParseNotSubquery(t *testing.T) {
	node, err := ParseQuery("history AND NOT (russia AND china)")
	if err != nil {
		t.Fatalf("parse NOT subquery: %v", err)
	}
	if node.Type != nodeAnd {
		t.Fatalf("root type: got %v, want NodeAnd", node.Type)
	}
	if node.Children[1].Type != nodeNot {
		t.Fatalf("right child type: got %v, want NodeNot", node.Children[1].Type)
	}
	notChild := node.Children[1].Children[0]
	if notChild.Type != nodeAnd {
		t.Fatalf("NOT child type: got %v, want NodeAnd", notChild.Type)
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

func TestCollectTermsNotExcluded(t *testing.T) {
	node, err := ParseQuery("cat AND NOT dog")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	terms := node.CollectTerms()
	if len(terms) != 1 {
		t.Fatalf("terms count: got %d, want 1 (NOT terms excluded)", len(terms))
	}
	if terms[0] != "cat" {
		t.Errorf("term: got %q, want %q", terms[0], "cat")
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

func TestParseTermsWithKeywordPrefix(t *testing.T) {
	tests := []string{
		"andreu",
		"fourcorner AND andreu",
		"orthogneiss OR dzongsar",
		"orchidaceae AND NOT damodara",
		"einfachste ADJ grizold",
		"johnscaddan NEAR/3 orchidaceae",
		"(orthogneiss OR dzongsar) AND typewriting",
	}
	for _, q := range tests {
		_, err := ParseQuery(q)
		if err != nil {
			t.Errorf("parse %q: %v", q, err)
		}
	}
}
