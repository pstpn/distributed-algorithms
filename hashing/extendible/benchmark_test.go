package extendible

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var defaultBenchmarkSizes = []int{10000, 100000, 1000000}
var benchmarkValueSink uint64

func BenchmarkTableInsert(b *testing.B) {
	baseDir := b.TempDir()
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			keys := makeDataset(size, 100+int64(size))
			iteration := 0
			runBatchBenchmark(b, size, func() {
				table := mustBenchmarkTable(b, filepath.Join(baseDir, fmt.Sprintf("insert-%d-%d.dat", size, iteration)), 64)
				defer func() {
					if err := table.Close(); err != nil {
						b.Fatalf("Close: %v", err)
					}
				}()
				for _, key := range keys {
					if err := table.Insert(key, key); err != nil {
						b.Fatalf("insert key=%d: %v", key, err)
					}
				}
				iteration++
			})
		})
	}
}

func BenchmarkTableUpdate(b *testing.B) {
	baseDir := b.TempDir()
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			keys := makeDataset(size, 200+int64(size))
			iteration := 0
			runBatchBenchmark(b, size, func() {
				b.StopTimer()
				table := mustBenchmarkTable(b, filepath.Join(baseDir, fmt.Sprintf("update-%d-%d.dat", size, iteration)), 64)
				defer func() {
					if err := table.Close(); err != nil {
						b.Fatalf("Close: %v", err)
					}
				}()
				for _, key := range keys {
					if err := table.Insert(key, key); err != nil {
						b.Fatalf("prepare update key=%d: %v", key, err)
					}
				}
				b.StartTimer()
				for _, key := range keys {
					if err := table.Update(key, key+1); err != nil {
						b.Fatalf("update key=%d: %v", key, err)
					}
				}
				iteration++
			})
		})
	}
}

func BenchmarkTableDelete(b *testing.B) {
	baseDir := b.TempDir()
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			keys := makeDataset(size, 300+int64(size))
			iteration := 0
			runBatchBenchmark(b, size, func() {
				b.StopTimer()
				table := mustBenchmarkTable(b, filepath.Join(baseDir, fmt.Sprintf("delete-%d-%d.dat", size, iteration)), 64)
				defer func() {
					if err := table.Close(); err != nil {
						b.Fatalf("Close: %v", err)
					}
				}()
				for _, key := range keys {
					if err := table.Insert(key, key); err != nil {
						b.Fatalf("prepare delete key=%d: %v", key, err)
					}
				}
				b.StartTimer()
				for _, key := range keys {
					if !table.Delete(key) {
						b.Fatalf("delete key=%d returned false", key)
					}
				}
				iteration++
			})
		})
	}
}

func BenchmarkTableGet(b *testing.B) {
	baseDir := b.TempDir()
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			keys := makeDataset(size, 400+int64(size))
			iteration := 0
			runBatchBenchmark(b, size, func() {
				b.StopTimer()
				table := mustBenchmarkTable(b, filepath.Join(baseDir, fmt.Sprintf("get-%d-%d.dat", size, iteration)), 64)
				defer func() {
					if err := table.Close(); err != nil {
						b.Fatalf("Close: %v", err)
					}
				}()
				for _, key := range keys {
					if err := table.Insert(key, key); err != nil {
						b.Fatalf("prepare get key=%d: %v", key, err)
					}
				}
				b.StartTimer()
				var sum uint64
				for _, key := range keys {
					value, ok := table.Get(key)
					if !ok {
						b.Fatalf("get key=%d returned false", key)
					}
					sum += value
				}
				benchmarkValueSink = sum
				iteration++
			})
		})
	}
}

func mustBenchmarkTable(b testing.TB, tablePath string, bucketCapacity int) *Table {
	b.Helper()
	table, err := NewTable(tablePath, bucketCapacity, Uint64Hasher)
	if err != nil {
		b.Fatalf("NewTable: %v", err)
	}
	return table
}

func runBatchBenchmark(b *testing.B, batchSize int, fn func()) {
	b.Helper()
	b.ReportAllocs()
	iterations := 0
	for b.Loop() {
		fn()
		iterations++
	}

	elapsed := b.Elapsed()
	processed := float64(batchSize * iterations)
	if processed == 0 {
		return
	}
	if elapsed <= 0 {
		return
	}
	b.ReportMetric(float64(elapsed.Nanoseconds())/processed, "ns/item")
	b.ReportMetric(processed/elapsed.Seconds(), "ops/s")
}

func benchmarkSizes(b testing.TB) []int {
	b.Helper()

	raw := strings.TrimSpace(os.Getenv("SIZES"))
	if raw == "" {
		return defaultBenchmarkSizes
	}

	parts := strings.Split(raw, ",")
	sizes := make([]int, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		size, err := strconv.Atoi(value)
		if err != nil {
			b.Fatalf("invalid SIZES value %q: %v", value, err)
		}
		if size <= 0 {
			b.Fatalf("invalid SIZES value %q: must be positive", value)
		}
		sizes = append(sizes, size)
	}

	if len(sizes) == 0 {
		b.Fatalf("SIZES=%q did not contain any valid values", raw)
	}
	return sizes
}
