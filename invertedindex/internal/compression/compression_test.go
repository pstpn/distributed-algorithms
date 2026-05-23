package compression

import (
	"testing"
)

func TestDeltaEncodePostings(t *testing.T) {
	entries := []PostingEntry{
		{DocID: 1, Positions: []uint32{0, 5, 10}},
		{DocID: 3, Positions: []uint32{2, 7}},
		{DocID: 10, Positions: []uint32{1}},
	}

	encoded := DeltaEncodePostings(entries)
	decoded := DeltaDecodePostings(encoded)

	if len(decoded) != len(entries) {
		t.Fatalf("length mismatch: got %d, want %d", len(decoded), len(entries))
	}

	for i, entry := range entries {
		if decoded[i].DocID != entry.DocID {
			t.Errorf("docID at index %d: got %d, want %d", i, decoded[i].DocID, entry.DocID)
		}
		if len(decoded[i].Positions) != len(entry.Positions) {
			t.Fatalf("positions length at index %d: got %d, want %d", i, len(decoded[i].Positions), len(entry.Positions))
		}
		for j, pos := range entry.Positions {
			if decoded[i].Positions[j] != pos {
				t.Errorf("position at index %d/%d: got %d, want %d", i, j, decoded[i].Positions[j], pos)
			}
		}
	}
}

func TestDeltaEncodePostingsEmpty(t *testing.T) {
	entries := []PostingEntry{}
	encoded := DeltaEncodePostings(entries)
	if encoded != nil {
		t.Errorf("expected nil for empty postings, got %v", encoded)
	}
}
