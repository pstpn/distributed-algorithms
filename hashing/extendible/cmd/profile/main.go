package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"extendible"
)

type profileSummary struct {
	Operation       string           `json:"operation"`
	Size            int              `json:"size"`
	BucketCapacity  int              `json:"bucket_capacity"`
	ElapsedNS       int64            `json:"elapsed_ns"`
	ThroughputOpsS  float64          `json:"throughput_ops_s"`
	CPUUserNS       int64            `json:"cpu_user_ns"`
	CPUSysNS        int64            `json:"cpu_sys_ns"`
	TotalAllocDiff  uint64           `json:"total_alloc_diff"`
	AllocCountTotal uint64           `json:"alloc_count_total"`
	Stats           extendible.Stats `json:"stats"`
	CPUProfile      string           `json:"cpu_profile"`
}

var profileValueSink uint64

func main() {
	var (
		op             = flag.String("op", "insert", "operation: insert, update, delete or get")
		size           = flag.Int("size", 100000, "number of keys in the workload")
		bucketCapacity = flag.Int("capacity", 64, "bucket capacity")
		seed           = flag.Int64("seed", 2026, "random seed")
		outDir         = flag.String("out", filepath.Join("metrics", "profiles"), "output directory")
	)
	flag.Parse()

	if *size <= 0 {
		fatalf("size must be positive")
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatalf("mkdir %s: %v", *outDir, err)
	}
	tablePath := filepath.Join(*outDir, fmt.Sprintf("table_%s_%d.dat", *op, *size))
	_ = os.Remove(tablePath)

	keys := makeDataset(*size, *seed)
	table, err := extendible.NewTable(tablePath, *bucketCapacity, extendible.Uint64Hasher)
	if err != nil {
		fatalf("NewTable: %v", err)
	}
	defer func() {
		if err := table.Close(); err != nil {
			fatalf("close table: %v", err)
		}
	}()

	switch *op {
	case "insert":
	case "update", "delete", "get":
		for _, key := range keys {
			if err := table.Insert(key, key); err != nil {
				fatalf("prepare key=%d: %v", key, err)
			}
		}
	default:
		fatalf("unsupported operation %q", *op)
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	var ruBefore syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ruBefore); err != nil {
		fatalf("getrusage before: %v", err)
	}

	cpuProfilePath := filepath.Join(*outDir, fmt.Sprintf("%s_%d_cpu.pprof", *op, *size))
	cpuFile, err := os.Create(cpuProfilePath)
	if err != nil {
		fatalf("create cpu profile: %v", err)
	}
	defer cpuFile.Close()

	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		fatalf("start cpu profile: %v", err)
	}
	start := time.Now()

	switch *op {
	case "insert":
		for _, key := range keys {
			if err := table.Insert(key, key); err != nil {
				fatalf("insert key=%d: %v", key, err)
			}
		}
	case "update":
		for _, key := range keys {
			if err := table.Update(key, key+1); err != nil {
				fatalf("update key=%d: %v", key, err)
			}
		}
	case "delete":
		for _, key := range keys {
			if !table.Delete(key) {
				fatalf("delete key=%d returned false", key)
			}
		}
	case "get":
		var sum uint64
		for _, key := range keys {
			value, ok := table.Get(key)
			if !ok {
				fatalf("get key=%d returned false", key)
			}
			sum += value
		}
		profileValueSink = sum
	}

	elapsed := time.Since(start)
	pprof.StopCPUProfile()

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	var ruAfter syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ruAfter); err != nil {
		fatalf("getrusage after: %v", err)
	}
	cpuUserNS := (ruAfter.Utime.Sec-ruBefore.Utime.Sec)*1_000_000_000 + int64(ruAfter.Utime.Usec-ruBefore.Utime.Usec)*1000
	cpuSysNS := (ruAfter.Stime.Sec-ruBefore.Stime.Sec)*1_000_000_000 + int64(ruAfter.Stime.Usec-ruBefore.Stime.Usec)*1000

	summary := profileSummary{
		Operation:       *op,
		Size:            *size,
		BucketCapacity:  *bucketCapacity,
		ElapsedNS:       elapsed.Nanoseconds(),
		ThroughputOpsS:  float64(*size) / elapsed.Seconds(),
		CPUUserNS:       cpuUserNS,
		CPUSysNS:        cpuSysNS,
		TotalAllocDiff:  after.TotalAlloc - before.TotalAlloc,
		AllocCountTotal: after.Mallocs,
		Stats:           table.Stats(),
		CPUProfile:      cpuProfilePath,
	}

	summaryPath := filepath.Join(*outDir, fmt.Sprintf("%s_%d_summary.json", *op, *size))
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		fatalf("marshal summary: %v", err)
	}
	if err := os.WriteFile(summaryPath, data, 0o644); err != nil {
		fatalf("write summary: %v", err)
	}

	fmt.Printf("profile written to %s\n", summaryPath)
}

func makeDataset(size int, seed int64) []uint64 {
	state := uint64(seed) + 0x9e3779b97f4a7c15
	keys := make([]uint64, size)
	for index := range keys {
		state += 0x9e3779b97f4a7c15
		keys[index] = extendible.Uint64Hasher(state ^ uint64(index))
	}
	return keys
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
