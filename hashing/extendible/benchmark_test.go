package extendible

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/montanaflynn/stats"
)

var defaultBenchmarkSizes = []int{10000, 100000, 1000000}

func BenchmarkTableWarmup(b *testing.B) {
	baseDir := b.TempDir()
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			warmupSamples := make([]float64, 0, 64)
			path := filepath.Join(baseDir, fmt.Sprintf("warmup-%d.dat", size))
			b.Cleanup(func() {
				if err := os.Remove(path); err != nil {
					b.Fatalf("Remove: %v", err)
				}
			})

			for b.Loop() {
				b.StopTimer()
				table := mustBenchmarkTable(b, path, 64)
				b.StartTimer()

				start := b.Elapsed()
				if err := table.PreallocateBuckets(estimateWarmupBuckets(size, 64)); err != nil {
					b.Fatalf("preallocate buckets: %v", err)
				}
				elapsed := b.Elapsed() - start
				warmupSamples = append(warmupSamples, float64(elapsed.Nanoseconds()))

				b.StopTimer()
				if err := table.Close(); err != nil {
					b.Fatalf("Close: %v", err)
				}
				b.StartTimer()
			}

			b.StopTimer()
			b.ReportMetric(ci95(warmupSamples), "ci95_ns/warmup")
		})
	}
}

func BenchmarkTableInsert(b *testing.B) {
	baseDir := b.TempDir()
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			keys := makeDataset(size)
			path := filepath.Join(baseDir, fmt.Sprintf("insert-%d.dat", size))
			table := mustBenchmarkTable(b, path, 64)
			warmupTable(b, table, size, 64)
			b.Cleanup(func() {
				if err := table.Close(); err != nil {
					b.Fatalf("Close: %v", err)
				}
				if err := os.Remove(path); err != nil {
					b.Fatalf("Remove: %v", err)
				}
			})

			runBatchBenchmark(b, table, size, func() {
				b.StopTimer()
				shuffledKeys := make([]uint64, len(keys))
				copy(shuffledKeys, keys)
				rand.Shuffle(len(shuffledKeys), func(i, j int) { shuffledKeys[i], shuffledKeys[j] = shuffledKeys[j], shuffledKeys[i] })
				b.StartTimer()

				for _, key := range shuffledKeys {
					if err := table.Insert(key, key); err != nil {
						b.Fatalf("insert key=%d: %v", key, err)
					}
				}
			})
		})
	}
}

func BenchmarkTableUpdate(b *testing.B) {
	baseDir := b.TempDir()
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			keys := makeDataset(size)
			path := filepath.Join(baseDir, fmt.Sprintf("update-%d.dat", size))
			table := mustBenchmarkTable(b, path, 64)
			warmupTable(b, table, size, 64)
			b.Cleanup(func() {
				if err := table.Close(); err != nil {
					b.Fatalf("Close: %v", err)
				}
				if err := os.Remove(path); err != nil {
					b.Fatalf("Remove: %v", err)
				}
			})

			runBatchBenchmark(b, table, size, func() {
				b.StopTimer()
				shuffledKeys := make([]uint64, len(keys))
				copy(shuffledKeys, keys)
				rand.Shuffle(len(shuffledKeys), func(i, j int) { shuffledKeys[i], shuffledKeys[j] = shuffledKeys[j], shuffledKeys[i] })
				for _, key := range shuffledKeys {
					if err := table.Insert(key, key); err != nil {
						b.Fatalf("prepare update key=%d: %v", key, err)
					}
				}
				b.StartTimer()

				for _, key := range shuffledKeys {
					if err := table.Update(key, key+1); err != nil {
						b.Fatalf("update key=%d: %v", key, err)
					}
				}
			})
		})
	}
}

func BenchmarkTableDelete(b *testing.B) {
	baseDir := b.TempDir()
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			keys := makeDataset(size)
			path := filepath.Join(baseDir, fmt.Sprintf("delete-%d.dat", size))
			table := mustBenchmarkTable(b, path, 64)
			warmupTable(b, table, size, 64)
			b.Cleanup(func() {
				if err := table.Close(); err != nil {
					b.Fatalf("Close: %v", err)
				}
				if err := os.Remove(path); err != nil {
					b.Fatalf("Remove: %v", err)
				}
			})

			runBatchBenchmark(b, table, size, func() {
				b.StopTimer()
				shuffledKeys := make([]uint64, len(keys))
				copy(shuffledKeys, keys)
				rand.Shuffle(len(shuffledKeys), func(i, j int) { shuffledKeys[i], shuffledKeys[j] = shuffledKeys[j], shuffledKeys[i] })
				for _, key := range shuffledKeys {
					if err := table.Insert(key, key); err != nil {
						b.Fatalf("prepare delete key=%d: %v", key, err)
					}
				}
				b.StartTimer()

				for _, key := range shuffledKeys {
					if !table.Delete(key) {
						b.Fatalf("delete key=%d returned false", key)
					}
				}
			})
		})
	}
}

func BenchmarkTableGet(b *testing.B) {
	baseDir := b.TempDir()
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			keys := makeDataset(size)
			path := filepath.Join(baseDir, fmt.Sprintf("get-%d.dat", size))
			table := mustBenchmarkTable(b, path, 64)
			warmupTable(b, table, size, 64)
			b.Cleanup(func() {
				if err := table.Close(); err != nil {
					b.Fatalf("Close: %v", err)
				}
				if err := os.Remove(path); err != nil {
					b.Fatalf("Remove: %v", err)
				}
			})

			runBatchBenchmark(b, table, size, func() {
				b.StopTimer()
				shuffledKeys := make([]uint64, len(keys))
				copy(shuffledKeys, keys)
				rand.Shuffle(len(shuffledKeys), func(i, j int) { shuffledKeys[i], shuffledKeys[j] = shuffledKeys[j], shuffledKeys[i] })
				for _, key := range shuffledKeys {
					if err := table.Insert(key, key); err != nil {
						b.Fatalf("prepare get key=%d: %v", key, err)
					}
				}
				b.StartTimer()

				var sum uint64
				for _, key := range shuffledKeys {
					value, ok := table.Get(key)
					if !ok {
						b.Fatalf("get key=%d returned false", key)
					}
					sum += value
				}
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

func runBatchBenchmark(b *testing.B, table *Table, batchSize int, fn func()) {
	b.ReportAllocs()
	iterations := 0
	nsItemSamples := make([]float64, 0, 64)
	for b.Loop() {
		b.StopTimer()
		if err := table.Clear(); err != nil {
			b.Fatalf("clear table: %v", err)
		}
		b.StartTimer()

		before := b.Elapsed()
		fn()
		after := b.Elapsed()

		b.StopTimer()
		delta := after - before
		if delta > 0 && batchSize > 0 {
			nsItemSamples = append(nsItemSamples, float64(delta.Nanoseconds())/float64(batchSize))
		}
		iterations++
		b.StartTimer()
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
	b.ReportMetric(ci95(nsItemSamples), "ci95_ns/item")
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

func warmupTable(b testing.TB, table *Table, size int, bucketCapacity int) {
	b.Helper()
	targetBuckets := estimateWarmupBuckets(size, bucketCapacity)
	if err := table.PreallocateBuckets(targetBuckets); err != nil {
		b.Fatalf("preallocate buckets: %v", err)
	}
	resident, total, err := table.ResidentBucketPages()
	if err != nil {
		b.Fatalf("check resident mapped pages: %v", err)
	}
	if total > 0 && resident*100/total < 80 {
		b.Fatalf("insufficient mmap residency after warmup: resident=%d total=%d", resident, total)
	}
}

func estimateWarmupBuckets(size int, bucketCapacity int) int {
	if size <= 0 || bucketCapacity <= 0 {
		return 1
	}
	buckets := (size + bucketCapacity - 1) / bucketCapacity
	if buckets < 1 {
		buckets = 1
	}
	return buckets*2 + 1
}

func ci95(values []float64) float64 {
	n := len(values)
	if n <= 1 {
		return 0
	}
	variance, _ := stats.StandardDeviation(values)
	return 1.96 * math.Sqrt(variance/float64(n))
}
