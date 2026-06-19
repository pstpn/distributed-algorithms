package compression

import (
	"testing"
)

func TestCompressUint32(t *testing.T) {
	arr := []uint32{1, 3, 10, 25, 100, 255}
	encoded := Compress(arr)
	decoded := Decompress(encoded)

	if len(decoded) != len(arr) {
		t.Fatalf("length mismatch: got %d, want %d", len(decoded), len(arr))
	}
	for i, v := range arr {
		if decoded[i] != v {
			t.Errorf("at index %d: got %d, want %d", i, decoded[i], v)
		}
	}
}

func TestCompressUint32Empty(t *testing.T) {
	arr := []uint32{}
	encoded := Compress(arr)
	decoded := Decompress(encoded)
	if len(decoded) != 0 {
		t.Errorf("expected empty, got %d elements", len(decoded))
	}
}

func TestCompressedSize(t *testing.T) {
	arr := []uint32{1, 3, 10, 25}
	encoded := Compress(arr)
	size := CompressedSize(encoded)
	if size != len(encoded) {
		t.Errorf("CompressedSize = %d, want %d", size, len(encoded))
	}
}

func TestCompressedSizeEmpty(t *testing.T) {
	arr := []uint32{}
	encoded := Compress(arr)
	size := CompressedSize(encoded)
	if size != len(encoded) {
		t.Errorf("CompressedSize = %d, want %d", size, len(encoded))
	}
}

func TestCompressLargeArray(t *testing.T) {
	arr := make([]uint32, 1000)
	for i := 0; i < 1000; i++ {
		arr[i] = uint32(i % 50)
	}
	encoded := Compress(arr)
	decoded := Decompress(encoded)

	if len(decoded) != len(arr) {
		t.Fatalf("length mismatch: got %d, want %d", len(decoded), len(arr))
	}
	for i, v := range arr {
		if decoded[i] != v {
			t.Errorf("at index %d: got %d, want %d", i, decoded[i], v)
		}
	}
}
