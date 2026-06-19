package ranking

import (
	"math"
	"sort"

	"github.com/pstpn/iidx/internal/index"
)

type BM25Params struct {
	K1 float64
	B  float64
}

func DefaultBM25Params() BM25Params {
	return BM25Params{
		K1: 1.5,
		B:  0.75,
	}
}

type Ranker struct {
	params BM25Params
	idx    *index.InvertedIndex
}

func NewRanker(idx *index.InvertedIndex, params BM25Params) *Ranker {
	return &Ranker{
		params: params,
		idx:    idx,
	}
}

type ScoredDoc struct {
	DocID uint32
	Score float64
}

func (r *Ranker) scoreDocument(termFreq uint32, docLength uint32, docFreq uint32) float64 {
	if docFreq == 0 || termFreq == 0 {
		return 0
	}

	numDocs := r.idx.NumDocs()
	avgDocLength := r.idx.AvgDocLength()
	idf := math.Log(1 + (float64(numDocs)-float64(docFreq)+0.5)/(float64(docFreq)+0.5))

	tf := float64(termFreq)
	if avgDocLength == 0 {
		avgDocLength = 1
	}
	docLenFactor := float64(docLength) / avgDocLength
	normalizedTF := tf * (r.params.K1 + 1) / (tf + r.params.K1*(1-r.params.B+r.params.B*docLenFactor))

	return idf * normalizedTF
}

func (r *Ranker) RankResults(pl *index.PostingList, termPostings map[string]*index.PostingList) []ScoredDoc {
	if pl == nil || pl.Len() == 0 {
		return nil
	}

	iterators := make(map[string]*index.PostingIterator, len(termPostings))
	for term, tpl := range termPostings {
		iterators[term] = tpl.NewIterator()
	}

	results := make([]ScoredDoc, 0, pl.Len())
	for i := 0; i < pl.Len(); i++ {
		p := pl.Posting(i)
		docLength := r.idx.GetDocLength(p.DocID)

		var score float64
		for term, it := range iterators {
			tf := it.AdvanceTo(p.DocID)
			if tf == 0 {
				continue
			}
			docFreq := termPostings[term].DF()
			score += r.scoreDocument(tf, docLength, docFreq)
		}

		results = append(results, ScoredDoc{
			DocID: p.DocID,
			Score: score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}
