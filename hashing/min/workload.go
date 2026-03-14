package minhash

import "fmt"

type Workload struct {
	Base     []Document
	Incoming []Document
	Queries  []Document
}

var corpusWords = []string{
	"distributed", "system", "hashing", "database", "replica", "network", "storage", "index",
	"query", "shard", "partition", "fault", "tolerance", "consensus", "latency", "throughput",
	"consistency", "snapshot", "transaction", "bucket", "directory", "signature", "similarity", "document",
	"search", "duplicate", "window", "token", "feature", "cluster", "service", "message",
	"stream", "cache", "engine", "metric", "profile", "benchmark", "kernel", "memory",
}

func MakeWorkload(size int, seed int64) Workload {
	state := uint64(seed) + 0x9e3779b97f4a7c15
	base := make([]Document, size)
	incoming := make([]Document, size)
	queries := make([]Document, size)
	for i := 0; i < size; i++ {
		text := syntheticText(&state, i)
		base[i] = Document{ID: fmt.Sprintf("base-%d", i), Text: text}
		incoming[i] = Document{ID: fmt.Sprintf("incoming-%d", i), Text: mutateText(text, &state, i)}
		queries[i] = Document{ID: fmt.Sprintf("query-%d", i), Text: mutateText(text, &state, i+17)}
	}
	return Workload{Base: base, Incoming: incoming, Queries: queries}
}

func syntheticText(state *uint64, index int) string {
	words := make([]string, 0, 28)
	for i := 0; i < 28; i++ {
		value := nextRand(state)
		word := corpusWords[int(value%uint64(len(corpusWords)))]
		if i%7 == 0 {
			word = fmt.Sprintf("%s%d", word, (index+i)%11)
		}
		words = append(words, word)
	}
	return joinWords(words)
}

func mutateText(text string, state *uint64, salt int) string {
	words := tokenize(text)
	if len(words) == 0 {
		return text
	}
	changes := 1
	if len(words) > 20 && salt%3 == 0 {
		changes = 2
	}
	for i := 0; i < changes; i++ {
		pos := int(nextRand(state) % uint64(len(words)))
		words[pos] = corpusWords[int((nextRand(state)+uint64(salt)+uint64(i))%uint64(len(corpusWords)))]
	}
	return joinWords(words)
}

func joinWords(words []string) string {
	if len(words) == 0 {
		return ""
	}
	text := words[0]
	for i := 1; i < len(words); i++ {
		text += " " + words[i]
	}
	return text
}

func nextRand(state *uint64) uint64 {
	*state += 0x9e3779b97f4a7c15
	return mix64(*state)
}
