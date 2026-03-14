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

	"minhash"
)

type profileSummary struct {
	Operation       string        `json:"operation"`
	Size            int           `json:"size"`
	ElapsedNS       int64         `json:"elapsed_ns"`
	ThroughputOpsS  float64       `json:"throughput_ops_s"`
	CPUUserNS       int64         `json:"cpu_user_ns"`
	CPUSysNS        int64         `json:"cpu_sys_ns"`
	TotalAllocDiff  uint64        `json:"total_alloc_diff"`
	AllocCountTotal uint64        `json:"alloc_count_total"`
	MatchCount      int           `json:"match_count"`
	Stats           minhash.Stats `json:"stats"`
	CPUProfile      string        `json:"cpu_profile"`
}

var profileSink int

func main() {
	var (
		op          = flag.String("op", "build", "operation: build, add or fullscan")
		size        = flag.Int("size", 5000, "number of documents in the workload")
		seed        = flag.Int64("seed", 2026, "random seed")
		threshold   = flag.Float64("threshold", 0.5, "similarity threshold for duplicate search")
		numHashes   = flag.Int("num-hashes", minhash.DefaultNumHashes, "number of minhash functions")
		bands       = flag.Int("bands", minhash.DefaultBands, "number of LSH bands")
		shingleSize = flag.Int("shingle-size", minhash.DefaultShingleSize, "token shingle size")
		outDir      = flag.String("out", filepath.Join("metrics", "profiles"), "output directory")
	)
	flag.Parse()

	if *size <= 0 {
		fatalf("size must be positive")
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatalf("mkdir %s: %v", *outDir, err)
	}

	cfg := minhash.Config{
		NumHashes:           *numHashes,
		Bands:               *bands,
		ShingleSize:         *shingleSize,
		SimilarityThreshold: *threshold,
	}
	idx, err := minhash.NewIndex(cfg)
	if err != nil {
		fatalf("NewIndex: %v", err)
	}
	workload := minhash.MakeWorkload(*size, *seed)

	switch *op {
	case "build":
	case "add", "fullscan":
		if err := idx.Build(workload.Base); err != nil {
			fatalf("Build: %v", err)
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
	matchCount := 0
	switch *op {
	case "build":
		idx, err = minhash.NewIndex(cfg)
		if err != nil {
			fatalf("NewIndex: %v", err)
		}
		for _, doc := range workload.Base {
			if err := idx.Add(doc); err != nil {
				fatalf("Add: %v", err)
			}
		}
	case "add":
		for _, doc := range workload.Incoming {
			if err := idx.Add(doc); err != nil {
				fatalf("Add: %v", err)
			}
		}
	case "fullscan":
		for _, query := range workload.Queries {
			matchCount += len(idx.FullScanDuplicates(query.Text, *threshold))
		}
		profileSink = matchCount
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
		ElapsedNS:       elapsed.Nanoseconds(),
		ThroughputOpsS:  float64(*size) / elapsed.Seconds(),
		CPUUserNS:       cpuUserNS,
		CPUSysNS:        cpuSysNS,
		TotalAllocDiff:  after.TotalAlloc - before.TotalAlloc,
		AllocCountTotal: after.Mallocs,
		MatchCount:      matchCount,
		Stats:           idx.Stats(),
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

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
