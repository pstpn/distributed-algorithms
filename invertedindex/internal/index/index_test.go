package index

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func buildTestIndex(t *testing.T, docs []string) *InvertedIndex {
	t.Helper()
	tmpDir := t.TempDir()
	indexFile := filepath.Join(tmpDir, "test.idx")

	b := NewIndexBuilder()
	for _, doc := range docs {
		b.AddDocument("", doc)
	}
	if err := b.Save(indexFile); err != nil {
		t.Fatalf("save index: %v", err)
	}

	idx, err := LoadInvertedIndex(indexFile)
	if err != nil {
		t.Fatalf("load index: %v", err)
	}
	return idx
}

func TestAddDocument(t *testing.T) {
	idx := buildTestIndex(t, []string{
		"hello world hello",
		"world foo bar",
		"hello foo",
	})
	defer idx.Close()

	if idx.NumDocs() != 3 {
		t.Errorf("NumDocs: got %d, want 3", idx.NumDocs())
	}

	pl := idx.GetPostings("hello")
	if pl == nil {
		t.Fatal("postings for 'hello' is nil")
	}
	if pl.Len() != 2 {
		t.Errorf("postings for 'hello': got %d, want 2", pl.Len())
	}
	if pl.DF() != 2 {
		t.Errorf("df for 'hello': got %d, want 2", pl.DF())
	}

	pl = idx.GetPostings("world")
	if pl == nil {
		t.Fatal("postings for 'world' is nil")
	}
	if pl.Len() != 2 {
		t.Errorf("postings for 'world': got %d, want 2", pl.Len())
	}

	pl = idx.GetPostings("foo")
	if pl == nil {
		t.Fatal("postings for 'foo' is nil")
	}
	if pl.Len() != 2 {
		t.Errorf("postings for 'foo': got %d, want 2", pl.Len())
	}

	pl = idx.GetPostings("nonexistent")
	if pl != nil {
		t.Errorf("postings for 'nonexistent': expected nil, got %v", pl)
	}
}

func TestIntersect(t *testing.T) {
	pl1 := NewPostingList([]Posting{
		{DocID: 1, Positions: []uint32{0}},
		{DocID: 3, Positions: []uint32{2}},
		{DocID: 5, Positions: []uint32{4}},
	}, 3)

	pl2 := NewPostingList([]Posting{
		{DocID: 2, Positions: []uint32{1}},
		{DocID: 3, Positions: []uint32{3}},
		{DocID: 5, Positions: []uint32{5}},
	}, 3)

	result := Intersect(pl1, pl2)

	if result.Len() != 2 {
		t.Fatalf("intersect: got %d, want 2", result.Len())
	}

	expected := []uint32{3, 5}
	for i, p := range result.Postings() {
		if p.DocID != expected[i] {
			t.Errorf("intersect at %d: got %d, want %d", i, p.DocID, expected[i])
		}
	}
}

func TestUnion(t *testing.T) {
	pl1 := NewPostingList([]Posting{
		{DocID: 1, Positions: []uint32{0}},
		{DocID: 3, Positions: []uint32{2}},
	}, 2)

	pl2 := NewPostingList([]Posting{
		{DocID: 2, Positions: []uint32{1}},
		{DocID: 3, Positions: []uint32{3}},
	}, 2)

	result := Union(pl1, pl2)

	if result.Len() != 3 {
		t.Fatalf("union: got %d, want 3", result.Len())
	}

	expected := []uint32{1, 2, 3}
	for i, p := range result.Postings() {
		if p.DocID != expected[i] {
			t.Errorf("union at %d: got %d, want %d", i, p.DocID, expected[i])
		}
	}
}

func TestDifference(t *testing.T) {
	pl1 := NewPostingList([]Posting{
		{DocID: 1, Positions: []uint32{0}},
		{DocID: 3, Positions: []uint32{2}},
		{DocID: 5, Positions: []uint32{4}},
	}, 3)

	pl2 := NewPostingList([]Posting{
		{DocID: 3, Positions: []uint32{3}},
		{DocID: 5, Positions: []uint32{5}},
	}, 2)

	result := Difference(pl1, pl2)

	if result.Len() != 1 {
		t.Fatalf("difference: got %d, want 1", result.Len())
	}

	if result.Postings()[0].DocID != 1 {
		t.Errorf("difference: got %d, want 1", result.Postings()[0].DocID)
	}
}

func TestAdjacent(t *testing.T) {
	pl1 := NewPostingList([]Posting{
		{DocID: 1, Positions: []uint32{0, 5}},
		{DocID: 2, Positions: []uint32{3}},
	}, 2)

	pl2 := NewPostingList([]Posting{
		{DocID: 1, Positions: []uint32{1, 6}},
		{DocID: 2, Positions: []uint32{5}},
	}, 2)

	result := Adjacent(pl1, pl2)

	if result.Len() != 1 {
		t.Fatalf("adjacent: got %d docs, want 1", result.Len())
	}

	if result.Postings()[0].DocID != 1 {
		t.Errorf("adjacent: got doc %d, want 1", result.Postings()[0].DocID)
	}
}

func TestNear(t *testing.T) {
	pl1 := NewPostingList([]Posting{
		{DocID: 1, Positions: []uint32{0, 10}},
		{DocID: 2, Positions: []uint32{3}},
	}, 2)

	pl2 := NewPostingList([]Posting{
		{DocID: 1, Positions: []uint32{3, 12}},
		{DocID: 2, Positions: []uint32{10}},
	}, 2)

	result := Near(pl1, pl2, 3)

	if result.Len() != 1 {
		t.Fatalf("near: got %d docs, want 1", result.Len())
	}

	if result.Postings()[0].DocID != 1 {
		t.Errorf("near: got doc %d, want 1", result.Postings()[0].DocID)
	}
}

func TestNearChained(t *testing.T) {
	plA := NewPostingList([]Posting{
		{DocID: 1, Positions: []uint32{5, 50}},
	}, 1)

	plB := NewPostingList([]Posting{
		{DocID: 1, Positions: []uint32{10, 55}},
	}, 1)

	plC := NewPostingList([]Posting{
		{DocID: 1, Positions: []uint32{12, 60}},
	}, 1)

	near1 := Near(plA, plB, 1000)
	if near1.Len() != 1 {
		t.Fatalf("near1: got %d docs, want 1", near1.Len())
	}

	positions := near1.Postings()[0].Positions
	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Fatalf("near1: positions not sorted: %v", positions)
		}
	}

	near2 := Near(near1, plC, 5)
	if near2.Len() != 1 {
		t.Fatalf("near2: got %d docs, want 1", near2.Len())
	}

	if near2.Postings()[0].DocID != 1 {
		t.Errorf("near2: got doc %d, want 1", near2.Postings()[0].DocID)
	}

	positions2 := near2.Postings()[0].Positions
	for i := 1; i < len(positions2); i++ {
		if positions2[i] <= positions2[i-1] {
			t.Fatalf("near2: positions not sorted: %v", positions2)
		}
	}
}

func TestNearPositionsSorted(t *testing.T) {
	pl1 := NewPostingList([]Posting{
		{DocID: 1, Positions: []uint32{5, 50}},
	}, 1)

	pl2 := NewPostingList([]Posting{
		{DocID: 1, Positions: []uint32{10, 55}},
	}, 1)

	result := Near(pl1, pl2, 1000)
	if result.Len() != 1 {
		t.Fatalf("near: got %d docs, want 1", result.Len())
	}

	positions := result.Postings()[0].Positions
	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Fatalf("positions not sorted: %v", positions)
		}
	}
}

func TestSkipList(t *testing.T) {
	postings := make([]Posting, 1000)
	for i := range postings {
		postings[i] = Posting{
			DocID:     uint32(i + 1),
			Positions: []uint32{uint32(i)},
		}
	}

	pl := NewPostingList(postings, 1000)

	if len(pl.skipLevels) == 0 {
		t.Error("skip list not built")
	}

	it := pl.iterator()
	found := it.skipTo(500)
	if !found {
		t.Error("skipTo(500) should find a posting")
	}
	if it.currentDocID() != 500 {
		t.Errorf("skipTo(500): got docID %d, want 500", it.currentDocID())
	}

	it.reset()
	found = it.skipTo(1)
	if !found {
		t.Error("skipTo(1) should find a posting")
	}
	if it.currentDocID() != 1 {
		t.Errorf("skipTo(1): got docID %d, want 1", it.currentDocID())
	}

	it.reset()
	found = it.skipTo(2000)
	if found {
		t.Error("skipTo(2000) should not find a posting")
	}

	it.reset()
	it.skipTo(100)
	if it.currentDocID() != 100 {
		t.Errorf("skipTo(100): got docID %d, want 100", it.currentDocID())
	}
	it.skipTo(700)
	if it.currentDocID() != 700 {
		t.Errorf("skipTo(700): got docID %d, want 700", it.currentDocID())
	}
	it.skipTo(300)
	if it.currentDocID() != 700 {
		t.Errorf("skipTo(300) backward: got docID %d, want 700 (can't go back)", it.currentDocID())
	}
}

func TestSkipListLevels(t *testing.T) {
	n := 100000
	postings := make([]Posting, n)
	for i := range postings {
		postings[i] = Posting{DocID: uint32(i + 1)}
	}

	pl := NewPostingList(postings, uint32(n))

	if len(pl.skipLevels) < 2 {
		t.Errorf("expected >= 2 skip levels for n=%d, got %d", n, len(pl.skipLevels))
	}

	lowest := pl.skipLevels[0]
	if len(lowest.indices) == 0 {
		t.Error("lowest skip level should have indices")
	}
	firstStep := lowest.indices[0]
	if firstStep < 200 || firstStep > 500 {
		t.Errorf("lowest level first step: got %d, want ~316", firstStep)
	}

	highest := pl.skipLevels[len(pl.skipLevels)-1]
	if len(highest.indices) >= len(lowest.indices) {
		t.Errorf("highest level (%d indices) should have fewer than lowest (%d)",
			len(highest.indices), len(lowest.indices))
	}
}

func TestSaveAndLoad(t *testing.T) {
	idx := buildTestIndex(t, []string{
		"hello world",
		"world foo bar",
		"hello foo baz",
	})
	defer idx.Close()

	if idx.NumDocs() != 3 {
		t.Errorf("loaded NumDocs: got %d, want 3", idx.NumDocs())
	}

	pl := idx.GetPostings("hello")
	if pl == nil {
		t.Fatal("loaded postings for 'hello' is nil")
	}
	if pl.Len() != 2 {
		t.Errorf("loaded postings for 'hello': got %d, want 2", pl.Len())
	}
}

func TestGetDocLength(t *testing.T) {
	idx := buildTestIndex(t, []string{"hello world foo"})
	defer idx.Close()

	if idx.GetDocLength(1) != 3 {
		t.Errorf("doc length: got %d, want 3", idx.GetDocLength(1))
	}
}

func TestGetTermFreq(t *testing.T) {
	idx := buildTestIndex(t, []string{"hello world hello"})
	defer idx.Close()

	if idx.GetTermFreq("hello", 1) != 2 {
		t.Errorf("term freq for 'hello': got %d, want 2", idx.GetTermFreq("hello", 1))
	}
	if idx.GetTermFreq("world", 1) != 1 {
		t.Errorf("term freq for 'world': got %d, want 1", idx.GetTermFreq("world", 1))
	}
}

func TestAvgDocLength(t *testing.T) {
	idx := buildTestIndex(t, []string{
		"a b c",
		"d e",
		"f g h i",
	})
	defer idx.Close()

	avg := idx.AvgDocLength()
	expected := float64(9) / 3.0
	if avg != expected {
		t.Errorf("avg doc length: got %f, want %f", avg, expected)
	}
}

func TestIndexFileCreated(t *testing.T) {
	tmpDir := t.TempDir()
	indexFile := filepath.Join(tmpDir, "test.idx")

	b := NewIndexBuilder()
	b.AddDocument("", "hello world")
	if err := b.Save(indexFile); err != nil {
		t.Fatalf("save index: %v", err)
	}

	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		t.Fatalf("index file not created")
	}

	docStoreFile := DocStoreFilename(indexFile)
	if _, err := os.Stat(docStoreFile); os.IsNotExist(err) {
		t.Fatalf("doc store file not created")
	}
}

func TestSkipListPersisted(t *testing.T) {
	tmpDir := t.TempDir()
	indexFile := filepath.Join(tmpDir, "test.idx")

	b := NewIndexBuilder()
	for i := 0; i < 100; i++ {
		b.AddDocument("", fmt.Sprintf("common doc%d", i))
	}
	if err := b.Save(indexFile); err != nil {
		t.Fatalf("save index: %v", err)
	}

	idx, err := LoadInvertedIndex(indexFile)
	if err != nil {
		t.Fatalf("load index: %v", err)
	}
	defer idx.Close()

	pl := idx.GetPostings("common")
	if pl == nil {
		t.Fatal("postings for 'common' is nil")
	}
	if pl.Len() != 100 {
		t.Fatalf("postings for 'common': got %d, want 100", pl.Len())
	}

	it := pl.iterator()
	found := it.skipTo(50)
	if !found {
		t.Error("skipTo(50) should find a posting")
	}
	if it.currentDocID() != 50 {
		t.Errorf("skipTo(50): got docID %d, want 50", it.currentDocID())
	}

	pl2 := NewPostingList(pl.Postings(), pl.DF())
	pl3 := NewPostingListWithSkipList(pl.Postings(), pl.DF(), pl2.skipLevels)

	it2 := pl2.iterator()
	it3 := pl3.iterator()

	it2.skipTo(50)
	it3.skipTo(50)
	if it2.currentDocID() != it3.currentDocID() {
		t.Errorf("skipTo(50) mismatch: built=%d, loaded=%d", it2.currentDocID(), it3.currentDocID())
	}
}
