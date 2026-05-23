DATA_RAW       := data/raw
DATA_DOCS      := data/documents

build:
	go build ./...

test:
	go test ./internal/... -v

test-short:
	go test ./internal/... -short

tui:
	go run ./cmd/tui/ -data $(DATA_DOCS)

parse:
	go run ./scripts/parse_wiki/ -input $(DATA_RAW) -output $(DATA_DOCS)

.PHONY: test tui parse clean build