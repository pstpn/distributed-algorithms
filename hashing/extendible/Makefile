SIZES ?= 1000,5000,10000,50000,100000,500000,1000000,2500000,5000000,10000000
CAPACITY ?= 64
PROFILE_OP ?= insert
PROFILE_SIZE ?= 1000000
PROFILE_OUT ?= metrics/profiles
METRICS_OUT ?= metrics
FUZZ_TIME ?= 15s
BENCH_FILTER ?= ^BenchmarkTable

.PHONY: test test-short fuzz bench profile collect metrics plot clean

test:
	go test ./...

test-short:
	go test -short ./...

fuzz:
	go test -run '^$$' -fuzz '^FuzzTableAgainstMap$$' -fuzztime $(FUZZ_TIME) .

bench:
	SIZES='$(SIZES)' go test -run '^$$' -bench '$(BENCH_FILTER)' -benchmem .

profile:
	go run ./cmd/profile -op $(PROFILE_OP) -size $(PROFILE_SIZE) -capacity $(CAPACITY) -out $(PROFILE_OUT)

collect:
	go run ./cmd/collectmetrics -sizes $(SIZES) -capacity $(CAPACITY) -out $(METRICS_OUT)

metrics: collect

plot:
	gnuplot -e "raw_dir='$(METRICS_OUT)/raw'; plot_dir='$(METRICS_OUT)/plots'" plot_metrics.gnuplot

clean:
	rm -rf metrics
