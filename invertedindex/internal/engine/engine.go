package engine

import (
	"fmt"

	"github.com/pstpn/iidx/internal/index"
	"github.com/pstpn/iidx/internal/query"
	"github.com/pstpn/iidx/internal/ranking"
	"github.com/pstpn/iidx/internal/storage"
)

type SearchResult struct {
	Docs []ScoredDocument
}

type ScoredDocument struct {
	DocID uint32
	Score float64
}

type Engine struct {
	idx      *index.InvertedIndex
	docStore *storage.DocStore
	ranker   *ranking.Ranker
}

func LoadEngine(filename string) (*Engine, error) {
	idx, err := index.LoadInvertedIndex(filename)
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}

	docStoreFile := index.DocStoreFilename(filename)
	docStore, err := storage.OpenDocStore(docStoreFile)
	if err != nil {
		idx.Close()
		return nil, fmt.Errorf("load doc store: %w", err)
	}

	return &Engine{
		idx:      idx,
		docStore: docStore,
		ranker:   ranking.NewRanker(idx, ranking.DefaultBM25Params()),
	}, nil
}

func (e *Engine) Evaluate(queryStr string) (*index.PostingList, map[string]*index.PostingList, error) {
	node, err := query.ParseQuery(queryStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parse query: %w", err)
	}

	evaluator := query.NewEvaluator(e.idx)
	pl := evaluator.Evaluate(node)
	return pl, evaluator.TermPostings, nil
}

func (e *Engine) Search(queryStr string) (*SearchResult, error) {
	pl, termPostings, err := e.Evaluate(queryStr)
	if err != nil {
		return nil, err
	}

	scoredDocs := e.ranker.RankResults(pl, termPostings)

	result := &SearchResult{Docs: make([]ScoredDocument, 0, len(scoredDocs))}
	for _, sd := range scoredDocs {
		result.Docs = append(result.Docs, ScoredDocument{
			DocID: sd.DocID,
			Score: sd.Score,
		})
	}

	return result, nil
}

func (e *Engine) GetDocument(docID uint32) string {
	text, err := e.docStore.GetDocumentText(docID)
	if err != nil {
		return ""
	}
	return text
}

func (e *Engine) GetDocumentTitle(docID uint32) string {
	title, err := e.docStore.GetDocumentTitle(docID)
	if err != nil {
		return ""
	}
	return title
}

func (e *Engine) Close() error {
	var err error
	if e.idx != nil {
		err = e.idx.Close()
	}
	if e.docStore != nil {
		if closeErr := e.docStore.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

func (e *Engine) NumDocs() uint32 {
	return e.idx.NumDocs()
}
