set datafile separator ","
set terminal pdfcairo enhanced color size 8in,5in font "Helvetica,11"
set key outside
set grid
set border linewidth 1.2
set pointsize 1.2
set logscale y 10

bench_csv = raw_dir . "/benchmarks.csv"
profile_csv = raw_dir . "/profiles.csv"

set output plot_dir . "/benchmark_latency.pdf"
set title "latency per item"
set xlabel "Dataset size"
set ylabel "ns/item"
plot \
	bench_csv using (strcol(1) eq "build" ? $2 : 1/0):4 with linespoints linewidth 2 title "build", \
	bench_csv using (strcol(1) eq "add" ? $2 : 1/0):4 with linespoints linewidth 2 title "add", \
	bench_csv using (strcol(1) eq "fullscan" ? $2 : 1/0):4 with linespoints linewidth 2 title "fullscan"

set output plot_dir . "/benchmark_throughput.pdf"
set title "throughput"
set xlabel "Dataset size"
set ylabel "ops/s"
plot \
	bench_csv using (strcol(1) eq "build" ? $2 : 1/0):5 with linespoints linewidth 2 title "build", \
	bench_csv using (strcol(1) eq "add" ? $2 : 1/0):5 with linespoints linewidth 2 title "add", \
	bench_csv using (strcol(1) eq "fullscan" ? $2 : 1/0):5 with linespoints linewidth 2 title "fullscan"

set output plot_dir . "/profile_alloc_size_total.pdf"
set title "total allocated size"
set xlabel "Dataset size"
set ylabel "allocated bytes"
plot \
	profile_csv using (strcol(1) eq "build" ? $2 : 1/0):7 with linespoints linewidth 2 title "build", \
	profile_csv using (strcol(1) eq "add" ? $2 : 1/0):7 with linespoints linewidth 2 title "add", \
	profile_csv using (strcol(1) eq "fullscan" ? $2 : 1/0):7 with linespoints linewidth 2 title "fullscan"

set output plot_dir . "/profile_alloc_count_total.pdf"
set title "total allocations"
set xlabel "Dataset size"
set ylabel "allocations"
plot \
	profile_csv using (strcol(1) eq "build" ? $2 : 1/0):8 with linespoints linewidth 2 title "build", \
	profile_csv using (strcol(1) eq "add" ? $2 : 1/0):8 with linespoints linewidth 2 title "add", \
	profile_csv using (strcol(1) eq "fullscan" ? $2 : 1/0):8 with linespoints linewidth 2 title "fullscan"
