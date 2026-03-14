package minhash

import (
	"fmt"
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
	workload := MakeWorkload(64, 2026)
	if err := idx.Build(workload.Base); err != nil {
		t.Fatalf("Build: %v", err)
	}
	for _, doc := range workload.Incoming[:8] {
		if err := idx.Add(doc); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	for _, query := range workload.Queries[:8] {
		matches := idx.FullScanDuplicates(query.Text, 0.5)
		if len(matches) == 0 {
			t.Fatalf("expected matches for query %q", query.ID)
		}
	}
}

func TestStatsReflectDocumentsAndBuckets(t *testing.T) {
	idx, err := NewIndex(DefaultConfig())
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	workload := MakeWorkload(32, 42)
	if err := idx.Build(workload.Base); err != nil {
		t.Fatalf("Build: %v", err)
	}
	stats := idx.Stats()
	if stats.DocumentCount != 32 {
		t.Fatalf("expected 32 docs, got %d", stats.DocumentCount)
	}
	if stats.BucketCount == 0 {
		t.Fatal("expected non-zero bucket count")
	}
	if stats.BandCount != DefaultBands {
		t.Fatalf("expected %d bands, got %d", DefaultBands, stats.BandCount)
	}
}

func FuzzExactDuplicatesAreFound(f *testing.F) {
	for _, sample := range []string{
		"distributed hashing duplicate search",
		"LSH supports approximate nearest neighbors in text corpora",
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

func BenchmarkSmokeConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := NewIndex(DefaultConfig())
		if err != nil {
			b.Fatalf("NewIndex: %v", err)
		}
	}
}

func ExampleIndex_FullScanDuplicates() {
	idx, _ := NewIndex(DefaultConfig())
	_ = idx.Build([]Document{{ID: "doc-1", Text: "minhash for duplicate text search"}})
	matches := idx.FullScanDuplicates("minhash for duplicate text search", 0.9)
	fmt.Println(matches[0].ID)
	// Output: doc-1
}
