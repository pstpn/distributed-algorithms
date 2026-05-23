package engine

import (
	"os"
	"path/filepath"
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

	result, err := eng.Search("hello NOT foo")
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

func TestSearchComplexQuery(t *testing.T) {
	eng := buildTestEngine(t, []string{
		"cat dog pet",
		"cat mouse",
		"dog pet park",
		"bird fish",
	})
	defer eng.Close()

	result, err := eng.Search("(cat OR dog) AND pet")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(result.Docs) != 2 {
		t.Fatalf("result count: got %d, want 2", len(result.Docs))
	}
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
