package index

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pstpn/iidx/internal/compression"
	"github.com/pstpn/iidx/internal/storage"
)

type IndexBuilder struct {
	postings    map[string]*PostingList
	docTitles   map[uint32]string
	docTexts    map[uint32]string
	docLength   map[uint32]uint32
	nextDocID   uint32
	numDocs     uint32
	totalTokens uint64
}

func NewIndexBuilder() *IndexBuilder {
	return &IndexBuilder{
		postings:  make(map[string]*PostingList),
		docTitles: make(map[uint32]string),
		docTexts:  make(map[uint32]string),
		docLength: make(map[uint32]uint32),
		nextDocID: 1,
	}
}

func (b *IndexBuilder) AddDocument(title, text string) uint32 {
	docID := b.nextDocID
	b.nextDocID++
	b.docTitles[docID] = title
	b.docTexts[docID] = text

	tokens := tokenize(text)
	b.docLength[docID] = uint32(len(tokens))
	b.totalTokens += uint64(len(tokens))

	termPositions := make(map[string][]uint32)
	for pos, token := range tokens {
		termPositions[token] = append(termPositions[token], uint32(pos))
	}

	for term, positions := range termPositions {
		posting := Posting{
			DocID:     docID,
			Positions: positions,
		}

		if pl, exists := b.postings[term]; exists {
			pl.postings = append(pl.postings, posting)
		} else {
			b.postings[term] = &PostingList{
				postings: []Posting{posting},
			}
		}
	}

	b.numDocs++
	return docID
}

func (b *IndexBuilder) NumDocs() uint32 {
	return b.numDocs
}

func (b *IndexBuilder) Save(filename string) error {
	terms := make([]string, 0, len(b.postings))
	for term := range b.postings {
		terms = append(terms, term)
	}
	sort.Strings(terms)

	docLengths := make([]uint32, b.numDocs)
	for docID, length := range b.docLength {
		docLengths[docID-1] = length
	}

	writer, err := storage.NewIndexWriter(filename, b.numDocs, b.totalTokens, docLengths)
	if err != nil {
		return fmt.Errorf("create index writer: %w", err)
	}

	for _, term := range terms {
		pl := b.postings[term]
		pl.buildSkipList()

		docIDs := make([]uint32, pl.Len())
		tfs := make([]uint32, pl.Len())
		var posDeltas []uint32
		for i := 0; i < pl.Len(); i++ {
			p := pl.Posting(i)
			docIDs[i] = p.DocID
			tfs[i] = uint32(len(p.Positions))
			prev := uint32(0)
			for _, pos := range p.Positions {
				posDeltas = append(posDeltas, pos-prev)
				prev = pos
			}
		}

		docDeltas := make([]uint32, len(docIDs))
		prev := uint32(0)
		for i, id := range docIDs {
			docDeltas[i] = id - prev
			prev = id
		}

		skipListFlat := flattenSkipLevels(pl.skipLevels)

		var flat []uint32
		flat = append(flat, uint32(len(docDeltas)))
		flat = append(flat, docDeltas...)
		flat = append(flat, uint32(len(posDeltas)))
		flat = append(flat, posDeltas...)
		flat = append(flat, uint32(len(tfs)))
		flat = append(flat, tfs...)
		flat = append(flat, uint32(len(skipListFlat)))
		flat = append(flat, skipListFlat...)

		writer.AddTerm(term, compression.Compress(flat))
	}

	if err := writer.Write(); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	docEntries := make([]storage.DocEntry, b.numDocs)
	for docID := uint32(1); docID <= b.numDocs; docID++ {
		docEntries[docID-1] = storage.DocEntry{
			Title: b.docTitles[docID],
			Text:  b.docTexts[docID],
		}
	}

	docStoreFilename := DocStoreFilename(filename)
	dsw, err := storage.NewDocStoreWriter(docStoreFilename, b.numDocs, docEntries)
	if err != nil {
		return fmt.Errorf("create doc store writer: %w", err)
	}
	if err := dsw.Write(); err != nil {
		return fmt.Errorf("write doc store: %w", err)
	}

	return nil
}

func DocStoreFilename(indexFilename string) string {
	return indexFilename[:len(indexFilename)-4] + ".docs"
}

func tokenize(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if isAlphaNumeric(r) {
			current.WriteRune(toLower(r))
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}

func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}
