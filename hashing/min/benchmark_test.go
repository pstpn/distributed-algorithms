package minhash

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
)

var defaultBenchmarkSizes = []int{500, 1000, 2500, 5000, 7500, 10000, 15000}

func BenchmarkIndexBuild(b *testing.B) {
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			docs := randomDocuments(size)

			runBatchBenchmark(b, size, func() {
				b.StopTimer()
				idx, err := NewIndex(DefaultConfig())
				if err != nil {
					b.Fatalf("NewIndex: %v", err)
				}
				rand.Shuffle(len(docs), func(i int, j int) { docs[i], docs[j] = docs[j], docs[i] })
				b.StartTimer()

				idx.Build(docs)
			})
		})
	}
}

func BenchmarkIndexAdd(b *testing.B) {
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			baseDocs := randomDocuments(size)

			runBatchBenchmark(b, size, func() {
				b.StopTimer()
				idx, err := NewIndex(DefaultConfig())
				if err != nil {
					b.Fatalf("NewIndex: %v", err)
				}
				if err := idx.Build(baseDocs); err != nil {
					b.Fatalf("Build: %v", err)
				}
				docsForInsert := randomDocuments(size)
				b.StartTimer()

				for _, doc := range docsForInsert {
					if err := idx.Add(doc); err != nil {
						b.Fatalf("Add: %v", err)
					}
				}
			})
		})
	}
}

func BenchmarkIndexFullScan(b *testing.B) {
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			baseDocs := randomDocuments(size)

			runBatchBenchmark(b, size, func() {
				b.StopTimer()
				idx, err := NewIndex(DefaultConfig())
				if err != nil {
					b.Fatalf("NewIndex: %v", err)
				}
				if err := idx.Build(baseDocs); err != nil {
					b.Fatalf("Build: %v", err)
				}
				queryDocs := randomDocuments(size)
				b.StartTimer()

				totalMatches := 0
				for _, query := range queryDocs {
					totalMatches += len(idx.FullScanDuplicates(query.Text, 0.5))
				}
			})
		})
	}
}

func runBatchBenchmark(b *testing.B, batchSize int, fn func()) {
	b.Helper()
	b.ReportAllocs()
	iterations := 0
	nsItemSamples := make([]float64, 0, 64)
	for b.Loop() {
		before := b.Elapsed()
		fn()
		after := b.Elapsed()
		delta := after - before
		if delta > 0 && batchSize > 0 {
			nsItemSamples = append(nsItemSamples, float64(delta.Nanoseconds())/float64(batchSize))
		}
		iterations++
	}
	elapsed := b.Elapsed()
	processed := float64(batchSize * iterations)
	if processed == 0 || elapsed <= 0 {
		return
	}
	b.ReportMetric(float64(elapsed.Nanoseconds())/processed, "ns/item")
	b.ReportMetric(processed/elapsed.Seconds(), "ops/s")
	b.ReportMetric(ci95(nsItemSamples), "ci95_ns/item")
}

func ci95(values []float64) float64 {
	n := len(values)
	if n <= 1 {
		return 0
	}
	var sum float64
	for _, value := range values {
		sum += value
	}
	mean := sum / float64(n)
	var squaredSum float64
	for _, value := range values {
		delta := value - mean
		squaredSum += delta * delta
	}
	variance := squaredSum / float64(n-1)
	stdDev := math.Sqrt(variance)
	return 1.96 * stdDev / math.Sqrt(float64(n))
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
