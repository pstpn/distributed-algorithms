DATA_RAW       := data/raw
DATA_DOCS      := data/documents

PROFILE_SIZE   ?= 10000
PROFILE_QUERY  ?= cat AND dog OR hello NEAR/5 world
PROFILE_ROUNDS ?= 1000

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

profile-build:
	mkdir -p data/profiles
	go run ./cmd/profile \
		-mode build \
		-size $(PROFILE_SIZE) \
		-cpuprofile data/profiles/build.pprof

profile-search:
	mkdir -p data/profiles
	go run ./cmd/profile \
		-mode search \
		-size $(PROFILE_SIZE) \
		-query '$(PROFILE_QUERY)' \
		-rounds $(PROFILE_ROUNDS) \
		-cpuprofile data/profiles/search.pprof

render-build:
	mkdir -p data/results
	./scripts/render_profiles.sh data/profiles/build.pprof data/results/build.pdf

render-search:
	mkdir -p data/results
	./scripts/render_profiles.sh data/profiles/search.pprof data/results/search.pdf

clean:
	rm -f data/profiles/*.pprof data/results/*.pdf data/results/*.dot

.PHONY: test tui parse profile-build profile-search render-build render-search clean build
