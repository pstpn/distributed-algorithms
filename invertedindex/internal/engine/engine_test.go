package engine

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pstpn/iidx/internal/index"
)

func buildTestEngine(t *testing.T, docs []string) *Engine {
	t.Helper()
	tmpDir := t.TempDir()
	indexFile := filepath.Join(tmpDir, "test.idx")

	b := index.NewIndexBuilder()
	for _, doc := range docs {
		b.AddDocument("", doc)
	}
	if err := b.Save(indexFile); err != nil {
		t.Fatalf("save index: %v", err)
	}

	eng, err := LoadEngine(indexFile)
	if err != nil {
		t.Fatalf("load engine: %v", err)
	}
	return eng
}

func TestSearchAnd(t *testing.T) {
	eng := buildTestEngine(t, []string{
		"hello world",
		"world foo bar",
		"hello foo baz",
	})
	defer eng.Close()

	result, err := eng.Search("hello AND world")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(result.Docs) != 1 {
		t.Fatalf("result count: got %d, want 1", len(result.Docs))
	}
	if result.Docs[0].DocID != 1 {
		t.Errorf("docID: got %d, want 1", result.Docs[0].DocID)
	}
}

func TestSearchOr(t *testing.T) {
	eng := buildTestEngine(t, []string{
		"hello world",
		"foo bar",
		"hello foo",
	})
	defer eng.Close()

	result, err := eng.Search("hello OR foo")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(result.Docs) != 3 {
		t.Fatalf("result count: got %d, want 3", len(result.Docs))
	}
}

func TestSearchNot(t *testing.T) {
	eng := buildTestEngine(t, []string{
		"hello world",
		"hello foo",
		"world foo",
	})
	defer eng.Close()

	result, err := eng.Search("hello AND NOT foo")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(result.Docs) != 1 {
		t.Fatalf("result count: got %d, want 1", len(result.Docs))
	}
	if result.Docs[0].DocID != 1 {
		t.Errorf("docID: got %d, want 1", result.Docs[0].DocID)
	}
}

func TestSearchNotSubquery(t *testing.T) {
	eng := buildTestEngine(t, []string{
		"hello world",
		"hello foo",
		"world foo",
		"hello bar",
	})
	defer eng.Close()

	result, err := eng.Search("hello AND NOT (foo OR bar)")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(result.Docs) != 1 {
		t.Fatalf("result count: got %d, want 1", len(result.Docs))
	}
	if result.Docs[0].DocID != 1 {
		t.Errorf("docID: got %d, want 1", result.Docs[0].DocID)
	}
}

func TestSearchAdj(t *testing.T) {
	eng := buildTestEngine(t, []string{
		"hello world",
		"hello beautiful world",
		"world hello",
	})
	defer eng.Close()

	result, err := eng.Search("hello ADJ world")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(result.Docs) != 1 {
		t.Fatalf("result count: got %d, want 1", len(result.Docs))
	}
	if result.Docs[0].DocID != 1 {
		t.Errorf("docID: got %d, want 1", result.Docs[0].DocID)
	}
}

func TestSearchNear(t *testing.T) {
	eng := buildTestEngine(t, []string{
		"hello world",
		"hello beautiful world",
		"hello very very far world",
		"world hello",
	})
	defer eng.Close()

	result, err := eng.Search("hello NEAR/2 world")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(result.Docs) < 1 {
		t.Fatalf("result count: got %d, want >= 1", len(result.Docs))
	}
}

func TestSearchSingleTerm(t *testing.T) {
	eng := buildTestEngine(t, []string{
		"hello world",
		"foo bar",
	})
	defer eng.Close()

	result, err := eng.Search("hello")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(result.Docs) != 1 {
		t.Fatalf("result count: got %d, want 1", len(result.Docs))
	}
}

func TestSearchComplex(t *testing.T) {
	tests := []struct {
		name    string
		docs    []string
		query   string
		wantIDs []uint32
	}{
		{
			name:    "OR_AND",
			docs:    []string{"cat dog pet", "cat mouse", "dog pet park", "bird fish"},
			query:   "(cat OR dog) AND pet",
			wantIDs: []uint32{1, 3},
		},
		{
			name:    "AND_NOT_simple",
			docs:    []string{"hello world", "hello foo", "world foo"},
			query:   "hello AND NOT foo",
			wantIDs: []uint32{1},
		},
		{
			name:    "AND_NOT_empty",
			docs:    []string{"hello world", "hello foo wow", "world foo"},
			query:   "hello AND NOT (foo AND wow)",
			wantIDs: []uint32{1},
		},
		{
			name:    "AND_NOT_subquery",
			docs:    []string{"hello world", "hello foo", "world foo", "hello bar"},
			query:   "hello AND NOT (foo OR bar)",
			wantIDs: []uint32{1},
		},
		{
			name:    "nested_OR_AND",
			docs:    []string{"a b c", "a d", "e b", "e d"},
			query:   "(a OR e) AND (b OR d)",
			wantIDs: []uint32{1, 2, 3, 4},
		},
		{
			name:    "OR_AND_NOT",
			docs:    []string{"a b c", "a d", "b d", "c d"},
			query:   "(a OR b) AND NOT d",
			wantIDs: []uint32{1},
		},
		{
			name:    "ADJ_AND",
			docs:    []string{"hello world foo", "world hello", "hello beautiful world foo"},
			query:   "hello ADJ world AND foo",
			wantIDs: []uint32{1},
		},
		{
			name:    "NEAR_AND_NOT",
			docs:    []string{"hello world foo", "hello far world", "hello very far world foo"},
			query:   "hello NEAR/2 world AND NOT foo",
			wantIDs: []uint32{2},
		},
		{
			name:    "deeply_nested",
			docs:    []string{"a b c d", "a b e", "f c d", "a e d"},
			query:   "(a AND b) AND (c OR d) AND NOT e",
			wantIDs: []uint32{1},
		},
		{
			name:    "OR_chain",
			docs:    []string{"a", "b", "c", "d"},
			query:   "a OR b OR c",
			wantIDs: []uint32{1, 2, 3},
		},
		{
			name:    "AND_chain",
			docs:    []string{"a b c", "a b", "a c", "b c"},
			query:   "a AND b AND c",
			wantIDs: []uint32{1},
		},
		{
			name:    "NOT_with_OR_and_AND",
			docs:    []string{"red big fast car", "red small car", "blue big car", "red big truck"},
			query:   "(red OR blue) AND car AND NOT big",
			wantIDs: []uint32{2},
		},
		{
			name:    "ADJ_OR_AND",
			docs:    []string{"quick brown fox", "brown quick rabbit", "slow brown fox"},
			query:   "(quick ADJ rabbit OR brown ADJ fox) AND NOT rabbit",
			wantIDs: []uint32{1, 3},
		},
		{
			name:    "ADJ_OR_AND_without_brackets",
			docs:    []string{"quick brown fox", "brown quick rabbit", "slow brown fox"},
			query:   "quick ADJ rabbit OR brown ADJ fox AND NOT rabbit",
			wantIDs: []uint32{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eng := buildTestEngine(t, tt.docs)
			defer eng.Close()

			result, err := eng.Search(tt.query)
			if err != nil {
				t.Fatalf("search: %v", err)
			}

			gotIDs := scoredDocIDs(result.Docs)
			slices.Sort(gotIDs)
			wantSorted := slices.Clone(tt.wantIDs)
			slices.Sort(wantSorted)

			if !slices.Equal(gotIDs, wantSorted) {
				t.Fatalf("docIDs: got %v, want %v", gotIDs, wantSorted)
			}
		})
	}
}

func scoredDocIDs(docs []ScoredDocument) []uint32 {
	ids := make([]uint32, len(docs))
	for i, d := range docs {
		ids[i] = d.DocID
	}
	return ids
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	indexFile := filepath.Join(tmpDir, "test.idx")

	b := index.NewIndexBuilder()
	b.AddDocument("", "hello world")
	b.AddDocument("", "world foo bar")
	if err := b.Save(indexFile); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadEngine(indexFile)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	defer loaded.Close()

	result, err := loaded.Search("hello")
	if err != nil {
		t.Fatalf("search after load: %v", err)
	}

	if len(result.Docs) != 1 {
		t.Fatalf("result count after load: got %d, want 1", len(result.Docs))
	}
}

func TestBM25Ranking(t *testing.T) {
	eng := buildTestEngine(t, []string{
		"hello hello hello world",
		"hello world",
		"world world world",
	})
	defer eng.Close()

	result, err := eng.Search("hello")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(result.Docs) != 2 {
		t.Fatalf("result count: got %d, want 2", len(result.Docs))
	}

	if result.Docs[0].DocID != 1 {
		t.Errorf("top result: got docID %d (score %.4f), want docID 1",
			result.Docs[0].DocID, result.Docs[0].Score)
	}
}

func TestBuildIndexFromFiles(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "doc1.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "doc2.txt"), []byte("foo bar"), 0644)

	b := index.NewIndexBuilder()

	files := []string{
		filepath.Join(tmpDir, "doc1.txt"),
		filepath.Join(tmpDir, "doc2.txt"),
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		b.AddDocument("", string(data))
	}

	indexFile := filepath.Join(t.TempDir(), "test.idx")
	if err := b.Save(indexFile); err != nil {
		t.Fatalf("save: %v", err)
	}

	eng, err := LoadEngine(indexFile)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	defer eng.Close()

	if eng.NumDocs() != 2 {
		t.Errorf("num docs: got %d, want 2", eng.NumDocs())
	}

	result, err := eng.Search("hello")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(result.Docs) != 1 {
		t.Errorf("result count: got %d, want 1", len(result.Docs))
	}
}

func TestGetDocument(t *testing.T) {
	eng := buildTestEngine(t, []string{
		"hello world",
		"foo bar",
	})
	defer eng.Close()

	if eng.GetDocument(1) != "hello world" {
		t.Errorf("doc 1: got %q, want %q", eng.GetDocument(1), "hello world")
	}
	if eng.GetDocument(2) != "foo bar" {
		t.Errorf("doc 2: got %q, want %q", eng.GetDocument(2), "foo bar")
	}
}

func TestGetDocumentTitle(t *testing.T) {
	tmpDir := t.TempDir()
	indexFile := filepath.Join(tmpDir, "test.idx")

	b := index.NewIndexBuilder()
	b.AddDocument("First Article", "hello world")
	b.AddDocument("Second Article", "foo bar baz")
	b.AddDocument("", "no title doc")
	if err := b.Save(indexFile); err != nil {
		t.Fatalf("save index: %v", err)
	}

	eng, err := LoadEngine(indexFile)
	if err != nil {
		t.Fatalf("load engine: %v", err)
	}
	defer eng.Close()

	if title := eng.GetDocumentTitle(1); title != "First Article" {
		t.Errorf("title doc 1: got %q, want %q", title, "First Article")
	}
	if title := eng.GetDocumentTitle(2); title != "Second Article" {
		t.Errorf("title doc 2: got %q, want %q", title, "Second Article")
	}
	if title := eng.GetDocumentTitle(3); title != "" {
		t.Errorf("title doc 3: got %q, want empty string", title)
	}

	if text := eng.GetDocument(1); text != "hello world" {
		t.Errorf("text doc 1: got %q, want %q", text, "hello world")
	}
	if text := eng.GetDocument(2); text != "foo bar baz" {
		t.Errorf("text doc 2: got %q, want %q", text, "foo bar baz")
	}
}
