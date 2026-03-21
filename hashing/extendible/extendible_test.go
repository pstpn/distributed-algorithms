package extendible

import (
	"errors"
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"
)

func TestTableRandomOperations(t *testing.T) {
	seeds := []int64{1, 7, 42, 99, 2026}
	for _, seed := range seeds {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			table := newTestTable(t, 8)
			mirror := make(map[uint64]uint64)
			rng := rand.New(rand.NewSource(seed))

			for step := 0; step < 50000; step++ {
				key := uint64(rng.Intn(12000))
				value := uint64(rng.Uint32())<<32 | uint64(rng.Uint32())

				switch rng.Intn(4) {
				case 0:
					err := table.Insert(key, value)
					_, exists := mirror[key]
					if exists {
						if !errors.Is(err, ErrKeyExists) {
							t.Fatalf("insert existing key=%d: got %v, want %v", key, err, ErrKeyExists)
						}
					} else {
						if err != nil {
							t.Fatalf("insert key=%d: %v", key, err)
						}
						mirror[key] = value
					}
				case 1:
					err := table.Update(key, value)
					_, exists := mirror[key]
					if exists {
						if err != nil {
							t.Fatalf("update existing key=%d: %v", key, err)
						}
						mirror[key] = value
					} else if !errors.Is(err, ErrKeyNotFound) {
						t.Fatalf("update missing key=%d: got %v, want %v", key, err, ErrKeyNotFound)
					}
				case 2:
					deleted := table.Delete(key)
					_, exists := mirror[key]
					if deleted != exists {
						t.Fatalf("delete key=%d: got %v, want %v", key, deleted, exists)
					}
					delete(mirror, key)
				case 3:
					got, ok := table.Get(key)
					want, exists := mirror[key]
					if ok != exists {
						t.Fatalf("get key=%d: got ok=%v, want ok=%v", key, ok, exists)
					}
					if ok && got != want {
						t.Fatalf("get key=%d: got value=%d, want %d", key, got, want)
					}
				}

				if step%1000 == 0 {
					assertMatchesMirror(t, table, mirror)
				}
			}

			assertMatchesMirror(t, table, mirror)
		})
	}
}

func TestTableSplitAndMerge(t *testing.T) {
	table := newTestTable(t, 2)
	keys := makeDataset(512)

	for index, key := range keys {
		if err := table.Insert(key, uint64(index)); err != nil {
			t.Fatalf("insert key=%d: %v", key, err)
		}
	}

	statsAfterInsert := table.Stats()
	if statsAfterInsert.GlobalDepth == 0 {
		t.Fatalf("expected directory growth, got globalDepth=%d", statsAfterInsert.GlobalDepth)
	}
	if statsAfterInsert.MaxBucketLoad > statsAfterInsert.BucketCapacity {
		t.Fatalf("bucket overflow: max=%d capacity=%d", statsAfterInsert.MaxBucketLoad, statsAfterInsert.BucketCapacity)
	}

	for _, key := range keys {
		if !table.Delete(key) {
			t.Fatalf("delete key=%d returned false", key)
		}
	}

	statsAfterDelete := table.Stats()
	if statsAfterDelete.Size != 0 {
		t.Fatalf("expected empty table, got size=%d", statsAfterDelete.Size)
	}
	if statsAfterDelete.GlobalDepth != 0 {
		t.Fatalf("expected directory shrink to depth 0, got %d", statsAfterDelete.GlobalDepth)
	}
	if statsAfterDelete.BucketCount != 1 {
		t.Fatalf("expected single bucket after shrink, got %d", statsAfterDelete.BucketCount)
	}
}

func TestTablePersistenceAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persistent-table.dat")
	table, err := NewTable(path, 8, Uint64Hasher)
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}

	keys := makeDataset(2000)
	for index, key := range keys {
		if err := table.Insert(key, uint64(index+1)); err != nil {
			t.Fatalf("insert key=%d: %v", key, err)
		}
	}
	if err := table.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if err := table.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenTable(path, Uint64Hasher)
	if err != nil {
		t.Fatalf("OpenTable: %v", err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Fatalf("Close reopened: %v", err)
		}
	}()

	for index, key := range keys {
		value, ok := reopened.Get(key)
		if !ok {
			t.Fatalf("missing key after reopen=%d", key)
		}
		if value != uint64(index+1) {
			t.Fatalf("value mismatch key=%d: got=%d want=%d", key, value, uint64(index+1))
		}
	}
}

func FuzzTableAgainstMap(f *testing.F) {
	f.Add(uint64(1), uint16(128))
	f.Add(uint64(7), uint16(777))
	f.Add(uint64(2026), uint16(1500))

	f.Fuzz(func(t *testing.T, seed uint64, steps uint16) {
		table := newTestTable(t, 4)
		mirror := make(map[uint64]uint64)
		rng := rand.New(rand.NewSource(int64(seed)))
		iterations := int(steps%2000) + 1

		for step := 0; step < iterations; step++ {
			key := uint64(rng.Intn(2048))
			value := uint64(rng.Uint32())<<32 | uint64(rng.Uint32())

			switch rng.Intn(3) {
			case 0:
				err := table.Insert(key, value)
				_, exists := mirror[key]
				if exists {
					if !errors.Is(err, ErrKeyExists) {
						t.Fatalf("insert existing key=%d: got %v", key, err)
					}
				} else {
					if err != nil {
						t.Fatalf("insert key=%d: %v", key, err)
					}
					mirror[key] = value
				}
			case 1:
				err := table.Update(key, value)
				_, exists := mirror[key]
				if exists {
					if err != nil {
						t.Fatalf("update key=%d: %v", key, err)
					}
					mirror[key] = value
				} else if !errors.Is(err, ErrKeyNotFound) {
					t.Fatalf("update missing key=%d: got %v", key, err)
				}
			case 2:
				deleted := table.Delete(key)
				_, exists := mirror[key]
				if deleted != exists {
					t.Fatalf("delete key=%d: got %v, want %v", key, deleted, exists)
				}
				delete(mirror, key)
			}
		}

		assertMatchesMirror(t, table, mirror)
	})
}

func newTestTable(t testing.TB, bucketCapacity int) *Table {
	t.Helper()
	tablePath := filepath.Join(t.TempDir(), "table.dat")
	table, err := NewTable(tablePath, bucketCapacity, Uint64Hasher)
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	t.Cleanup(func() {
		if err := table.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	})
	return table
}

func assertMatchesMirror(t testing.TB, table *Table, mirror map[uint64]uint64) {
	t.Helper()
	if table.Len() != len(mirror) {
		t.Fatalf("len mismatch: table=%d mirror=%d", table.Len(), len(mirror))
	}

	stats := table.Stats()
	if stats.MaxBucketLoad > stats.BucketCapacity {
		t.Fatalf("bucket overflow: max=%d capacity=%d", stats.MaxBucketLoad, stats.BucketCapacity)
	}

	for key, expected := range mirror {
		actual, ok := table.Get(key)
		if !ok {
			t.Fatalf("missing key=%d", key)
		}
		if actual != expected {
			t.Fatalf("value mismatch key=%d: got=%d want=%d", key, actual, expected)
		}
	}
}

func makeDataset(size int) []uint64 {
	keys := make([]uint64, size)
	for index := range keys {
		keys[index] = rand.Uint64()
	}
	return keys
}
