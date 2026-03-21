package minhash

import (
	"strings"
	"testing"
)

func TestNewIndexRejectsInvalidConfig(t *testing.T) {
	_, err := NewIndex(Config{NumHashes: 10, Bands: 3, ShingleSize: 2, SimilarityThreshold: 0.8})
	if err == nil {
		t.Fatal("expected config validation error")
	}
}

func TestAddAndFindDuplicates(t *testing.T) {
	idx, err := NewIndex(DefaultConfig())
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}

	docs := []Document{
		{ID: "a", Text: "Distributed hashing for duplicate document search with local sensitive hashing"},
		{ID: "b", Text: "Consensus protocols and replication improve distributed storage reliability"},
	}
	if err := idx.Build(docs); err != nil {
		t.Fatalf("Build: %v", err)
	}

	query := "distributed hashing for duplicate document search with local sensitive hashing"
	matches := idx.FindDuplicates(query, 0.8)
	if len(matches) == 0 {
		t.Fatal("expected at least one duplicate match")
	}
	if matches[0].ID != "a" {
		t.Fatalf("expected first match to be a, got %q", matches[0].ID)
	}
	if matches[0].Score < 0.99 {
		t.Fatalf("expected near-perfect similarity, got %.3f", matches[0].Score)
	}
}

func TestFullScanMatchesAddedDuplicates(t *testing.T) {
	idx, err := NewIndex(DefaultConfig())
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	baseDocs := randomDocuments(2026)
	if err := idx.Build(baseDocs); err != nil {
		t.Fatalf("Build: %v", err)
	}

	newDocs := randomDocuments(8)
	for _, doc := range newDocs {
		if err := idx.Add(doc); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	for _, query := range newDocs {
		matches := idx.FullScanDuplicates(query.Text, 0.99)
		if len(matches) == 0 {
			t.Fatalf("expected matches for query %q", query.ID)
		}
	}
}

func FuzzExactDuplicatesAreFound(f *testing.F) {
	for _, sample := range []string{
		"distributed hashing duplicate search",
		"LSH supports approximate nearest neighbors",
		strings.Repeat("token ", 8),
	} {
		f.Add(sample)
	}

	f.Fuzz(func(t *testing.T, text string) {
		idx, err := NewIndex(DefaultConfig())
		if err != nil {
			t.Fatalf("NewIndex: %v", err)
		}
		doc := Document{ID: "doc-1", Text: text}
		if err := idx.Add(doc); err != nil {
			t.Fatalf("Add: %v", err)
		}
		matches := idx.FindDuplicates(text, 0.95)
		if len(matches) == 0 || matches[0].ID != doc.ID {
			t.Fatalf("expected exact duplicate for %q, got %#v", text, matches)
		}
		scan := idx.FullScanDuplicates(text, 0.95)
		if len(scan) == 0 || scan[0].ID != doc.ID {
			t.Fatalf("expected exact full-scan duplicate for %q, got %#v", text, scan)
		}
	})
}
