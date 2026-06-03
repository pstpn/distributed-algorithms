package index

import (
	"math"
	"sort"
)

type Posting struct {
	DocID     uint32
	Positions []uint32
}

type skipLevel struct {
	indices []int
}

type PostingList struct {
	postings   []Posting
	skipLevels []skipLevel
	df         uint32
}

func NewPostingList(postings []Posting, df uint32) *PostingList {
	pl := &PostingList{
		postings: postings,
		df:       df,
	}
	pl.buildSkipList()
	return pl
}

func NewPostingListWithSkipList(postings []Posting, df uint32, levels []skipLevel) *PostingList {
	pl := &PostingList{
		postings:   postings,
		df:         df,
		skipLevels: levels,
	}
	return pl
}

func (pl *PostingList) buildSkipList() {
	n := len(pl.postings)
	if n <= 1 {
		pl.skipLevels = nil
		return
	}

	var levels []skipLevel
	for step := max(2, int(math.Sqrt(float64(n)))); step < n; step *= 2 {
		var indices []int
		for i := step; i < n; i += step {
			indices = append(indices, i)
		}
		if len(indices) > 0 {
			levels = append(levels, skipLevel{indices: indices})
		}
	}

	pl.skipLevels = levels
}

func (pl *PostingList) Len() int {
	return len(pl.postings)
}

func (pl *PostingList) DF() uint32 {
	return pl.df
}

func (pl *PostingList) Posting(i int) Posting {
	return pl.postings[i]
}

func (pl *PostingList) Postings() []Posting {
	return pl.postings
}

func (pl *PostingList) iterator() *PostingIterator {
	cursors := make([]int, len(pl.skipLevels))
	for i := range cursors {
		cursors[i] = -1
	}
	return &PostingIterator{
		pl:           pl,
		pos:          -1,
		levelCursors: cursors,
	}
}

func (pl *PostingList) NewIterator() *PostingIterator {
	return pl.iterator()
}

type PostingIterator struct {
	pl           *PostingList
	pos          int
	levelCursors []int
}

func (it *PostingIterator) hasNext() bool {
	return it.pos+1 < it.pl.Len()
}

func (it *PostingIterator) next() Posting {
	it.pos++
	return it.pl.Posting(it.pos)
}

func (it *PostingIterator) currentPosting() Posting {
	return it.pl.Posting(it.pos)
}

func (it *PostingIterator) currentDocID() uint32 {
	return it.pl.Posting(it.pos).DocID
}

func (it *PostingIterator) skipTo(targetDocID uint32) bool {
	if it.pos < 0 {
		it.pos = 0
	}

	for lvl := len(it.pl.skipLevels) - 1; lvl >= 0; lvl-- {
		indices := it.pl.skipLevels[lvl].indices

		cursor := it.levelCursors[lvl]
		if cursor < 0 {
			cursor = 0
		}

		for cursor < len(indices) {
			skipIdx := indices[cursor]
			if skipIdx <= it.pos {
				cursor++
				continue
			}
			if it.pl.postings[skipIdx].DocID >= targetDocID {
				break
			}
			it.pos = skipIdx
			cursor++
		}

		it.levelCursors[lvl] = cursor
	}

	for it.pos < it.pl.Len() {
		if it.pl.postings[it.pos].DocID >= targetDocID {
			return true
		}
		it.pos++
	}

	return false
}

func (it *PostingIterator) AdvanceTo(docID uint32) uint32 {
	if !it.skipTo(docID) {
		return 0
	}
	if it.currentDocID() != docID {
		return 0
	}
	return uint32(len(it.currentPosting().Positions))
}

func (it *PostingIterator) reset() {
	it.pos = -1
	for i := range it.levelCursors {
		it.levelCursors[i] = -1
	}
}

func Intersect(pl1, pl2 *PostingList) *PostingList {
	short, long := pl1, pl2
	if short.Len() > long.Len() {
		short, long = pl2, pl1
	}

	itShort := short.iterator()
	itLong := long.iterator()

	var result []Posting
	for itShort.hasNext() {
		pShort := itShort.next()
		if !itLong.skipTo(pShort.DocID) {
			break
		}
		if itLong.currentDocID() == pShort.DocID {
			result = append(result, Posting{
				DocID:     pShort.DocID,
				Positions: mergePositions(pShort.Positions, itLong.currentPosting().Positions),
			})
		}
	}

	df := uint32(len(result))
	return NewPostingList(result, df)
}

func Union(pl1, pl2 *PostingList) *PostingList {
	i, j := 0, 0

	var result []Posting
	for i < pl1.Len() && j < pl2.Len() {
		p1 := pl1.Posting(i)
		p2 := pl2.Posting(j)

		if p1.DocID == p2.DocID {
			result = append(result, Posting{
				DocID:     p1.DocID,
				Positions: mergePositions(p1.Positions, p2.Positions),
			})
			i++
			j++
		} else if p1.DocID < p2.DocID {
			result = append(result, p1)
			i++
		} else {
			result = append(result, p2)
			j++
		}
	}

	for ; i < pl1.Len(); i++ {
		result = append(result, pl1.Posting(i))
	}
	for ; j < pl2.Len(); j++ {
		result = append(result, pl2.Posting(j))
	}

	df := uint32(len(result))
	return NewPostingList(result, df)
}

func Difference(pl1, pl2 *PostingList) *PostingList {
	it1 := pl1.iterator()
	it2 := pl2.iterator()

	var result []Posting
	for it1.hasNext() {
		p1 := it1.next()

		if !it2.skipTo(p1.DocID) {
			result = append(result, p1)
			continue
		}
		if it2.currentDocID() == p1.DocID {
			continue
		}

		result = append(result, p1)
	}

	df := uint32(len(result))
	return NewPostingList(result, df)
}

func Adjacent(pl1, pl2 *PostingList) *PostingList {
	short, long := pl1, pl2
	shortIsLeft := true
	if short.Len() > long.Len() {
		short, long = pl2, pl1
		shortIsLeft = false
	}

	itShort := short.iterator()
	itLong := long.iterator()

	var result []Posting
	for itShort.hasNext() {
		pShort := itShort.next()
		if !itLong.skipTo(pShort.DocID) {
			break
		}
		if itLong.currentDocID() != pShort.DocID {
			continue
		}
		pLong := itLong.currentPosting()

		var pLeft, pRight Posting
		if shortIsLeft {
			pLeft, pRight = pShort, pLong
		} else {
			pLeft, pRight = pLong, pShort
		}

		var adjPositions []uint32
		i, j := 0, 0
		for i < len(pLeft.Positions) && j < len(pRight.Positions) {
			posLeft := pLeft.Positions[i]
			posRight := pRight.Positions[j]
			if posLeft+1 == posRight {
				adjPositions = append(adjPositions, posLeft, posRight)
				i++
				j++
			} else if posLeft+1 < posRight {
				i++
			} else {
				j++
			}
		}

		if len(adjPositions) > 0 {
			result = append(result, Posting{
				DocID:     pShort.DocID,
				Positions: adjPositions,
			})
		}
	}

	df := uint32(len(result))
	return NewPostingList(result, df)
}

func Near(pl1, pl2 *PostingList, distance int) *PostingList {
	short, long := pl1, pl2
	if short.Len() > long.Len() {
		short, long = pl2, pl1
	}

	itShort := short.iterator()
	itLong := long.iterator()

	var result []Posting
	for itShort.hasNext() {
		pShort := itShort.next()
		if !itLong.skipTo(pShort.DocID) {
			break
		}
		if itLong.currentDocID() != pShort.DocID {
			continue
		}
		pLong := itLong.currentPosting()

		var nearPositions []uint32
		i, j := 0, 0
		for i < len(pShort.Positions) && j < len(pLong.Positions) {
			diff := int(pShort.Positions[i]) - int(pLong.Positions[j])
			absDiff := diff
			if absDiff < 0 {
				absDiff = -absDiff
			}

			if absDiff <= distance {
				nearPositions = sortedInsertUnique(nearPositions, pShort.Positions[i])
				nearPositions = sortedInsertUnique(nearPositions, pLong.Positions[j])
			}

			if pShort.Positions[i] < pLong.Positions[j] {
				i++
			} else {
				j++
			}
		}

		if len(nearPositions) > 0 {
			result = append(result, Posting{
				DocID:     pShort.DocID,
				Positions: nearPositions,
			})
		}
	}

	df := uint32(len(result))
	return NewPostingList(result, df)
}

func (pl *PostingList) FindPositions(docID uint32) []uint32 {
	it := pl.iterator()
	if !it.skipTo(docID) {
		return nil
	}
	if it.currentDocID() != docID {
		return nil
	}
	return it.currentPosting().Positions
}

func sortedInsertUnique(sorted []uint32, val uint32) []uint32 {
	i := sort.Search(len(sorted), func(j int) bool { return sorted[j] >= val })
	if i < len(sorted) && sorted[i] == val {
		return sorted
	}
	sorted = append(sorted, 0)
	copy(sorted[i+1:], sorted[i:])
	sorted[i] = val
	return sorted
}

func mergePositions(p1, p2 []uint32) []uint32 {
	result := make([]uint32, 0, len(p1)+len(p2))
	i, j := 0, 0
	for i < len(p1) && j < len(p2) {
		if p1[i] < p2[j] {
			result = append(result, p1[i])
			i++
		} else if p1[i] > p2[j] {
			result = append(result, p2[j])
			j++
		} else {
			result = append(result, p1[i])
			i++
			j++
		}
	}
	result = append(result, p1[i:]...)
	result = append(result, p2[j:]...)
	return result
}

func flattenSkipLevels(levels []skipLevel) []uint32 {
	if len(levels) == 0 {
		return nil
	}
	var flat []uint32
	flat = append(flat, uint32(len(levels)))
	for _, lvl := range levels {
		flat = append(flat, uint32(len(lvl.indices)))
		for _, idx := range lvl.indices {
			flat = append(flat, uint32(idx))
		}
	}
	return flat
}

func reconstructSkipLevels(flat []uint32) []skipLevel {
	if len(flat) == 0 {
		return nil
	}
	numLevels := flat[0]
	levels := make([]skipLevel, 0, numLevels)
	off := uint32(1)
	for i := uint32(0); i < numLevels && off < uint32(len(flat)); i++ {
		numIndices := flat[off]
		off++
		indices := make([]int, 0, numIndices)
		for j := uint32(0); j < numIndices && off < uint32(len(flat)); j++ {
			indices = append(indices, int(flat[off]))
			off++
		}
		levels = append(levels, skipLevel{indices: indices})
	}
	return levels
}
