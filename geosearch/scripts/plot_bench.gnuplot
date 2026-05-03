set datafile separator ","
set key outside
set grid
set xlabel "rows"
set logscale y 10
set logscale x 2

if (!exists("input")) input = "data/bench_results.csv"
if (!exists("outdir")) outdir = "data"

set terminal pdfcairo size 11in,6in enhanced font "Helvetica,12"

set output outdir . "/bench_avg_latency.pdf"
set title "average time per operation"
set ylabel "ns/op"
plot \
	input using 2:($4*1e9):($5*1e9) with yerrorlines lw 2 pt 5 title "insert\\_avg\\_ns", \
	input using 2:($8*1e9):($9*1e9) with yerrorlines lw 2 pt 7 title "search\\_avg\\_ns"

set output outdir . "/bench_throughput.pdf"
set title "throughput"
set ylabel "ops/sec"
plot \
	input using 2:6:7 with yerrorlines lw 2 pt 5 title "insert\\_ops\\_sec", \
	input using 2:10:11 with yerrorlines lw 2 pt 7 title "search\\_ops\\_sec"