set datafile separator ","
set key outside
set grid
set xlabel "rows"
set logscale y 10
set logscale x 10

if (!exists("input")) input = "data/bench_results.csv"
if (!exists("outdir")) outdir = "data"

set terminal pdfcairo size 11in,6in enhanced font "Helvetica,12"

set output outdir . "/bench_avg_latency.pdf"
set title "average time per one operation"
set ylabel "ns/op"
plot input using 2:($5*1e9) with linespoints lw 2 pt 5 title "get\\_avg\\_ns"

set output outdir . "/bench_throughput.pdf"
set title "throughput per one element"
set ylabel "ops/sec"
plot input using 2:7 with linespoints lw 2 pt 5 title "get\\_ops\\_sec"