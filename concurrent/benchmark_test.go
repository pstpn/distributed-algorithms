package concurrent

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type mapOps interface {
	Put(key string, value string)
	Get(key string) (string, bool)
}

type mapFactory struct {
	name string
	new  func() mapOps
}

type MapSync struct {
	data sync.Map
}

func NewMapSync() *MapSync {
	return &MapSync{}
}

func (m *MapSync) Get(key string) (string, bool) {
	value, ok := m.data.Load(key)
	if !ok {
		return "", false
	}
	return value.(string), true
}

func (m *MapSync) Put(key string, value string) {
	m.data.Store(key, value)
}

var benchFileOnce sync.Once
var benchFileMu sync.Mutex
var benchFilePath string

func ensureBenchFile() string {
	benchFileOnce.Do(func() {
		_ = os.MkdirAll("data", 0755)
		benchFilePath = filepath.Join("data", "benchmarks.csv")
		f, err := os.Create(benchFilePath)
		if err != nil {
			panic(err)
		}
		_, _ = fmt.Fprintln(f, "op,mode,variant,size,avg_latency_ns,avg_throughput_ops,ci95_latency_ns,ci95_throughput_ops,bytes_per_op,allocs_per_op")
		_ = f.Close()
	})
	return benchFilePath
}

func appendBenchRecord(op string, mode string, variant string, size int, avgLatency float64, avgThroughput float64, ci95Latency float64, ci95Throughput float64, bytesPerOp float64, allocsPerOp float64) {
	path := ensureBenchFile()
	benchFileMu.Lock()
	defer benchFileMu.Unlock()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	_, _ = fmt.Fprintf(f, "%s,%s,%s,%d,%.2f,%.2f,%.2f,%.2f,%.1f,%.1f\n", op, mode, variant, size, avgLatency, avgThroughput, ci95Latency, ci95Throughput, bytesPerOp, allocsPerOp)
	_ = f.Close()
}

func benchmarkSizes(b testing.TB) []int {
	b.Helper()
	raw := strings.TrimSpace(os.Getenv("BENCH_SIZES"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("SIZES"))
	}
	if raw == "" {
		return []int{1000, 5000, 10000, 50000, 100000}
	}
	parts := strings.Split(raw, ",")
	sizes := make([]int, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		size, err := strconv.Atoi(value)
		if err != nil || size <= 0 {
			b.Fatalf("invalid BENCH_SIZES value %q", value)
		}
		sizes = append(sizes, size)
	}
	if len(sizes) == 0 {
		b.Fatalf("BENCH_SIZES did not contain valid values")
	}
	return sizes
}

func meanStdDev(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	if len(values) < 2 {
		return mean, 0
	}
	var varianceSum float64
	for _, v := range values {
		d := v - mean
		varianceSum += d * d
	}
	stddev := math.Sqrt(varianceSum / float64(len(values)-1))
	return mean, stddev
}

func ci95HalfWidth(values []float64) (float64, float64) {
	mean, stddev := meanStdDev(values)
	if len(values) < 2 {
		return mean, 0
	}
	se := stddev / math.Sqrt(float64(len(values)))
	return mean, 1.96 * se
}

func makeKeys(size int) []string {
	keys := make([]string, size)
	for i := 0; i < size; i++ {
		keys[i] = "k" + strconv.Itoa(i)
	}
	return keys
}

func fillMap(m mapOps, keys []string) {
	for i, k := range keys {
		m.Put(k, "v"+strconv.Itoa(i))
	}
}

func runParallel(totalOps int, workers int, fn func(int)) {
	var counter int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for {
				i := int(atomic.AddInt64(&counter, 1)) - 1
				if i >= totalOps {
					return
				}
				fn(i)
			}
		}()
	}
	wg.Wait()
}

func runBatchBenchmark(b *testing.B, batchSize int, setup func() mapOps, run func(mapOps)) (float64, float64, float64, float64, float64, float64) {
	b.ReportAllocs()
	iterations := 0
	latSamples := make([]float64, 0, 64)
	thrSamples := make([]float64, 0, 64)
	var totalHeapAlloc uint64
	var totalMallocs uint64
	for b.Loop() {
		b.StopTimer()
		m := setup()
		runtime.GC()
		var memBefore runtime.MemStats
		runtime.ReadMemStats(&memBefore)
		b.StartTimer()
		before := b.Elapsed()
		run(m)
		after := b.Elapsed()
		b.StopTimer()
		var memAfter runtime.MemStats
		runtime.ReadMemStats(&memAfter)
		delta := after - before
		if delta > 0 && batchSize > 0 {
			latSamples = append(latSamples, float64(delta.Nanoseconds())/float64(batchSize))
			thrSamples = append(thrSamples, float64(batchSize)/delta.Seconds())
		}
		totalHeapAlloc += memAfter.HeapAlloc - memBefore.HeapAlloc
		totalMallocs += memAfter.Mallocs - memBefore.Mallocs
		iterations++
		b.StartTimer()
	}

	elapsed := b.Elapsed()
	processed := float64(batchSize * iterations)
	if processed == 0 || elapsed <= 0 {
		return 0, 0, 0, 0, 0, 0
	}
	avgLatency := float64(elapsed.Nanoseconds()) / processed
	avgThroughput := processed / elapsed.Seconds()

	meanLatency, ciLatency := ci95HalfWidth(latSamples)
	meanThroughput, ciThroughput := ci95HalfWidth(thrSamples)
	if len(latSamples) > 0 {
		avgLatency = meanLatency
	}
	if len(thrSamples) > 0 {
		avgThroughput = meanThroughput
	}

	bytesPerOp := float64(totalHeapAlloc) / processed
	allocsPerOp := float64(totalMallocs) / processed

	return avgLatency, avgThroughput, ciLatency, ciThroughput, bytesPerOp, allocsPerOp
}

func benchmarkOp(b *testing.B, op string, mode string, factory mapFactory, size int) {
	workers := runtime.GOMAXPROCS(0)
	keys := makeKeys(size)
	rng := rand.New(rand.NewSource(int64(size)))

	setup := func() mapOps {
		m := factory.new()
		switch op {
		case "put":
			rng.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
		case "get":
			fillMap(m, keys)
		}
		return m
	}

	run := func(m mapOps) {
		switch op {
		case "put":
			if mode == "single" {
				for _, k := range keys {
					m.Put(k, "v")
				}
				return
			}
			runParallel(len(keys), workers, func(i int) {
				m.Put(keys[i], "v")
			})
		case "get":
			if mode == "single" {
				for _, k := range keys {
					_, _ = m.Get(k)
				}
				return
			}
			runParallel(len(keys), workers, func(i int) {
				_, _ = m.Get(keys[i])
			})
		}
	}

	batchSize := len(keys)
	avgLatency, avgThroughput, ci95Latency, ci95Throughput, bytesPerOp, allocsPerOp := runBatchBenchmark(b, batchSize, setup, run)
	appendBenchRecord(op, mode, factory.name, size, avgLatency, avgThroughput, ci95Latency, ci95Throughput, bytesPerOp, allocsPerOp)
	b.ReportMetric(avgLatency, "avg_ns/op")
	b.ReportMetric(ci95Latency, "ci95_ns/op")
	b.ReportMetric(avgThroughput, "avg_ops/s")
	b.ReportMetric(ci95Throughput, "ci95_ops/s")
}

func BenchmarkMap(b *testing.B) {
	singleFactories := []mapFactory{
		{name: "concurrent", new: func() mapOps { return NewMap() }},
		{name: "baseline", new: func() mapOps { return NewMapBaseline() }},
	}
	multiFactories := []mapFactory{
		{name: "concurrent", new: func() mapOps { return NewMap() }},
		{name: "syncmap", new: func() mapOps { return NewMapSync() }},
	}

	for _, size := range benchmarkSizes(b) {
		for _, op := range []string{"put", "get"} {
			for _, factory := range singleFactories {
				name := fmt.Sprintf("%s/single/%s/size=%d", factory.name, op, size)
				b.Run(name, func(b *testing.B) {
					benchmarkOp(b, op, "single", factory, size)
				})
			}
			for _, factory := range multiFactories {
				name := fmt.Sprintf("%s/multi/%s/size=%d", factory.name, op, size)
				b.Run(name, func(b *testing.B) {
					benchmarkOp(b, op, "multi", factory, size)
				})
			}
		}
	}
}
