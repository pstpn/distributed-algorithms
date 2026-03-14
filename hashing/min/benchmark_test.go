package minhash

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

var defaultBenchmarkSizes = []int{100, 500, 1000, 5000, 10000}
var benchmarkMatchSink int

func BenchmarkIndexBuild(b *testing.B) {
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			workload := MakeWorkload(size, 100+int64(size))
			runBatchBenchmark(b, size, func() {
				idx, err := NewIndex(DefaultConfig())
				if err != nil {
					b.Fatalf("NewIndex: %v", err)
				}
				for _, doc := range workload.Base {
					if err := idx.Add(doc); err != nil {
						b.Fatalf("Add: %v", err)
					}
				}
				benchmarkMatchSink = idx.Stats().DocumentCount
			})
		})
	}
}

func BenchmarkIndexAdd(b *testing.B) {
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			workload := MakeWorkload(size, 200+int64(size))
			runBatchBenchmark(b, size, func() {
				b.StopTimer()
				idx, err := NewIndex(DefaultConfig())
				if err != nil {
					b.Fatalf("NewIndex: %v", err)
				}
				if err := idx.Build(workload.Base); err != nil {
					b.Fatalf("Build: %v", err)
				}
				b.StartTimer()
				for _, doc := range workload.Incoming {
					if err := idx.Add(doc); err != nil {
						b.Fatalf("Add: %v", err)
					}
				}
				benchmarkMatchSink = idx.Stats().DocumentCount
			})
		})
	}
}

func BenchmarkIndexFullScan(b *testing.B) {
	for _, size := range benchmarkSizes(b) {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			workload := MakeWorkload(size, 300+int64(size))
			runBatchBenchmark(b, size, func() {
				b.StopTimer()
				idx, err := NewIndex(DefaultConfig())
				if err != nil {
					b.Fatalf("NewIndex: %v", err)
				}
				if err := idx.Build(workload.Base); err != nil {
					b.Fatalf("Build: %v", err)
				}
				b.StartTimer()
				totalMatches := 0
				for _, query := range workload.Queries {
					totalMatches += len(idx.FullScanDuplicates(query.Text, 0.5))
				}
				benchmarkMatchSink = totalMatches
			})
		})
	}
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
	if processed == 0 || elapsed <= 0 {
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
