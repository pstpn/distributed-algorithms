SIZES ?= 1000,5000,10000,50000,100000,500000,1000000
METRICS_OUT ?= metrics
FUZZ_TIME ?= 15s
BENCH_FILTER ?= ^BenchmarkTable
RAW_DIR := $(METRICS_OUT)/raw
PLOT_DIR := $(METRICS_OUT)/plots

.PHONY: test test-short fuzz bench collect metrics plot clean

test:
	go test ./...

test-short:
	go test -short ./...

fuzz:
	go test -run '^$$' -fuzz '^FuzzTableAgainstMap$$' -fuzztime $(FUZZ_TIME) .

bench:
	SIZES='$(SIZES)' go test -run '^$$' -bench '$(BENCH_FILTER)' -benchmem .

collect:
	mkdir -p $(RAW_DIR) $(PLOT_DIR)
	SIZES='$(SIZES)' go test -run '^$$' -bench '$(BENCH_FILTER)' -benchmem . > $(RAW_DIR)/benchmarks.txt
	awk 'BEGIN { OFS=","; print "operation,size,iterations,ns_per_op,ns_per_item,ci95_ns_per_item,ops_per_sec,bytes_per_op,allocs_per_op" } \
	/^BenchmarkTable(Warmup|Insert|Update|Delete|Get)\/size=[0-9]+-[0-9]+/ { \
		name=$$1; iters=$$2; \
		split(name, a, "/"); split(a[1], b, "BenchmarkTable"); op=tolower(b[2]); \
		split(a[2], c, "="); split(c[2], d, "-"); size=d[1]; \
		delete m; \
		for (i=3; i+1<=NF; i+=2) m[$$(i+1)]=$$i; \
		ns_item=("ns/item" in m)?m["ns/item"]:m["ns/warmup"]; \
		ci=("ci95_ns/item" in m)?m["ci95_ns/item"]:m["ci95_ns/warmup"]; \
		ops=("ops/s" in m)?m["ops/s"]:0; \
		bop=("B/op" in m)?m["B/op"]:0; \
		alloc=("allocs/op" in m)?m["allocs/op"]:0; \
		nsop=("ns/op" in m)?m["ns/op"]:0; \
		print op,size,iters,nsop,ns_item,ci,ops,bop,alloc; \
	}' $(RAW_DIR)/benchmarks.txt > $(RAW_DIR)/benchmarks.csv

metrics: collect plot

plot:
	mkdir -p $(PLOT_DIR)
	gnuplot -e "raw_dir='$(RAW_DIR)'; plot_dir='$(PLOT_DIR)'" plot_metrics.gnuplot

clean:
	rm -rf metrics
