package index

import (
	"encoding/binary"
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

func NewPostingListWithSkipList(postings []Posting, df uint32, skipListData []byte) *PostingList {
	pl := &PostingList{
		postings: postings,
		df:       df,
	}
	if len(skipListData) > 0 {
		pl.skipLevels = unmarshalSkipList(skipListData)
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
	it1 := pl1.iterator()
	it2 := pl2.iterator()

	var result []Posting
	for it1.hasNext() && it2.hasNext() {
		p1 := it1.next()
		p2 := it2.next()

		for {
			if p1.DocID == p2.DocID {
				result = append(result, Posting{
					DocID:     p1.DocID,
					Positions: mergePositions(p1.Positions, p2.Positions),
				})
				break
			}
			if p1.DocID < p2.DocID {
				if !it1.skipTo(p2.DocID) {
					break
				}
				p1 = it1.currentPosting()
			} else {
				if !it2.skipTo(p1.DocID) {
					break
				}
				p2 = it2.currentPosting()
			}
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
	it1 := pl1.iterator()
	it2 := pl2.iterator()

	var result []Posting
	for it1.hasNext() && it2.hasNext() {
		p1 := it1.next()
		p2 := it2.next()

		for {
			if p1.DocID == p2.DocID {
				var adjPositions []uint32
				for _, pos1 := range p1.Positions {
					for _, pos2 := range p2.Positions {
						if pos2 == pos1+1 {
							adjPositions = append(adjPositions, pos1, pos2)
							break
						}
					}
				}

				if len(adjPositions) > 0 {
					result = append(result, Posting{
						DocID:     p1.DocID,
						Positions: adjPositions,
					})
				}
				break
			}
			if p1.DocID < p2.DocID {
				if !it1.skipTo(p2.DocID) {
					break
				}
				p1 = it1.currentPosting()
			} else {
				if !it2.skipTo(p1.DocID) {
					break
				}
				p2 = it2.currentPosting()
			}
		}
	}

	df := uint32(len(result))
	return NewPostingList(result, df)
}

func Near(pl1, pl2 *PostingList, distance int) *PostingList {
	it1 := pl1.iterator()
	it2 := pl2.iterator()

	var result []Posting
	for it1.hasNext() && it2.hasNext() {
		p1 := it1.next()
		p2 := it2.next()

		for {
			if p1.DocID == p2.DocID {
				var nearPositions []uint32
				i, j := 0, 0
				for i < len(p1.Positions) && j < len(p2.Positions) {
					diff := int(p1.Positions[i]) - int(p2.Positions[j])
					absDiff := diff
					if absDiff < 0 {
						absDiff = -absDiff
					}

					if absDiff <= distance {
						nearPositions = sortedInsertUnique(nearPositions, p1.Positions[i])
						nearPositions = sortedInsertUnique(nearPositions, p2.Positions[j])
					}

					if p1.Positions[i] < p2.Positions[j] {
						i++
					} else {
						j++
					}
				}

				if len(nearPositions) > 0 {
					result = append(result, Posting{
						DocID:     p1.DocID,
						Positions: nearPositions,
					})
				}
				break
			}
			if p1.DocID < p2.DocID {
				if !it1.skipTo(p2.DocID) {
					break
				}
				p1 = it1.currentPosting()
			} else {
				if !it2.skipTo(p1.DocID) {
					break
				}
				p2 = it2.currentPosting()
			}
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

func (pl *PostingList) MarshalSkipList() []byte {
	if len(pl.skipLevels) == 0 {
		return nil
	}

	size := 2
	for _, lvl := range pl.skipLevels {
		size += 2 + len(lvl.indices)*4
	}

	buf := make([]byte, 0, size)
	le := binary.LittleEndian

	var tmp [2]byte
	le.PutUint16(tmp[:], uint16(len(pl.skipLevels)))
	buf = append(buf, tmp[:]...)

	for _, lvl := range pl.skipLevels {
		le.PutUint16(tmp[:], uint16(len(lvl.indices)))
		buf = append(buf, tmp[:]...)

		for _, idx := range lvl.indices {
			var idxBuf [4]byte
			le.PutUint32(idxBuf[:], uint32(idx))
			buf = append(buf, idxBuf[:]...)
		}
	}

	return buf
}

func unmarshalSkipList(data []byte) []skipLevel {
	if len(data) < 2 {
		return nil
	}

	le := binary.LittleEndian
	numLevels := le.Uint16(data[0:2])
	offset := 2

	levels := make([]skipLevel, 0, numLevels)
	for i := uint16(0); i < numLevels; i++ {
		if offset+2 > len(data) {
			break
		}
		numIndices := le.Uint16(data[offset : offset+2])
		offset += 2

		indices := make([]int, 0, numIndices)
		for j := uint16(0); j < numIndices; j++ {
			if offset+4 > len(data) {
				break
			}
			indices = append(indices, int(le.Uint32(data[offset:offset+4])))
			offset += 4
		}

		levels = append(levels, skipLevel{indices: indices})
	}

	return levels
}
