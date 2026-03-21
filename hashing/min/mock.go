package minhash

import (
	"fmt"
	"math/rand"
	"strings"
)

var benchmarkWords = []string{
	"distributed", "system", "hash", "index", "query", "storage", "replica", "bucket",
	"network", "token", "document", "search", "stream", "latency", "throughput", "node",
	"cache", "partition", "transaction", "window", "signature", "cluster", "message", "shard",
}

func randomDocuments(size int) []Document {
	docs := make([]Document, size)
	for i := 0; i < size; i++ {
		docs[i] = Document{
			ID:   fmt.Sprintf("mock-%d", rand.Int63()),
			Text: randomText(24),
		}
	}
	return docs
}

func randomText(wordCount int) string {
	if wordCount <= 0 {
		return ""
	}
	var builder strings.Builder
	for i := 0; i < wordCount; i++ {
		if i > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(benchmarkWords[rand.Intn(len(benchmarkWords))])
	}
	return builder.String()
}
