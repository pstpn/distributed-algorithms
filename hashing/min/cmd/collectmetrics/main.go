package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type benchmarkRecord struct {
	Operation   string
	Size        int
	NSPerOp     float64
	NSPerItem   float64
	OpsPerSec   float64
	BytesPerOp  float64
	AllocsPerOp float64
}

type profileRecord struct {
	Operation       string  `json:"operation"`
	Size            int     `json:"size"`
	ElapsedNS       int64   `json:"elapsed_ns"`
	ThroughputOpsS  float64 `json:"throughput_ops_s"`
	CPUUserNS       int64   `json:"cpu_user_ns"`
	CPUSysNS        int64   `json:"cpu_sys_ns"`
	TotalAllocDiff  uint64  `json:"total_alloc_diff"`
	AllocCountTotal uint64  `json:"alloc_count_total"`
	MatchCount      int     `json:"match_count"`
	Stats           struct {
		DocumentCount int `json:"DocumentCount"`
		BucketCount   int `json:"BucketCount"`
		BandCount     int `json:"BandCount"`
		NumHashes     int `json:"NumHashes"`
		ShingleSize   int `json:"ShingleSize"`
	} `json:"stats"`
}

func main() {
	var (
		sizesFlag   = flag.String("sizes", "100,500,1000,5000,10000", "comma-separated workload sizes")
		threshold   = flag.Float64("threshold", 0.5, "similarity threshold for duplicate search")
		numHashes   = flag.Int("num-hashes", 64, "number of minhash functions")
		bands       = flag.Int("bands", 8, "number of LSH bands")
		shingleSize = flag.Int("shingle-size", 2, "token shingle size")
		outDir      = flag.String("out", "metrics", "directory for raw metrics and generated plots")
	)
	flag.Parse()

	sizes, err := parseSizes(*sizesFlag)
	if err != nil {
		fatalf("parse sizes: %v", err)
	}

	rawDir := filepath.Join(*outDir, "raw")
	plotDir := filepath.Join(*outDir, "plots")
	profileDir := filepath.Join(rawDir, "profiles")
	for _, dir := range []string{rawDir, plotDir, profileDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fatalf("mkdir %s: %v", dir, err)
		}
	}

	benchOutputPath := filepath.Join(rawDir, "benchmarks.txt")
	benchOutput, records, err := runBenchmarks(*sizesFlag)
	if err != nil {
		fatalf("benchmarks failed: %v", err)
	}
	if err := os.WriteFile(benchOutputPath, []byte(benchOutput), 0o644); err != nil {
		fatalf("write %s: %v", benchOutputPath, err)
	}
	if err := writeBenchmarkCSV(filepath.Join(rawDir, "benchmarks.csv"), records); err != nil {
		fatalf("write benchmark csv: %v", err)
	}

	profileRecords := make([]profileRecord, 0, len(sizes)*3)
	for _, operation := range []string{"build", "add", "fullscan"} {
		for _, size := range sizes {
			record, err := runProfile(operation, size, *threshold, *numHashes, *bands, *shingleSize, profileDir)
			if err != nil {
				fatalf("profile %s size=%d failed: %v", operation, size, err)
			}
			profileRecords = append(profileRecords, record)
		}
	}
	sort.Slice(profileRecords, func(i, j int) bool {
		if profileRecords[i].Operation == profileRecords[j].Operation {
			return profileRecords[i].Size < profileRecords[j].Size
		}
		return profileRecords[i].Operation < profileRecords[j].Operation
	})
	if err := writeProfileCSV(filepath.Join(rawDir, "profiles.csv"), profileRecords); err != nil {
		fatalf("write profile csv: %v", err)
	}

	if err := renderPlots(rawDir, plotDir); err != nil {
		fatalf("render plots: %v", err)
	}
	fmt.Printf("metrics collected in %s\n", *outDir)
}

func runBenchmarks(sizesStr string) (string, []benchmarkRecord, error) {
	cmd := exec.Command("go", "test", "-run", "^$", "-bench", "^BenchmarkIndex", "-benchmem", ".")
	cmd.Env = append(os.Environ(), "SIZES="+sizesStr)
	data, err := cmd.CombinedOutput()
	text := string(data)
	if err != nil {
		return text, nil, fmt.Errorf("go test failed: %w\n%s", err, text)
	}
	records, err := parseBenchmarkOutput(text)
	if err != nil {
		return text, nil, err
	}
	return text, records, nil
}

func runProfile(operation string, size int, threshold float64, numHashes int, bands int, shingleSize int, outDir string) (profileRecord, error) {
	summaryPath := filepath.Join(outDir, fmt.Sprintf("%s_%d_summary.json", operation, size))
	cmd := exec.Command(
		"go", "run", "./cmd/profile",
		"-op", operation,
		"-size", strconv.Itoa(size),
		"-threshold", fmt.Sprintf("%.4f", threshold),
		"-num-hashes", strconv.Itoa(numHashes),
		"-bands", strconv.Itoa(bands),
		"-shingle-size", strconv.Itoa(shingleSize),
		"-out", outDir,
	)
	data, err := cmd.CombinedOutput()
	if err != nil {
		return profileRecord{}, fmt.Errorf("go run ./cmd/profile failed: %w\n%s", err, string(data))
	}
	jsonData, err := os.ReadFile(summaryPath)
	if err != nil {
		return profileRecord{}, err
	}
	var record profileRecord
	if err := json.Unmarshal(jsonData, &record); err != nil {
		return profileRecord{}, err
	}
	return record, nil
}

func renderPlots(rawDir string, plotDir string) error {
	cmd := exec.Command(
		"gnuplot",
		"-e", fmt.Sprintf("raw_dir='%s'; plot_dir='%s'", escapeForGnuplot(rawDir), escapeForGnuplot(plotDir)),
		"plot_metrics.gnuplot",
	)
	data, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gnuplot failed: %w\n%s", err, string(data))
	}
	return nil
}

func writeBenchmarkCSV(path string, records []benchmarkRecord) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"operation", "size", "ns_per_op", "ns_per_item", "ops_per_sec", "bytes_per_op", "allocs_per_op"}); err != nil {
		return err
	}
	for _, record := range records {
		row := []string{
			record.Operation,
			strconv.Itoa(record.Size),
			fmt.Sprintf("%.6f", record.NSPerOp),
			fmt.Sprintf("%.6f", record.NSPerItem),
			fmt.Sprintf("%.6f", record.OpsPerSec),
			fmt.Sprintf("%.6f", record.BytesPerOp),
			fmt.Sprintf("%.6f", record.AllocsPerOp),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return writer.Error()
}

func writeProfileCSV(path string, records []profileRecord) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"operation", "size", "elapsed_ns", "throughput_ops_s", "cpu_user_ns", "cpu_sys_ns", "total_alloc_diff", "alloc_count_total", "match_count", "document_count", "bucket_count", "band_count", "num_hashes", "shingle_size"}); err != nil {
		return err
	}
	for _, record := range records {
		row := []string{
			record.Operation,
			strconv.Itoa(record.Size),
			strconv.FormatInt(record.ElapsedNS, 10),
			fmt.Sprintf("%.6f", record.ThroughputOpsS),
			strconv.FormatInt(record.CPUUserNS, 10),
			strconv.FormatInt(record.CPUSysNS, 10),
			strconv.FormatUint(record.TotalAllocDiff, 10),
			strconv.FormatUint(record.AllocCountTotal, 10),
			strconv.Itoa(record.MatchCount),
			strconv.Itoa(record.Stats.DocumentCount),
			strconv.Itoa(record.Stats.BucketCount),
			strconv.Itoa(record.Stats.BandCount),
			strconv.Itoa(record.Stats.NumHashes),
			strconv.Itoa(record.Stats.ShingleSize),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return writer.Error()
}

func parseBenchmarkOutput(output string) ([]benchmarkRecord, error) {
	pattern := regexp.MustCompile(`^BenchmarkIndex(Build|Add|FullScan)/size=(\d+)-\d+$`)
	scanner := bufio.NewScanner(strings.NewReader(output))
	records := make([]benchmarkRecord, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "BenchmarkIndex") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			return nil, fmt.Errorf("unexpected benchmark line: %s", line)
		}
		matches := pattern.FindStringSubmatch(fields[0])
		if matches == nil {
			return nil, fmt.Errorf("cannot parse benchmark name: %s", fields[0])
		}
		size, err := strconv.Atoi(matches[2])
		if err != nil {
			return nil, err
		}
		metrics := make(map[string]float64)
		for index := 2; index+1 < len(fields); index += 2 {
			value, err := strconv.ParseFloat(fields[index], 64)
			if err != nil {
				continue
			}
			metrics[fields[index+1]] = value
		}
		records = append(records, benchmarkRecord{
			Operation:   strings.ToLower(matches[1]),
			Size:        size,
			NSPerOp:     metrics["ns/op"],
			NSPerItem:   metrics["ns/item"],
			OpsPerSec:   metrics["ops/s"],
			BytesPerOp:  metrics["B/op"],
			AllocsPerOp: metrics["allocs/op"],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("no benchmark records parsed")
	}
	return records, nil
}

func parseSizes(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	sizes := make([]int, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		size, err := strconv.Atoi(trimmed)
		if err != nil {
			return nil, err
		}
		if size <= 0 {
			return nil, fmt.Errorf("size must be positive: %d", size)
		}
		sizes = append(sizes, size)
	}
	if len(sizes) == 0 {
		return nil, errors.New("empty size list")
	}
	return sizes, nil
}

func escapeForGnuplot(value string) string {
	return strings.ReplaceAll(value, "'", "\\'")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
