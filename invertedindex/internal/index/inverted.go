package index

import (
	"fmt"
	"strings"

	"github.com/pstpn/iidx/internal/compression"
	"github.com/pstpn/iidx/internal/storage"
)

type InvertedIndex struct {
	mmapStorage *storage.MMapStorage
	header      *storage.Header
	termEntries []storage.TermEntry
	docLengths  []uint32
}

func LoadInvertedIndex(filename string) (*InvertedIndex, error) {
	mmapStorage, err := storage.OpenMMap(filename)
	if err != nil {
		return nil, fmt.Errorf("open mmap storage: %w", err)
	}

	header, err := storage.ReadHeader(mmapStorage)
	if err != nil {
		mmapStorage.Close()
		return nil, fmt.Errorf("read header: %w", err)
	}

	termEntries, err := storage.ReadTermEntries(mmapStorage, header.IndexOffset, header.NumTerms)
	if err != nil {
		mmapStorage.Close()
		return nil, fmt.Errorf("read term entries: %w", err)
	}

	docLengths, err := storage.ReadDocLengths(mmapStorage, header)
	if err != nil {
		mmapStorage.Close()
		return nil, fmt.Errorf("read doc lengths: %w", err)
	}

	return &InvertedIndex{
		mmapStorage: mmapStorage,
		header:      header,
		termEntries: termEntries,
		docLengths:  docLengths,
	}, nil
}

func (idx *InvertedIndex) GetPostings(term string) *PostingList {
	entry := idx.findTermEntry(term)
	if entry == nil {
		return nil
	}

	var skipListData []byte
	if entry.SkipListLength > 0 {
		skipListData, _ = storage.ReadPostingsData(idx.mmapStorage, entry.PostingsOffset, entry.SkipListLength)
	}

	postingsOffset := entry.PostingsOffset + int64(entry.SkipListLength)
	data, err := storage.ReadPostingsData(idx.mmapStorage, postingsOffset, entry.PostingsLength)
	if err != nil {
		return nil
	}

	entries := compression.DeltaDecodePostings(data)
	postings := make([]Posting, 0, len(entries))
	for _, e := range entries {
		postings = append(postings, Posting{
			DocID:     e.DocID,
			Positions: e.Positions,
		})
	}

	return NewPostingListWithSkipList(postings, entry.DocFreq, skipListData)
}

func (idx *InvertedIndex) GetDocLength(docID uint32) uint32 {
	if docID == 0 || int(docID) > len(idx.docLengths) {
		return 0
	}
	return idx.docLengths[docID-1]
}

func (idx *InvertedIndex) GetDocFreq(term string) uint32 {
	entry := idx.findTermEntry(term)
	if entry == nil {
		return 0
	}
	return entry.DocFreq
}

func (idx *InvertedIndex) GetTermFreq(term string, docID uint32) uint32 {
	pl := idx.GetPostings(term)
	if pl == nil {
		return 0
	}
	positions := pl.FindPositions(docID)
	return uint32(len(positions))
}

func (idx *InvertedIndex) NumDocs() uint32 {
	return idx.header.NumDocs
}

func (idx *InvertedIndex) AvgDocLength() float64 {
	if idx.header.NumDocs == 0 {
		return 0
	}
	return float64(idx.header.TotalTokens) / float64(idx.header.NumDocs)
}

func (idx *InvertedIndex) Close() error {
	if idx.mmapStorage != nil {
		return idx.mmapStorage.Close()
	}
	return nil
}

func (idx *InvertedIndex) findTermEntry(term string) *storage.TermEntry {
	lo, hi := 0, len(idx.termEntries)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		entry := &idx.termEntries[mid]
		cmp := strings.Compare(term, entry.Term)
		if cmp == 0 {
			return entry
		}
		if cmp < 0 {
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}
	return nil
}
