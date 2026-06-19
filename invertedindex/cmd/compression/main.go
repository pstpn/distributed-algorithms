package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pstpn/iidx/internal/compression"
	"github.com/pstpn/iidx/internal/index"
	"github.com/pstpn/iidx/internal/storage"
)

const indexHeaderSize = 40

var defaultSizes = []int{10000, 25000, 50000, 100000, 200000}

type wikiDoc struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

func loadDocuments(docsDir string, maxCount int) ([]wikiDoc, error) {
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return nil, fmt.Errorf("read documents dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	docs := make([]wikiDoc, 0, maxCount)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		f, err := os.Open(filepath.Join(docsDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", entry.Name(), err)
		}

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

		for scanner.Scan() {
			var doc wikiDoc
			if err := json.Unmarshal(scanner.Bytes(), &doc); err != nil {
				continue
			}
			docs = append(docs, doc)
			if len(docs)%50000 == 0 {
				fmt.Printf("  loaded %d documents...\r", len(docs))
			}
			if len(docs) >= maxCount {
				f.Close()
				fmt.Printf("  loaded %d documents        \n", len(docs))
				return docs, nil
			}
		}
		f.Close()

		if len(docs) >= maxCount {
			fmt.Printf("  loaded %d documents        \n", len(docs))
			return docs, nil
		}
	}

	fmt.Printf("  loaded %d documents        \n", len(docs))
	return docs, nil
}

type compressionStats struct {
	numDocs              int
	numTerms             int
	postingsUncompressed int64
	postingsCompressed   int64
	metaUncompressed     int64
	metaCompressed       int64
	totalUncompressed    int64
	totalCompressed      int64
	ratio                float64
	fileSize             int64
}

func computeStats(indexFile string, numDocs int) (*compressionStats, error) {
	s, err := storage.OpenMMap(indexFile)
	if err != nil {
		return nil, fmt.Errorf("open mmap: %w", err)
	}
	defer s.Close()

	header, err := storage.ReadHeader(s)
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	termEntries, err := storage.ReadTermEntries(s, header)
	if err != nil {
		return nil, fmt.Errorf("read term entries: %w", err)
	}

	stats := &compressionStats{
		numDocs:  numDocs,
		numTerms: len(termEntries),
	}

	// Compute term strings size (shared between formats)
	var termStringsSize int64
	for _, entry := range termEntries {
		termStringsSize += int64(2 + len(entry.Term))
	}

	// Compute per-term posting sizes
	var totalPostingsUncompressed, totalPostingsCompressed int64

	for i, entry := range termEntries {
		compressedData := storage.ReadPostingsData(s, entry.PostingsOffset, entry.PostingsLength)
		compressedSize := int64(len(compressedData))

		// Decompress to get the flat uint32 array
		flat := compression.Decompress(compressedData)
		// Uncompressed: same flat array stored as raw uint32 (4 bytes each)
		uncompressedSize := int64(len(flat)) * 4

		totalPostingsUncompressed += uncompressedSize
		totalPostingsCompressed += compressedSize

		if (i+1)%50000 == 0 || i+1 == len(termEntries) {
			pct := float64(i+1) / float64(len(termEntries)) * 100
			fmt.Printf("  analyzed %d/%d terms (%.0f%%)\r", i+1, len(termEntries), pct)
		}
	}
	fmt.Println()

	stats.postingsUncompressed = totalPostingsUncompressed
	stats.postingsCompressed = totalPostingsCompressed

	// Compute metadata sizes
	// Layout: [header][term strings][docFreqs][offsetDeltas][postingsLengths][docLengths][posting data...]
	metaStartOffset := int64(indexHeaderSize) + termStringsSize
	compressedArraysEnd := header.PostingsOffset - int64(header.DocLengthsSize)
	compressedArraysSize := compressedArraysEnd - metaStartOffset
	buf := s.Slice(metaStartOffset, int(compressedArraysSize))

	off := 0
	var totalMetaUncompressed, totalMetaCompressed int64

	// docFreqs
	dfCompSize := compression.CompressedSize(buf[off:])
	dfValues := compression.Decompress(buf[off:])
	totalMetaCompressed += int64(dfCompSize)
	totalMetaUncompressed += int64(len(dfValues)) * 4
	off += dfCompSize

	// offsetDeltas
	odCompSize := compression.CompressedSize(buf[off:])
	odValues := compression.Decompress(buf[off:])
	totalMetaCompressed += int64(odCompSize)
	totalMetaUncompressed += int64(len(odValues)) * 4
	off += odCompSize

	// postingsLengths
	plCompSize := compression.CompressedSize(buf[off:])
	plValues := compression.Decompress(buf[off:])
	totalMetaCompressed += int64(plCompSize)
	totalMetaUncompressed += int64(len(plValues)) * 4
	off += plCompSize

	// docLengths (stored just before postings)
	dlOffset := header.PostingsOffset - int64(header.DocLengthsSize)
	dlBuf := s.Slice(dlOffset, int(header.DocLengthsSize))
	dlValues := compression.Decompress(dlBuf)
	totalMetaCompressed += int64(header.DocLengthsSize)
	totalMetaUncompressed += int64(len(dlValues)) * 4

	stats.metaUncompressed = totalMetaUncompressed
	stats.metaCompressed = totalMetaCompressed

	// Total sizes
	// Both formats share: header (40 bytes) + term strings
	sharedSize := int64(indexHeaderSize) + termStringsSize
	stats.totalUncompressed = sharedSize + totalMetaUncompressed + totalPostingsUncompressed
	stats.totalCompressed = sharedSize + totalMetaCompressed + totalPostingsCompressed

	if stats.totalCompressed > 0 {
		stats.ratio = float64(stats.totalUncompressed) / float64(stats.totalCompressed)
	}

	// Actual file size for verification
	fileInfo, _ := os.Stat(indexFile)
	if fileInfo != nil {
		stats.fileSize = fileInfo.Size()
	}

	return stats, nil
}

func parseSizes(s string) ([]int, error) {
	if s == "" {
		return defaultSizes, nil
	}
	parts := strings.Split(s, ",")
	sizes := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid size %q: %w", p, err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("size must be positive, got %d", n)
		}
		sizes = append(sizes, n)
	}
	if len(sizes) == 0 {
		return nil, fmt.Errorf("no valid sizes provided")
	}
	return sizes, nil
}

func formatBytes(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func main() {
	dataDir := flag.String("data", "data/documents", "directory with JSONL document files")
	outputDir := flag.String("output", "data/bench", "output directory for CSV")
	sizesFlag := flag.String("sizes", "", "comma-separated document counts (default: 10000,25000,50000,100000,200000)")
	flag.Parse()

	sizes, err := parseSizes(*sizesFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loading documents from %s...\n", *dataDir)
	allDocs, err := loadDocuments(*dataDir, 2_000_000)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading documents: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Loaded %d documents total\n\n", len(allDocs))

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating output dir: %v\n", err)
		os.Exit(1)
	}

	csvPath := filepath.Join(*outputDir, "compression.csv")
	csvFile, err := os.Create(csvPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating CSV: %v\n", err)
		os.Exit(1)
	}
	defer csvFile.Close()

	// CSV columns: num_docs, num_terms, postings_raw, postings_compressed,
	//              meta_raw, meta_compressed, total_raw, total_compressed, ratio
	fmt.Fprintf(csvFile, "num_docs,num_terms,postings_raw,postings_compressed,meta_raw,meta_compressed,total_raw,total_compressed,ratio\n")

	for _, size := range sizes {
		if len(allDocs) < size {
			fmt.Printf("Skipping size=%d: not enough documents (have %d)\n", size, len(allDocs))
			continue
		}

		fmt.Printf("=== %d documents ===\n", size)
		docs := allDocs[:size]

		// Build index
		fmt.Printf("Building index...\n")
		tmpDir, err := os.MkdirTemp("", "iidx-compression-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "temp dir: %v\n", err)
			os.Exit(1)
		}

		indexFile := filepath.Join(tmpDir, "compression.idx")
		builder := index.NewIndexBuilder()
		for i, doc := range docs {
			builder.AddDocument(doc.Title, doc.Text)
			if (i+1)%10000 == 0 || i+1 == size {
				pct := float64(i+1) / float64(size) * 100
				fmt.Printf("  indexed %d/%d docs (%.0f%%)\r", i+1, size, pct)
			}
		}
		fmt.Println()

		if err := builder.Save(indexFile); err != nil {
			fmt.Fprintf(os.Stderr, "save index: %v\n", err)
			os.RemoveAll(tmpDir)
			os.Exit(1)
		}

		// Compute compression stats
		fmt.Printf("Computing compression stats...\n")
		stats, err := computeStats(indexFile, size)
		if err != nil {
			fmt.Fprintf(os.Stderr, "compute stats: %v\n", err)
			os.RemoveAll(tmpDir)
			os.Exit(1)
		}

		fmt.Printf("  Terms:           %d\n", stats.numTerms)
		fmt.Printf("  Postings raw:    %s\n", formatBytes(stats.postingsUncompressed))
		fmt.Printf("  Postings cmp:    %s\n", formatBytes(stats.postingsCompressed))
		fmt.Printf("  Meta raw:        %s\n", formatBytes(stats.metaUncompressed))
		fmt.Printf("  Meta cmp:        %s\n", formatBytes(stats.metaCompressed))
		fmt.Printf("  Total raw:       %s\n", formatBytes(stats.totalUncompressed))
		fmt.Printf("  Total compressed:%s\n", formatBytes(stats.totalCompressed))
		fmt.Printf("  Ratio:           %.2fx\n", stats.ratio)
		fmt.Printf("  File size:       %s\n", formatBytes(stats.fileSize))

		fmt.Fprintf(csvFile, "%d,%d,%d,%d,%d,%d,%d,%d,%.4f\n",
			size, stats.numTerms,
			stats.postingsUncompressed, stats.postingsCompressed,
			stats.metaUncompressed, stats.metaCompressed,
			stats.totalUncompressed, stats.totalCompressed,
			stats.ratio)
		csvFile.Sync()

		// Cleanup temp directory
		os.RemoveAll(tmpDir)
		fmt.Println()
	}

	fmt.Printf("Results written to %s\n", csvPath)
}
