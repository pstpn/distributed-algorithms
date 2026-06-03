BEGIN { OFS=","; print "operation,size,value,ci95" }
/^Benchmark(Build|Warmup|Search)/ {
	name = $1

	split(name, parts, "/")
	bench_type = substr(parts[1], 10)

	split(parts[2], eq_parts, "=")
	split(eq_parts[2], dash_parts, "-")
	size = dash_parts[1]

	if (bench_type == "Build") {
		op = "build"
	} else if (bench_type == "Warmup") {
		op = "warmup"
	} else if (bench_type == "SearchRank") {
		if (length(parts) >= 3) {
			split(parts[3], sub_parts, "-")
			op = "searchrank_" sub_parts[1]
		} else {
			op = "searchrank"
		}
	} else if (bench_type == "Search") {
		if (length(parts) >= 3) {
			split(parts[3], sub_parts, "-")
			op = "search_" sub_parts[1]
		} else {
			op = "search"
		}
	}

	delete m
	for (i = 3; i + 1 <= NF; i += 2) {
		m[$(i+1)] = $i
	}

	if (op == "build") {
		value = ("ns/op" in m) ? m["ns/op"] : 0
		ci = ("ci95_ns/op" in m) ? m["ci95_ns/op"] : 0
		idxSize = ("bytes/index" in m) ? m["bytes/index"] : 0
		fname_size = raw_dir "/index_size.csv"
		print size "," idxSize > fname_size
	} else if (op == "warmup") {
		value = ("ns/op" in m) ? m["ns/op"] : 0
		ci = ("ci95_ns/warmup" in m) ? m["ci95_ns/warmup"] : 0
	} else {
		value = ("avg_ns/query" in m) ? m["avg_ns/query"] : 0
		ci = ("ci95_ns/query" in m) ? m["ci95_ns/query"] : 0
	}

	print op, size, value, ci
	fname = raw_dir "/" op ".csv"
	print size "," value "," ci > fname
}
